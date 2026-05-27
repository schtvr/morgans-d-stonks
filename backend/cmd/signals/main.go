package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	osignal "os/signal"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	agentmcp "github.com/schtvr/morgans-d-stonks/internal/mcp/agent"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

// Package-level agent state — set once at startup and read by the signal loop.
var (
	agentWorker        *agent.Worker
	agentPromptVersion string
	lastSnap           atomic.Pointer[portfolio.IngestSnapshotRequest]
	portfolioFloors    config.Trading
)

func main() {
	log := logging.New("signals")
	cfg := config.LoadSignals()
	brokerCfg := config.LoadBroker()
	portfolioFloors = config.LoadTrading()

	rules, err := sigpkg.LoadRulesFile(cfg.RulesPath)
	if err != nil {
		log.Error("signal rules load", "path", cfg.RulesPath, "err", err)
		os.Exit(1)
	}
	ruleByID := indexRules(rules)
	log.Info("signal_rules_loaded", "path", cfg.RulesPath, "count", len(rules))

	state, err := sigpkg.NewAlertState(cfg.StatePath)
	if err != nil {
		log.Error("state", "err", err)
		os.Exit(1)
	}
	ruleDedup, err := sigpkg.NewDedup(cfg.RulesDedupPath)
	if err != nil {
		log.Error("rule dedup state", "err", err)
		os.Exit(1)
	}

	cb := brokerCfg.ToLegacyBrokerConfig()
	coinbaseClient := coinbase.NewReadOnly(nil, "", cb.CoinbaseAPIKey, cb.CoinbaseAPISecret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ch := make(chan os.Signal, 1)
		osignal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	// Agent startup — initialise when AGENT_ENABLED=true.
	agentCfg := config.LoadAgent()
	if err := agentCfg.Validate(); err != nil {
		log.Error("invalid agent config", "err", err)
		os.Exit(1)
	}

	if agentCfg.Enabled {
		prompt, err := agent.LoadPrompt(agentCfg.PromptPath)
		if err != nil {
			log.Error("agent prompt load", "err", err)
			os.Exit(1)
		}
		agentPromptVersion = prompt.Version

		hcAgent := &http.Client{Timeout: 25 * time.Second}
		_, mcpCli, err := agentmcp.NewInProcessServer(
			coinbaseClient,
			agentCfg.PortfolioAPIURL,
			agentCfg.InternalAPIKey,
			hcAgent,
			0, // 0 = read INGEST_INTERVAL from env
		)
		if err != nil {
			log.Error("mcp server init", "err", err)
			os.Exit(1)
		}

		provider, err := newAgentProvider(agentCfg, mcpCli, prompt)
		if err != nil {
			log.Error("agent provider init", "err", err)
			os.Exit(1)
		}

		costRepo := &httpCostRepo{hc: hcAgent, baseURL: agentCfg.PortfolioAPIURL, apiKey: agentCfg.InternalAPIKey}
		costTracker := agent.NewCostTracker(costRepo, agentCfg.DailyCostCapCents)

		agentWorker = agent.NewWorker(agent.WorkerConfig{
			Provider:             provider,
			Concurrency:          agentCfg.Concurrency,
			CostTracker:          costTracker,
			PortfolioAPIURL:      agentCfg.PortfolioAPIURL,
			InternalAPIKey:       agentCfg.InternalAPIKey,
			Log:                  log,
			TradeEnabled:         agentCfg.TradeEnabled,
			MinTradeConfidence:   agentCfg.MinTradeConfidence,
			DefaultTradeNotional: agentCfg.DefaultTradeNotional,
		})
		agentWorker.Start(ctx)
		defer agentWorker.Stop()

		latestSnap := func() *portfolio.IngestSnapshotRequest { return lastSnap.Load() }
		go runDailyTimer(ctx, log, agentCfg.DailyTimerUTC, agentWorker.Enqueue, latestSnap, agentPromptVersion, time.Now)

		log.Info("agent_started",
			"provider", agentCfg.Provider,
			"model", agentCfg.Model,
			"prompt_version", agentPromptVersion,
			"concurrency", agentCfg.Concurrency,
			"daily_timer_utc", agentCfg.DailyTimerUTC,
			"trade_enabled", agentCfg.TradeEnabled,
			"min_trade_confidence", agentCfg.MinTradeConfidence,
			"default_trade_notional", agentCfg.DefaultTradeNotional,
		)
	}

	hc := &http.Client{Timeout: 30 * time.Second}
	t := time.NewTicker(cfg.Interval)
	defer t.Stop()

	runOpts := runOnceOpts{
		rules:                  rules,
		ruleByID:               ruleByID,
		ruleDedup:              ruleDedup,
		defaultRuleCooldown:    cfg.RuleCooldown,
		agentMaxPerSymbol24h:   cfg.AgentMaxPerSymbol24h,
	}

	run := func() {
		if err := runOnce(ctx, log, hc, coinbaseClient, state, cfg.PortfolioAPIURL, cfg.InternalAPIKey, cfg.ThresholdPct, cfg.Cooldown, runOpts); err != nil {
			log.Warn("tick", "err", err)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-t.C:
			run()
		}
	}
}

type runOnceOpts struct {
	rules                  []sigpkg.Rule
	ruleByID               map[string]sigpkg.Rule
	ruleDedup              *sigpkg.Dedup
	defaultRuleCooldown    time.Duration
	agentMaxPerSymbol24h   int
}

func runOnce(
	ctx context.Context,
	log *slog.Logger,
	hc *http.Client,
	coinbaseClient *coinbase.Client,
	state *sigpkg.AlertState,
	baseURL string,
	apiKey string,
	defaultThresholdPct float64,
	defaultCooldown time.Duration,
	opts runOnceOpts,
) error {
	start := time.Now()
	settings, err := fetchSignalSettings(ctx, hc, baseURL, apiKey)
	if err != nil && log != nil {
		log.Warn("signal settings", "err", err)
	}
	thresholdPct := defaultThresholdPct
	cooldown := defaultCooldown
	if settings != nil {
		if settings.MoveThresholdPct > 0 {
			thresholdPct = settings.MoveThresholdPct
		}
		if parsed, err := time.ParseDuration(settings.Cooldown); err == nil {
			cooldown = parsed
		}
	}
	followed, err := fetchFollowedSymbols(ctx, hc, baseURL, apiKey)
	if err != nil {
		return err
	}
	snap, err := fetchSnapshot(ctx, hc, baseURL, apiKey)
	if err != nil {
		return err
	}
	lastSnap.Store(snap)

	positions := make(map[string]broker.Position, len(snap.Positions))
	for _, p := range snap.Positions {
		positions[normalizeSymbol(p.Symbol)] = p
	}

	evaluated := 0
	moveFired := 0
	moveAgent := 0
	moveAgentSkippedNotHeld := 0
	moveAgentSkippedCap := 0
	skipped := 0
	var portfolioStats portfolioRulesStats

	defer func() {
		log.Info("signals_tick",
			"duration_ms", time.Since(start).Milliseconds(),
			"followed_count", len(followed),
			"move_alerts_evaluated", evaluated,
			"move_alerts_fired", moveFired,
			"move_agent_enqueued", moveAgent,
			"move_agent_skipped_not_held", moveAgentSkippedNotHeld,
			"move_agent_skipped_cap", moveAgentSkippedCap,
			"portfolio_rules_fired", portfolioStats.Fired,
			"portfolio_agent_enqueued", portfolioStats.AgentEnqueued,
			"portfolio_rules_skipped_dedup", portfolioStats.SkippedDedup,
			"portfolio_agent_skipped_cap", portfolioStats.SkippedAgentCap,
			"alerts_skipped", skipped,
		)
	}()

	now := time.Now().UTC()

	portfolioStats, err = processPortfolioRules(
		ctx, log, hc, opts.rules, opts.ruleByID, snap, opts.ruleDedup,
		opts.defaultRuleCooldown, opts.agentMaxPerSymbol24h,
		baseURL, apiKey, agentPromptVersion, now,
	)
	if err != nil {
		return fmt.Errorf("portfolio rules: %w", err)
	}

	for _, item := range followed {
		symbol := coinbase.CanonicalToProviderSymbol(item.Symbol)
		if symbol == "" {
			skipped++
			continue
		}
		q, err := quoteForSymbol(ctx, coinbaseClient, symbol)
		if err != nil {
			log.Warn("quote", "symbol", symbol, "err", err)
			skipped++
			continue
		}
		if q.Last <= 0 {
			skipped++
			continue
		}
		evaluated++
		decision, err := state.Evaluate(symbol, q.Last, thresholdPct, cooldown, now)
		if err != nil {
			return err
		}
		if !decision.Alert {
			continue
		}
		gate := sigpkg.EvaluateGate(sigpkg.GateInput{
			ProductID:          symbol,
			Return5mPct:        decision.DeltaPct,
			RollingVol5mPct:    0,
			SpreadBps:          0,
			SpreadCapBps:       0,
			QuoteVolume24h:     0,
			MinQuoteVolume24h:  0,
			CooldownActive:     false,
			Restricted:         false,
			PersistsTwoOfThree: true,
			FloorPct:           thresholdPct,
		})
		if !gate.Emit {
			skipped++
			continue
		}
		moveFired++
		alert := buildCryptoAlert(item, symbol, q.Last, decision, positions[normalizeSymbol(symbol)], now, thresholdPct, gate.ReasonFlags)

		if err := persistRecentAlert(ctx, hc, baseURL, apiKey, alert); err != nil && log != nil {
			log.Warn("recent alert persist", "symbol", alert.Symbol, "err", err)
		}

		log.Info("price_move_fired",
			"type", alert.Type,
			"symbol", alert.Symbol,
			"delta_pct", alert.DeltaPct,
			"threshold_pct", alert.ThresholdPct,
		)

		pos := positions[normalizeSymbol(symbol)]
		if !hasOpenPosition(pos) {
			moveAgentSkippedNotHeld++
			continue
		}
		if ok, reason := tryEnqueueAgent(ctx, log, hc, snap, alert, agentPromptVersion, opts.agentMaxPerSymbol24h, baseURL, apiKey, now); ok {
			moveAgent++
		} else if reason == "symbol_decision_cap" {
			moveAgentSkippedCap++
		}
	}
	return nil
}

func persistRecentAlert(ctx context.Context, hc *http.Client, baseURL, apiKey string, alert sigpkg.CryptoAlert) error {
	body, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/internal/recent-alerts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 0 {
			return fmt.Errorf("portfolio: recent alert persist failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
		}
		return fmt.Errorf("portfolio: recent alert persist failed: status=%d", resp.StatusCode)
	}
	return nil
}

func buildCryptoAlert(item portfolio.FollowedSymbol, symbol string, currentPrice float64, decision sigpkg.AlertDecision, pos broker.Position, firedAt time.Time, thresholdPct float64, reasonFlags []string) sigpkg.CryptoAlert {
	alert := sigpkg.CryptoAlert{
		SchemaVersion: sigpkg.CryptoSignalSchemaVersion,
		ID:            stableCryptoAlertID(symbol, firedAt),
		Type:          "crypto_price_move",
		ReasonFlags:   reasonFlags,
		Symbol:        symbol,
		ProductID:     symbol,
		Source:        item.Source,
		CurrentPrice:  currentPrice,
		DeltaPct:      decision.DeltaPct,
		ThresholdPct:  thresholdPct,
		FiredAt:       firedAt,
	}
	if decision.PreviousPrice > 0 {
		prev := decision.PreviousPrice
		alert.PreviousPrice = &prev
	}
	if decision.DeltaAmount != 0 {
		delta := decision.DeltaAmount
		alert.DeltaAmount = &delta
	}
	if pos.Quantity != 0 {
		qty := pos.Quantity
		alert.Quantity = &qty
	}
	if pos.AvgCost != 0 {
		avgCost := pos.AvgCost
		alert.AvgCost = &avgCost
		costBasis := pos.AvgCost * pos.Quantity
		alert.CostBasis = &costBasis
		if costBasis != 0 {
			pl := pos.UnrealizedPL
			alert.UnrealizedPL = &pl
			plPct := (pl / costBasis) * 100
			alert.UnrealizedPLPct = &plPct
		}
	}
	return alert
}

func stableCryptoAlertID(symbol string, firedAt time.Time) string {
	sym := strings.ToLower(strings.ReplaceAll(symbol, "-", "_"))
	return fmt.Sprintf("%s-%s", sym, firedAt.UTC().Format("20060102T150405Z"))
}

func quoteForSymbol(ctx context.Context, c *coinbase.Client, symbol string) (*broker.Quote, error) {
	quotes, err := c.Quotes(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("coinbase quote: no quote for %s", symbol)
	}
	return &quotes[0], nil
}

func fetchFollowedSymbols(ctx context.Context, hc *http.Client, baseURL, apiKey string) ([]portfolio.FollowedSymbol, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/followed-symbols", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", apiKey)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("portfolio: followed symbols fetch failed: status=%d", resp.StatusCode)
	}
	var payload portfolio.FollowedSymbolsResponse
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	sort.SliceStable(payload.Symbols, func(i, j int) bool {
		return strings.ToUpper(payload.Symbols[i].Symbol) < strings.ToUpper(payload.Symbols[j].Symbol)
	})
	return payload.Symbols, nil
}

func fetchSignalSettings(ctx context.Context, hc *http.Client, baseURL, apiKey string) (*portfolio.SignalSettings, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/signal-settings", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", apiKey)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("portfolio: signal settings fetch failed: status=%d", resp.StatusCode)
	}
	var settings portfolio.SignalSettings
	if err := json.Unmarshal(b, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func fetchSnapshot(ctx context.Context, hc *http.Client, baseURL, apiKey string) (*portfolio.IngestSnapshotRequest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/snapshot/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", apiKey)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return &portfolio.IngestSnapshotRequest{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("portfolio: snapshot fetch failed: status=%d", resp.StatusCode)
	}
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}
