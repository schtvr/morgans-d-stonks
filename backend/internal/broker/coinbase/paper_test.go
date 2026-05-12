package coinbase

import (
	"context"
	"testing"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

func TestPaperBrokerPlaceAndCancel(t *testing.T) {
	b := NewPaperExecution()
	order, err := b.PlaceOrder(context.Background(), broker.OrderIntent{Symbol: "BTC-USD", Side: "buy", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if order.ID == "" || order.Status == "" {
		t.Fatalf("unexpected paper order: %+v", order)
	}
	if err := b.CancelOrder(context.Background(), order.ID); err != nil {
		t.Fatal(err)
	}
}
