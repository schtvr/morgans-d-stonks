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
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/discord"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func main() {
	log := logging.New("signals")
	cfg := config.LoadSignals()

	state, err := sigpkg.NewAlertState(cfg.StatePath)
	if err != nil {
		log.Error("state", "err", err)
		os.Exit(1)
	}
	dc := discord.NewClient(cfg.DiscordWebhookURL)
	coinbaseClient := coinbase.NewReadOnly(nil, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ch := make(chan os.Signal, 1)
		osignal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	hc := &http.Client{Timeout: 30 * time.Second}
	discordEnabled := cfg.DiscordWebhookURL != ""
	t := time.NewTicker(cfg.Interval)
	defer t.Stop()

	run := func() {
		if err := runOnce(ctx, log, hc, coinbaseClient, state, cfg.PortfolioAPIURL, cfg.InternalAPIKey, cfg.ThresholdPct, cfg.Cooldown, dc, discordEnabled); err != nil {
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
	dc *discord.Client,
	discordEnabled bool,
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
	positions := make(map[string]broker.Position, len(snap.Positions))
	for _, p := range snap.Positions {
		positions[strings.ToUpper(strings.TrimSpace(p.Symbol))] = p
	}

	evaluated := 0
	fired := 0
	sent := 0
	discordErrors := 0
	skipped := 0

	defer func() {
		log.Info("signals_tick",
			"duration_ms", time.Since(start).Milliseconds(),
			"followed_count", len(followed),
			"alerts_evaluated", evaluated,
			"alerts_fired", fired,
			"alerts_sent", sent,
			"alerts_skipped", skipped,
			"discord_enabled", discordEnabled,
			"discord_errors", discordErrors,
		)
	}()

	now := time.Now().UTC()
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
		fired++
		alert := buildCryptoAlert(item, symbol, q.Last, decision, positions[symbol], now, thresholdPct)
		payload, err := discord.CryptoAlertWebhookContent(alert)
		if err != nil {
			return err
		}
		if err := persistRecentAlert(ctx, hc, baseURL, apiKey, alert); err != nil && log != nil {
			log.Warn("recent alert persist", "symbol", alert.Symbol, "err", err)
		}
		if !discordEnabled {
			log.Info("crypto_alert", "payload", payload)
			sent++
			continue
		}
		if err := dc.SendMessage(ctx, payload); err != nil {
			discordErrors++
			log.Warn("discord", "err", err)
			continue
		}
		sent++
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

func buildCryptoAlert(item portfolio.FollowedSymbol, symbol string, currentPrice float64, decision sigpkg.AlertDecision, pos broker.Position, firedAt time.Time, thresholdPct float64) sigpkg.CryptoAlert {
	alert := sigpkg.CryptoAlert{
		Type:         "crypto_alert",
		Symbol:       symbol,
		ProductID:    symbol,
		Source:       item.Source,
		CurrentPrice: currentPrice,
		DeltaPct:     decision.DeltaPct,
		ThresholdPct: thresholdPct,
		FiredAt:      firedAt,
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
