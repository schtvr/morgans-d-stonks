package agent

import (
	"context"
	"errors"
	"time"
)

// ErrCapReached is returned by CostTracker.CheckAndReserve when the daily
// spend cap would be exceeded.
var ErrCapReached = errors.New("agent: daily cost cap reached")

// CostRepo provides persistent daily cost aggregation queries.
type CostRepo interface {
	SumCostCentsForDay(ctx context.Context, day time.Time) (int64, error)
}

// CostTracker guards against exceeding the daily LLM spend cap.
type CostTracker struct {
	repo     CostRepo
	capCents int64
}

// NewCostTracker creates a CostTracker that allows at most capCents of LLM
// spend per UTC calendar day.
func NewCostTracker(repo CostRepo, capCents int64) *CostTracker {
	return &CostTracker{repo: repo, capCents: capCents}
}

// CheckAndReserve returns ErrCapReached if today's total plus estimateCents
// would exceed the cap. The estimate is conservative; the actual cost is
// reconciled after the call via the repository.
func (t *CostTracker) CheckAndReserve(ctx context.Context, estimateCents int64) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	sum, err := t.repo.SumCostCentsForDay(ctx, today)
	if err != nil {
		// On repo failure, allow the call through rather than hard-blocking all
		// decisions — cost overruns are preferable to a total outage.
		return nil
	}
	if sum+estimateCents > t.capCents {
		return ErrCapReached
	}
	return nil
}
