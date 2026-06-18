package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// ── scorer fake repo ──────────────────────────────────────────────────────────

// scorerTestRepo embeds agentTestRepo for all the interface stubs and overrides
// the three scorer-relevant methods with configurable behaviour.
type scorerTestRepo struct {
	agentTestRepo

	horizons    []portfolio.UnscoredHorizon
	horizonsErr error

	// outcomes tracks what the scorer actually inserted.
	outcomes []portfolio.AgentDecisionOutcome
	// insertErrAfterN: return an error once len(outcomes) >= insertErrAfterN (0 = never error).
	insertErrAfterN int

	// snapshotSeries is ascending by TakenAt; returned by ListSnapshotsSince.
	snapshotSeries []portfolio.SnapshotRecord
}

func (f *scorerTestRepo) ListUnscoredDecisionHorizons(_ context.Context, _ time.Time, _ int) ([]portfolio.UnscoredHorizon, error) {
	if f.horizonsErr != nil {
		return nil, f.horizonsErr
	}
	return f.horizons, nil
}

func (f *scorerTestRepo) InsertAgentDecisionOutcome(_ context.Context, o portfolio.AgentDecisionOutcome) (*portfolio.AgentDecisionOutcome, error) {
	if f.insertErrAfterN > 0 && len(f.outcomes) >= f.insertErrAfterN {
		return nil, errors.New("unique constraint: (decision_id, horizon) already exists")
	}
	o.ID = int64(len(f.outcomes) + 1)
	o.ScoredAt = time.Now().UTC()
	f.outcomes = append(f.outcomes, o)
	return &f.outcomes[len(f.outcomes)-1], nil
}

func (f *scorerTestRepo) ListSnapshotsSince(_ context.Context, since time.Time, limit int) ([]portfolio.SnapshotRecord, error) {
	out := make([]portfolio.SnapshotRecord, 0, len(f.snapshotSeries))
	for _, rec := range f.snapshotSeries {
		if rec.TakenAt.Before(since) {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ── candle server helpers ─────────────────────────────────────────────────────

// candleServer creates a fake Coinbase HTTP server that serves two-bar candle
// responses keyed by product symbol.
//   - symbolBars: map[symbol][first.Close, last.Close]
//   - Any unrecognised symbol gets a 400 response.
func candleServer(t *testing.T, symbolBars map[string][2]float64) *coinbase.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for sym, bars := range symbolBars {
			if strings.Contains(r.URL.Path, sym) {
				w.Header().Set("Content-Type", "application/json")
				// Timestamps fall within any tested horizon window rooted at triggerAt (1700000000).
				fmt.Fprintf(w, `{"candles":[
					{"start":"1700000000","low":"0","high":"9999999","open":"%.4f","close":"%.4f","volume":"100"},
					{"start":"1700003600","low":"0","high":"9999999","open":"%.4f","close":"%.4f","volume":"100"}
				]}`, bars[0], bars[0], bars[0], bars[1])
				return
			}
		}
		http.Error(w, "unknown symbol", http.StatusBadRequest)
	}))
	t.Cleanup(ts.Close)
	return coinbase.NewReadOnly(ts.Client(), ts.URL, "", "")
}

// failCandleServer returns a client whose every candle request fails with 400.
func failCandleServer(t *testing.T) *coinbase.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forced failure", http.StatusBadRequest)
	}))
	t.Cleanup(ts.Close)
	return coinbase.NewReadOnly(ts.Client(), ts.URL, "", "")
}

// ── snapshot helper ───────────────────────────────────────────────────────────

// makeSnapshot builds a SnapshotRecord with the given total position MV and cash.
func makeSnapshot(takenAt time.Time, positionsMV, cash float64) portfolio.SnapshotRecord {
	snap := portfolio.IngestSnapshotRequest{
		TakenAt: takenAt,
		Positions: []broker.Position{
			{Symbol: "BTC-USD", MarketValue: positionsMV},
		},
		Summary: broker.AccountSummary{TotalCash: cash},
	}
	b, _ := json.Marshal(snap)
	return portfolio.SnapshotRecord{TakenAt: takenAt, Data: b}
}

