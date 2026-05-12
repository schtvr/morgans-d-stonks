package trading

import (
	"context"
	"log/slog"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Executor is the narrow execution contract used by the trading worker.
type Executor interface {
	PlaceOrder(ctx context.Context, intent broker.OrderIntent) (*broker.Order, error)
	CancelOrder(ctx context.Context, orderID string) error
}

// Worker submits accepted orders and records reconciliation state.
type Worker struct {
	Repo     Repository
	Executor Executor
	Interval time.Duration
	Log      *slog.Logger
	Metrics  *Metrics
	Clock    func() time.Time
}

// Run loops until the context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	if w.Log == nil {
		w.Log = slog.Default()
	}
	if w.Interval <= 0 {
		w.Interval = 30 * time.Second
	}
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	orders, err := w.Repo.ListOpenOrders(ctx)
	if err != nil {
		w.Log.Warn("list open orders", "err", err)
		return
	}
	for _, order := range orders {
		w.reconcileOrder(ctx, order)
	}
}

func (w *Worker) reconcileOrder(ctx context.Context, order Order) {
	now := w.now()
	observed := order.Status
	action := "noop"
	switch {
	case !IsOpen(order.Status):
		action = "skip"
	case w.Executor == nil:
		action = "pending-executor"
	case order.ProviderOrderID == "":
		intent := broker.OrderIntent{Symbol: order.Symbol, Side: string(order.Side), Quantity: order.Quantity}
		placeStart := time.Now()
		placed, err := w.Executor.PlaceOrder(ctx, intent)
		if err != nil {
			w.Log.Warn("place order", "order_id", order.ID, "err", err)
			_ = w.Repo.RecordReconciliation(ctx, Reconciliation{
				OrderID:        order.ID,
				ExpectedStatus: order.Status,
				ObservedStatus: order.Status,
				Drift:          true,
				Action:         "place_error",
				Details:        err.Error(),
				CheckedAt:      now,
			})
			return
		}
		if w.Metrics != nil {
			w.Metrics.ObservePlacementLatency(time.Since(placeStart))
		}
		observed = mapExecutionStatus(placed.Status)
		action = "place"
		if err := w.Repo.SetProviderOrderID(ctx, order.ID, placed.ID); err != nil {
			w.Log.Warn("set provider order id", "order_id", order.ID, "err", err)
		}
		if err := w.Repo.UpdateOrderStatus(ctx, order.ID, observed, "broker execution update", now); err != nil {
			w.Log.Warn("update order status", "order_id", order.ID, "err", err)
		}
		if err := w.Repo.AppendOrderEvent(ctx, OrderEvent{
			OrderID:    order.ID,
			EventType:  "submit",
			FromStatus: order.Status,
			ToStatus:   observed,
			Reason:     "broker execution update",
			CreatedAt:  now,
		}); err != nil {
			w.Log.Warn("append submit event", "order_id", order.ID, "err", err)
		}
		if observed == OrderStatusFilled || observed == OrderStatusPartiallyFilled {
			price := order.LimitPrice
			if price <= 0 && order.Quantity > 0 {
				price = order.Notional / order.Quantity
			}
			if price <= 0 {
				price = 1
			}
			if err := w.Repo.RecordFill(ctx, Fill{
				OrderID:    order.ID,
				Price:      price,
				Quantity:   order.Quantity,
				Currency:   "USD",
				ExecutedAt: now,
			}); err != nil {
				w.Log.Warn("record fill", "order_id", order.ID, "err", err)
			}
		}
	default:
		if err := w.Repo.UpdateOrderStatus(ctx, order.ID, order.Status, "reconciled", now); err != nil {
			w.Log.Warn("touch order", "order_id", order.ID, "err", err)
		}
	}
	drift := observed != order.Status
	if w.Metrics != nil {
		w.Metrics.ObserveReconciliationLag(now.Sub(order.UpdatedAt))
	}
	if err := w.Repo.RecordReconciliation(ctx, Reconciliation{
		OrderID:        order.ID,
		ExpectedStatus: order.Status,
		ObservedStatus: observed,
		Drift:          drift,
		Action:         action,
		Details:        "reconciliation tick",
		CheckedAt:      now,
	}); err != nil {
		w.Log.Warn("record reconciliation", "order_id", order.ID, "err", err)
	}
}

func (w *Worker) now() time.Time {
	if w.Clock != nil {
		return w.Clock().UTC()
	}
	return time.Now().UTC()
}

func mapExecutionStatus(status string) OrderStatus {
	switch status {
	case "filled":
		return OrderStatusFilled
	case "partially_filled":
		return OrderStatusPartiallyFilled
	case "rejected":
		return OrderStatusRejected
	case "canceled":
		return OrderStatusCanceled
	default:
		return OrderStatusAccepted
	}
}
