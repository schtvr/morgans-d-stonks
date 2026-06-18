package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleMarketCandles_noCoinbaseClient(t *testing.T) {
	t.Parallel()
	a := &app{cb: nil, log: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/market/candles?symbol=BTC-USD&range=1d", nil)
	rec := httptest.NewRecorder()
	a.handleMarketCandles(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMarketCandles_badMethod(t *testing.T) {
	t.Parallel()
	a := &app{cb: nil, log: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/market/candles", nil)
	rec := httptest.NewRecorder()
	a.handleMarketCandles(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}
