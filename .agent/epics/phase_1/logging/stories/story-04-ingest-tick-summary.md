# Story 04 — `ingest` INFO tick summary for Loki dashboards

**Epic**: [epic_P1_logging.md](../epic_P1_logging.md)  
**Branch for PR**: `cursor/p1-logging-story-04-ingest-tick`  
**PR merges into**: `cursor/p1-logging`  
**Depends on**: Story 02 merged into `cursor/p1-logging` (Story 03 can land before or after; no file conflict expected)

## Goal

Each ingest **tick** emits a single **`info`** summary suitable for “last successful run” / error-rate panels, without per-position spam.

## Requirements

1. At **tick** granularity, log **one structured `info` line** when the tick completes (success, skip, or early return), including at minimum:
   - `duration_ms` (full tick wall time)
   - `tick_outcome` — small enum, e.g. `posted`, `skipped_market_closed`, `skipped_broker_error`, `skipped_post_error`, etc.
   - When snapshot successfully posted: `position_count`, `taken_at` (UTC timestamp from snapshot) if available
2. Existing **`warn`** lines for broker/post errors may remain; ensure they do **not** include full HTTP response bodies from the portfolio API (status or short message only).
3. Field names **snake_case** per `.agent/skills/logging.md`.

## Definition of done

- [ ] `go test ./...` passes.
- [ ] `internal/ingest` tests updated or added if timing/outcome logic is extracted for testability.

## Files

| Action | Path |
|--------|------|
| Modify | `internal/ingest/runner.go` |
| Modify | `internal/ingest/runner_test.go` (if needed) |
