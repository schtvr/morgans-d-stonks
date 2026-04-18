# Story 03 — `portfolio-api` structured access logs + request correlation

**Epic**: [epic_P1_logging.md](../epic_P1_logging.md)  
**Branch for PR**: `cursor/p1-logging-story-03-access-logs`  
**PR merges into**: `cursor/p1-logging`  
**Depends on**: Story 02 merged into `cursor/p1-logging`

## Goal

Emit **one JSON log line per HTTP request** after the response completes, suitable for Loki queries and correlation with handler logs.

## Log fields (snake_case)

Minimum on every access line:

| Field | Source |
|--------|--------|
| `msg` | fixed string e.g. `http_request` |
| `method` | `r.Method` |
| `path` | chi **route pattern** if available after routing; else `r.URL.Path` (no query string) |
| `status` | HTTP status code |
| `duration_ms` | integer elapsed milliseconds |
| `bytes` | response bytes written (best effort via wrapped `ResponseWriter`) |
| `request_id` | chi `middleware.GetReqID(r)` |

## Middleware order

Register access middleware **after** `RequestID` so `request_id` is populated. It must run for **all** routes including `/api/health` and `/internal/*`.

## Redaction / safety

- Do **not** log: `Authorization`, `Cookie` / session values, request bodies, or `X-Internal-Key`.
- Access middleware should only use request metadata listed above.

## Optional (nice to have)

- Attach `*slog.Logger` with `request_id` to `r.Context()` using `slog.With` so handlers can log with the same id — only if small; otherwise skip.

## Definition of done

- [ ] Manual or test verification: hit `/api/health` and see one access line with `status=200`, `request_id` non-empty.
- [ ] `go test ./...` passes (add `httptest` test if straightforward).

## Files

| Action | Path |
|--------|------|
| Modify | `cmd/portfolio-api/main.go` (middleware; may extract to `internal/logging/middleware.go` if kept tiny) |
