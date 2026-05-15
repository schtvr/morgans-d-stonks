package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

func TestHandlePortfolioHistory(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	tOld := now.Add(-2 * time.Hour)
	tMid := now.Add(-1 * time.Hour)

	mk := func(ts time.Time, total, symMV float64) portfolio.SnapshotRecord {
		b, err := json.Marshal(portfolio.IngestSnapshotRequest{
			TakenAt: ts,
			Summary: broker.AccountSummary{
				NetLiquidation: total,
				Currency:       "USD",
			},
			Positions: []broker.Position{
				{Symbol: "ZZZ", MarketValue: symMV, Currency: "USD"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return portfolio.SnapshotRecord{TakenAt: ts, Data: b}
	}

	repo := newFakePortfolioRepo()
	repo.snapshotSeries = []portfolio.SnapshotRecord{
		mk(tOld, 10_000, 1000),
		mk(tMid, 10_500, 1200),
		mk(now, 11_000, 1500),
	}

	app := &app{repo: repo, log: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/portfolio/history?range=1d", nil)
	rec := httptest.NewRecorder()
	app.handlePortfolioHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.PortfolioHistoryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Points) < 2 {
		t.Fatalf("expected points, got %d", len(resp.Points))
	}
	if resp.Points[len(resp.Points)-1].TotalValue != 11_000 {
		t.Fatalf("last total %v", resp.Points[len(resp.Points)-1].TotalValue)
	}
	if resp.Points[len(resp.Points)-1].BySymbol["ZZZ"] != 1500 {
		t.Fatalf("last sym %v", resp.Points[len(resp.Points)-1].BySymbol["ZZZ"])
	}
}

func TestHandlePortfolioHistory_badMethod(t *testing.T) {
	t.Parallel()
	app := &app{repo: newFakePortfolioRepo(), log: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/portfolio/history", nil)
	rec := httptest.NewRecorder()
	app.handlePortfolioHistory(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}
