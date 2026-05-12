package mock

import (
	"context"
	"os"
	"strconv"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Broker is a deterministic in-memory broker for local development.
type Broker struct {
	// MarketOpen controls IsMarketOpen when MOCK_MARKET_OPEN is unset.
	MarketOpen bool
}

// New builds a mock broker.
func New() *Broker { return &Broker{MarketOpen: true} }

// Capabilities declares supported operations.
func (b *Broker) Capabilities() map[broker.Capability]bool {
	return map[broker.Capability]bool{
		broker.CapabilityReadPositions: true,
		broker.CapabilityReadSummary:   true,
		broker.CapabilityQuote:         true,
		broker.CapabilityPlaceOrder:    false,
		broker.CapabilityCancelOrder:   false,
		broker.CapabilityStreamOrders:  false,
	}
}

func (b *Broker) Positions(ctx context.Context) ([]broker.Position, error) {
	_ = ctx
	return []broker.Position{
		{Symbol: "AAPL", Quantity: 10, AvgCost: 180.5, MarketValue: 1850.0, UnrealizedPL: 45.0},
		{Symbol: "MSFT", Quantity: 5, AvgCost: 330.0, MarketValue: 1700.0, UnrealizedPL: 50.0},
	}, nil
}

func (b *Broker) AccountSummary(ctx context.Context) (*broker.AccountSummary, error) {
	_ = ctx
	return &broker.AccountSummary{
		AccountID:      "DU-MOCK",
		NetLiquidation: 100000,
		BuyingPower:    200000,
		TotalCash:      25000,
		Currency:       "USD",
	}, nil
}

func (b *Broker) Quotes(ctx context.Context, symbols []string) ([]broker.Quote, error) {
	_ = ctx
	out := make([]broker.Quote, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, broker.Quote{Symbol: s, Last: 100.0, Bid: 99.5, Ask: 100.5})
	}
	return out, nil
}

func (b *Broker) IsMarketOpen(ctx context.Context) (bool, error) {
	_ = ctx
	if v := os.Getenv("MOCK_MARKET_OPEN"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n != 0, nil
		}
	}
	return b.MarketOpen, nil
}

func (b *Broker) Close() error { return nil }
