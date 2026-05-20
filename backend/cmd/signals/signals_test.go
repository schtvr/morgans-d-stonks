package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func TestFetchSnapshot_doesNotLeakBodyInError(t *testing.T) {
	secret := "SECRET_BODY_SHOULD_NOT_APPEAR"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, secret, http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchSnapshot(context.Background(), srv.Client(), srv.URL, "key")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked body: %v", err)
	}
}

func TestRunOnce_cryptoAlertThreshold(t *testing.T) {
	var price atomic.Value
	price.Store(100.0)
	var recentAlertCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"symbols":[{"symbol":"BTC-USD","source":"manual","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-01-01T00:00:00Z"}]}`)
		case "/internal/signal-settings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"moveThresholdPct":1,"cooldown":"1m","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			cur := price.Load().(float64)
			body := map[string]any{
				"takenAt": "2020-01-01T00:00:00Z",
				"positions": []map[string]any{
					{"symbol": "BTC-USD", "quantity": 1, "avgCost": 90, "marketValue": cur, "unrealizedPL": cur - 90, "currency": "USD"},
				},
				"summary": map[string]any{},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case "/internal/recent-alerts":
			recentAlertCalls.Add(1)
			w.WriteHeader(http.StatusCreated)
		case "/api/v3/brokerage/market/products":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false}]}`)
		case "/v2/prices/BTC-USD/spot":
			w.Header().Set("Content-Type", "application/json")
			cur := price.Load().(float64)
			_, _ = io.WriteString(w, fmt.Sprintf(`{"data":{"amount":"%.2f"}}`, cur))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	statePath := filepath.Join(t.TempDir(), "state.json")
	state, err := sigpkg.NewAlertState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	dedup, err := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	client := coinbase.NewReadOnly(srv.Client(), srv.URL, "", "")
	opts := runOnceOpts{ruleDedup: dedup, agentMaxPerSymbol24h: 10}

	err = runOnce(context.Background(), log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "price_move_fired") {
		t.Fatalf("unexpected alert on baseline tick: %s", buf.String())
	}

	buf.Reset()
	price.Store(102.0)
	err = runOnce(context.Background(), log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "price_move_fired") {
		t.Fatalf("missing price_move_fired log: %s", out)
	}
	if !strings.Contains(out, "BTC-USD") {
		t.Fatalf("missing symbol context: %s", out)
	}
	if recentAlertCalls.Load() == 0 {
		t.Fatal("expected recent alert persistence call")
	}
}

// zeroCostRepo is a CostRepo stub that always returns 0 (no spend).
type zeroCostRepo struct{}

