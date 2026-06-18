package broker

import "context"
import "time"

// OrderIntent is a provider-agnostic trade request.
type OrderIntent struct {
	Symbol   string
	Side     string
	Quantity float64
}

// Order is an execution result.
type Order struct {
	ID          string
	Symbol      string
	Status      string
	CreatedAt   time.Time
	FilledPrice float64 // average fill price (live brokers only; 0 if unknown)
	FilledSize  float64 // base asset quantity filled (live brokers only; 0 if unknown)
}

// OrderPoller is an optional ExecutionBroker extension for polling live order status.
// Implementations that do not support polling (e.g. paper broker) do not need to
// implement this interface; the trading worker uses type assertion to detect support.
type OrderPoller interface {
	GetOrder(ctx context.Context, providerOrderID string) (*Order, error)
}

// OrderEvent is a streaming event for order lifecycle updates.
type OrderEvent struct {
	OrderID string
	Type    string
	At      time.Time
}
