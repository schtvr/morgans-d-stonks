package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// errDeferred signals that required data is not yet available.
// The scorer retries on the next tick — callers must not log this as a failure.
var errDeferred = errors.New("scorer: deferred — missing data")

func ptrF(f float64) *float64 { return &f }

// runScorer polls every 60s for (decision, horizon) pairs whose deadline has
// passed and persists the computed outcome.
func (a *app) runScorer(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.scoreBatch(ctx); err != nil {
				a.log.Warn("scorer", "err", err)
			}
		}
	}
}

// scoreBatch processes up to 50 pending (decision, horizon) pairs.
func (a *app) scoreBatch(ctx context.Context) error {
	pending, err := a.repo.ListUnscoredDecisionHorizons(ctx, time.Now().UTC(), 50)
	if err != nil {
		return err
	}
	for _, h := range pending {
		outcome, err := a.computeOutcome(ctx, h)
		if err != nil {
			if errors.Is(err, errDeferred) {
				continue // data not ready yet; silent retry
			}
			a.log.Warn("score_outcome", "decision_id", h.DecisionID, "horizon", h.Horizon, "err", err)
			continue
		}
		if _, err := a.repo.InsertAgentDecisionOutcome(ctx, *outcome); err != nil {
			a.log.Warn("insert_outcome", "err", err)
		}
	}
	return nil
}

// computeOutcome computes the scored return for one (decision, horizon) pair.
func (a *app) computeOutcome(ctx context.Context, h portfolio.UnscoredHorizon) (*portfolio.AgentDecisionOutcome, error) {
	// Step 1: resolve horizon duration.
	horizonDur, err := parseHorizonDuration(h.Horizon)
	if err != nil {
		return nil, fmt.Errorf("computeOutcome: %w", err)
	}

	since := h.TriggerAt
	until := h.TriggerAt.Add(horizonDur)

	// Step 2: BTC return (benchmark).
	if a.cb == nil {
		return nil, fmt.Errorf("scorer: no Coinbase client configured")
	}
	btcBars, err := a.cb.FetchProductCandles(ctx, "BTC-USD", since, until)
	if err != nil {
		return nil, fmt.Errorf("scorer: fetch BTC candles: %w", err)
	}
	if len(btcBars) == 0 {
		return nil, errDeferred
	}
	btcStart := btcBars[0].Close
	btcEnd := btcBars[len(btcBars)-1].Close
	btcReturnPct := (btcEnd - btcStart) / btcStart * 100
	// Fee deduction applies ONLY at horizon=14d; short-horizon raw return avoids artificial drag.
	if h.Horizon == "14d" {
		btcReturnPct -= 0.6
	}

	// Step 3: symbol / NAV return.
	isDaily := h.Symbol == "" || h.Symbol == "_portfolio"

	var (
		symbolReturnPct   *float64
		priceAtDecision   *float64
		priceAtHorizon    *float64
		realizedReturnPct *float64
		excessReturnPct   *float64
	)

	if isDaily {
		navStart, navEnd, err := a.computeNAV(ctx, h.TriggerAt, horizonDur)
		if err != nil {
			return nil, err // may be errDeferred
		}
		navReturn := (navEnd - navStart) / navStart * 100
		symbolReturnPct = ptrF(navReturn)
		// excessReturnPct only for 7d/14d on daily triggers; 1h/24h are excluded.
		if h.Horizon == "7d" || h.Horizon == "14d" {
			excessReturnPct = ptrF(navReturn - btcReturnPct)
		}
		// realizedReturnPct = nil — daily triggers excluded from headline metric.
	} else {
		symBars, err := a.cb.FetchProductCandles(ctx, h.Symbol, since, until)
		if err != nil {
			return nil, fmt.Errorf("scorer: fetch %s candles: %w", h.Symbol, err)
		}
		if len(symBars) == 0 {
			return nil, errDeferred
		}
		symStart := symBars[0].Close
		symEnd := symBars[len(symBars)-1].Close
		symReturn := (symEnd - symStart) / symStart * 100
		symbolReturnPct = ptrF(symReturn)
		priceAtDecision = ptrF(symStart)
		priceAtHorizon = ptrF(symEnd)

		switch h.Action {
		case "buy":
			// Long: price up is profitable.
			realizedReturnPct = ptrF(symReturn)
			excessReturnPct = ptrF(symReturn - btcReturnPct)
		case "sell":
			// Short/exit: price down is profitable.
			neg := -symReturn
			realizedReturnPct = ptrF(neg)
			excessReturnPct = ptrF(neg - btcReturnPct)
		case "ignore":
			// Shadow mode cannot claim P&L for inaction.
			// symbolReturnPct is still populated for opportunity-cost display.
		}
	}

	return &portfolio.AgentDecisionOutcome{
		DecisionID:        h.DecisionID,
		Horizon:           h.Horizon,
		PriceAtDecision:   priceAtDecision,
		PriceAtHorizon:    priceAtHorizon,
		SymbolReturnPct:   symbolReturnPct,
		BTCReturnPct:      btcReturnPct,
		RealizedReturnPct: realizedReturnPct,
		ExcessReturnPct:   excessReturnPct,
		FeesModeledPct:    0,
	}, nil
}

// computeNAV finds the nearest ingest snapshots bracketing [triggerAt, triggerAt+H]
// and returns the portfolio NAV for each. Returns errDeferred if either is missing.
func (a *app) computeNAV(ctx context.Context, triggerAt time.Time, horizonDur time.Duration) (navStart, navEnd float64, err error) {
	// Find snapshot at or before triggerAt: list from triggerAt-7d with limit 10, take last ≤ triggerAt.
	lookback := 7 * 24 * time.Hour
	before, err := a.repo.ListSnapshotsSince(ctx, triggerAt.Add(-lookback), 10)
	if err != nil {
		return 0, 0, fmt.Errorf("scorer: list snapshots before: %w", err)
	}
	var startSnap *portfolio.SnapshotRecord
	for i := len(before) - 1; i >= 0; i-- {
		if !before[i].TakenAt.After(triggerAt) {
			startSnap = &before[i]
			break
		}
	}
	if startSnap == nil {
		return 0, 0, errDeferred
	}

	// Find snapshot at or after triggerAt+H: list from afterTime with limit 1.
	afterTime := triggerAt.Add(horizonDur)
	after, err := a.repo.ListSnapshotsSince(ctx, afterTime, 1)
	if err != nil {
		return 0, 0, fmt.Errorf("scorer: list snapshots after: %w", err)
	}
	if len(after) == 0 {
		return 0, 0, errDeferred
	}
	endSnap := &after[0]

	navStart, err = snapshotNAV(startSnap)
	if err != nil {
		return 0, 0, fmt.Errorf("scorer: nav start: %w", err)
	}
	navEnd, err = snapshotNAV(endSnap)
	if err != nil {
		return 0, 0, fmt.Errorf("scorer: nav end: %w", err)
	}
	return navStart, navEnd, nil
}

// snapshotNAV computes NAV = sum(position.MarketValue) + summary.TotalCash.
func snapshotNAV(rec *portfolio.SnapshotRecord) (float64, error) {
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(rec.Data, &snap); err != nil {
		return 0, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	nav := snap.Summary.TotalCash
	for _, p := range snap.Positions {
		nav += p.MarketValue
	}
	return nav, nil
}

// parseHorizonDuration maps horizon token to duration.
func parseHorizonDuration(horizon string) (time.Duration, error) {
	switch horizon {
	case "1h":
		return time.Hour, nil
	case "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	case "14d":
		return 14 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown horizon: %s", horizon)
	}
}
