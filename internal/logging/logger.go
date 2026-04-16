// Package logging provides a shared JSON slog logger for service stdout (Loki-friendly).
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog.Logger to stdout with attrs service, optional version,
// and level from LOG_LEVEL (default info). Invalid LOG_LEVEL writes one line to stderr and uses info.
func New(service string) *slog.Logger {
	return newLogger(service, os.Getenv("LOG_LEVEL"), os.Getenv("APP_VERSION"), os.Stdout, os.Stderr)
}

func newLogger(service, logLevelEnv, appVersion string, out, errOut io.Writer) *slog.Logger {
	if service == "" {
		service = "unknown"
	}
	level, ok := parseLevel(logLevelEnv)
	if !ok && strings.TrimSpace(logLevelEnv) != "" {
		_, _ = fmt.Fprintf(errOut, "logging: invalid LOG_LEVEL=%q, using info\n", logLevelEnv)
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	h := slog.NewJSONHandler(out, opts)
	var lg *slog.Logger
	if v := strings.TrimSpace(appVersion); v != "" {
		lg = slog.New(h).With("service", service, "version", v)
	} else {
		lg = slog.New(h).With("service", service)
	}
	return lg
}

func parseLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return slog.LevelInfo, true
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}
