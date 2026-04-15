package broker

import "context"

// Broker is the shared contract for market/account data (SCH-20 implements).
type Broker interface {
	Positions(ctx context.Context) ([]Position, error)
	AccountSummary(ctx context.Context) (*AccountSummary, error)
	Quotes(ctx context.Context, symbols []string) ([]Quote, error)
	IsMarketOpen(ctx context.Context) (bool, error)
	Close() error
}