func (z *zeroCostRepo) SumCostCentsForDay(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestRunOnce_AgentEnqueue_MockProvider(t *testing.T) {
	var price atomic.Value
	price.Store(100.0)

	type capturedBody struct {
		Request  json.RawMessage `json:"request"`
		Decision json.RawMessage `json:"decision"`
	}
	var agentDecisionCh = make(chan capturedBody, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"symbols":[{"symbol":"BTC-USD","source":"manual","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-01-01T00:00:00Z"}]}`)
		case "/internal/signal-settings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"moveThresholdPct":1,"cooldown":"1m","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			cur := price.Load().(float64)
			body := map[string]any{
				"takenAt": "2020-01-01T00:00:00Z",
				"positions": []map[string]any{
					{"symbol": "BTC-USD", "quantity": 1, "avgCost": 90.0, "marketValue": cur, "unrealizedPL": cur - 90.0, "currency": "USD"},
				},
				"summary": map[string]any{"netLiquidation": cur, "totalCash": 0.0},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case "/internal/recent-alerts":
			w.WriteHeader(http.StatusCreated)
		case "/internal/agent-decisions/count":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"count":0}`)
		case "/internal/agent-decisions":
			var body capturedBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			select {
			case agentDecisionCh <- body:
			default:
			}
			w.WriteHeader(http.StatusCreated)
		case "/api/v3/brokerage/market/products":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false}]}`)
		case "/v2/prices/BTC-USD/spot":
			w.Header().Set("Content-Type", "application/json")
			cur := price.Load().(float64)
			_, _ = io.WriteString(w, fmt.Sprintf(`{"data":{"amount":"%.2f"}}`, cur))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	mockProvider := agent.NewMockProvider()
	costTracker := agent.NewCostTracker(&zeroCostRepo{}, 10_000)
	worker := agent.NewWorker(agent.WorkerConfig{
		Provider:        mockProvider,
		Concurrency:     1,
		CostTracker:     costTracker,
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "test-key",
		Log:             log,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker.Start(ctx)

	agentWorker = worker
	agentPromptVersion = "test-prompt-v1"
	t.Cleanup(func() {
		worker.Stop()
		agentWorker = nil
		agentPromptVersion = ""
		lastSnap.Store(nil)
	})

	statePath := filepath.Join(t.TempDir(), "state.json")
	state, err := sigpkg.NewAlertState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	dedup, err := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	if err != nil {
		t.Fatal(err)
	}
	coinbaseClient := coinbase.NewReadOnly(srv.Client(), srv.URL, "", "")
	opts := runOnceOpts{ruleDedup: dedup, agentMaxPerSymbol24h: 10}

	if err := runOnce(ctx, log, srv.Client(), coinbaseClient, state, srv.URL, "test-key", 1.0, time.Minute, opts); err != nil {
		t.Fatal(err)
	}
	select {
	case <-agentDecisionCh:
		t.Fatal("unexpected agent decision on baseline tick")
	case <-time.After(200 * time.Millisecond):
	}

	price.Store(102.0)
	if err := runOnce(ctx, log, srv.Client(), coinbaseClient, state, srv.URL, "test-key", 1.0, time.Minute, opts); err != nil {
		t.Fatal(err)
	}

	var captured capturedBody
	select {
	case captured = <-agentDecisionCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for agent decision POST")
	}

	var req struct {
		TriggerKind    string `json:"triggerKind"`
		IdempotencyKey string `json:"idempotencyKey"`
		PromptVersion  string `json:"promptVersion"`
		Signal         *struct {
			Symbol string `json:"symbol"`
		} `json:"signal"`
	}
	if err := json.Unmarshal(captured.Request, &req); err != nil {
		t.Fatalf("unmarshal request: %v (raw=%s)", err, captured.Request)
	}

	if req.TriggerKind != string(agent.TriggerSignal) {
		t.Errorf("triggerKind: want %q, got %q", agent.TriggerSignal, req.TriggerKind)
	}
	if req.Signal == nil || req.Signal.Symbol != "BTC-USD" {
		t.Errorf("signal.symbol: want BTC-USD, got %+v", req.Signal)
	}
	if req.PromptVersion != "test-prompt-v1" {
		t.Errorf("promptVersion: want test-prompt-v1, got %q", req.PromptVersion)
	}
	if !strings.Contains(req.IdempotencyKey, "btc_usd") {
		t.Errorf("idempotencyKey %q does not look like a BTC-USD alert ID", req.IdempotencyKey)
	}
}

func TestRunOnce_PortfolioRuleAgentEnqueue(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rulesYAML := `version: 1
rules:
  - id: drawdown-10pct
    name: "10% Drawdown"
    agent: true
    cooldown: "1m"
    condition:
      type: price_change_pct
      operator: lte
      threshold: -10.0
