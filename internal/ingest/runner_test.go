package ingest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestBuildSnapshot(t *testing.T) {
	s := BuildSnapshot(time.Unix(0, 0).UTC(), nil, &broker.AccountSummary{AccountID: "Z"})
	if s.Summary.AccountID != "Z" {
		t.Fatal(s.Summary.AccountID)
	}
}
