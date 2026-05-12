package trading

import (
	"context"
	"time"
)

// Repository persists trading orders, events, fills, and reconciliation data.
type Repository interface {
	RunMigrations(ctx context.Context) error
	Close()

	CreateOrder(ctx context.Context, order Order) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	GetOrderByIdempotencyKey(ctx context.Context, key string) (*Order, error)
	ListOpenOrders(ctx context.Context) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, id string, status OrderStatus, reason string, updatedAt time.Time) error
	SetProviderOrderID(ctx context.Context, id string, providerOrderID string) error
	SaveResponseJSON(ctx context.Context, id string, payload []byte) error
	AppendOrderEvent(ctx context.Context, event OrderEvent) error
	RecordFill(ctx context.Context, fill Fill) error
	RecordReconciliation(ctx context.Context, rec Reconciliation) error
}
