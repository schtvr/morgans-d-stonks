package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	agentpkg "github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

// ── agent methods on fakePortfolioRepo ───────────────────────────────────────

func (f *fakePortfolioRepo) InsertAgentDecision(_ context.Context, d portfolio.AgentDecision) (*portfolio.AgentDecision, error) {
	d.ID = int64(len(f.agentDecisions) + 1)
	d.CreatedAt = time.Now().UTC()
	f.agentDecisions = append(f.agentDecisions, d)
	return &f.agentDecisions[len(f.agentDecisions)-1], nil
}

func (f *fakePortfolioRepo) GetAgentDecision(_ context.Context, id int64) (*portfolio.AgentDecision, error) {
	for _, d := range f.agentDecisions {
		if d.ID == id {
			return &d, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (f *fakePortfolioRepo) GetAgentDecisionByIdempotencyKey(_ context.Context, key string) (*portfolio.AgentDecision, error) {
	for _, d := range f.agentDecisions {
		if d.IdempotencyKey == key {
			return &d, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (f *fakePortfolioRepo) ListAgentDecisions(_ context.Context, filter portfolio.AgentDecisionFilter) ([]portfolio.AgentDecision, error) {
	out := []portfolio.AgentDecision{}
	for _, d := range f.agentDecisions {
		if filter.Symbol != "" && d.Symbol != filter.Symbol {
			continue
		}
		if filter.Action != "" && d.Action != filter.Action {
			continue
		}
		if filter.From != nil && d.TriggerAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && d.TriggerAt.After(*filter.To) {
			continue
		}
		out = append(out, d)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakePortfolioRepo) CountDecisionsForSymbolSince(_ context.Context, symbol string, since time.Time) (int, error) {
	n := 0
	for _, d := range f.agentDecisions {
		if d.Symbol == symbol && !d.TriggerAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (f *fakePortfolioRepo) SumCostCentsForDay(_ context.Context, day time.Time) (int64, error) {
	dayStr := day.Format("2006-01-02")
	var sum int64
	for _, d := range f.agentDecisions {
		if d.CreatedAt.Format("2006-01-02") == dayStr {
			sum += d.CostCents
		}
	}
	return sum, nil
}

func (f *fakePortfolioRepo) ListAgentCostDaily(_ context.Context, days int) ([]portfolio.AgentCostPoint, error) {
	return f.agentCostPoints, nil
}

func (f *fakePortfolioRepo) InsertAgentDecisionOutcome(_ context.Context, o portfolio.AgentDecisionOutcome) (*portfolio.AgentDecisionOutcome, error) {
	o.ID = int64(len(f.agentOutcomes) + 1)
	o.ScoredAt = time.Now().UTC()
	f.agentOutcomes = append(f.agentOutcomes, o)
	return &f.agentOutcomes[len(f.agentOutcomes)-1], nil
}

func (f *fakePortfolioRepo) ListUnscoredDecisionHorizons(_ context.Context, _ time.Time, limit int) ([]portfolio.UnscoredHorizon, error) {
	return nil, nil
}

func (f *fakePortfolioRepo) ListAgentDecisionOutcomes(_ context.Context, filter portfolio.AgentDecisionOutcomeFilter) ([]portfolio.AgentDecisionOutcome, error) {
	out := []portfolio.AgentDecisionOutcome{}
	for _, o := range f.agentOutcomes {
		if filter.Horizon != "" && o.Horizon != filter.Horizon {
			continue
		}
		if len(filter.DecisionIDs) > 0 {
			found := false
			for _, id := range filter.DecisionIDs {
				if o.DecisionID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *fakePortfolioRepo) ListBenchmarkDaily(_ context.Context, horizon string, days int) ([]portfolio.AgentBenchmarkPoint, error) {
	return f.benchmarkPoints, nil
}

// Add agent fields to fakePortfolioRepo (extends the struct declared in followed_handlers_test.go).
// Go allows adding fields by embedding; instead we use a separate type that satisfies the interface.
// Since fakePortfolioRepo is already declared in followed_handlers_test.go with the base fields, we
// cannot redeclare it here. We patch the struct via a Go build-tag-free approach: the agent fields
// are added to fakePortfolioRepo by this file extending the methods on the existing struct.
// The struct fields (agentDecisions, etc.) are declared in fakeAgentFields below and embedded.
// ──────────────────────────────────────────────────────────────────────────────────────────────────
// Actually, since all test files share the same package and the struct is declared in
// followed_handlers_test.go, we cannot add new fields here. We use a separate fake that wraps it.

// agentTestRepo is a self-contained fake for agent-handler tests, implementing portfolio.Repository.
type agentTestRepo struct {
	agentDecisions  []portfolio.AgentDecision
	agentOutcomes   []portfolio.AgentDecisionOutcome
	agentCostPoints []portfolio.AgentCostPoint
	benchmarkPoints []portfolio.AgentBenchmarkPoint
}

var _ portfolio.Repository = (*agentTestRepo)(nil)

// ── Repository stubs (not used in agent tests) ────────────────────────────────

func (f *agentTestRepo) RunMigrations(context.Context) error                     { return nil }
func (f *agentTestRepo) UpsertSnapshot(context.Context, time.Time, []byte) error { return nil }
func (f *agentTestRepo) LatestSnapshot(context.Context) (time.Time, []byte, error) {
	return time.Time{}, nil, pgx.ErrNoRows
}
func (f *agentTestRepo) ListSnapshotsSince(_ context.Context, _ time.Time, _ int) ([]portfolio.SnapshotRecord, error) {
	return nil, nil
}
func (f *agentTestRepo) ListFollowedSymbols(context.Context) ([]portfolio.FollowedSymbol, error) {
	return nil, nil
}
func (f *agentTestRepo) UpsertFollowedSymbol(context.Context, string, string) error { return nil }
func (f *agentTestRepo) RemoveFollowedSymbol(context.Context, string) error         { return nil }
func (f *agentTestRepo) FollowedSymbolsSeeded(context.Context) (bool, error)        { return false, nil }
func (f *agentTestRepo) MarkFollowedSymbolsSeeded(context.Context, time.Time) error { return nil }
func (f *agentTestRepo) GetSignalSettings(context.Context) (*portfolio.SignalSettings, error) {
	return &portfolio.SignalSettings{}, nil
}
func (f *agentTestRepo) UpdateSignalSettings(context.Context, portfolio.SignalSettingsRequest) error {
	return nil
}
func (f *agentTestRepo) ListRecentAlertsFiltered(_ context.Context, _ string, _ time.Time, _ int) ([]portfolio.RecentAlert, error) {
	return nil, nil
}
func (f *agentTestRepo) ListRecentAlerts(context.Context, int) ([]portfolio.RecentAlert, error) {
	return nil, nil
}
func (f *agentTestRepo) InsertRecentAlert(context.Context, portfolio.RecentAlert) error { return nil }
func (f *agentTestRepo) InsertLabSignalEvent(context.Context, portfolio.RecentAlert) (*portfolio.LabSignalEvent, error) {
	return nil, nil
}
func (f *agentTestRepo) ListLabSignalEvents(context.Context, portfolio.LabSignalFilter) ([]portfolio.LabSignalEvent, error) {
	return nil, nil
}
func (f *agentTestRepo) GetLabSignalEvent(context.Context, int64) (*portfolio.LabSignalEvent, error) {
	return nil, pgx.ErrNoRows
}
func (f *agentTestRepo) UpsertLabOpenClawRun(context.Context, portfolio.LabOpenClawRun) error {
	return nil
}
func (f *agentTestRepo) ListLabOpenClawRuns(context.Context, portfolio.LabRunFilter) ([]portfolio.LabOpenClawRun, error) {
	return nil, nil
}
func (f *agentTestRepo) GetLabOpenClawRun(context.Context, string) (*portfolio.LabOpenClawRun, error) {
	return nil, pgx.ErrNoRows
}
func (f *agentTestRepo) InsertLabNote(context.Context, portfolio.LabNoteRequest) (*portfolio.LabNote, error) {
	return nil, nil
}
func (f *agentTestRepo) ListLabTelemetry(context.Context, string, string) ([]portfolio.LabTelemetryPoint, error) {
	return nil, nil
}
func (f *agentTestRepo) GetLabControlState(context.Context) (*portfolio.LabControlState, error) {
	return &portfolio.LabControlState{UpdatedAt: time.Unix(0, 0)}, nil
}
func (f *agentTestRepo) UpdateLabControlState(_ context.Context, c portfolio.LabControlState) (*portfolio.LabControlState, error) {
	return &c, nil
}
func (f *agentTestRepo) InsertSignalSettingsVersion(context.Context, portfolio.SignalSettingsRequest, string) (*portfolio.SignalSettingsVersion, error) {
	return nil, nil
}
func (f *agentTestRepo) ListSignalSettingsVersions(context.Context, int) ([]portfolio.SignalSettingsVersion, error) {
	return nil, nil
}
func (f *agentTestRepo) RevertSignalSettings(context.Context, int64) (*portfolio.SignalSettings, error) {
	return nil, pgx.ErrNoRows
}
func (f *agentTestRepo) CompactLabOpenClawPayloads(context.Context, time.Time) error { return nil }
func (f *agentTestRepo) CreateSession(context.Context, string, string, time.Time) error {
	return nil
}
func (f *agentTestRepo) SessionUser(context.Context, string) (string, error) {
	return "", pgx.ErrNoRows
}
func (f *agentTestRepo) DeleteSession(context.Context, string) error { return nil }

// ── Agent methods ─────────────────────────────────────────────────────────────

func (f *agentTestRepo) InsertAgentDecision(_ context.Context, d portfolio.AgentDecision) (*portfolio.AgentDecision, error) {
	d.ID = int64(len(f.agentDecisions) + 1)
	d.CreatedAt = time.Now().UTC()
	f.agentDecisions = append(f.agentDecisions, d)
	return &f.agentDecisions[len(f.agentDecisions)-1], nil
}

func (f *agentTestRepo) GetAgentDecision(_ context.Context, id int64) (*portfolio.AgentDecision, error) {
	for _, d := range f.agentDecisions {
		if d.ID == id {
			return &d, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (f *agentTestRepo) GetAgentDecisionByIdempotencyKey(_ context.Context, key string) (*portfolio.AgentDecision, error) {
	for _, d := range f.agentDecisions {
		if d.IdempotencyKey == key {
			return &d, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (f *agentTestRepo) ListAgentDecisions(_ context.Context, filter portfolio.AgentDecisionFilter) ([]portfolio.AgentDecision, error) {
	out := []portfolio.AgentDecision{}
	for _, d := range f.agentDecisions {
		if filter.Symbol != "" && d.Symbol != filter.Symbol {
			continue
		}
		if filter.Action != "" && d.Action != filter.Action {
			continue
		}
		if filter.From != nil && d.TriggerAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && d.TriggerAt.After(*filter.To) {
			continue
		}
		out = append(out, d)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *agentTestRepo) CountDecisionsForSymbolSince(_ context.Context, symbol string, since time.Time) (int, error) {
	n := 0
	for _, d := range f.agentDecisions {
		if d.Symbol == symbol && !d.TriggerAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (f *agentTestRepo) SumCostCentsForDay(_ context.Context, day time.Time) (int64, error) {
	dayStr := day.Format("2006-01-02")
	var sum int64
	for _, d := range f.agentDecisions {
		if d.CreatedAt.Format("2006-01-02") == dayStr {
			sum += d.CostCents
		}
	}
	return sum, nil
}

func (f *agentTestRepo) ListAgentCostDaily(_ context.Context, _ int) ([]portfolio.AgentCostPoint, error) {
	return f.agentCostPoints, nil
}

func (f *agentTestRepo) InsertAgentDecisionOutcome(_ context.Context, o portfolio.AgentDecisionOutcome) (*portfolio.AgentDecisionOutcome, error) {
	o.ID = int64(len(f.agentOutcomes) + 1)
	o.ScoredAt = time.Now().UTC()
	f.agentOutcomes = append(f.agentOutcomes, o)
	return &f.agentOutcomes[len(f.agentOutcomes)-1], nil
}

func (f *agentTestRepo) ListUnscoredDecisionHorizons(_ context.Context, _ time.Time, _ int) ([]portfolio.UnscoredHorizon, error) {
	return nil, nil
}

func (f *agentTestRepo) ListAgentDecisionOutcomes(_ context.Context, filter portfolio.AgentDecisionOutcomeFilter) ([]portfolio.AgentDecisionOutcome, error) {
	out := []portfolio.AgentDecisionOutcome{}
	for _, o := range f.agentOutcomes {
		if filter.Horizon != "" && o.Horizon != filter.Horizon {
			continue
		}
		if len(filter.DecisionIDs) > 0 {
			found := false
			for _, id := range filter.DecisionIDs {
				if o.DecisionID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *agentTestRepo) ListBenchmarkDaily(_ context.Context, _ string, _ int) ([]portfolio.AgentBenchmarkPoint, error) {
	return f.benchmarkPoints, nil
}

// ── test helpers ──────────────────────────────────────────────────────────────

func buildDecisionCreateBody(t *testing.T, ikey, symbol string) []byte {
	t.Helper()
	sig := (*signal.CryptoAlert)(nil)
	if symbol != "" {
		sig = &signal.CryptoAlert{Symbol: symbol}
	}
	body := agentDecisionCreateRequest{
		Request: agentpkg.DecisionRequest{
			TriggerKind:    agentpkg.TriggerSignal,
			TriggerAt:      time.Now().UTC(),
			IdempotencyKey: ikey,
			Signal:         sig,
			PromptVersion:  "v1",
		},
		Decision: agentpkg.Decision{
			Action:     agentpkg.ActionBuy,
			Confidence: 0.85,
			Rationale:  "test rationale",
			Model:      "claude-test",
			LatencyMS:  250,
			CostCents:  3,
			ToolCalls:  []agentpkg.ToolCall{{Name: "get_positions", DurationMS: 10}},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestInternalAgentDecisionCreate_new(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	body := buildDecisionCreateBody(t, "ikey-001", "BTC-USD")
	req := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentDecision
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != 1 {
		t.Fatalf("expected ID 1, got %d", resp.ID)
	}
	if resp.Symbol != "BTC-USD" {
		t.Fatalf("expected BTC-USD, got %s", resp.Symbol)
	}
	if resp.Action != "buy" {
		t.Fatalf("expected buy, got %s", resp.Action)
	}
}

func TestInternalAgentDecisionCreate_idempotent(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	body := buildDecisionCreateBody(t, "ikey-dup", "ETH-USD")

	// First call → 201.
	req1 := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	rec1 := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first call: status %d", rec1.Code)
	}

	// Second call with same key → 200 with same row.
	req2 := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second call: status %d body %s", rec2.Code, rec2.Body.String())
	}

	// Still only one row.
	if len(repo.agentDecisions) != 1 {
		t.Fatalf("expected 1 row, got %d", len(repo.agentDecisions))
	}
}

func TestInternalAgentDecisionsList_filterBySymbol(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	// Insert two decisions for different symbols.
	for _, sym := range []string{"BTC-USD", "ETH-USD"} {
		body := buildDecisionCreateBody(t, fmt.Sprintf("ikey-%s", sym), sym)
		req := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		a.handleInternalAgentDecisionCreate(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("insert %s: %d", sym, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/agent-decisions/list?symbol=BTC-USD&since=24h", nil)
	rec := httptest.NewRecorder()
	a.handleInternalAgentDecisionsList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentDecisionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Decisions) != 1 || resp.Decisions[0].Symbol != "BTC-USD" {
		t.Fatalf("expected 1 BTC-USD decision, got %+v", resp.Decisions)
	}
	// Tool calls JSON should be stripped.
	if resp.Decisions[0].ToolCallsJSON != nil {
		t.Fatal("tool_calls_json should be nil in list response")
	}
}

func TestInternalAgentDecisionsCount(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	// Insert one BTC-USD decision.
	body := buildDecisionCreateBody(t, "count-key", "BTC-USD")
	req := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("insert: %d", rec.Code)
	}

	since := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	req2 := httptest.NewRequest(http.MethodGet, "/internal/agent-decisions/count?symbol=BTC-USD&since="+since, nil)
	rec2 := httptest.NewRecorder()
	a.handleInternalAgentDecisionsCount(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec2.Code, rec2.Body.String())
	}
	var resp map[string]int
	if err := json.NewDecoder(rec2.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["count"] != 1 {
		t.Fatalf("expected count 1, got %d", resp["count"])
	}
}

func TestInternalAgentDecisionOutcomes(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	// Pre-seed an outcome.
	excess := 0.05
	repo.agentOutcomes = []portfolio.AgentDecisionOutcome{
		{ID: 1, DecisionID: 1, Horizon: "14d", BTCReturnPct: 0.1, ExcessReturnPct: &excess, ScoredAt: time.Now().UTC()},
	}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/internal/agent-decisions/outcomes?horizon=14d", nil)
	rec := httptest.NewRecorder()
	a.handleInternalAgentDecisionOutcomes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	outcomes, ok := resp["outcomes"].([]any)
	if !ok || len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %v", resp)
	}
}

func TestInternalAgentCostToday(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	// Pre-seed a decision with cost today.
	repo.agentDecisions = []portfolio.AgentDecision{
		{ID: 1, CostCents: 42, CreatedAt: time.Now().UTC()},
	}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/internal/agent-cost/today", nil)
	rec := httptest.NewRecorder()
	a.handleInternalAgentCostToday(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["day"] == "" {
		t.Fatal("expected day field")
	}
	// costCents comes back as float64 in JSON.
	if v, ok := resp["costCents"].(float64); !ok || v != 42 {
		t.Fatalf("expected costCents=42, got %v", resp["costCents"])
	}
}

func TestAgentDecisionsList_session(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	// Insert a decision with blobs.
	body := buildDecisionCreateBody(t, "session-list-key", "BTC-USD")
	createReq := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("insert: %d", createRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/decisions", nil)
	rec := httptest.NewRecorder()
	a.handleAgentDecisionsList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentDecisionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(resp.Decisions))
	}
	d := resp.Decisions[0]
	if d.RequestJSON != nil || d.ResponseJSON != nil || d.ToolCallsJSON != nil {
		t.Fatal("blobs should be nil in dashboard list response")
	}
}

func TestAgentDecisionGet_session(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	body := buildDecisionCreateBody(t, "get-key", "BTC-USD")
	createReq := httptest.NewRequest(http.MethodPost, "/internal/agent-decisions", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	a.handleInternalAgentDecisionCreate(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("insert: %d", createRec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/agent/decisions/1", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "1")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	rec := httptest.NewRecorder()
	a.handleAgentDecisionGet(rec, req2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentDecision
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != 1 {
		t.Fatalf("expected ID 1, got %d", resp.ID)
	}
	// Full row includes blobs.
	if resp.RequestJSON == nil {
		t.Fatal("expected RequestJSON in full get response")
	}
}

func TestAgentDecisionGet_notFound(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/decisions/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	a.handleAgentDecisionGet(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAgentBenchmark_withData(t *testing.T) {
	t.Parallel()
	excess1, excess2 := 0.05, 0.03
	repo := &agentTestRepo{
		benchmarkPoints: []portfolio.AgentBenchmarkPoint{
			{AsOf: time.Now().UTC().Add(-13 * 24 * time.Hour), ExcessReturnPct: excess1, DecisionCount: 2},
			{AsOf: time.Now().UTC().Add(-12 * 24 * time.Hour), ExcessReturnPct: excess2, DecisionCount: 1},
		},
	}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/benchmark?window=14d", nil)
	rec := httptest.NewRecorder()
	a.handleAgentBenchmark(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentBenchmarkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Window != "14d" {
		t.Fatalf("expected window 14d, got %s", resp.Window)
	}
	if !resp.NoteShadowFeesNotPaid {
		t.Fatal("expected NoteShadowFeesNotPaid=true")
	}
	if !resp.NoteWindowIncomplete {
		t.Fatal("expected NoteWindowIncomplete=true (only 2 of 14 days)")
	}
	want := (excess1 + excess2) / 2
	if resp.HeadlineExcessPct != want {
		t.Fatalf("expected headline %.4f, got %.4f", want, resp.HeadlineExcessPct)
	}
}

func TestAgentBenchmark_windowIncomplete(t *testing.T) {
	t.Parallel()
	// Fewer points than requested days → NoteWindowIncomplete.
	repo := &agentTestRepo{
		benchmarkPoints: []portfolio.AgentBenchmarkPoint{
			{AsOf: time.Now().UTC(), DecisionCount: 1},
		},
	}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/benchmark?window=14d", nil)
	rec := httptest.NewRecorder()
	a.handleAgentBenchmark(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var resp portfolio.AgentBenchmarkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.NoteWindowIncomplete {
		t.Fatal("expected NoteWindowIncomplete=true")
	}
}

func TestAgentCost(t *testing.T) {
	t.Parallel()
	repo := &agentTestRepo{
		agentCostPoints: []portfolio.AgentCostPoint{
			{Day: time.Now().UTC().Format("2006-01-02"), CostCents: 25, Decisions: 5},
		},
		agentDecisions: []portfolio.AgentDecision{
			{ID: 1, CostCents: 25, CreatedAt: time.Now().UTC()},
		},
	}
	a := &app{repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/cost?window=7d", nil)
	rec := httptest.NewRecorder()
	a.handleAgentCost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp portfolio.AgentCostResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Window != "7d" {
		t.Fatalf("expected window 7d, got %s", resp.Window)
	}
	if resp.Today.CostCents != 25 {
		t.Fatalf("expected today.costCents=25, got %d", resp.Today.CostCents)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("expected 1 cost point, got %d", len(resp.Points))
	}
}
