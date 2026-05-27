package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// captureRequest records the last HTTP request received by a test server.
type captureRequest struct {
	called  bool
	method  string
	path    string
	body    []byte
	headers http.Header
}

func newTradeServer(t *testing.T, cap *captureRequest, statusCode int, respBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.called = true
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.headers = r.Header.Clone()
		cap.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(respBody))
	}))
}

func signalReq(symbol string) DecisionRequest {
	return DecisionRequest{
		TriggerKind:    TriggerSignal,
		IdempotencyKey: "test-key-001",
		Signal: &sigpkg.CryptoAlert{
			Symbol: symbol,
		},
	}
}

func dailyReq() DecisionRequest {
	return DecisionRequest{
		TriggerKind:    TriggerDaily,
		IdempotencyKey: "daily-2026-05-22",
	}
}

func buyDecision(confidence float64, hint *float64) *Decision {
	return &Decision{
		Action:             ActionBuy,
		Confidence:         confidence,
		Rationale:          "test",
		SizingHintNotional: hint,
		ToolCalls:          []ToolCall{},
	}
}

func ptr(f float64) *float64 { return &f }

func acceptedResp() string {
	return `{"order":{"id":"ord-1","status":"accepted"},"decision":{"allowed":true,"reasonCodes":[]}}`
}

func rejectedResp() string {
	return `{"order":{"id":"ord-2","status":"rejected","reason":"kill_switch"},"decision":{"allowed":false,"reasonCodes":["kill_switch"]}}`
}

func workerWith(cfg WorkerConfig) *Worker {
	w := NewWorker(cfg)
	return w
}

// TestMaybeSubmitOrder_TradeDisabled: no HTTP call when TradeEnabled=false.
func TestMaybeSubmitOrder_TradeDisabled(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL: srv.URL,
		TradeEnabled:    false,
		Log:             testLogger(),
	})
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), buyDecision(0.9, ptr(100)))

	if cap.called {
		t.Error("expected no HTTP call when TradeEnabled=false")
	}
}

// TestMaybeSubmitOrder_ActionIgnore: no HTTP call for ignore decisions.
func TestMaybeSubmitOrder_ActionIgnore(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL: srv.URL,
		TradeEnabled:    true,
		Log:             testLogger(),
	})
	d := &Decision{Action: ActionIgnore, Confidence: 0.3, ToolCalls: []ToolCall{}}
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), d)

	if cap.called {
		t.Error("expected no HTTP call for ActionIgnore")
	}
}

// TestMaybeSubmitOrder_DailyTrigger: no HTTP call for daily triggers (no symbol).
func TestMaybeSubmitOrder_DailyTrigger(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL: srv.URL,
		TradeEnabled:    true,
		Log:             testLogger(),
	})
	w.maybeSubmitOrder(context.Background(), dailyReq(), buyDecision(0.9, ptr(100)))

	if cap.called {
		t.Error("expected no HTTP call for daily trigger")
	}
}

// TestMaybeSubmitOrder_LowConfidence: no HTTP call when confidence below threshold.
func TestMaybeSubmitOrder_LowConfidence(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:    srv.URL,
		TradeEnabled:       true,
		MinTradeConfidence: 0.70,
		Log:                testLogger(),
	})
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), buyDecision(0.65, ptr(100)))

	if cap.called {
		t.Error("expected no HTTP call when confidence < MinTradeConfidence")
	}
}

// TestMaybeSubmitOrder_NoNotional: no HTTP call when notional hint absent and no default.
func TestMaybeSubmitOrder_NoNotional(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:      srv.URL,
		TradeEnabled:         true,
		MinTradeConfidence:   0.70,
		DefaultTradeNotional: 0,
		Log:                  testLogger(),
	})
	d := &Decision{Action: ActionBuy, Confidence: 0.9, ToolCalls: []ToolCall{}} // no hint
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), d)

	if cap.called {
		t.Error("expected no HTTP call when notional=0 and no default")
	}
}