// ── test scaffolding ──────────────────────────────────────────────────────────

// triggerAt is the fixed reference time used in all scorer tests.
// Candle timestamps (1700000000 and 1700003600) fall within every horizon window.
var scorerTriggerAt = time.Unix(1700000000, 0).UTC()

func newScorerApp(cb *coinbase.Client, repo portfolio.Repository) *app {
	return &app{
		cb:   cb,
		repo: repo,
		log:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// approxF asserts |got - want| <= eps.
func approxF(t *testing.T, label string, got, want, eps float64) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Fatalf("%s: got %.6f, want %.6f (eps %.6f)", label, got, want, eps)
	}
}

func assertNilPtrF(t *testing.T, label string, p *float64) {
	t.Helper()
	if p != nil {
		t.Fatalf("%s: expected nil, got %v", label, *p)
	}
}

func assertPtrF(t *testing.T, label string, p *float64, want, eps float64) {
	t.Helper()
	if p == nil {
		t.Fatalf("%s: expected non-nil pointer", label)
	}
	approxF(t, label, *p, want, eps)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// BTC: 40000 → 42000 = 5.000%
// Symbol ETH: 100 → 110 = 10.000%
const (
	btcClose0 = 40000.0
	btcClose1 = 42000.0
	symClose0 = 100.0
	symClose1 = 110.0

	btcRaw    = (btcClose1 - btcClose0) / btcClose0 * 100 // 5.0
	symRaw    = (symClose1 - symClose0) / symClose0 * 100 // 10.0
	btcFee14d = 0.6
	btcAdj14d = btcRaw - btcFee14d // 4.4
)

func TestComputeOutcome_Buy_14d(t *testing.T) {
	t.Parallel()
	repo := &scorerTestRepo{
		horizons: []portfolio.UnscoredHorizon{{
			DecisionID: 1,
			Symbol:     "ETH-USD",
			TriggerAt:  scorerTriggerAt,
			Horizon:    "14d",
			Action:     "buy",
		}},
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
		"ETH-USD": {symClose0, symClose1},
	})
	a := newScorerApp(cb, repo)

	outcome, err := a.computeOutcome(context.Background(), repo.horizons[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	approxF(t, "btcReturnPct", outcome.BTCReturnPct, btcAdj14d, 0.001)
	assertPtrF(t, "symbolReturnPct", outcome.SymbolReturnPct, symRaw, 0.001)
	assertPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct, symRaw, 0.001)
	assertPtrF(t, "excessReturnPct", outcome.ExcessReturnPct, symRaw-btcAdj14d, 0.001)
	assertPtrF(t, "priceAtDecision", outcome.PriceAtDecision, symClose0, 0.001)
	assertPtrF(t, "priceAtHorizon", outcome.PriceAtHorizon, symClose1, 0.001)
	if outcome.FeesModeledPct != 0 {
		t.Fatalf("feesModeledPct: got %v, want 0", outcome.FeesModeledPct)
	}
}

func TestComputeOutcome_Buy_1h(t *testing.T) {
	t.Parallel()
	h := portfolio.UnscoredHorizon{
		DecisionID: 2,
		Symbol:     "ETH-USD",
		TriggerAt:  scorerTriggerAt,
		Horizon:    "1h",
		Action:     "buy",
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
		"ETH-USD": {symClose0, symClose1},
	})
	a := newScorerApp(cb, &scorerTestRepo{})

	outcome, err := a.computeOutcome(context.Background(), h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No fee at 1h.
	approxF(t, "btcReturnPct", outcome.BTCReturnPct, btcRaw, 0.001)
	assertPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct, symRaw, 0.001)
	assertPtrF(t, "excessReturnPct", outcome.ExcessReturnPct, symRaw-btcRaw, 0.001)
}

