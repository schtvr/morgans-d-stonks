# Story 02 — Wire all Go `cmd/*` binaries to `internal/logging`

**Epic**: [epic_P1_logging.md](../epic_P1_logging.md)  
**Branch for PR**: `cursor/p1-logging-story-02-wire-cmd`  
**PR merges into**: `cursor/p1-logging`  
**Depends on**: Story 01 merged into `cursor/p1-logging`

## Goal

Remove ad-hoc `slog.New(slog.NewJSONHandler(os.Stdout, …))` from entrypoints; use `internal/logging` with a **distinct `service` value per binary**.

## Requirements

1. **`portfolio-api`**: `service=portfolio-api`.
2. **`ingest`**: `service=ingest`.
3. **`signals`**: `service=signals`.
4. **`internal/ingest` Runner** fallback when `Log == nil`: must still produce JSON with `service=ingest` — either set logger in `cmd/ingest` before `Run` (already does) and change fallback to use `internal/logging.New("ingest")`, or document that Runner always receives non-nil `Log` after this story; **prefer** non-nil from `main` + safe fallback using `internal/logging` for consistency.

## Out of scope

- Access log middleware (Story 03).
- Tick summary fields beyond what already exists (Stories 04–05).

## Definition of done

- [ ] `grep -R "NewJSONHandler" cmd` shows no remaining direct stdout JSON handler setup in `cmd/` (Runner fallback allowed only via `internal/logging`).
- [ ] `go test ./...` passes.

## Files

| Action | Path |
|--------|------|
| Modify | `cmd/portfolio-api/main.go` |
| Modify | `cmd/ingest/main.go` |
| Modify | `cmd/signals/main.go` |
| Modify | `internal/ingest/runner.go` (fallback logger only, if needed) |
