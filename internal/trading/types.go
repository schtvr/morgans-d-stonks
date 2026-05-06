package trading

import "time"

// OrderSide is the buy/sell direction for an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderStatus represents the canonical lifecycle state.
type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "new"
	OrderStatusAccepted        OrderStatus = "accepted"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled          OrderStatus = "filled"
	OrderStatusCanceled        OrderStatus = "canceled"
	OrderStatusRejected        OrderStatus = "rejected"
)

// OrderRequest is the internal API payload for order operations.
type OrderRequest struct {
	Symbol           string    `json:"symbol"`
	Side             OrderSide `json:"side"`
	Quantity         float64   `json:"quantity"`
	LimitPrice       float64   `json:"limitPrice,omitempty"`
	AvailableCash    float64   `json:"availableCash,omitempty"`
	IdempotencyKey   string    `json:"idempotencyKey,omitempty"`
	RequestHash      string    `json:"requestHash,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	ProviderEnv      string    `json:"providerEnv,omitempty"`
	ReferenceOrderID string    `json:"referenceOrderId,omitempty"`
}

// Order is the persisted trading record.
type Order struct {
	ID              string      `json:"id"`
	Symbol          string      `json:"symbol"`
	Side            OrderSide   `json:"side"`
	Quantity        float64     `json:"quantity"`
	LimitPrice      float64     `json:"limitPrice,omitempty"`
	Notional        float64     `json:"notional"`
	Status          OrderStatus `json:"status"`
	Reason          string      `json:"reason,omitempty"`
	IdempotencyKey  string      `json:"idempotencyKey,omitempty"`
	RequestHash     string      `json:"requestHash,omitempty"`
	ResponseJSON    []byte      `json:"-"`
	Provider        string      `json:"provider,omitempty"`
	ProviderOrderID string      `json:"providerOrderId,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

// OrderResponse is returned by validate/create/get handlers.
type OrderResponse struct {
	Order    Order        `json:"order"`
	Decision RiskDecision `json:"decision"`
}

// OrderEvent records state transitions immutably.
type OrderEvent struct {
	ID         int64       `json:"id"`
	OrderID    string      `json:"orderId"`
	EventType  string      `json:"eventType"`
	FromStatus OrderStatus `json:"fromStatus,omitempty"`
	ToStatus   OrderStatus `json:"toStatus,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	Payload    []byte      `json:"payload,omitempty"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// Fill captures an execution fill.
type Fill struct {
	ID         int64     `json:"id"`
	OrderID    string    `json:"orderId"`
	Price      float64   `json:"price"`
	Quantity   float64   `json:"quantity"`
	Fee        float64   `json:"fee"`
	Currency   string    `json:"currency"`
	ExecutedAt time.Time `json:"executedAt"`
}

// Reconciliation records drift detection results.
type Reconciliation struct {
	ID             int64       `json:"id"`
	OrderID        string      `json:"orderId"`
	ExpectedStatus OrderStatus `json:"expectedStatus"`
	ObservedStatus OrderStatus `json:"observedStatus"`
	Drift          bool        `json:"drift"`
	Action         string      `json:"action,omitempty"`
	Details        string      `json:"details,omitempty"`
	CheckedAt      time.Time   `json:"checkedAt"`
}

// RiskDecision is the deterministic output of the policy engine.
type RiskDecision struct {
	Allowed     bool      `json:"allowed"`
	ReasonCodes []string  `json:"reasonCodes,omitempty"`
	Notional    float64   `json:"notional"`
	CheckedAt   time.Time `json:"checkedAt"`
}