func TestComputeOutcome_Sell_24h(t *testing.T) {
	t.Parallel()
	h := portfolio.UnscoredHorizon{
		DecisionID: 3,
		Symbol:     "ETH-USD",
		TriggerAt:  scorerTriggerAt,
		Horizon:    "24h",
		Action:     "sell",
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
		"ETH-USD": {symClose0, symClose1},
	})
	a := newScorerApp(cb, &scorerTestRepo{})

	outcome, err := a.computeOutcome(context.Background(), h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sell: realized = -symReturn (price down is good for short).
	approxF(t, "btcReturnPct", outcome.BTCReturnPct, btcRaw, 0.001)
	assertPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct, -symRaw, 0.001)
	assertPtrF(t, "excessReturnPct", outcome.ExcessReturnPct, -symRaw-btcRaw, 0.001)
}

func TestComputeOutcome_Ignore(t *testing.T) {
	t.Parallel()
	h := portfolio.UnscoredHorizon{
		DecisionID: 4,
		Symbol:     "ETH-USD",
		TriggerAt:  scorerTriggerAt,
		Horizon:    "24h",
		Action:     "ignore",
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
		"ETH-USD": {symClose0, symClose1},
	})
	a := newScorerApp(cb, &scorerTestRepo{})

	outcome, err := a.computeOutcome(context.Background(), h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// symbol_return_pct populated for opportunity-cost display.
	assertPtrF(t, "symbolReturnPct", outcome.SymbolReturnPct, symRaw, 0.001)
	// realized and excess are nil — shadow mode cannot claim P&L for inaction.
	assertNilPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct)
	assertNilPtrF(t, "excessReturnPct", outcome.ExcessReturnPct)
}

func TestComputeOutcome_Daily_7d(t *testing.T) {
	t.Parallel()
	// navStart = 900 (positions) + 100 (cash) = 1000
	// navEnd   = 1000 (positions) + 100 (cash) = 1100
	// navReturn = 10%; excess = 10% - 5% (raw BTC, no fee at 7d) = 5%.
	horizonDur := 7 * 24 * time.Hour
	endTime := scorerTriggerAt.Add(horizonDur)

	repo := &scorerTestRepo{
		snapshotSeries: []portfolio.SnapshotRecord{
			makeSnapshot(scorerTriggerAt, 900.0, 100.0),
			makeSnapshot(endTime, 1000.0, 100.0),
		},
	}
	h := portfolio.UnscoredHorizon{
		DecisionID: 5,
		Symbol:     "_portfolio",
		TriggerAt:  scorerTriggerAt,
		Horizon:    "7d",
		Action:     "ignore", // daily triggers have no meaningful action for scoring
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
	})
	a := newScorerApp(cb, repo)

	outcome, err := a.computeOutcome(context.Background(), h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const navReturn = 10.0 // (1100-1000)/1000*100
	approxF(t, "btcReturnPct (7d no fee)", outcome.BTCReturnPct, btcRaw, 0.001)
	assertPtrF(t, "symbolReturnPct (NAV)", outcome.SymbolReturnPct, navReturn, 0.001)
	// daily triggers: realizedReturnPct = nil.
	assertNilPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct)
	// excessReturnPct populated for 7d.
	assertPtrF(t, "excessReturnPct", outcome.ExcessReturnPct, navReturn-btcRaw, 0.001)
}

func TestComputeOutcome_Daily_1h(t *testing.T) {
	t.Parallel()
	horizonDur := time.Hour
	endTime := scorerTriggerAt.Add(horizonDur)

	repo := &scorerTestRepo{
		snapshotSeries: []portfolio.SnapshotRecord{
			makeSnapshot(scorerTriggerAt, 900.0, 100.0),
			makeSnapshot(endTime, 1000.0, 100.0),
		},
	}
	h := portfolio.UnscoredHorizon{
		DecisionID: 6,
		Symbol:     "_portfolio",
		TriggerAt:  scorerTriggerAt,
		Horizon:    "1h",
		Action:     "ignore",
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
	})
	a := newScorerApp(cb, repo)

	outcome, err := a.computeOutcome(context.Background(), h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// symbolReturnPct populated, but excessReturnPct = nil for 1h daily.
	assertPtrF(t, "symbolReturnPct", outcome.SymbolReturnPct, 10.0, 0.001)
	assertNilPtrF(t, "realizedReturnPct", outcome.RealizedReturnPct)
	assertNilPtrF(t, "excessReturnPct", outcome.ExcessReturnPct)
}

