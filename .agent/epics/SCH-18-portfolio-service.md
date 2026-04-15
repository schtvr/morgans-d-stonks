# SCH-18: Portfolio Service (API, Persistence, Auth)

> **Linear**: [SCH-18](https://linear.app/schtvr/issue/SCH-18/epic-p0-portfolio-service-api-persistence-auth)
> **Milestone**: P0: MVP
> **Wave**: 2 (depends on SCH-19 for repo layout; parallel with SCH-20)
> **Depends on**: SCH-19

## Objective

Build the Go HTTP service that owns portfolio reads (for the dashboard), ingested snapshot writes (from the ingest job), and simple auth so the dashboard is not publicly accessible on the LAN.

## Scope

### API contract

Implement a REST API in `cmd/portfolio-api/`. Recommended routes:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/auth/login` | Public | Validate credentials, return session |
| `POST` | `/api/auth/logout` | Session | Invalidate session |
| `GET` | `/api/portfolio/positions` | Session | Latest positions for the table |
| `GET` | `/api/portfolio/summary` | Session | Account summary (net liq, cash, etc.) |
| `GET` | `/api/health` | Public | Healthcheck |
| `POST` | `/internal/snapshots` | Internal key | Write a snapshot (called by ingest) |

- Use `net/http` or a lightweight router (chi, echo — pick one, document).
- JSON request/response bodies.
- Return proper HTTP status codes (401, 403, 500, etc.).

### Auth

- **Single-user** for P0. Username/password loaded from env.
- Session: issue a signed JWT or opaque session token stored in a cookie.
- Middleware: reject unauthenticated requests to protected routes.
- Internal endpoint (`/internal/snapshots`): secured with a shared secret header (`X-Internal-Key`) loaded from env. This is what the ingest job uses.

```env
AUTH_USERNAME=admin
AUTH_PASSWORD=changeme
AUTH_SECRET=changeme-32-char-min    # JWT signing key
INTERNAL_API_KEY=changeme           # for ingest → portfolio-api
```

### Persistence

- **Postgres** (container provided by SCH-19's Compose file).
- Own all schema and migrations in `internal/portfolio/migrations/` or an embedded migration tool.
- Tables (minimum):

```sql
-- Snapshots written by ingest
CREATE TABLE snapshots (
    id          BIGSERIAL PRIMARY KEY,
    taken_at    TIMESTAMPTZ NOT NULL,
    data        JSONB NOT NULL,          -- serialized positions + summary
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_snapshots_taken_at ON snapshots (taken_at DESC);

-- Sessions (if not using stateless JWT)
CREATE TABLE sessions (
    token       TEXT PRIMARY KEY,
    username    TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

- Repository pattern: `internal/portfolio/repository.go` with interface + Postgres implementation.

### Response shapes

Positions endpoint response (consumed by dashboard SCH-17):

```json
{
  "positions": [
    {
      "symbol": "AAPL",
      "quantity": 100,
      "avgCost": 150.25,
      "lastPrice": 175.50,
      "marketValue": 17550.00,
      "unrealizedPL": 2525.00,
      "currency": "USD"
    }
  ],
  "asOf": "2026-04-15T15:30:00Z"
}
```

Summary endpoint response:

```json
{
  "accountId": "DU12345",
  "netLiquidation": 125000.00,
  "totalCash": 25000.00,
  "buyingPower": 50000.00,
  "currency": "USD",
  "asOf": "2026-04-15T15:30:00Z"
}
```

## Do NOT

- Implement the ingest scheduler or IBKR calls (SCH-21 and SCH-20).
- Build the dashboard UI (SCH-17 consumes this API).
- Add multi-tenant RBAC (single-user is fine for P0).
- Add TLS termination (homelab behind LAN).

## Acceptance criteria

- [ ] `POST /api/auth/login` returns a session token for valid credentials.
- [ ] Protected endpoints return 401 without a valid session.
- [ ] `GET /api/portfolio/positions` returns latest snapshot data.
- [ ] `POST /internal/snapshots` writes snapshot to DB, secured by internal key.
- [ ] Migrations run automatically on service start (or via a clear command).
- [ ] Health endpoint returns 200.
- [ ] API contract documented in code comments or an OpenAPI stub.

## Shared contracts

This service is consumed by:

- **SCH-17** (Dashboard) — calls auth + portfolio endpoints.
- **SCH-21** (Ingest) — calls `POST /internal/snapshots`.

The snapshot write contract and auth mechanism must be documented clearly for those consumers.

## Files to create/modify

| File | Action |
|------|--------|
| `cmd/portfolio-api/main.go` | Implement (server bootstrap) |
| `internal/portfolio/types.go` | Implement (domain types) |
| `internal/portfolio/repository.go` | Implement (interface) |
| `internal/portfolio/postgres/repository.go` | Implement |
| `internal/portfolio/postgres/migrations/` | Create migration files |
| `internal/auth/auth.go` | Implement (session/JWT logic) |
| `internal/auth/middleware.go` | Implement |
| `internal/config/config.go` | Implement (shared config loading) |
