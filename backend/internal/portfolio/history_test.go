package portfolio

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

func TestParseHistoryRange(t *testing.T) {
	if _, ok := ParseHistoryRange(""); ok {
		t.Fatal("empty should miss")
	}
	d, ok := ParseHistoryRange("1h")
	if !ok || d != time.Hour {
		t.Fatalf("1h: %v %v", d, ok)
	}
}

func TestDownsampleHistoryPoints(t *testing.T) {
	pts := make([]PortfolioHistoryPoint, 100)
	for i := range pts {
		pts[i].TotalValue = float64(i)
	}
	out := DownsampleHistoryPoints(pts, 5)
	if len(out) != 5 {
		t.Fatalf("len got %d", len(out))
	}
	if out[0].TotalValue != 0 || out[len(out)-1].TotalValue != 99 {
		t.Fatalf("endpoints: first=%v last=%v", out[0].TotalValue, out[len(out)-1].TotalValue)
	}
}

func TestHistoryPointsFromRecords(t *testing.T) {
	t1 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	b, err := json.Marshal(IngestSnapshotRequest{
		TakenAt: t1,
		Summary: broker.AccountSummary{
			NetLiquidation: 1000,
			Currency:       "USD",
		},
		Positions: []broker.Position{
			{Symbol: "AAA", MarketValue: 400, Currency: "USD"},
			{Symbol: "BBB", MarketValue: 600, Currency: "USD"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pts, err := HistoryPointsFromRecords([]SnapshotRecord{{TakenAt: t1, Data: b}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("len %d", len(pts))
	}
	if pts[0].TotalValue != 1000 || pts[0].BySymbol["AAA"] != 400 || pts[0].BySymbol["BBB"] != 600 {
		t.Fatalf("values %+v", pts[0])
	}
}
