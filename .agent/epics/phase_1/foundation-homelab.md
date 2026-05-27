# SCH-19: Foundation & Homelab Delivery

> **Status: Implemented** — verified 2026-05

> **Linear**: [SCH-19](https://linear.app/schtvr/issue/SCH-19/epic-p0-foundation-homelab-delivery)
> **Milestone**: P0: MVP
> **Wave**: 1 (no dependencies — land first)

## Objective

Bootstrap the monorepo layout, Docker Compose runtime, baseline CI, and env/secrets pattern so every other P0 epic can ship incrementally on a stable skeleton.

## Scope

### Repository layout

Create the following top-level structure:

```
apps/
  web/              # Next.js dashboard (SCH-17 owns implementation)
cmd/
  portfolio-api/    # Go entry-point for portfolio service (SCH-18 owns logic)
  ingest/           # Go entry-point for ingest job (SCH-21 owns logic)
  signals/          # Go entry-point for signal engine (SCH-16 owns logic)
internal/
  broker/           # Broker interface + types (SCH-20 owns implementation)
  portfolio/        # Portfolio domain types + repository interface
  signal/           # Signal domain types
  discord/          # Discord client
  config/           # Shared config loading
pkg/                # Public-API Go packages (if any)
docker/
  gateway/          # Optional stubs / docs (no default compose service)
```

- Each `cmd/` binary should have a minimal `main.go` that compiles and prints a placeholder message.
- `apps/web/` should have a bootstrapped Next.js project (use `create-next-app` with TypeScript, Tailwind, App Router).
- Add a root `go.mod` (`module github.com/schtvr/morgans-d-stonks`) and `go.sum`.

### Docker Compose

Create `docker-compose.yml` at root:

| Service          | Image / Build           | Ports              | Notes                                    |
|------------------|-------------------------|--------------------|------------------------------------------|
| `web`            | `./apps/web`            | `3000:3000`        | Next.js dev server                       |
| `portfolio-api`  | `./` (target cmd)       | `8080:8080`        | Go HTTP service                          |
| `ingest`         | `./` (target cmd)       | —                  | One-shot or long-running job             |
| `signals`        | `./` (target cmd)       | —                  | Triggered after ingest or on schedule    |
| `db`             | `postgres:16-alpine`    | `5432:5432`        | Dev convenience; schema owned by SCH-18  |

- Define a shared network `portfolio-net` (bridge).
- Use `.env` file for secrets/config; create `.env.example` with placeholder keys.
- Add `docker-compose.override.yml` for dev-only tweaks (volume mounts, hot-reload).

### CI (GitHub Actions)

Create `.github/workflows/ci.yml`:

- Trigger on push to `main` and PRs.
- Jobs: `go-lint` (golangci-lint), `go-test`, `go-build`, `web-lint`, `web-build`.
- Use caching for Go modules and npm/pnpm.

### Env contract

Create `.env.example`:

```env
# Database
POSTGRES_USER=portfolio
POSTGRES_PASSWORD=changeme
POSTGRES_DB=portfolio
DATABASE_URL=postgres://portfolio:changeme@db:5432/portfolio?sslmode=disable

# Coinbase read (ingest / signals)
BROKER_PROVIDER=coinbase
COINBASE_READ_API_KEY=
COINBASE_READ_API_SECRET=

# Auth
AUTH_SECRET=changeme-32-char-min

# Discord
DISCORD_WEBHOOK_URL=
```

Add `.env` to `.gitignore`.

### Documentation

Add a `README.md` section:

```
## Local Development

1. `cp .env.example .env` and fill in values
2. `docker compose up` — starts web, API, DB, ingest, signals
3. Web UI: http://localhost:3000
4. API: http://localhost:8080/health
```

Set `COINBASE_READ_API_KEY` and `COINBASE_READ_API_SECRET` for live read data. For CI or smoke tests without keys, use `BROKER_PROVIDER=mock` where supported.

## Do NOT

- Implement the Coinbase broker adapter or ingest scheduler (owned by SCH-20 / SCH-21).
- Create DB migrations or schema (owned by SCH-18).
- Build dashboard UI beyond the Next.js scaffold (owned by SCH-17).
- Add production infrastructure (TLS termination, reverse proxy config).

## Acceptance criteria

- [ ] `go build ./cmd/...` succeeds with placeholder mains.
- [ ] `docker compose config` validates without errors.
- [ ] `docker compose up db` starts Postgres and is reachable.
- [ ] `apps/web` dev server starts via `npm run dev` (or pnpm).
- [ ] CI workflow file is syntactically valid.
- [ ] `.env.example` documents every required env var.
- [ ] `.gitignore` excludes `.env`, `node_modules`, Go binaries.
- [ ] README has "Local Development" section.

## Files to create/modify

| File | Action |
|------|--------|
| `go.mod`, `go.sum` | Create |
| `cmd/portfolio-api/main.go` | Create (placeholder) |
| `cmd/ingest/main.go` | Create (placeholder) |
| `cmd/signals/main.go` | Create (placeholder) |
| `internal/broker/broker.go` | Create (interface stub) |
| `internal/portfolio/types.go` | Create (domain type stubs) |
| `internal/signal/types.go` | Create (domain type stubs) |
| `apps/web/` | Create (Next.js scaffold) |
| `docker-compose.yml` | Create |
| `docker-compose.override.yml` | Create |
| `.env.example` | Create |
| `.gitignore` | Create |
| `.github/workflows/ci.yml` | Create |
| `Dockerfile` | Create (multi-stage Go) |
| `apps/web/Dockerfile` | Create |
| `README.md` | Replace with full project README |
