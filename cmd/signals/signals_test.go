package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestRunOnce_tickSummaryDiscordDisabled(t *testing.T) {
	snapJSON := `{"takenAt":"2020-01-01T00:00:00Z","positions":[{"symbol":"AAPL","quantity":1,"avgCost":100,"marketValue":100,"unrealizedPLPct":-10}],"summary":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/snapshot/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, snapJSON)
	}))
	defer srv.Close()

	rules := []sigpkg.Rule{{
		ID:   "r1",
		Name: "drop",
		Condition: sigpkg.Condition{
			Type:      "price_change_pct",
			Field:     "unrealizedPLPct",
			Operator:  "lte",
			Threshold: -5,
		},
	}}
	ded, err := sigpkg.NewDedup(filepath.Join(t.TempDir(), "dedup.json"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	err = runOnce(context.Background(), log, srv.Client(), srv.URL, "k", rules, ded, time.Nanosecond, discord.NewClient(""), false, "")
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"msg":"signals_tick"`) {
		t.Fatalf("missing signals_tick: %s", out)
	}
	if !strings.Contains(out, `"events_evaluated"`) {
		t.Fatalf("missing summary fields: %s", out)
	}
}
