# Story 01 — `internal/logging` package

**Epic**: [epic_P1_logging.md](../epic_P1_logging.md)  
**Branch for PR**: `cursor/p1-logging-story-01-internal-logging`  
**PR merges into**: `cursor/p1-logging`  
**Depends on**: none (base branch already has epic + stories)

## Goal

Add `internal/logging` so every binary can construct a JSON `slog` logger to stdout with `service`, optional `version`, and env-driven level.

## Requirements

1. **API** (names may vary; keep idiomatic Go):
   - `New(service string) *slog.Logger` — reads `LOG_LEVEL` and `APP_VERSION` from the environment (use `os.Getenv` only in this package or in `main` passing values in — prefer **one place**: `NewFromEnv` with `os.Getenv` inside `internal/logging` is fine for this epic).
   - JSON handler → **stdout**.
2. **`LOG_LEVEL`**: `debug`, `info`, `warn`, `error` (case-insensitive). Empty or invalid → **`info`** and **one** diagnostic line to **stderr** (not JSON logger) explaining the fallback.
3. **`APP_VERSION`**: optional; if non-empty, add attribute `version` to every log via `Logger.With`.
4. **Required attribute**: `service` on every emitted record (non-empty string from caller).
5. **Tests** (`internal/logging/logger_test.go`):
   - Level parsing / behavior for valid levels (e.g. `debug` emits debug record when logged).
   - JSON output contains `"service":"test-service"` (decode one line).
   - Invalid `LOG_LEVEL` falls back to info (assert default behavior).

## Out of scope

- Wiring `cmd/*` (Story 02).
- HTTP access middleware (Story 03).

## Definition of done

- [ ] `go test ./...` passes.
- [ ] No new third-party logging dependencies.

## Files

| Action | Path |
|--------|------|
| Create | `internal/logging/logger.go` |
| Create | `internal/logging/logger_test.go` |
