package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/mock"
)

func TestRunnerTickMock(t *testing.T) {
	t.Setenv("MOCK_MARKET_OPEN", "true")
	b := mock.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/snapshots" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Internal-Key") != "k" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	r := &Runner{
		Broker:   b,
		Client:   NewClient(srv.URL, "k"),
		Interval: time.Hour,
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx := context.Background()
	r.tick(ctx, time.Now())
}

func TestRunnerTick_emitsIngestTickSummary(t *testing.T) {
	t.Setenv("MOCK_MARKET_OPEN", "true")
	b := mock.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/snapshots" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	r := &Runner{
		Broker:   b,
		Client:   NewClient(srv.URL, "k"),
		Interval: time.Hour,
		Log:      log,
	}
	r.tick(context.Background(), time.Now())

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, `"msg":"ingest_tick"`) {
		t.Fatalf("expected ingest_tick log, got: %s", line)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatal(err)
	}
	if m["tick_outcome"] != "posted" {
		t.Fatalf("tick_outcome = %v", m["tick_outcome"])
	}
}

func TestBuildSnapshot(t *testing.T) {
	s := BuildSnapshot(time.Unix(0, 0).UTC(), nil, &broker.AccountSummary{AccountID: "Z"})
	if s.Summary.AccountID != "Z" {
		t.Fatal(s.Summary.AccountID)
	}
}
