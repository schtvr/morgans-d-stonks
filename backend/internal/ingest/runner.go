package ingest

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
)

// Runner orchestrates periodic snapshot ingestion.
type Runner struct {
	Broker   broker.Broker
	Client   *Client
	Interval time.Duration
	Log      *slog.Logger
}

// Run blocks until SIGINT/SIGTERM, executing ticks on Interval.
func (r *Runner) Run(ctx context.Context) error {
	if r.Log == nil {
		r.Log = logging.New("ingest")
	}
	t := time.NewTicker(r.Interval)
	defer t.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	r.tick(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sig:
			r.Log.Info("shutdown")
			return nil
		case now := <-t.C:
			r.tick(ctx, now)
		}
	}
}

func (r *Runner) tick(ctx context.Context, now time.Time) {
	_ = now
	start := time.Now()
	outcome := "unknown"
	positionCount := 0
	var takenAt time.Time

	defer func() {
		attrs := []any{
			"tick_outcome", outcome,
			"duration_ms", time.Since(start).Milliseconds(),
			"position_count", positionCount,
		}
		if !takenAt.IsZero() {
			attrs = append(attrs, "taken_at", takenAt.UTC())
		}
		r.Log.Info("ingest_tick", attrs...)
	}()

	open, err := r.Broker.IsMarketOpen(ctx)
	if err != nil {
		outcome = "skipped_broker_market_check"
		r.Log.Warn("broker market check", "err", err)
		return
	}
	if !open {
		outcome = "skipped_market_closed"
		return
	}

	positions, err := r.Broker.Positions(ctx)
	if err != nil {
		outcome = "skipped_broker_positions"
		r.Log.Warn("broker positions", "err", err)
		return
	}
	positionCount = len(positions)
	summary, err := r.Broker.AccountSummary(ctx)
	if err != nil {
		outcome = "skipped_broker_summary"
		r.Log.Warn("broker summary", "err", err)
		return
	}

	symbols := make([]string, 0, len(positions))
	for _, p := range positions {
		symbols = append(symbols, p.Symbol)
	}
	if _, err := r.Broker.Quotes(ctx, symbols); err != nil {
		r.Log.Warn("broker quotes", "err", err)
	}

	taken := time.Now().UTC()
	snap := BuildSnapshot(taken, positions, summary)
	payload, err := MarshalSnapshot(snap)
	if err != nil {
		outcome = "skipped_marshal_error"
		r.Log.Warn("marshal snapshot", "err", err)
		return
	}
	if err := r.Client.PostSnapshotRetry(ctx, payload); err != nil {
		outcome = "skipped_post_error"
		r.Log.Warn("post snapshot", "err", err)
		return
	}
	outcome = "posted"
	takenAt = snap.TakenAt
}
