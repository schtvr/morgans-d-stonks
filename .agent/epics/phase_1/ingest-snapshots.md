# SCH-21: Ingest, Snapshots & Session Gating

> **Linear**: [SCH-21](https://linear.app/schtvr/issue/SCH-21/epic-p0-ingest-snapshots-and-session-gating)
> **Milestone**: P0: MVP
> **Wave**: 3 (depends on SCH-20 for Broker interface + SCH-18 for snapshot persistence)
> **Depends on**: SCH-19, SCH-20, SCH-18

## Objective

Build a periodic job that pulls portfolio/market state from IBKR every ~10 minutes during active equity sessions, then persists snapshots via the portfolio service for downstream signals and the dashboard.

## Scope

### Job binary (`cmd/ingest/`)

- Long-running process with an internal ticker (~10 minute interval, configurable via env).
- On each tick:
  1. Check `broker.IsMarketOpen(ctx)` — skip if market is closed.
  2. Fetch `broker.Positions(ctx)` and `broker.AccountSummary(ctx)`.
  3. Optionally fetch `broker.Quotes(ctx, watchlist)` for enrichment.
  4. Build a snapshot payload.
  5. POST to portfolio service: `POST /internal/snapshots` with `X-Internal-Key` header.
  6. Log result (success, skip, error).
- Graceful shutdown on SIGINT/SIGTERM.

### Configuration

```env
INGEST_INTERVAL=10m           # ticker interval
IBKR_MODE=paper               # passed to broker factory
PORTFOLIO_API_URL=http://portfolio-api:8080
INTERNAL_API_KEY=changeme     # shared secret for internal endpoint
```

### Session gating

- Use the `Broker.IsMarketOpen()` method (implemented in SCH-20).
- Do NOT hard-code ET market hours. Trust IBKR's session flags.
- When the market is closed, log at `INFO` level and sleep until next tick.
- On IBKR connectivity errors during the session check, log at `WARN` and retry on the next tick (do not crash).

### Snapshot payload

Build from broker domain types (defined in SCH-20):

```json
{
  "takenAt": "2026-04-15T15:30:00Z",
  "positions": [ ... ],  // broker.Position mapped to JSON
  "summary": { ... }     // broker.AccountSummary mapped to JSON
}
```

### Idempotency

- Include `takenAt` timestamp rounded to the minute.
- The portfolio service should handle de-duplication (upsert on `taken_at`), but the ingest job should also avoid double-posting within the same tick by tracking last successful post time.

### Error handling

- Broker errors: log and skip tick; do not crash.
- Portfolio API errors: log and retry once; if still failing, skip and continue.
- Never corrupt the last-good snapshot — the job only creates new records.

### Compose wiring

- Service `ingest` in `docker-compose.yml` (provided by SCH-19).
- Depends on: `portfolio-api`, `ib-gateway` (or mock).
- Restart policy: `unless-stopped` (so it recovers from transient failures).

## Do NOT

- Own the snapshot DB schema or migrations (SCH-18 owns persistence).
- Build the signal engine (SCH-16 consumes snapshots separately).
- Implement tick-level streaming or historical backfill.
- Hard-code market hours in Eastern Time.

## Acceptance criteria

- [ ] Job starts, connects to broker (mock mode), and writes a snapshot to the portfolio API.
- [ ] Job skips ticks when `IsMarketOpen()` returns false (verified in mock mode).
- [ ] Interval is configurable via `INGEST_INTERVAL` env var.
- [ ] Graceful shutdown on SIGINT (no partial writes).
- [ ] Errors are logged without crashing the process.
- [ ] Works end-to-end in Docker Compose with `IBKR_MODE=mock`.

## Shared contracts

This epic **consumes**:

- **SCH-20** `Broker` interface — `Positions()`, `AccountSummary()`, `Quotes()`, `IsMarketOpen()`
- **SCH-18** internal API — `POST /internal/snapshots` with `X-Internal-Key`

This epic **produces** data consumed by:

- **SCH-16** (Signals) — reads snapshots from the portfolio service
- **SCH-17** (Dashboard) — reads latest positions from the portfolio service

## Files to create/modify

| File | Action |
|------|--------|
| `cmd/ingest/main.go` | Implement |
| `internal/ingest/runner.go` | Implement (ticker + orchestration) |
| `internal/ingest/snapshot.go` | Implement (payload builder) |
| `internal/ingest/client.go` | Implement (portfolio API HTTP client) |
| `internal/ingest/runner_test.go` | Implement (with mock broker) |
