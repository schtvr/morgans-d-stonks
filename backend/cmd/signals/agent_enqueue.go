package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func hasOpenPosition(p broker.Position) bool {
	return p.Quantity > 0 && p.MarketValue > 0
}

func agentCapApplies(symbol string) bool {
	return symbol != "" && symbol != sigpkg.PortfolioRuleSymbol
}

// tryEnqueueAgent enqueues a shadow decision when the worker is up and per-symbol caps allow.
func tryEnqueueAgent(
	ctx context.Context,
	log *slog.Logger,
	hc *http.Client,
	snap *portfolio.IngestSnapshotRequest,
	alert sigpkg.CryptoAlert,
	promptVersion string,
	maxPerSymbol24h int,
	baseURL, apiKey string,
	now time.Time,
) (enqueued bool, skipReason string) {
	if agentWorker == nil {
		return false, "agent_disabled"
	}
	var countPtr *int
	if agentCapApplies(alert.Symbol) {
		count := fetchDecisionsCount(ctx, hc, baseURL, apiKey, alert.Symbol)
		countPtr = &count
		if maxPerSymbol24h > 0 && count >= maxPerSymbol24h {
			if log != nil {
				log.Info("agent_skip",
					"reason", "symbol_decision_cap",
					"symbol", alert.Symbol,
					"count_24h", count,
					"max", maxPerSymbol24h,
				)
			}
			return false, "symbol_decision_cap"
		}
	}
	agentWorker.Enqueue(agent.DecisionRequest{
		TriggerKind:    agent.TriggerSignal,
		TriggerAt:      now,
		IdempotencyKey: alert.ID,
		Signal:         &alert,
		EagerContext:   buildEagerContext(snap, alert.Symbol, countPtr),
		PromptVersion:  promptVersion,
	})
	if log != nil {
		log.Info("agent_enqueue",
			"symbol", alert.Symbol,
			"idempotency_key", alert.ID,
			"trigger_kind", agent.TriggerSignal,
			"reason_flags", alert.ReasonFlags,
		)
	}
	return true, ""
}
