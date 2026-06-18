# morgans-d-stonks

Local homelab **crypto portfolio and watchlist** dashboard with deterministic **Coinbase** price-move alerts and an in-process LLM agent. OpenClaw integration has been removed; decisions are handled by the **in-process agent** running inside `signals`. **Target posture:** **closed-loop** signal handling and trading (**no human-in-the-loop**), gated only by software (`TRADING_*`, kill switch). Optional **order execution** is behind `TRADING_ENABLED` and trade keys — not the default MVP compose path.

## Stack

- **Backend**: Go in `backend/` — `portfolio-api`, `ingest`, `signals`
- **Frontend**: Next.js + Tailwind + shadcn/ui in `frontend/`
- **Market data**: Coinbase read API (balances + spot prices for snapshots and signal quotes)
- **DB**: Postgres 16
- **Infra**: Docker Compose

## Architecture

Services share `portfolio-net`. The dashboard talks to `portfolio-api`. `ingest` writes snapshots via the internal API. `signals` reads followed symbols and settings, evaluates price moves, persists recent alerts, and optionally posts to Discord.

```mermaid
flowchart TB
  subgraph Clients
    B[Browser]
  end

  subgraph Compose["Docker Compose (MVP)"]
    W[web<br/>Next.js :3000]
    API[portfolio-api<br/>Go :8080]
    DB[(Postgres :5432)]
    ING[ingest<br/>Go]
    SIG[signals<br/>Go]
  end

  DC[Discord]

  B -->|session / dashboard| W
  W -->|HTTP API| API
  API --> DB
  ING -->|Coinbase read| CB[Coinbase API]
  ING -->|internal snapshots| API
  SIG -->|followed symbols + quotes| CB
  SIG -->|internal API| API
  SIG -.->|webhook (trade outcomes only)| DC
```

> **Note**: `signals` does not post Discord messages. The dashed `SIG → DC` arrow above represents `portfolio-api` sending trade outcome webhooks when `DISCORD_WEBHOOK_URL` is set — signals itself has no Discord code. Alert firing persists to Postgres via `POST /internal/recent-alerts`.

## Local development

**Quick path (interactive):**

```bash
./scripts/quickstart.sh
```

Prompts for API keys and secrets, then builds in **dry** (monitor only), **paper** (simulated trades), or **live** (real trade keys — live broker not implemented yet; see [docs/plans/live-execution-broker.md](docs/plans/live-execution-broker.md)).

**Manual path:**

1. `cp .env.example .env` and set **`DATABASE_URL`**, **`INTERNAL_API_KEY`**, **`COINBASE_READ_API_KEY`**, **`COINBASE_READ_API_SECRET`**, and optional **`DISCORD_WEBHOOK_URL`** (portfolio-api trade outcome webhooks only).
2. `docker compose up --build` — starts **web**, **portfolio-api**, **ingest**, **signals**, and **Postgres** (no optional **trading-worker** in the default stack).
3. Open http://localhost:3000 — sign in with **`AUTH_USERNAME`** / **`AUTH_PASSWORD`** from `.env`.
4. API health: http://localhost:8080/api/health

Use Node **22.22.0** for local frontend work (see `frontend/.nvmrc`).

### Crypto alerts and the in-process agent

- The **signals** service uses persisted **alert settings** (threshold %, cooldown) and the **watchlist**.
- Each gate-passed alert is persisted to Postgres via `POST /internal/recent-alerts` and triggers the **in-process LLM agent** (when `AGENT_ENABLED=true`). No Discord messages are sent by signals.
- **Recent alerts** are stored in Postgres (including the exact JSON payload in **`payload_json`**) for dashboard review and replay.
- **`DISCORD_WEBHOOK_URL`** is consumed by `portfolio-api` only, for trade outcome notifications.

### Backlog (not default)

- **trading-worker** is not part of the default MVP compose story (enable with `TRADING_ENABLED=true` when you intentionally want execution).
- **Order execution** (`/internal/orders`, `/mcp/v1/trades`, `trading-worker`) is implemented in-repo but **disabled** unless **`TRADING_ENABLED=true`**. To experiment, merge the optional override:

  `docker compose -f docker-compose.yml -f docker-compose.coinbase.yml up -d --build`

  See [docs/runbooks/coinbase-trading.md](docs/runbooks/coinbase-trading.md) and [docs/mcp-crypto-execution-spec.md](docs/mcp-crypto-execution-spec.md).

### Stylekit (dashboard)

The UI uses **Tailwind** and **shadcn/ui**. To add primitives:

```bash
cd frontend
npx shadcn@latest add dialog
```

## Project structure

```
backend/         Go backend module
frontend/        Next.js dashboard
.agent/epics/    Historical agent/epic notes (not the live MVP spec)
docs/            Runbooks and MVP verification notes
```

See [AGENTS.md](AGENTS.md) for contributor workflow. **Post-deploy verification** and payload notes: [docs/crypto-mvp-refactor.md](docs/crypto-mvp-refactor.md).
