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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/discord"
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
		case "/api/v3/brokerage/products":
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
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	client := coinbase.NewReadOnly(srv.Client(), srv.URL)

	err = runOnce(context.Background(), log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, discord.NewClient(""), false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"crypto_alert"`) {
		t.Fatalf("unexpected alert on baseline tick: %s", buf.String())
	}

	buf.Reset()
	price.Store(102.0)
	err = runOnce(context.Background(), log, srv.Client(), client, state, srv.URL, "k", 1.0, time.Minute, discord.NewClient(""), false)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"crypto_alert"`) {
		t.Fatalf("missing crypto_alert log: %s", out)
	}
	if !strings.Contains(out, `"symbol":"BTC-USD"`) && !strings.Contains(out, "BTC-USD") {
		t.Fatalf("missing symbol context: %s", out)
	}
	if recentAlertCalls.Load() == 0 {
		t.Fatal("expected recent alert persistence call")
	}
}
