package mock

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Broker is a deterministic mock implementation for CI and local dev.
type Broker struct {
	// MarketOpen controls IsMarketOpen when MOCK_MARKET_OPEN is unset.
	MarketOpen bool
}

// New returns a MockBroker. If env MOCK_MARKET_OPEN is "false", market is closed.
func New() *Broker {
	open := true
	if v := os.Getenv("MOCK_MARKET_OPEN"); v != "" {
		open, _ = strconv.ParseBool(v)
	}
	return &Broker{MarketOpen: open}
}

func (b *Broker) Positions(ctx context.Context) ([]broker.Position, error) {
	now := time.Now().UTC()
	return []broker.Position{
		{
			Symbol:       "AAPL",
			ConID:        265598,
			Quantity:     10,
			AvgCost:      150.25,
			MarketValue:  1755.00,
			UnrealizedPL: 252.50,
			RealizedPL:   0,
			Currency:     "USD",
			UpdatedAt:    now,
		},
		{
			Symbol:       "MSFT",
			ConID:        272093,
			Quantity:     5,
			AvgCost:      300.00,
			MarketValue:  1900.00,
			UnrealizedPL: 400.00,
			RealizedPL:   0,
			Currency:     "USD",
			UpdatedAt:    now,
		},
	}, nil
}

func (b *Broker) AccountSummary(ctx context.Context) (*broker.AccountSummary, error) {
	now := time.Now().UTC()
	return &broker.AccountSummary{
		AccountID:      "MOCK",
		NetLiquidation: 125000.00,
		TotalCash:      25000.00,
		BuyingPower:    50000.00,
		Currency:       "USD",
		UpdatedAt:      now,
	}, nil
}

func (b *Broker) Quotes(ctx context.Context, symbols []string) ([]broker.Quote, error) {
	now := time.Now().UTC()
	out := make([]broker.Quote, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, broker.Quote{
			Symbol:    s,
			Last:      100,
			Bid:       99.5,
			Ask:       100.5,
			Volume:    1_000_000,
			UpdatedAt: now,
		})
	}
	return out, nil
}

func (b *Broker) IsMarketOpen(ctx context.Context) (bool, error) {
	if v := os.Getenv("MOCK_MARKET_OPEN"); v != "" {
		return strconv.ParseBool(v)
	}
	return b.MarketOpen, nil
}

func (b *Broker) Close() error { return nil }
