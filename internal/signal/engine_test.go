package signal

import (
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

func TestEvaluateConcentration(t *testing.T) {
	rule := Rule{
		ID:   "large-position",
		Name: "Large Position Alert",
		Condition: Condition{
			Type:      "concentration",
			Operator:  "gte",
			Threshold: 20,
		},
	}
	snap := &portfolio.IngestSnapshotRequest{
		TakenAt: time.Now().UTC(),
		Positions: []broker.Position{
			{Symbol: "AAPL", Quantity: 1, AvgCost: 100, MarketValue: 8000, UnrealizedPL: 0, Currency: "USD"},
			{Symbol: "MSFT", Quantity: 1, AvgCost: 100, MarketValue: 1000, UnrealizedPL: 0, Currency: "USD"},
		},
		Summary: broker.AccountSummary{AccountID: "T"},
	}
	evs, err := Evaluate(rule, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Symbol != "AAPL" {
		t.Fatalf("unexpected events: %+v", evs)
	}
}

func TestEvaluatePriceChange(t *testing.T) {
	rule := Rule{
		ID:   "price-drop-5pct",
		Name: "5% Price Drop",
		Condition: Condition{
			Type:      "price_change_pct",
			Operator:  "lte",
			Threshold: -5,
		},
	}
	snap := &portfolio.IngestSnapshotRequest{
		TakenAt: time.Now().UTC(),
		Positions: []broker.Position{
			{Symbol: "AAPL", Quantity: 10, AvgCost: 100, MarketValue: 900, UnrealizedPL: -100, Currency: "USD"},
		},
		Summary: broker.AccountSummary{AccountID: "T"},
	}
	evs, err := Evaluate(rule, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %+v", evs)
	}
}
