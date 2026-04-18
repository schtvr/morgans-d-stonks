package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel_valid(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"  ", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tt := range tests {
		got, ok := parseLevel(tt.in)
		if !ok {
			t.Fatalf("parseLevel(%q): expected ok", tt.in)
		}
		if got != tt.want {
			t.Fatalf("parseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseLevel_invalid(t *testing.T) {
	_, ok := parseLevel("nope")
	if ok {
		t.Fatal("expected invalid")
	}
}

func TestNewLogger_JSONContainsService(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer
	log := newLogger("test-service", "info", "", &out, &errBuf)
	log.Info("hello", "k", 1)
	if errBuf.Len() != 0 {
		t.Fatalf("stderr: %s", errBuf.String())
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["service"] != "test-service" {
		t.Fatalf("service = %v", m["service"])
	}
	if m["msg"] != "hello" {
		t.Fatalf("msg = %v", m["msg"])
	}
}

func TestNewLogger_versionAttr(t *testing.T) {
	var out bytes.Buffer
	log := newLogger("ingest", "info", "1.2.3", &out, io.Discard)
	log.Warn("x")
	var m map[string]any
	_ = json.Unmarshal(out.Bytes(), &m)
	if m["version"] != "1.2.3" {
		t.Fatalf("version = %v", m["version"])
	}
}

func TestNewLogger_invalidLevelFallbackAndStderr(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer
	log := newLogger("svc", "bogus", "", &out, &errBuf)
	if !strings.Contains(errBuf.String(), "invalid LOG_LEVEL") {
		t.Fatalf("expected stderr warning, got %q", errBuf.String())
	}
	// Fallback info: debug should not appear
	if log.Enabled(context.TODO(), slog.LevelDebug) {
		t.Fatal("expected info level")
	}
	log.Info("ok")
	var m map[string]any
	_ = json.Unmarshal(out.Bytes(), &m)
	if m["msg"] != "ok" {
		t.Fatalf("msg = %v", m["msg"])
	}
}

func TestNewLogger_debugLevel(t *testing.T) {
	var out bytes.Buffer
	log := newLogger("svc", "debug", "", &out, io.Discard)
	if !log.Enabled(context.TODO(), slog.LevelDebug) {
		t.Fatal("expected debug enabled")
	}
	log.Debug("trace")
	if !bytes.Contains(out.Bytes(), []byte(`"msg":"trace"`)) {
		t.Fatalf("output: %s", out.String())
	}
}
