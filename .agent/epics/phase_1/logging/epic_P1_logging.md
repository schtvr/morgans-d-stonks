# P1: Structured logging for Loki / homelab observability

> **Linear**: _(create issue and link — e.g. SCH-XX)_  
> **Milestone**: P1: Follow-up (cross-cutting; implement after core P0 paths exist)  
> **Wave**: After P0 MVP services are stable; can ship in parallel with Phase 2 epics where touch points are isolated.  
> **Depends on**: SCH-19 (layout), SCH-18 (`portfolio-api`), SCH-21 (`ingest`), SCH-16 (`signals`) — for meaningful access and tick logs.

## Objective

Make every Go service emit **consistent, JSON structured logs** on stdout suitable for **Grafana Loki + Promtail** (and any log shipper): stable field names, low-cardinality labels via log attributes, request correlation on the API, worker “heartbeat” summaries, and **no secrets** in log lines.

## Scope

### Shared package: `internal/logging`

- Provide a single constructor for the process root `*slog.Logger` (JSON handler → **stdout**).
- **Required** static attribute: `service` — one of: `portfolio-api`, `ingest`, `signals` (and future binaries as they appear).
- **Optional** static attribute: `version` — from build (`-ldflags`) or env `APP_VERSION` (homelab-friendly).
- **Log level** from env `LOG_LEVEL` (values: `debug`, `info`, `warn`, `error`; default `info`). Invalid values → default + one warning line or silent fallback (document choice).
- Package must include `_test.go` (level parsing, JSON contains `service`).

### `cmd/*` entrypoints

- Replace ad-hoc `slog.New(slog.NewJSONHandler(...))` with `internal/logging`.
- Each binary passes its **distinct** `service` name.

### `portfolio-api` (`cmd/portfolio-api`, HTTP stack)

- **Access logging**: one structured log line per HTTP request after completion with at least: `method`, `path` (route pattern preferred over raw URL to limit cardinality; strip query string minimum), `status`, `duration_ms`, `bytes` (response size if practical).
- **Request correlation**: include chi `RequestID` on every access log line; prefer propagating `request_id` into context for any handler-level logs (middleware or `slog` context pattern).
- **Redaction**: never log `Authorization`, session cookie values, request bodies for auth routes, or `X-Internal-Key`. Sanitize or omit upstream error bodies that might echo secrets.

### `ingest` (`internal/ingest` / `cmd/ingest`)

- **Per-tick summary** at `INFO`: e.g. tick outcome, duration, position/snapshot counts or equivalent high-signal fields, market open/closed path when applicable.
- **WARN** on retry/skip paths with stable attributes; do not log full HTTP response bodies from portfolio-api.

### `signals` (`cmd/signals`)

- **Per-tick summary** at `INFO`: rules loaded count, events evaluated, fires sent (or suppressed), Discord enabled flag, Discord send errors aggregated count, duration.
- Enrich per-fire logs with `rule_id`, `symbol` where not already present; keep Discord webhook URL out of logs.

### Configuration surface

- Document `LOG_LEVEL` and `APP_VERSION` in **`.env.example`** (optional vars with defaults described in comments).

### TypeScript / Next.js (`apps/web`)

- **Out of scope** for this epic unless extended: dashboard logging remains unchanged. Promtail may still scrape container stdout for `web` using docker labels.

## Do NOT

- Add Prometheus metrics or OpenTelemetry tracing (separate epic if desired).
- Add new HTTP endpoints solely for logging (keep using stdout for Loki).
- Log secrets, API keys, webhook URLs, or raw session tokens.
- Introduce high-cardinality fields as required labels (e.g. per-symbol streams on every access log line).

## Acceptance criteria

- [ ] `internal/logging` exists with tests; all Go services use it for the root logger.
- [ ] Every log line JSON includes `service` (and `version` when `APP_VERSION` or build metadata is set).
- [ ] `LOG_LEVEL` changes verbosity without code change (verified with at least one test or manual note in PR).
- [ ] `portfolio-api` emits structured access logs with `request_id` and without sensitive headers/bodies.
- [ ] `ingest` emits an `INFO` tick summary suitable for “last successful run” dashboards.
- [ ] `signals` emits an `INFO` tick summary and does not log `DISCORD_WEBHOOK_URL` or internal key material.
- [ ] `.env.example` documents `LOG_LEVEL` / `APP_VERSION`.
- [ ] `go test ./...` passes.

## Shared contracts

- **Consumers**: homelab Grafana/Loki dashboards; future runbooks.
- **No change** to HTTP API contracts, broker interface, or `SignalEvent` JSON shape — only observability.

## Files to create/modify

| File | Action |
|------|--------|
| `internal/logging/logger.go` | Create (constructor, level parsing, attrs) |
| `internal/logging/logger_test.go` | Create |
| `cmd/portfolio-api/main.go` | Modify (logger + access middleware) |
| `cmd/ingest/main.go` | Modify (logger) |
| `cmd/signals/main.go` | Modify (logger + tick summary fields) |
| `internal/ingest/runner.go` (and related) | Modify (tick summary logs) |
| `.env.example` | Modify (`LOG_LEVEL`, `APP_VERSION`) |
| `AGENTS.md` | Modify (layout, env table, skills pointer) |
| `.agent/skills/logging.md` | Create (agent quick reference) |

## Agent execution notes

- Read `.agent/skills/logging.md` for field naming and redaction checklist before editing handlers.
- One logical PR is fine; if splitting, land `internal/logging` + `cmd/*` wiring first, then API middleware, then worker summaries.
