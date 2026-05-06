package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

type fakePortfolioRepo struct {
	followed      map[string]portfolio.FollowedSymbol
	seeded        bool
	snapshot      []byte
	signalSetting *portfolio.SignalSettings
	recentAlerts  []portfolio.RecentAlert
}

func newFakePortfolioRepo() *fakePortfolioRepo {
	return &fakePortfolioRepo{
		followed: map[string]portfolio.FollowedSymbol{},
		signalSetting: &portfolio.SignalSettings{
			MoveThresholdPct: 1.0,
			Cooldown:         "15m",
			UpdatedAt:        time.Unix(0, 0).UTC(),
		},
	}
}

func (f *fakePortfolioRepo) RunMigrations(context.Context) error                     { return nil }
func (f *fakePortfolioRepo) UpsertSnapshot(context.Context, time.Time, []byte) error { return nil }
func (f *fakePortfolioRepo) LatestSnapshot(context.Context) (time.Time, []byte, error) {
	if len(f.snapshot) == 0 {
		return time.Time{}, nil, pgx.ErrNoRows
	}
	return time.Now().UTC(), f.snapshot, nil
}
func (f *fakePortfolioRepo) ListFollowedSymbols(context.Context) ([]portfolio.FollowedSymbol, error) {
	out := make([]portfolio.FollowedSymbol, 0, len(f.followed))
	for _, v := range f.followed {
		out = append(out, v)
	}
	return out, nil
}
func (f *fakePortfolioRepo) UpsertFollowedSymbol(_ context.Context, symbol, source string) error {
	f.followed[symbol] = portfolio.FollowedSymbol{Symbol: symbol, Source: source, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	return nil
}
func (f *fakePortfolioRepo) RemoveFollowedSymbol(_ context.Context, symbol string) error {
	delete(f.followed, symbol)
	return nil
}
func (f *fakePortfolioRepo) FollowedSymbolsSeeded(context.Context) (bool, error) {
	return f.seeded, nil
}
func (f *fakePortfolioRepo) MarkFollowedSymbolsSeeded(context.Context, time.Time) error {
	f.seeded = true
	return nil
}
func (f *fakePortfolioRepo) GetSignalSettings(context.Context) (*portfolio.SignalSettings, error) {
	return f.signalSetting, nil
}
func (f *fakePortfolioRepo) UpdateSignalSettings(_ context.Context, req portfolio.SignalSettingsRequest) error {
	f.signalSetting = &portfolio.SignalSettings{
		MoveThresholdPct: req.MoveThresholdPct,
		Cooldown:         req.Cooldown,
		UpdatedAt:        time.Now().UTC(),
	}
	return nil
}
func (f *fakePortfolioRepo) ListRecentAlerts(context.Context, int) ([]portfolio.RecentAlert, error) {
	out := make([]portfolio.RecentAlert, len(f.recentAlerts))
	copy(out, f.recentAlerts)
	return out, nil
}
func (f *fakePortfolioRepo) InsertRecentAlert(_ context.Context, alert portfolio.RecentAlert) error {
	alert.ID = int64(len(f.recentAlerts) + 1)
	alert.CreatedAt = time.Now().UTC()
	f.recentAlerts = append(f.recentAlerts, alert)
	return nil
}
func (f *fakePortfolioRepo) CreateSession(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakePortfolioRepo) SessionUser(context.Context, string) (string, error) {
	return "", pgx.ErrNoRows
}
func (f *fakePortfolioRepo) DeleteSession(context.Context, string) error { return nil }

var _ portfolio.Repository = (*fakePortfolioRepo)(nil)

func TestFollowedSymbolHandlers(t *testing.T) {
	repo := newFakePortfolioRepo()
	app := &app{repo: repo}

	addReq := httptest.NewRequest(http.MethodPost, "/api/trading/followed-symbols", bytes.NewBufferString(`{"symbol":"btc-usd"}`))
	addRec := httptest.NewRecorder()
	app.handleFollowedSymbolsAdd(addRec, addReq)
	if addRec.Code != http.StatusNoContent {
		t.Fatalf("add status: %d", addRec.Code)
	}
	if _, ok := repo.followed["BTC-USD"]; !ok {
		t.Fatalf("symbol not stored: %+v", repo.followed)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/trading/followed-symbols", nil)
	listRec := httptest.NewRecorder()
	app.handleFollowedSymbolsList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status: %d", listRec.Code)
	}
	var resp portfolio.FollowedSymbolsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Symbols) != 1 || resp.Symbols[0].Symbol != "BTC-USD" {
		t.Fatalf("unexpected list: %+v", resp.Symbols)
	}

	rmReq := httptest.NewRequest(http.MethodDelete, "/api/trading/followed-symbols/BTC-USD", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("symbol", "BTC-USD")
	rmReq = rmReq.WithContext(context.WithValue(rmReq.Context(), chi.RouteCtxKey, rctx))
	rmRec := httptest.NewRecorder()
	app.handleFollowedSymbolRemove(rmRec, rmReq)
	if rmRec.Code != http.StatusNoContent {
		t.Fatalf("remove status: %d", rmRec.Code)
	}
	if len(repo.followed) != 0 {
		t.Fatalf("symbol not removed: %+v", repo.followed)
	}
}

func TestSeedFollowedSymbolsFromPositions(t *testing.T) {
	repo := newFakePortfolioRepo()
	positions := []broker.Position{
		{Symbol: "btc/usd"},
		{Symbol: "eth-usdc"},
	}
	if err := seedFollowedSymbolsFromPositions(context.Background(), repo, positions); err != nil {
		t.Fatal(err)
	}
	if !repo.seeded {
		t.Fatal("expected seed marker")
	}
	if _, ok := repo.followed["BTC-USD"]; !ok {
		t.Fatalf("missing BTC-USD: %+v", repo.followed)
	}
	if _, ok := repo.followed["ETH-USDC"]; !ok {
		t.Fatalf("missing ETH-USDC: %+v", repo.followed)
	}
}

func TestAlertSettingsHandlers(t *testing.T) {
	repo := newFakePortfolioRepo()
	app := &app{repo: repo}

	getReq := httptest.NewRequest(http.MethodGet, "/api/trading/alert-settings", nil)
	getRec := httptest.NewRecorder()
	app.handleAlertSettingsGet(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status: %d", getRec.Code)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/trading/alert-settings", bytes.NewBufferString(`{"moveThresholdPct":2.5,"cooldown":"30m"}`))
	putRec := httptest.NewRecorder()
	app.handleAlertSettingsUpdate(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status: %d", putRec.Code)
	}
	if repo.signalSetting == nil || repo.signalSetting.MoveThresholdPct != 2.5 || repo.signalSetting.Cooldown != "30m" {
		t.Fatalf("unexpected setting: %+v", repo.signalSetting)
	}
}

func TestRecentAlertsHandlers(t *testing.T) {
	repo := newFakePortfolioRepo()
	app := &app{repo: repo}

	createReq := httptest.NewRequest(http.MethodPost, "/internal/recent-alerts", bytes.NewBufferString(`{
		"type":"crypto_alert",
		"symbol":"BTC-USD",
		"productId":"BTC-USD",
		"source":"manual",
		"currentPrice":101,
		"deltaPct":1.25,
		"thresholdPct":1,
		"firedAt":"2020-01-01T00:00:00Z"
	}`))
	createRec := httptest.NewRecorder()
	app.handleInternalRecentAlertCreate(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status: %d", createRec.Code)
	}
	if len(repo.recentAlerts) != 1 || repo.recentAlerts[0].Symbol != "BTC-USD" {
		t.Fatalf("unexpected insert: %+v", repo.recentAlerts)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/trading/recent-alerts?limit=10", nil)
	listRec := httptest.NewRecorder()
	app.handleRecentAlertsList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status: %d", listRec.Code)
	}
	var resp portfolio.RecentAlertsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Alerts) != 1 || resp.Alerts[0].Symbol != "BTC-USD" {
		t.Fatalf("unexpected alerts: %+v", resp.Alerts)
	}
}
