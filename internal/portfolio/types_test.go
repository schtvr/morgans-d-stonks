package portfolio

import (
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

func TestMapIngestToViews(t *testing.T) {
	tm := time.Date(2026, 4, 15, 15, 30, 0, 0, time.UTC)
	req := &IngestSnapshotRequest{
		TakenAt: tm,
		Positions: []broker.Position{
			{Symbol: "AAPL", Quantity: 10, AvgCost: 100, MarketValue: 1200, UnrealizedPL: 200, Currency: "USD"},
		},
		Summary: broker.AccountSummary{AccountID: "X"},
	}
	v := MapIngestToViews(req)
	if len(v.Positions) != 1 || v.Positions[0].LastPrice != 120 {
		t.Fatalf("unexpected view: %+v", v.Positions)
	}
}
