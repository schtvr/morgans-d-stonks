package broker

import "time"

// OrderIntent is a provider-agnostic trade request.
type OrderIntent struct {
	Symbol   string
	Side     string
	Quantity float64
}

// Order is an execution result.
type Order struct {
	ID        string
	Symbol    string
	Status    string
	CreatedAt time.Time
}

// OrderEvent is a streaming event for order lifecycle updates.
type OrderEvent struct {
	OrderID string
	Type    string
	At      time.Time
}