`
	if err := os.WriteFile(rulesPath, []byte(rulesYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	rules, err := sigpkg.LoadRulesFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}

	type capturedBody struct {
		Request json.RawMessage `json:"request"`
	}
	agentCh := make(chan capturedBody, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			_, _ = io.WriteString(w, `{"symbols":[]}`)
		case "/internal/signal-settings":
			_, _ = io.WriteString(w, `{"moveThresholdPct":99,"cooldown":"1h","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			body := map[string]any{
				"takenAt": "2020-01-01T00:00:00Z",
				"positions": []map[string]any{
					{"symbol": "ETH-USD", "quantity": 10, "avgCost": 100, "marketValue": 850, "unrealizedPL": -150, "currency": "USD"},
				},
				"summary": map[string]any{"netLiquidation": 850},
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/internal/recent-alerts":
			w.WriteHeader(http.StatusCreated)
		case "/internal/agent-decisions/count":
			_, _ = io.WriteString(w, `{"count":0}`)
		case "/internal/agent-decisions":
			var body capturedBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			agentCh <- body
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	worker := agent.NewWorker(agent.WorkerConfig{
		Provider:        agent.NewMockProvider(),
		Concurrency:     1,
		CostTracker:     agent.NewCostTracker(&zeroCostRepo{}, 10_000),
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "k",
		Log:             log,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	worker.Start(ctx)
	agentWorker = worker
	agentPromptVersion = "v-test"
	t.Cleanup(func() {
		worker.Stop()
		agentWorker = nil
	})

	state, _ := sigpkg.NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	dedup, _ := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	opts := runOnceOpts{
		rules:                  rules,
		ruleByID:               indexRules(rules),
		ruleDedup:              dedup,
		defaultRuleCooldown:    time.Hour,
		agentMaxPerSymbol24h:   10,
	}

	if err := runOnce(ctx, log, srv.Client(), coinbase.NewReadOnly(srv.Client(), srv.URL, "", ""), state, srv.URL, "k", 1, time.Hour, opts); err != nil {
		t.Fatal(err)
	}

	var captured capturedBody
	select {
	case captured = <-agentCh:
	case <-time.After(3 * time.Second):
		t.Fatal("expected agent decision from portfolio rule")
	}

	var req struct {
		Signal *struct {
			Type   string   `json:"type"`
			Symbol string   `json:"symbol"`
			Flags  []string `json:"reasonFlags"`
		} `json:"signal"`
	}
	if err := json.Unmarshal(captured.Request, &req); err != nil {
		t.Fatal(err)
	}
	if req.Signal == nil {
		t.Fatal("nil signal")
	}
	if req.Signal.Type != "portfolio_rule" {
		t.Errorf("type: got %q", req.Signal.Type)
	}
	if req.Signal.Symbol != "ETH-USD" {
		t.Errorf("symbol: got %q", req.Signal.Symbol)
	}
	if len(req.Signal.Flags) != 1 || req.Signal.Flags[0] != "drawdown-10pct" {
		t.Errorf("reasonFlags: %+v", req.Signal.Flags)
	}
}

func TestRunOnce_PortfolioRulesCoalescedAgent(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rulesYAML := `version: 1
rules:
  - id: drawdown-10pct
    agent: true
    cooldown: "1m"
    condition:
      type: price_change_pct
      operator: lte
      threshold: -10.0
  - id: concentration-25pct
    agent: true
    cooldown: "1m"
    condition:
      type: concentration
      operator: gte
      threshold: 25.0
`
	if err := os.WriteFile(rulesPath, []byte(rulesYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	rules, err := sigpkg.LoadRulesFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}

	agentCh := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			_, _ = io.WriteString(w, `{"symbols":[]}`)
		case "/internal/signal-settings":
			_, _ = io.WriteString(w, `{"moveThresholdPct":99,"cooldown":"1h","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"takenAt": "2020-01-01T00:00:00Z",
				"positions": []map[string]any{
					{"symbol": "ETH-USD", "quantity": 10, "avgCost": 100, "marketValue": 850, "unrealizedPL": -150, "currency": "USD"},
				},
				"summary": map[string]any{"netLiquidation": 850},
			})
		case "/internal/recent-alerts":
			w.WriteHeader(http.StatusCreated)
		case "/internal/agent-decisions/count":
			_, _ = io.WriteString(w, `{"count":0}`)
		case "/internal/agent-decisions":
			agentCh <- struct{}{}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	worker := agent.NewWorker(agent.WorkerConfig{
		Provider:        agent.NewMockProvider(),
		Concurrency:     1,
		CostTracker:     agent.NewCostTracker(&zeroCostRepo{}, 10_000),
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "k",
		Log:             log,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	worker.Start(ctx)
	agentWorker = worker
	agentPromptVersion = "v-test"
	t.Cleanup(func() {
		worker.Stop()
		agentWorker = nil
	})

	state, _ := sigpkg.NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	dedup, _ := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	opts := runOnceOpts{
		rules:                  rules,
		ruleByID:               indexRules(rules),
		ruleDedup:              dedup,
		defaultRuleCooldown:    time.Hour,
		agentMaxPerSymbol24h:   10,
	}
	if err := runOnce(ctx, log, srv.Client(), coinbase.NewReadOnly(srv.Client(), srv.URL, "", ""), state, srv.URL, "k", 1, time.Hour, opts); err != nil {
		t.Fatal(err)
	}

	select {
	case <-agentCh:
	case <-time.After(3 * time.Second):
		t.Fatal("expected coalesced agent enqueue")
	}
	select {
	case <-agentCh:
		t.Fatal("expected only one agent enqueue for two rules on same symbol")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRunOnce_PriceMoveSkipsAgentWhenNotHeld(t *testing.T) {
	var price atomic.Value
	price.Store(100.0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			_, _ = io.WriteString(w, `{"symbols":[{"symbol":"BTC-USD","source":"manual","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-01-01T00:00:00Z"}]}`)
		case "/internal/signal-settings":
			_, _ = io.WriteString(w, `{"moveThresholdPct":1,"cooldown":"1m","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			_, _ = io.WriteString(w, `{"takenAt":"2020-01-01T00:00:00Z","positions":[],"summary":{}}`)
		case "/internal/recent-alerts":
			w.WriteHeader(http.StatusCreated)
		case "/api/v3/brokerage/market/products":
			_, _ = io.WriteString(w, `{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false}]}`)
		case "/v2/prices/BTC-USD/spot":
			cur := price.Load().(float64)
			_, _ = io.WriteString(w, fmt.Sprintf(`{"data":{"amount":"%.2f"}}`, cur))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	state, _ := sigpkg.NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	dedup, _ := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	client := coinbase.NewReadOnly(srv.Client(), srv.URL, "", "")

	worker := agent.NewWorker(agent.WorkerConfig{
		Provider:        agent.NewMockProvider(),
		Concurrency:     1,
		CostTracker:     agent.NewCostTracker(&zeroCostRepo{}, 10_000),
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "k",
		Log:             log,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	worker.Start(ctx)
	agentWorker = worker
	t.Cleanup(func() {
		worker.Stop()
		agentWorker = nil
	})

	opts := runOnceOpts{ruleDedup: dedup, agentMaxPerSymbol24h: 10}
	_ = runOnce(ctx, log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)
	price.Store(102.0)
	_ = runOnce(ctx, log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)

	out := buf.String()
	if !strings.Contains(out, "price_move_fired") {
		t.Fatalf("expected price move: %s", out)
	}
	if strings.Contains(out, `"msg":"agent_enqueue"`) {
		t.Fatalf("agent should not enqueue without open position: %s", out)
	}
	if !strings.Contains(out, `"move_agent_skipped_not_held":1`) {
		t.Fatalf("expected skip metric in tick log: %s", out)
	}
}

func TestRunOnce_AgentCapSkipsEnqueue(t *testing.T) {
	var price atomic.Value
	price.Store(100.0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/followed-symbols":
			_, _ = io.WriteString(w, `{"symbols":[{"symbol":"BTC-USD","source":"manual","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-01-01T00:00:00Z"}]}`)
		case "/internal/signal-settings":
			_, _ = io.WriteString(w, `{"moveThresholdPct":1,"cooldown":"1m","updatedAt":"2020-01-01T00:00:00Z"}`)
		case "/internal/snapshot/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"takenAt": "2020-01-01T00:00:00Z",
				"positions": []map[string]any{
					{"symbol": "BTC-USD", "quantity": 1, "avgCost": 90, "marketValue": 102, "unrealizedPL": 12, "currency": "USD"},
				},
				"summary": map[string]any{"netLiquidation": 102},
			})
		case "/internal/recent-alerts":
			w.WriteHeader(http.StatusCreated)
		case "/internal/agent-decisions/count":
			_, _ = io.WriteString(w, `{"count":2}`)
		case "/api/v3/brokerage/market/products":
			_, _ = io.WriteString(w, `{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false}]}`)
		case "/v2/prices/BTC-USD/spot":
			cur := price.Load().(float64)
			_, _ = io.WriteString(w, fmt.Sprintf(`{"data":{"amount":"%.2f"}}`, cur))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	state, _ := sigpkg.NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	dedup, _ := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	client := coinbase.NewReadOnly(srv.Client(), srv.URL, "", "")

	worker := agent.NewWorker(agent.WorkerConfig{
		Provider:        agent.NewMockProvider(),
		Concurrency:     1,
		CostTracker:     agent.NewCostTracker(&zeroCostRepo{}, 10_000),
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "k",
		Log:             log,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	worker.Start(ctx)
	agentWorker = worker
	t.Cleanup(func() {
		worker.Stop()
		agentWorker = nil
	})

	opts := runOnceOpts{ruleDedup: dedup, agentMaxPerSymbol24h: 2}
	_ = runOnce(ctx, log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)
	price.Store(102.0)
	_ = runOnce(ctx, log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, opts)

	out := buf.String()
	if strings.Contains(out, `"msg":"agent_enqueue"`) {
		t.Fatalf("agent should be capped: %s", out)
	}
	if !strings.Contains(out, "symbol_decision_cap") {
		t.Fatalf("expected cap skip log: %s", out)
	}
}

func TestDailyTimer_EnqueuesOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	var received []agent.DecisionRequest
	enqueueFn := func(req agent.DecisionRequest) {
		mu.Lock()
		received = append(received, req)
		mu.Unlock()
		cancel()
	}

	fixedNow := time.Date(2026, 5, 18, 11, 59, 59, 900_000_000, time.UTC)
	nowFn := func() time.Time { return fixedNow }

	go runDailyTimer(ctx, slog.Default(), "12:00", enqueueFn, func() *portfolio.IngestSnapshotRequest { return nil }, "v1", nowFn)

	<-ctx.Done()

	mu.Lock()
	n := len(received)
	var req agent.DecisionRequest
	if n > 0 {
		req = received[0]
	}
	mu.Unlock()

	if n == 0 {
		t.Fatal("expected one daily trigger, got none")
	}
	if req.TriggerKind != agent.TriggerDaily {
		t.Errorf("triggerKind: want %q, got %q", agent.TriggerDaily, req.TriggerKind)
	}
	if !strings.HasPrefix(req.IdempotencyKey, "daily-") {
		t.Errorf("idempotencyKey %q should start with \"daily-\"", req.IdempotencyKey)
	}
	if req.Signal != nil {
		t.Errorf("daily trigger should have nil Signal, got %+v", req.Signal)
	}
}

func TestBuildEagerContext_Top3(t *testing.T) {
	snap := &portfolio.IngestSnapshotRequest{
		TakenAt: time.Now(),
		Positions: []broker.Position{
			{Symbol: "ETH-USD", MarketValue: 500, Quantity: 2},
			{Symbol: "BTC-USD", MarketValue: 10000, Quantity: 0.1},
			{Symbol: "SOL-USD", MarketValue: 2000, Quantity: 20},
			{Symbol: "DOGE-USD", MarketValue: 100, Quantity: 1000},
			{Symbol: "ADA-USD", MarketValue: 300, Quantity: 500},
		},
		Summary: broker.AccountSummary{NetLiquidation: 12900, TotalCash: 0},
	}
	count := 3
	ec := buildEagerContext(snap, "BTC-USD", &count)

	top := ec.PortfolioSummary.TopPositions
	if len(top) != 3 {
		t.Fatalf("expected 3 top positions, got %d: %+v", len(top), top)
	}
	expected := []string{"BTC-USD", "SOL-USD", "ETH-USD"}
	for i, sym := range expected {
		if top[i].Symbol != sym {
			t.Errorf("position[%d]: want %q, got %q", i, sym, top[i].Symbol)
		}
	}
	if snap.Positions[0].Symbol != "ETH-USD" {
		t.Errorf("snapshot was mutated: positions[0].Symbol = %q", snap.Positions[0].Symbol)
	}
	if ec.DecisionsForSymbol24h == nil || *ec.DecisionsForSymbol24h != 3 {
		t.Errorf("DecisionsForSymbol24h: want &3, got %v", ec.DecisionsForSymbol24h)
	}
}

func TestBuildEagerContext_Daily_NilCount(t *testing.T) {
	snap := &portfolio.IngestSnapshotRequest{
		TakenAt:   time.Now(),
		Positions: []broker.Position{{Symbol: "BTC-USD", MarketValue: 5000, Quantity: 0.05}},
		Summary:   broker.AccountSummary{NetLiquidation: 5000, TotalCash: 1000},
	}
	ec := buildEagerContext(snap, "", nil)
	if ec.DecisionsForSymbol24h != nil {
		t.Errorf("daily trigger: DecisionsForSymbol24h should be nil, got %v", ec.DecisionsForSymbol24h)
	}
}
