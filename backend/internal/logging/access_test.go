package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestAccessLog_emitsJSONWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(AccessLog(log))
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["msg"] != "http_request" {
		t.Fatalf("msg = %v", m["msg"])
	}
	if m["method"] != "GET" {
		t.Fatalf("method = %v", m["method"])
	}
	if m["path"] != "/api/health" {
		t.Fatalf("path = %v", m["path"])
	}
	if m["status"] != float64(200) { // json numbers
		t.Fatalf("status = %v", m["status"])
	}
	rid, _ := m["request_id"].(string)
	if rid == "" {
		t.Fatal("expected request_id")
	}
}
