# Story 05 — `signals` INFO tick summary + safe errors

**Epic**: [epic_P1_logging.md](../epic_P1_logging.md)  
**Branch for PR**: `cursor/p1-logging-story-05-signals-tick`  
**PR merges into**: `cursor/p1-logging`  
**Depends on**: Story 02 merged into `cursor/p1-logging`

## Goal

Each signals **tick** emits one **`info`** summary; per-fire logs include **`rule_id`** and **`symbol`**; never log secrets or full error bodies from upstream.

## Requirements

1. **Per-tick `info` summary** after evaluation/send loop, including at minimum:
   - `duration_ms`
   - `rule_count` (rules loaded)
   - `events_evaluated` (count of firing candidates from engine — typically `len(evs)`)
   - `events_sent` (Discord sends attempted or log-only fires; define clearly in code comments)
   - `discord_enabled` (bool)
   - `discord_errors` (count of failed Discord posts in that tick)
2. **Per-fire** log when Discord disabled (existing path): add `rule_id`, `symbol` (and keep human-readable signal text if present).
3. **`fetchSnapshot` / errors**: do not include raw portfolio response **body** in errors that may be logged (avoid leaking internal payloads); status code is enough for operators.
4. Never log `DISCORD_WEBHOOK_URL`, `X-Internal-Key`, or session material.

## Definition of done

- [ ] `go test ./...` passes.
- [ ] Add or extend tests for `runOnce` summary counts if extracted for testability.

## Files

| Action | Path |
|--------|------|
| Modify | `cmd/signals/main.go` |
