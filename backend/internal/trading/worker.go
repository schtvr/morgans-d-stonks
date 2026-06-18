package trading

import (
	"context"
	"fmt"
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
	// Notify is called on significant order transitions (fills, rejects, cancels).
	// Nil disables notifications. Use it to wire Discord or other alerting.
	Notify func(ctx context.Context, msg string)
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
		// Order has been accepted but not yet sent to the broker — place it now.
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
		w.maybeRecordFill(ctx, order, placed, now)
		w.maybeNotifyTransition(ctx, order, observed, placed.FilledPrice, placed.FilledSize)

	default:
		// Order is open and already has a broker ID — poll for current status.
		poller, canPoll := w.Executor.(broker.OrderPoller)
		if !canPoll {
			// Paper broker: no polling — just touch the updated_at timestamp.
			action = "touch"
			if err := w.Repo.UpdateOrderStatus(ctx, order.ID, order.Status, "reconciled", now); err != nil {
				w.Log.Warn("touch order", "order_id", order.ID, "err", err)
			}
			break
		}
		polled, err := poller.GetOrder(ctx, order.ProviderOrderID)
		if err != nil {
			w.Log.Warn("poll order status", "order_id", order.ID, "provider_id", order.ProviderOrderID, "err", err)
			if w.Metrics != nil {
				w.Metrics.IncLiveReconciliationError()
			}
			action = "poll_error"
			// Record drift so we have an audit trail but keep the current status.
			_ = w.Repo.RecordReconciliation(ctx, Reconciliation{
				OrderID:        order.ID,
				ExpectedStatus: order.Status,
				ObservedStatus: order.Status,
				Drift:          true,
				Action:         action,
				Details:        err.Error(),
				CheckedAt:      now,
			})
			return
		}
		observed = mapExecutionStatus(polled.Status)
		if observed != order.Status {
			action = "poll_transition"
			if err := w.Repo.UpdateOrderStatus(ctx, order.ID, observed, "broker poll", now); err != nil {
				w.Log.Warn("update order status (poll)", "order_id", order.ID, "err", err)
			}
			if err := w.Repo.AppendOrderEvent(ctx, OrderEvent{
				OrderID:    order.ID,
				EventType:  "poll",
				FromStatus: order.Status,
				ToStatus:   observed,
				Reason:     "broker poll",
				CreatedAt:  now,
			}); err != nil {
				w.Log.Warn("append poll event", "order_id", order.ID, "err", err)
			}
			w.maybeRecordFill(ctx, order, polled, now)
			w.maybeNotifyTransition(ctx, order, observed, polled.FilledPrice, polled.FilledSize)
		} else {
			action = "poll_unchanged"
			if err := w.Repo.UpdateOrderStatus(ctx, order.ID, order.Status, "poll_unchanged", now); err != nil {
				w.Log.Warn("touch order (poll)", "order_id", order.ID, "err", err)
			}
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

// maybeRecordFill records a fill row when the broker reports a filled or partially-filled status.
func (w *Worker) maybeRecordFill(ctx context.Context, order Order, bOrder *broker.Order, now time.Time) {
	if bOrder == nil {
		return
	}
	observed := mapExecutionStatus(bOrder.Status)
	if observed != OrderStatusFilled && observed != OrderStatusPartiallyFilled {
		return
	}
	price := bOrder.FilledPrice
	qty := bOrder.FilledSize
	if price <= 0 {
		price = order.LimitPrice
	}
	if price <= 0 && order.Quantity > 0 {
		price = order.Notional / order.Quantity
	}
	if price <= 0 {
		price = 1
	}
	if qty <= 0 {
		qty = order.Quantity
	}
	if err := w.Repo.RecordFill(ctx, Fill{
		OrderID:    order.ID,
		Price:      price,
		Quantity:   qty,
		Currency:   "USD",
		ExecutedAt: now,
	}); err != nil {
		w.Log.Warn("record fill", "order_id", order.ID, "err", err)
	}
}

// maybeNotifyTransition fires a Notify message on terminal or fill transitions.
func (w *Worker) maybeNotifyTransition(ctx context.Context, order Order, newStatus OrderStatus, filledPrice, filledSize float64) {
	switch newStatus {
	case OrderStatusFilled, OrderStatusPartiallyFilled:
		price := filledPrice
		qty := filledSize
		if qty <= 0 {
			qty = order.Quantity
		}
		if price <= 0 && order.Quantity > 0 {
			price = order.Notional / order.Quantity
		}
		w.notify(ctx, fmt.Sprintf(
			"trade_fill status=%s symbol=%s side=%s qty=%.8g price=%.8g order_id=%s",
			string(newStatus), order.Symbol, string(order.Side), qty, price, order.ID,
		))
	case OrderStatusRejected, OrderStatusCanceled:
		w.notify(ctx, fmt.Sprintf(
			"trade_outcome status=%s symbol=%s side=%s order_id=%s",
			string(newStatus), order.Symbol, string(order.Side), order.ID,
		))
	}
}

func (w *Worker) notify(ctx context.Context, msg string) {
	if w.Notify != nil {
		w.Notify(ctx, msg)
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
