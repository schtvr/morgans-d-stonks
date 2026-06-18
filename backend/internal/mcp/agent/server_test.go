package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// ----- helpers -----

func mustNewServer(t *testing.T, cb *coinbase.Client, portfolioAPIURL string, ingestSec int) (*mcpServerHandle, MCPClient) {
	t.Helper()
	srv, cli, err := NewInProcessServer(cb, portfolioAPIURL, "test-key", nil, ingestSec)
	if err != nil {
		t.Fatalf("NewInProcessServer: %v", err)
	}
	return &mcpServerHandle{srv}, cli
}

// mcpServerHandle wraps the server just for documentation; callers never need to call anything on it.
type mcpServerHandle struct {
	srv interface{}
}

func fakeCoinbaseServer(t *testing.T, bars []fakeCandleBar) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/candles") {
			http.NotFound(w, r)
			return
		}
		candles := make([]map[string]string, 0, len(bars))
		for _, b := range bars {
			candles = append(candles, map[string]string{
				"start":  strconv.FormatInt(b.ts.Unix(), 10),
				"open":   strconv.FormatFloat(b.open, 'f', 2, 64),
				"high":   strconv.FormatFloat(b.high, 'f', 2, 64),
				"low":    strconv.FormatFloat(b.low, 'f', 2, 64),
				"close":  strconv.FormatFloat(b.close, 'f', 2, 64),
				"volume": strconv.FormatFloat(b.volume, 'f', 4, 64),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"candles": candles})
	}))
}

type fakeCandleBar struct {
	ts                             time.Time
	open, high, low, close, volume float64
}

func fakePortfolioServer(t *testing.T, snap *portfolio.IngestSnapshotRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/snapshot/latest" {
			if snap == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snap)
			return
		}
		http.NotFound(w, r)
	}))
}

func callTool(t *testing.T, cli MCPClient, name string, input any) map[string]any {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	raw, err := cli.CallTool(context.Background(), name, json.RawMessage(b))
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal result: %v (raw=%s)", err, raw)
	}
	return out
}

// ----- TestServerRegistersAllTools -----

func TestServerRegistersAllTools(t *testing.T) {
	srv, _, err := NewInProcessServer(nil, "http://localhost:8080", "key", nil, 600)
	if err != nil {
		t.Fatalf("NewInProcessServer: %v", err)
	}

	tools := srv.ListTools()
	want := []string{
		"get_market_candles",
		"get_holdings",
		"get_position",
		"get_correlated_symbols",
		"get_recent_signals",
		"get_recent_decisions",
		"get_decision_outcomes",
	}
	registered := make([]string, 0, len(tools))
	for k := range tools {
		registered = append(registered, k)
	}
	for _, name := range want {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q not registered; registered: %v", name, registered)
		}
	}
	if len(tools) != len(want) {
		t.Errorf("expected %d tools, got %d: %v", len(want), len(tools), registered)
	}
}

// ----- get_market_candles -----

func TestGetMarketCandles_HappyPath(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	bars := []fakeCandleBar{
		{ts: now.Add(-4 * time.Minute), open: 100, high: 110, low: 95, close: 105, volume: 1.5},
		{ts: now.Add(-3 * time.Minute), open: 105, high: 115, low: 100, close: 112, volume: 2.0},
		{ts: now.Add(-2 * time.Minute), open: 112, high: 120, low: 108, close: 118, volume: 1.8},
		{ts: now.Add(-1 * time.Minute), open: 118, high: 125, low: 115, close: 122, volume: 2.2},
		{ts: now, open: 122, high: 130, low: 119, close: 128, volume: 1.9},
	}

	cbSrv := fakeCoinbaseServer(t, bars)
	defer cbSrv.Close()

	cb := coinbase.NewReadOnly(cbSrv.Client(), cbSrv.URL, "", "")
	_, cli := mustNewServer(t, cb, "http://unused:8080", 600)

	out := callTool(t, cli, "get_market_candles", map[string]string{"symbol": "BTC-USD", "window": "1h"})

	if out["symbol"] != "BTC-USD" {
		t.Errorf("symbol: got %v", out["symbol"])
	}
	if out["window"] != "1h" {
		t.Errorf("window: got %v", out["window"])
	}
	if out["granularity"] != "ONE_MINUTE" {
		t.Errorf("granularity: got %v", out["granularity"])
	}
	pts, _ := out["points"].([]any)
	if len(pts) == 0 {
		t.Error("expected non-empty points")
	}
	// Each point must have asOf, open, high, low, close, volume.
	for i, p := range pts {
		pm, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("point %d not object", i)
		}
		for _, field := range []string{"asOf", "open", "high", "low", "close", "volume"} {
			if _, ok := pm[field]; !ok {
				t.Errorf("point %d missing field %q", i, field)
			}
		}
	}
}

