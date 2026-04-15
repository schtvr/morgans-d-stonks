# morgans-d-stonks

US equities portfolio tracker — Go services + Next.js dashboard, connected to Interactive Brokers, running on a homelab via Docker Compose.

## Stack

- **Backend**: Go services (`portfolio-api`, `ingest`, `signals`)
- **Frontend**: Next.js 14 + Tailwind + shadcn/ui
- **Broker**: Interactive Brokers (paper mode by default)
- **DB**: Postgres 16
- **Infra**: Docker Compose

## Local development

```bash
cp .env.example .env   # fill in values
docker compose up      # starts web, API, DB, IB Gateway stub
```

- Web UI: http://localhost:3000
- API: http://localhost:8080/api/health

Set `IBKR_MODE=mock` to run without a live IB Gateway.  
For a gateway running on the host: `IBKR_GATEWAY_HOST=host.docker.internal`.

## Project structure

```
apps/web/        Next.js dashboard
cmd/             Go service entry points
internal/        Go business logic (not public)
config/          Runtime config files (e.g. signals.yaml)
.agent/epics/    Agent instruction files per epic
```

See [AGENTS.md](AGENTS.md) for the full architecture, shared contracts, and agent workflow.

## Linear

[Portfolio platform project](https://linear.app/schtvr/project/portfolio-platform-1e44112535d4) — team `SCH`.