func TestComputeOutcome_Daily_MissingSnapshot(t *testing.T) {
	t.Parallel()
	// Only the start snapshot is present; end snapshot is missing → errDeferred.
	repo := &scorerTestRepo{
		horizons: []portfolio.UnscoredHorizon{{
			DecisionID: 7,
			Symbol:     "_portfolio",
			TriggerAt:  scorerTriggerAt,
			Horizon:    "7d",
			Action:     "ignore",
		}},
		snapshotSeries: []portfolio.SnapshotRecord{
			makeSnapshot(scorerTriggerAt, 900.0, 100.0),
			// end snapshot intentionally absent
		},
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
	})
	a := newScorerApp(cb, repo)

	// computeOutcome must return errDeferred.
	_, err := a.computeOutcome(context.Background(), repo.horizons[0])
	if !errors.Is(err, errDeferred) {
		t.Fatalf("expected errDeferred, got: %v", err)
	}

	// scoreBatch must NOT log an error (silent) and must NOT insert a row.
	if err := a.scoreBatch(context.Background()); err != nil {
		t.Fatalf("scoreBatch: %v", err)
	}
	if len(repo.outcomes) != 0 {
		t.Fatalf("expected 0 outcomes, got %d", len(repo.outcomes))
	}
}

func TestComputeOutcome_BTCFetchFail(t *testing.T) {
	t.Parallel()
	repo := &scorerTestRepo{
		horizons: []portfolio.UnscoredHorizon{{
			DecisionID: 8,
			Symbol:     "ETH-USD",
			TriggerAt:  scorerTriggerAt,
			Horizon:    "24h",
			Action:     "buy",
		}},
	}
	cb := failCandleServer(t)
	a := newScorerApp(cb, repo)

	// computeOutcome must return an error (not errDeferred).
	_, err := a.computeOutcome(context.Background(), repo.horizons[0])
	if err == nil {
		t.Fatal("expected error from BTC candle fetch failure")
	}
	if errors.Is(err, errDeferred) {
		t.Fatalf("expected real error, got errDeferred")
	}

	// scoreBatch must warn and continue; no outcome inserted.
	if err := a.scoreBatch(context.Background()); err != nil {
		t.Fatalf("scoreBatch returned unexpected error: %v", err)
	}
	if len(repo.outcomes) != 0 {
		t.Fatalf("expected 0 outcomes, got %d", len(repo.outcomes))
	}
}

func TestScoreBatch_Idempotent(t *testing.T) {
	t.Parallel()
	repo := &scorerTestRepo{
		horizons: []portfolio.UnscoredHorizon{{
			DecisionID: 9,
			Symbol:     "ETH-USD",
			TriggerAt:  scorerTriggerAt,
			Horizon:    "24h",
			Action:     "buy",
		}},
		// Succeed on the first insert; error on the second.
		insertErrAfterN: 1,
	}
	cb := candleServer(t, map[string][2]float64{
		"BTC-USD": {btcClose0, btcClose1},
		"ETH-USD": {symClose0, symClose1},
	})
	a := newScorerApp(cb, repo)

	// First run: outcome inserted.
	if err := a.scoreBatch(context.Background()); err != nil {
		t.Fatalf("first scoreBatch: %v", err)
	}
	if len(repo.outcomes) != 1 {
		t.Fatalf("expected 1 outcome after first run, got %d", len(repo.outcomes))
	}

	// Second run: insert fails (unique constraint). Scorer must not panic.
	if err := a.scoreBatch(context.Background()); err != nil {
		t.Fatalf("second scoreBatch: %v", err)
	}
	// Still only 1 outcome (duplicate insert was rejected).
	if len(repo.outcomes) != 1 {
		t.Fatalf("expected still 1 outcome after second run, got %d", len(repo.outcomes))
	}
}