func TestGetMarketCandles_Cap200(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	bars := make([]fakeCandleBar, 300)
	for i := range bars {
		bars[i] = fakeCandleBar{
			ts:     now.Add(-time.Duration(300-i) * time.Minute),
			open:   100,
			high:   110,
			low:    90,
			close:  105,
			volume: 1.0,
		}
	}

	cbSrv := fakeCoinbaseServer(t, bars)
	defer cbSrv.Close()

	cb := coinbase.NewReadOnly(cbSrv.Client(), cbSrv.URL, "", "")
	_, cli := mustNewServer(t, cb, "http://unused:8080", 600)

	out := callTool(t, cli, "get_market_candles", map[string]string{"symbol": "BTC-USD", "window": "1h"})

	pts, _ := out["points"].([]any)
	if len(pts) > 200 {
		t.Errorf("expected ≤200 points, got %d", len(pts))
	}
}

// ----- get_holdings -----

func makeSnapshotAgeSec(ageSec int, positions []broker.Position) *portfolio.IngestSnapshotRequest {
	return &portfolio.IngestSnapshotRequest{
		TakenAt:   time.Now().UTC().Add(-time.Duration(ageSec) * time.Second),
		Positions: positions,
		Summary:   broker.AccountSummary{NetLiquidation: 50000, TotalCash: 10000, Currency: "USD"},
	}
}

func TestGetHoldings_Fresh(t *testing.T) {
	snap := makeSnapshotAgeSec(60, []broker.Position{{Symbol: "BTC-USD", Quantity: 0.5, MarketValue: 30000}})

	apiSrv := fakePortfolioServer(t, snap)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	out := callTool(t, cli, "get_holdings", map[string]any{})

	if out["stale"] != false {
		t.Errorf("expected stale=false for 60s old snapshot with 600s interval, got %v", out["stale"])
	}
	if out["source"] != "ingest_snapshot" {
		t.Errorf("source: got %v", out["source"])
	}
}

func TestGetHoldings_Stale(t *testing.T) {
	// ingestIntervalSec=5 → threshold=15s; snapshot is 60s old → stale
	snap := makeSnapshotAgeSec(60, []broker.Position{{Symbol: "BTC-USD", Quantity: 0.5}})

	apiSrv := fakePortfolioServer(t, snap)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 5)
	out := callTool(t, cli, "get_holdings", map[string]any{})

	if out["stale"] != true {
		t.Errorf("expected stale=true, got %v", out["stale"])
	}
}

// ----- get_position -----

func TestGetPosition_Found(t *testing.T) {
	snap := makeSnapshotAgeSec(30, []broker.Position{
		{Symbol: "BTC-USD", Quantity: 0.5, AvgCost: 50000, MarketValue: 60000, UnrealizedPL: 5000},
		{Symbol: "ETH-USD", Quantity: 2.0, MarketValue: 8000},
	})

	apiSrv := fakePortfolioServer(t, snap)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	out := callTool(t, cli, "get_position", map[string]string{"symbol": "BTC-USD"})

	pos, ok := out["position"].(map[string]any)
	if !ok {
		t.Fatalf("position not object; got %T: %v", out["position"], out["position"])
	}
	if pos["symbol"] != "BTC-USD" {
		t.Errorf("position.symbol: got %v", pos["symbol"])
	}
}

func TestGetPosition_NotFound(t *testing.T) {
	snap := makeSnapshotAgeSec(30, []broker.Position{
		{Symbol: "BTC-USD", Quantity: 0.5},
	})

	apiSrv := fakePortfolioServer(t, snap)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	out := callTool(t, cli, "get_position", map[string]string{"symbol": "XYZ-USD"})

	errObj, ok := out["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got: %v", out)
	}
	if errObj["code"] != "not_found" {
		t.Errorf("error.code: got %v, want not_found", errObj["code"])
	}
}

