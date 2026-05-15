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
	followed         map[string]portfolio.FollowedSymbol
	seeded           bool
	snapshot         []byte
	snapshotSeries   []portfolio.SnapshotRecord
	signalSetting    *portfolio.SignalSettings
	recentAlerts     []portfolio.RecentAlert
	labSignals       []portfolio.LabSignalEvent
	labRuns          map[string]portfolio.LabOpenClawRun
	labNotes         []portfolio.LabNote
	control          portfolio.LabControlState
	settingsVersions []portfolio.SignalSettingsVersion
}

func newFakePortfolioRepo() *fakePortfolioRepo {
	return &fakePortfolioRepo{
		followed: map[string]portfolio.FollowedSymbol{},
		labRuns:  map[string]portfolio.LabOpenClawRun{},
		control:  portfolio.LabControlState{UpdatedAt: time.Unix(0, 0).UTC()},
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
func (f *fakePortfolioRepo) ListSnapshotsSince(_ context.Context, since time.Time, limit int) ([]portfolio.SnapshotRecord, error) {
	out := make([]portfolio.SnapshotRecord, 0, len(f.snapshotSeries))
	for _, rec := range f.snapshotSeries {
		if rec.TakenAt.Before(since) {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
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
func (f *fakePortfolioRepo) InsertLabSignalEvent(_ context.Context, alert portfolio.RecentAlert) (*portfolio.LabSignalEvent, error) {
	item := portfolio.LabSignalEvent{
		ID:            int64(len(f.labSignals) + 1),
		Type:          alert.Type,
		Symbol:        alert.Symbol,
		ProductID:     alert.ProductID,
		Source:        alert.Source,
		CurrentPrice:  alert.CurrentPrice,
		PreviousPrice: alert.PreviousPrice,
		DeltaAmount:   alert.DeltaAmount,
		DeltaPct:      alert.DeltaPct,
		ThresholdPct:  alert.ThresholdPct,
		FiredAt:       alert.FiredAt,
		PayloadJSON:   alert.PayloadJSON,
		DiscordStatus: "signal_only",
		CreatedAt:     time.Now().UTC(),
	}
	f.labSignals = append(f.labSignals, item)
	return &item, nil
}
func (f *fakePortfolioRepo) ListLabSignalEvents(_ context.Context, filter portfolio.LabSignalFilter) ([]portfolio.LabSignalEvent, error) {
	out := make([]portfolio.LabSignalEvent, len(f.labSignals))
	copy(out, f.labSignals)
	return out, nil
}
func (f *fakePortfolioRepo) GetLabSignalEvent(_ context.Context, id int64) (*portfolio.LabSignalEvent, error) {
	for _, item := range f.labSignals {
		if item.ID == id {
			return &item, nil
		}
	}
	return nil, pgx.ErrNoRows
}
func (f *fakePortfolioRepo) UpsertLabOpenClawRun(_ context.Context, run portfolio.LabOpenClawRun) error {
	if run.Attempts == 0 {
		run.Attempts = 1
	}
	run.UpdatedAt = time.Now().UTC()
	f.labRuns[run.RequestID] = run
	return nil
}
func (f *fakePortfolioRepo) ListLabOpenClawRuns(_ context.Context, filter portfolio.LabRunFilter) ([]portfolio.LabOpenClawRun, error) {
	out := make([]portfolio.LabOpenClawRun, 0, len(f.labRuns))
	for _, run := range f.labRuns {
		out = append(out, run)
	}
	return out, nil
}
func (f *fakePortfolioRepo) GetLabOpenClawRun(_ context.Context, requestID string) (*portfolio.LabOpenClawRun, error) {
	run, ok := f.labRuns[requestID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return &run, nil
}
func (f *fakePortfolioRepo) InsertLabNote(_ context.Context, req portfolio.LabNoteRequest) (*portfolio.LabNote, error) {
	note := portfolio.LabNote{ID: int64(len(f.labNotes) + 1), SignalID: req.SignalID, RequestID: req.RequestID, Body: req.Body, CreatedAt: time.Now().UTC()}
	f.labNotes = append(f.labNotes, note)
	return &note, nil
}
func (f *fakePortfolioRepo) ListLabTelemetry(context.Context, string, string) ([]portfolio.LabTelemetryPoint, error) {
	return nil, nil
}
func (f *fakePortfolioRepo) GetLabControlState(context.Context) (*portfolio.LabControlState, error) {
	return &f.control, nil
}
func (f *fakePortfolioRepo) UpdateLabControlState(_ context.Context, control portfolio.LabControlState) (*portfolio.LabControlState, error) {
	control.UpdatedAt = time.Now().UTC()
	f.control = control
	return &f.control, nil
}
func (f *fakePortfolioRepo) InsertSignalSettingsVersion(_ context.Context, req portfolio.SignalSettingsRequest, reason string) (*portfolio.SignalSettingsVersion, error) {
	version := portfolio.SignalSettingsVersion{ID: int64(len(f.settingsVersions) + 1), MoveThresholdPct: req.MoveThresholdPct, Cooldown: req.Cooldown, Reason: reason, CreatedAt: time.Now().UTC()}
	f.settingsVersions = append(f.settingsVersions, version)
	return &version, nil
}
func (f *fakePortfolioRepo) ListSignalSettingsVersions(context.Context, int) ([]portfolio.SignalSettingsVersion, error) {
	out := make([]portfolio.SignalSettingsVersion, len(f.settingsVersions))
	copy(out, f.settingsVersions)
	return out, nil
}
func (f *fakePortfolioRepo) RevertSignalSettings(_ context.Context, versionID int64) (*portfolio.SignalSettings, error) {
	for _, version := range f.settingsVersions {
		if version.ID == versionID {
			f.signalSetting = &portfolio.SignalSettings{MoveThresholdPct: version.MoveThresholdPct, Cooldown: version.Cooldown, UpdatedAt: time.Now().UTC()}
			return f.signalSetting, nil
		}
	}
	return nil, pgx.ErrNoRows
}
func (f *fakePortfolioRepo) CompactLabOpenClawPayloads(context.Context, time.Time) error {
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
		"schemaVersion":"crypto_signal_v1",
		"id":"test-id",
		"type":"crypto_price_move",
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
	if len(repo.labSignals) != 1 || repo.labSignals[0].Symbol != "BTC-USD" {
		t.Fatalf("expected lab signal insert: %+v", repo.labSignals)
	}
	if _, ok := repo.labRuns["lab-signal-1-attempt-1"]; !ok {
		t.Fatalf("expected queued lab run: %+v", repo.labRuns)
	}
	if len(repo.recentAlerts[0].PayloadJSON) == 0 || !bytes.Contains(repo.recentAlerts[0].PayloadJSON, []byte(`"schemaVersion":"crypto_signal_v1"`)) {
		t.Fatalf("expected payload_json: %s", repo.recentAlerts[0].PayloadJSON)
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

func TestLabOperationHandlers(t *testing.T) {
	repo := newFakePortfolioRepo()
	app := &app{repo: repo}

	pauseReq := httptest.NewRequest(http.MethodPost, "/api/lab/openclaw/pause", nil)
	pauseRec := httptest.NewRecorder()
	app.handleLabOpenClawPause(pauseRec, pauseReq)
	if pauseRec.Code != http.StatusOK {
		t.Fatalf("pause status: %d", pauseRec.Code)
	}
	if !repo.control.OpenClawPaused {
		t.Fatal("expected OpenClaw paused")
	}

	resumeReq := httptest.NewRequest(http.MethodPost, "/api/lab/openclaw/resume", nil)
	resumeRec := httptest.NewRecorder()
	app.handleLabOpenClawResume(resumeRec, resumeReq)
	if resumeRec.Code != http.StatusOK {
		t.Fatalf("resume status: %d", resumeRec.Code)
	}
	if repo.control.OpenClawPaused {
		t.Fatal("expected OpenClaw resumed")
	}
}
