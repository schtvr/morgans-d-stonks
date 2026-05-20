package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	agentmcp "github.com/schtvr/morgans-d-stonks/internal/mcp/agent"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

// buildEagerContext constructs the EagerContext embedded in every DecisionRequest.
// symbol="" means a daily trigger — DecisionsForSymbol24h will be nil.
// decisionsCount is set only when symbol != ""; pass nil for daily.
func buildEagerContext(snap *portfolio.IngestSnapshotRequest, symbol string, decisionsCount *int) agent.EagerContext {
	ec := agent.EagerContext{}
	if snap != nil {
		ec.PortfolioSummary = agent.PortfolioSummaryLine{
			NetLiquidation: snap.Summary.NetLiquidation,
			TotalCash:      snap.Summary.TotalCash,
		}
		// Clone slice so we don't mutate the snapshot, then sort desc by MarketValue.
		positions := append(snap.Positions[:0:0], snap.Positions...)
		sort.Slice(positions, func(i, j int) bool {
			return positions[i].MarketValue > positions[j].MarketValue
		})
		if len(positions) > 3 {
			positions = positions[:3]
		}
		top := make([]agent.PositionLine, 0, len(positions))
		for _, p := range positions {
			top = append(top, agent.PositionLine{
				Symbol:      p.Symbol,
				MarketValue: p.MarketValue,
				Quantity:    p.Quantity,
			})
		}
		ec.PortfolioSummary.TopPositions = top
	}
	if symbol != "" && symbol != sigpkg.PortfolioRuleSymbol {
		ec.DecisionsForSymbol24h = decisionsCount
	}
	return ec
}

type decisionCountResponse struct {
	Count int `json:"count"`
}

// fetchDecisionsCount calls GET /internal/agent-decisions/count to count recent
// decisions for a symbol in the last 24 hours. Returns 0 on any error (non-fatal).
func fetchDecisionsCount(ctx context.Context, hc *http.Client, baseURL, apiKey, symbol string) int {
	since := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	u, err := url.Parse(baseURL + "/internal/agent-decisions/count")
	if err != nil {
		return 0
	}
	q := u.Query()
	q.Set("symbol", symbol)
	q.Set("since", since)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0
	}
	req.Header.Set("X-Internal-Key", apiKey)

	resp, err := hc.Do(req)
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}
	var res decisionCountResponse
	if err := json.Unmarshal(b, &res); err != nil {
		return 0
	}
	return res.Count
}

// newAgentProvider constructs the Provider from configuration.
// Actual signature of NewAnthropicProvider: (apiKey, model, mcpClient, prompt).
func newAgentProvider(cfg config.Agent, mcpCli agentmcp.MCPClient, prompt *agent.Prompt) (agent.Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "mock":
		return agent.NewMockProvider(), nil
	case "anthropic":
		return agent.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.Model, mcpCli, prompt), nil
	default:
		return nil, fmt.Errorf("unknown agent provider %q; must be \"mock\" or \"anthropic\"", cfg.Provider)
	}
}

// runDailyTimer fires at timerUTC (HH:MM UTC) once per calendar day, enqueuing a
// TriggerDaily DecisionRequest. nowFn is injectable for testing; callers pass time.Now.
func runDailyTimer(
	ctx context.Context,
	log *slog.Logger,
	timerUTC string,
	enqueueFn func(agent.DecisionRequest),
	snapFn func() *portfolio.IngestSnapshotRequest,
	promptVersion string,
	nowFn func() time.Time,
) {
	hour, min, err := parseDailyTimer(timerUTC)
	if err != nil {
		log.Error("daily timer: invalid AGENT_DAILY_TIMER_UTC", "value", timerUTC, "err", err)
		return
	}

	for {
		now := nowFn().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		delay := next.Sub(now)
		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			fireTime := nowFn().UTC()
			key := "daily-" + fireTime.Format("2006-01-02")
			snap := snapFn()
			req := agent.DecisionRequest{
				TriggerKind:    agent.TriggerDaily,
				TriggerAt:      fireTime,
				IdempotencyKey: key,
				Signal:         nil,
				EagerContext:   buildEagerContext(snap, "", nil),
				PromptVersion:  promptVersion,
			}
			log.Info("daily_agent_trigger", "key", key)
			enqueueFn(req)
		}
	}
}

// parseDailyTimer parses an "HH:MM" string and returns hour and minute.
func parseDailyTimer(s string) (hour, min int, err error) {
	_, err = fmt.Sscanf(s, "%d:%d", &hour, &min)
	if err != nil {
		return 0, 0, fmt.Errorf("expected HH:MM, got %q: %w", s, err)
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("time %q out of range", s)
	}
	return hour, min, nil
}