// ----- get_correlated_symbols -----

func TestGetCorrelatedSymbols_Dedup(t *testing.T) {
	snap := makeSnapshotAgeSec(30, []broker.Position{
		{Symbol: "BTC-USD", MarketValue: 50000},
		{Symbol: "ETH-USD", MarketValue: 20000},
		{Symbol: "SOL-USD", MarketValue: 5000},
	})

	apiSrv := fakePortfolioServer(t, snap)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	out := callTool(t, cli, "get_correlated_symbols", map[string]string{"symbol": "BTC-USD"})

	corr, _ := out["correlated"].([]any)
	if len(corr) == 0 {
		t.Fatal("correlated must not be empty")
	}
	if len(corr) > 6 {
		t.Errorf("correlated exceeds 6 items: %v", corr)
	}

	// No duplicates.
	seen := make(map[string]int)
	for i, s := range corr {
		sym, _ := s.(string)
		seen[sym]++
		if seen[sym] > 1 {
			t.Errorf("duplicate symbol %q at index %d", sym, i)
		}
	}

	// BTC-USD must be first (input symbol).
	if corr[0] != "BTC-USD" {
		t.Errorf("first correlated must be input symbol BTC-USD, got %v", corr[0])
	}
}

// ----- history tools (stub tests) -----

type historyHandler struct {
	path    string
	lastURL string
}

func fakeHistoryServer(t *testing.T, h *historyHandler, responseBody any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.lastURL = r.URL.String()
		if r.URL.Path != h.path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responseBody)
	}))
}

func TestGetRecentSignals_Stub(t *testing.T) {
	h := &historyHandler{path: "/internal/recent-alerts/list"}
	stub := map[string]any{"alerts": []any{}}
	apiSrv := fakeHistoryServer(t, h, stub)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	callTool(t, cli, "get_recent_signals", map[string]any{"symbol": "BTC-USD", "limit": 10, "window": "12h"})

	if h.lastURL == "" {
		t.Fatal("expected HTTP call to portfolio-api, got none")
	}
	if !strings.Contains(h.lastURL, "symbol=BTC-USD") {
		t.Errorf("expected symbol param in URL, got: %s", h.lastURL)
	}
	if !strings.Contains(h.lastURL, fmt.Sprintf("limit=%d", 10)) {
		t.Errorf("expected limit=10 in URL, got: %s", h.lastURL)
	}
	if !strings.Contains(h.lastURL, "since=") {
		t.Errorf("expected since param in URL, got: %s", h.lastURL)
	}
}

func TestGetRecentDecisions_Stub(t *testing.T) {
	h := &historyHandler{path: "/internal/agent-decisions/list"}
	stub := map[string]any{"decisions": []any{}}
	apiSrv := fakeHistoryServer(t, h, stub)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	callTool(t, cli, "get_recent_decisions", map[string]any{"symbol": "ETH-USD", "limit": 5})

	if !strings.Contains(h.lastURL, "symbol=ETH-USD") {
		t.Errorf("expected symbol=ETH-USD in URL, got: %s", h.lastURL)
	}
	if !strings.Contains(h.lastURL, "limit=5") {
		t.Errorf("expected limit=5 in URL, got: %s", h.lastURL)
	}
}

func TestGetDecisionOutcomes_Stub(t *testing.T) {
	h := &historyHandler{path: "/internal/agent-decisions/outcomes"}
	stub := map[string]any{"outcomes": []any{}}
	apiSrv := fakeHistoryServer(t, h, stub)
	defer apiSrv.Close()

	_, cli := mustNewServer(t, nil, apiSrv.URL, 600)
	callTool(t, cli, "get_decision_outcomes", map[string]any{"symbol": "SOL-USD", "horizon": "7d", "limit": 15})

	if !strings.Contains(h.lastURL, "symbol=SOL-USD") {
		t.Errorf("expected symbol=SOL-USD in URL, got: %s", h.lastURL)
	}
	if !strings.Contains(h.lastURL, "horizon=7d") {
		t.Errorf("expected horizon=7d in URL, got: %s", h.lastURL)
	}
	if !strings.Contains(h.lastURL, "limit=15") {
		t.Errorf("expected limit=15 in URL, got: %s", h.lastURL)
	}
}
