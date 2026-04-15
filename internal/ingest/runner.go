package ingest

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
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
		r.Log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
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
	open, err := r.Broker.IsMarketOpen(ctx)
	if err != nil {
		r.Log.Warn("broker market check", "err", err)
		return
	}
	if !open {
		r.Log.Info("market closed; skipping tick")
		return
	}

	positions, err := r.Broker.Positions(ctx)
	if err != nil {
		r.Log.Warn("broker positions", "err", err)
		return
	}
	summary, err := r.Broker.AccountSummary(ctx)
	if err != nil {
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
		r.Log.Warn("marshal snapshot", "err", err)
		return
	}
	if err := r.Client.PostSnapshotRetry(ctx, payload); err != nil {
		r.Log.Warn("post snapshot", "err", err)
		return
	}
	r.Log.Info("snapshot posted", "takenAt", snap.TakenAt)
}
