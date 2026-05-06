package broker

import "context"

// Capability indicates supported broker operations.
type Capability string

const (
	CapabilityReadPositions Capability = "read_positions"
	CapabilityReadSummary   Capability = "read_summary"
	CapabilityQuote         Capability = "quote"
	CapabilityPlaceOrder    Capability = "place_order"
	CapabilityCancelOrder   Capability = "cancel_order"
	CapabilityStreamOrders  Capability = "stream_orders"
)

// ReadBroker is the shared contract for market/account data (SCH-20 implements).
type ReadBroker interface {
	Positions(ctx context.Context) ([]Position, error)
	AccountSummary(ctx context.Context) (*AccountSummary, error)
	Quotes(ctx context.Context, symbols []string) ([]Quote, error)
	IsMarketOpen(ctx context.Context) (bool, error)
	Close() error
}

// ExecutionBroker defines trading operations.
type ExecutionBroker interface {
	PlaceOrder(ctx context.Context, intent OrderIntent) (*Order, error)
	CancelOrder(ctx context.Context, orderID string) error
}

// StreamBroker defines streaming subscriptions.
type StreamBroker interface {
	StreamOrders(ctx context.Context) (<-chan OrderEvent, error)
}

// CapabilitiesBroker declares broker capabilities.
type CapabilitiesBroker interface {
	Capabilities() map[Capability]bool
}

// Broker remains backward-compatible with existing read consumers.
type Broker = ReadBroker

// HasCapability checks capability support, defaulting to false when unknown.
func HasCapability(b any, capability Capability) bool {
	cb, ok := b.(CapabilitiesBroker)
	if !ok {
		return false
	}
	return cb.Capabilities()[capability]
}
