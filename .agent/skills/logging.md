# Skill: Structured logging (`internal/logging`)

Use this when adding or changing logs in Go services for this repo.

## Where logging lives

- **Root logger**: built once in each `cmd/<binary>/main.go` via `internal/logging` — do not create duplicate `slog.NewJSONHandler` setups in binaries.
- **Libraries**: accept `*slog.Logger` or `slog.Handler` from the caller when non-trivial operational logs are needed; avoid global loggers.

## Required conventions

- **Field names**: use **snake_case** for attribute keys in structured logs (`request_id`, `duration_ms`, `rule_id`) so Loki/Grafana queries stay consistent.
- **Service identity**: every process must emit `service` (`portfolio-api`, `ingest`, `signals`, …) on each line — handled by `internal/logging`, not repeated in call sites.
- **Levels**: `debug` for noisy diagnostics; `info` for normal lifecycle and tick summaries; `warn` for recoverable degradation; `error` for failures that may need human attention.

## Never log

- Passwords, `AUTH_SECRET`, session tokens or cookie values, `X-Internal-Key`, full `DISCORD_WEBHOOK_URL`, database URLs with credentials.
- Full HTTP request/response bodies from auth or internal routes unless explicitly sanitized and truncated.

## portfolio-api

- Include **`request_id`** on access logs (from chi middleware).
- Prefer **route pattern** over raw path+query for the logged `path` where possible to avoid cardinality explosions in Loki.

## Workers (ingest, signals)

- Emit one **`info`** line per successful tick with **counts and duration**, not per-position spam unless `LOG_LEVEL=debug`.

## Env vars

| Variable | Purpose |
|----------|---------|
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error` (default `info`) |
| `APP_VERSION` | Optional semver or git SHA attached to every log line |

## Epic source of truth

Implementation scope and acceptance criteria: `.agent/epics/phase_1/logging/epic_P1_logging.md`.
