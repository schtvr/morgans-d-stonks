# Skill: Structured logging (`internal/logging`)

Use when changing Go service logs. Epic: `.agent/epics/phase_1/logging/epic_P1_logging.md`.

## Conventions

- Attribute keys: **snake_case** (`request_id`, `duration_ms`, `rule_id`).
- Never log passwords, session cookies, `X-Internal-Key`, or full Discord webhook URLs.

## Env

| Variable | Purpose |
|----------|---------|
| `LOG_LEVEL` | `debug` \| `info` \| `warn` \| `error` (default `info`) |
| `APP_VERSION` | Optional string on every log line |
