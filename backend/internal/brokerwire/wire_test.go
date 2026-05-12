package brokerwire

import (
	"context"
	"testing"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

func TestNewMock(t *testing.T) {
	b, err := New(broker.Config{Mode: "mock"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Positions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !broker.HasCapability(b, broker.CapabilityQuote) {
		t.Fatal("expected quote capability")
	}
}

func TestNewExecutionUnsupportedMode(t *testing.T) {
	_, err := NewExecution(broker.Config{Mode: "mock"})
	if err == nil {
		t.Fatal("expected unsupported execution error")
	}
}

func TestNewExecutionCoinbasePaper(t *testing.T) {
	b, err := NewExecution(broker.Config{Provider: "coinbase", Environment: "paper"})
	if err != nil {
		t.Fatal(err)
	}
	order, err := b.PlaceOrder(context.Background(), broker.OrderIntent{Symbol: "BTC-USD", Side: "buy", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status == "" {
		t.Fatal("expected paper order status")
	}
}

func TestNewCoinbase(t *testing.T) {
	b, err := New(broker.Config{Provider: "coinbase"})
	if err != nil {
		t.Fatal(err)
	}
	if !broker.HasCapability(b, broker.CapabilityQuote) {
		t.Fatal("expected coinbase quote capability")
	}
}
