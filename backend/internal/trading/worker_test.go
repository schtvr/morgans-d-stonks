package trading

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

type fakeRepo struct {
	open   []Order
	events []OrderEvent
	fills  []Fill
	recs   []Reconciliation
	status map[string]OrderStatus
	ids    map[string]string
}

func (f *fakeRepo) RunMigrations(context.Context) error              { return nil }
func (f *fakeRepo) Close()                                           {}
func (f *fakeRepo) CreateOrder(context.Context, Order) error         { return nil }
func (f *fakeRepo) GetOrder(context.Context, string) (*Order, error) { return nil, ErrOrderNotFound }
func (f *fakeRepo) GetOrderByIdempotencyKey(context.Context, string) (*Order, error) {
	return nil, ErrOrderNotFound
}
func (f *fakeRepo) ListOpenOrders(context.Context) ([]Order, error) {
	return append([]Order(nil), f.open...), nil
}
func (f *fakeRepo) UpdateOrderStatus(_ context.Context, id string, status OrderStatus, reason string, _ time.Time) error {
	if f.status == nil {
		f.status = map[string]OrderStatus{}
	}
	f.status[id] = status
	_ = reason
	return nil
}
func (f *fakeRepo) SetProviderOrderID(_ context.Context, id string, providerOrderID string) error {
	if f.ids == nil {
		f.ids = map[string]string{}
	}
	f.ids[id] = providerOrderID
	return nil
}
func (f *fakeRepo) SaveResponseJSON(context.Context, string, []byte) error { return nil }
func (f *fakeRepo) AppendOrderEvent(_ context.Context, ev OrderEvent) error {
	f.events = append(f.events, ev)
	return nil
}
func (f *fakeRepo) RecordFill(_ context.Context, fill Fill) error {
	f.fills = append(f.fills, fill)
	return nil
}
func (f *fakeRepo) RecordReconciliation(_ context.Context, rec Reconciliation) error {
	f.recs = append(f.recs, rec)
	return nil
}

type fakeExec struct{ mode string }

func (f fakeExec) PlaceOrder(context.Context, broker.OrderIntent) (*broker.Order, error) {
	status := f.mode
	if status == "" {
		status = "filled"
	}
	return &broker.Order{ID: "paper-1", Status: status, CreatedAt: time.Now().UTC()}, nil
}
func (fakeExec) CancelOrder(context.Context, string) error { return nil }

func TestWorkerReconcilesAndRecordsFill(t *testing.T) {
	repo := &fakeRepo{open: []Order{{
		ID: "o1", Symbol: "BTC-USD", Side: OrderSideBuy, Quantity: 1, LimitPrice: 50, Status: OrderStatusAccepted,
	}}}
	w := &Worker{
		Repo:     repo,
		Executor: fakeExec{},
		Interval: time.Second,
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	w.tick(context.Background())
	if got := repo.status["o1"]; got != OrderStatusFilled {
		t.Fatalf("expected filled status, got %s", got)
	}
	if repo.ids["o1"] != "paper-1" {
		t.Fatalf("expected provider id set, got %q", repo.ids["o1"])
	}
	if len(repo.fills) != 1 {
		t.Fatalf("expected fill recorded, got %d", len(repo.fills))
	}
	if len(repo.recs) == 0 || !repo.recs[0].Drift {
		t.Fatalf("expected reconciliation drift entry, got %+v", repo.recs)
	}
}

func TestWorkerNoExecutorMarksPending(t *testing.T) {
	repo := &fakeRepo{open: []Order{{ID: "o1", Symbol: "BTC-USD", Status: OrderStatusAccepted}}}
	w := &Worker{Repo: repo, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.tick(context.Background())
	if len(repo.recs) == 0 {
		t.Fatal("expected reconciliation record")
	}
}

func TestWorkerRejectAndPartialFillPaths(t *testing.T) {
	t.Run("reject", func(t *testing.T) {
		repo := &fakeRepo{open: []Order{{ID: "o1", Symbol: "BTC-USD", Status: OrderStatusAccepted}}}
		w := &Worker{Repo: repo, Executor: fakeExec{mode: "rejected"}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		w.tick(context.Background())
		if got := repo.status["o1"]; got != OrderStatusRejected {
			t.Fatalf("expected rejected, got %s", got)
		}
	})
	t.Run("partial", func(t *testing.T) {
		repo := &fakeRepo{open: []Order{{ID: "o1", Symbol: "BTC-USD", Status: OrderStatusAccepted}}}
		w := &Worker{Repo: repo, Executor: fakeExec{mode: "partially_filled"}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		w.tick(context.Background())
		if got := repo.status["o1"]; got != OrderStatusPartiallyFilled {
			t.Fatalf("expected partial fill, got %s", got)
		}
	})
}

var _ Repository = (*fakeRepo)(nil)
var _ Executor = fakeExec{}