// TestMaybeSubmitOrder_BuyWithHint: posts correct MCP envelope for a buy.
func TestMaybeSubmitOrder_BuyWithHint(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:    srv.URL,
		InternalAPIKey:     "test-internal-key",
		TradeEnabled:       true,
		MinTradeConfidence: 0.70,
		Log:                testLogger(),
	})
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), buyDecision(0.85, ptr(150.0)))

	if !cap.called {
		t.Fatal("expected HTTP call to trade endpoint")
	}
	if cap.path != "/mcp/v1/trades/create" {
		t.Errorf("expected path /mcp/v1/trades/create, got %q", cap.path)
	}
	if cap.headers.Get("X-Internal-Key") != "test-internal-key" {
		t.Errorf("expected X-Internal-Key header, got %q", cap.headers.Get("X-Internal-Key"))
	}

	var req mcpTradeRequest
	if err := json.Unmarshal(cap.body, &req); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if req.SchemaVersion != "v1" {
		t.Errorf("schema_version: got %q, want v1", req.SchemaVersion)
	}
	if req.IdempotencyKey != "agent-test-key-001" {
		t.Errorf("idempotency_key: got %q, want agent-test-key-001", req.IdempotencyKey)
	}
	if req.Order.ProductID != "BTC-USD" {
		t.Errorf("product_id: got %q, want BTC-USD", req.Order.ProductID)
	}
	if req.Order.Side != "buy" {
		t.Errorf("side: got %q, want buy", req.Order.Side)
	}
	if req.Order.Type != "market" {
		t.Errorf("type: got %q, want market", req.Order.Type)
	}
	if req.Order.QuoteSize != 150.0 {
		t.Errorf("quote_size: got %v, want 150.0", req.Order.QuoteSize)
	}
}

// TestMaybeSubmitOrder_SellWithDefault: uses DefaultTradeNotional when hint absent.
func TestMaybeSubmitOrder_SellWithDefault(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:      srv.URL,
		TradeEnabled:         true,
		MinTradeConfidence:   0.70,
		DefaultTradeNotional: 50.0,
		Log:                  testLogger(),
	})
	d := &Decision{Action: ActionSell, Confidence: 0.80, ToolCalls: []ToolCall{}} // no hint
	w.maybeSubmitOrder(context.Background(), signalReq("ETH-USD"), d)

	if !cap.called {
		t.Fatal("expected HTTP call using default notional")
	}
	var req mcpTradeRequest
	if err := json.Unmarshal(cap.body, &req); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if req.Order.QuoteSize != 50.0 {
		t.Errorf("quote_size: got %v, want 50.0 (default)", req.Order.QuoteSize)
	}
	if req.Order.Side != "sell" {
		t.Errorf("side: got %q, want sell", req.Order.Side)
	}
}

// TestMaybeSubmitOrder_PolicyReject: logs warning but does not error when policy rejects.
func TestMaybeSubmitOrder_PolicyReject(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, rejectedResp()) // 201 but decision.allowed=false
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:    srv.URL,
		TradeEnabled:       true,
		MinTradeConfidence: 0.70,
		Log:                testLogger(),
	})
	// Should not panic or error — just log warning.
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), buyDecision(0.85, ptr(100)))

	if !cap.called {
		t.Error("expected HTTP call even for policy-rejected response")
	}
}

// TestMaybeSubmitOrder_HintOverridesDefault: sizing hint takes precedence over default.
func TestMaybeSubmitOrder_HintOverridesDefault(t *testing.T) {
	t.Parallel()
	cap := &captureRequest{}
	srv := newTradeServer(t, cap, 201, acceptedResp())
	defer srv.Close()

	w := workerWith(WorkerConfig{
		PortfolioAPIURL:      srv.URL,
		TradeEnabled:         true,
		MinTradeConfidence:   0.70,
		DefaultTradeNotional: 999.0,
		Log:                  testLogger(),
	})
	w.maybeSubmitOrder(context.Background(), signalReq("BTC-USD"), buyDecision(0.90, ptr(200.0)))

	var req mcpTradeRequest
	_ = json.Unmarshal(cap.body, &req)
	if req.Order.QuoteSize != 200.0 {
		t.Errorf("hint should override default: got %v, want 200.0", req.Order.QuoteSize)
	}
}
