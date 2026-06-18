package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubCostRepo struct {
	sum int64
	err error
}

func (s *stubCostRepo) SumCostCentsForDay(_ context.Context, _ time.Time) (int64, error) {
	return s.sum, s.err
}

func TestCostTracker_NotReached(t *testing.T) {
	t.Parallel()
	repo := &stubCostRepo{sum: 10}
	tracker := NewCostTracker(repo, 100)
	// 10 + 20 = 30 < 100 → nil
	if err := tracker.CheckAndReserve(context.Background(), 20); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCostTracker_ExactlyAtCap(t *testing.T) {
	t.Parallel()
	repo := &stubCostRepo{sum: 80}
	tracker := NewCostTracker(repo, 100)
	// 80 + 20 = 100 which is NOT > cap, so should allow
	if err := tracker.CheckAndReserve(context.Background(), 20); err != nil {
		t.Errorf("expected nil at exact cap, got %v", err)
	}
	// 80 + 21 = 101 > 100 → cap reached
	if err := tracker.CheckAndReserve(context.Background(), 21); !errors.Is(err, ErrCapReached) {
		t.Errorf("expected ErrCapReached, got %v", err)
	}
}

func TestCostTracker_SumAloneExceedsCap(t *testing.T) {
	t.Parallel()
	repo := &stubCostRepo{sum: 150}
	tracker := NewCostTracker(repo, 100)
	if err := tracker.CheckAndReserve(context.Background(), 1); !errors.Is(err, ErrCapReached) {
		t.Errorf("expected ErrCapReached when sum alone exceeds cap, got %v", err)
	}
}

func TestCostTracker_RepoError_AllowsThrough(t *testing.T) {
	t.Parallel()
	repo := &stubCostRepo{err: errors.New("db down")}
	tracker := NewCostTracker(repo, 100)
	// On repo failure the tracker should not block (fail open).
	if err := tracker.CheckAndReserve(context.Background(), 20); err != nil {
		t.Errorf("expected nil on repo error (fail open), got %v", err)
	}
}
