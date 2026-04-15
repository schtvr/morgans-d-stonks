# AGENTS.md — morgans-d-stonks

## Project context

**Portfolio Platform** — US equities portfolio tracker built with Go services and a Next.js dashboard, connected to Interactive Brokers, deployed via Docker Compose on a homelab.

- **Linear project**: [Portfolio platform](https://linear.app/schtvr/project/portfolio-platform-1e44112535d4)
- **Stack**: Go (services), Next.js + Tailwind + shadcn/ui (dashboard), Postgres, Docker Compose
- **Broker**: Interactive Brokers (IBKR) — paper mode default, live via config flag

## Repository layout (target)

```
morgans-d-stonks/
├── AGENTS.md                    # this file
├── README.md
├── go.mod / go.sum
├── docker-compose.yml
├── docker-compose.override.yml
├── Dockerfile                   # multi-stage Go build
├── .env.example
├── .gitignore
├── .github/workflows/ci.yml
├── .agent/
│   └── epics/                   # agent instruction files (one per epic)
│       ├── SCH-19-foundation-homelab.md       # P0 Wave 1
│       ├── SCH-20-ibkr-connectivity.md        # P0 Wave 2
│       ├── SCH-18-portfolio-service.md        # P0 Wave 2
│       ├── SCH-17-dashboard.md                # P0 Wave 2–3
│       ├── SCH-21-ingest-snapshots.md         # P0 Wave 3
│       ├── SCH-16-signals-discord.md          # P0 Wave 4
│       ├── SCH-22-rich-alerts-dashboard-analytics.md  # P1 Wave 5
│       └── SCH-23-openclaw-mcp-alerts.md      # P1 Wave 5
├── apps/
│   └── web/                     # Next.js dashboard
├── cmd/
│   ├── portfolio-api/           # Go HTTP service
│   ├── ingest/                  # Go periodic job
│   ├── signals/                 # Go signal engine
│   └── openclaw-proxy/          # Go OpenClaw proxy (P1)
├── internal/
│   ├── broker/                  # Broker interface + IBKR/mock impls
│   ├── portfolio/               # Portfolio domain + persistence + history
│   ├── auth/                    # Auth logic + middleware
│   ├── ingest/                  # Ingest runner logic
│   ├── signal/                  # Signal engine + rules
│   ├── discord/                 # Discord webhook + rich alerts
│   ├── openclaw/                # OpenClaw client + proxy (P1)
│   ├── mcp/                     # MCP tool servers (P1)
│   │   ├── portfolio/           # Portfolio snapshot MCP
│   │   └── market/              # News/fundamentals MCP (stub)
│   └── config/                  # Shared config
├── config/
│   └── signals.yaml             # Signal rule definitions
└── pkg/                         # Public Go packages (if any)
```

## Agent instructions

Each P0 and P1 epic has a detailed agent instruction file in `.agent/epics/`. These files contain everything an agent needs: objective, scope, interface contracts, file lists, acceptance criteria, and explicit boundaries to avoid duplication.

### How to use the instruction files

1. Read the instruction file for your assigned epic.
2. Check the **Wave** number and **Depends on** fields to understand ordering.
3. Follow the **Scope** section for implementation details.
4. Respect the **Do NOT** section to avoid stepping on other epics.
5. Verify against the **Acceptance criteria** before marking done.
6. If you need to change a **Shared contract**, coordinate with the listed consuming epics.

## Execution waves (parallelism guide)

Epics are organized into waves based on their dependency graph. Agents working on the same wave can run **in parallel**.

```
Wave 1 ──────────────────────────────────────────────────────
│
│  SCH-19: Foundation & Homelab
│  (repo layout, Compose, CI, env contract)
│
Wave 2 ──────────────────────────────────────────────────────
│         (all three can run in parallel)
│
│  SCH-20: IBKR Connectivity     SCH-18: Portfolio Service     SCH-17: Dashboard (stylekit only)
│  (broker interface + impl)     (API, DB, auth)               (Next.js shell, theme, layout)
│
Wave 3 ──────────────────────────────────────────────────────
│
│  SCH-21: Ingest & Snapshots    SCH-17: Dashboard (data integration)
│  (periodic job, session gating) (auth pages, positions table, API calls)
│
Wave 4 ──────────────────────────────────────────────────────
│
│  SCH-16: Signals & Discord
│  (rule engine, dedup, webhook)
│
═══════════════════ P0 complete ═════════════════════════════

Wave 5 ──────────────────────────────────────────────────────
│         (both can run in parallel)
│
│  SCH-22: Rich Alerts &           SCH-23: OpenClaw, MCP &
│  Dashboard Analytics             Alert Intelligence
│  (rich Discord, charts, metrics) (proxy svc, MCP servers, circuit breaker)
│
```

### Parallelism rules

- **Same wave**: agents can work simultaneously without conflicts.
- **Cross-wave**: later waves depend on interfaces/contracts from earlier waves. If an earlier wave hasn't merged yet, code against the documented interface contracts in the instruction files — they are the source of truth.
- **SCH-17 (Dashboard)** spans two waves: the stylekit/shell work (Wave 2) has no backend dependency, but data integration (Wave 3) needs SCH-18's API.
- **P1 epics (Wave 5)**: require all P0 epics to be merged. SCH-22 and SCH-23 can run in parallel — SCH-22 extends the Discord client and dashboard, while SCH-23 builds the OpenClaw proxy. They share the `SignalEvent` type (P0 contract) but don't modify each other's files.

## Shared contracts

These are the integration points between epics. Agents must respect these contracts. Changes require updating all consuming instruction files.

### Broker interface (`internal/broker/broker.go`)

Owner: **SCH-20** | Consumers: SCH-21, SCH-16

```go
type Broker interface {
    Positions(ctx context.Context) ([]Position, error)
    AccountSummary(ctx context.Context) (*AccountSummary, error)
    Quotes(ctx context.Context, symbols []string) ([]Quote, error)
    IsMarketOpen(ctx context.Context) (bool, error)
    Close() error
}
```

### Portfolio API endpoints

Owner: **SCH-18** | Consumers: SCH-17, SCH-21

| Method | Path | Auth | Consumer |
|--------|------|------|----------|
| `POST` | `/api/auth/login` | Public | SCH-17 |
| `POST` | `/api/auth/logout` | Session | SCH-17 |
| `GET` | `/api/portfolio/positions` | Session | SCH-17 |
| `GET` | `/api/portfolio/summary` | Session | SCH-17 |
| `GET` | `/api/health` | Public | all |
| `POST` | `/internal/snapshots` | Internal key | SCH-21 |

### SignalEvent type

Owner: **SCH-16** | Consumers: SCH-22 (P1 rich alerts), SCH-23 (P1 OpenClaw)

```go
type SignalEvent struct {
    ID        string    `json:"id"`
    RuleID    string    `json:"ruleId"`
    RuleName  string    `json:"ruleName"`
    Symbol    string    `json:"symbol"`
    Signal    string    `json:"signal"`
    Value     float64   `json:"value"`
    Threshold float64   `json:"threshold"`
    FiredAt   time.Time `json:"firedAt"`
}
```

### Portfolio API — P1 time-series endpoints

Owner: **SCH-22** (extends SCH-18) | Consumers: SCH-22 dashboard charts

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/portfolio/history` | Session | Portfolio value over time (`interval`, `from`, `to` params) |
| `GET` | `/api/portfolio/positions/:symbol/history` | Session | Per-position price/value history |
| `GET` | `/api/portfolio/metrics` | Session | Computed period returns + drawdown |

### OpenClaw integration contract

Owner: **SCH-23** | Future consumer: SCH-24 (P2 auto-trade)

```go
type OpenClawRequest struct {
    RequestID     string            `json:"requestId"`
    Signal        signal.SignalEvent `json:"signal"`
    PortfolioCtx  PortfolioContext   `json:"portfolioCtx"`
    MCPTools      []string           `json:"mcpTools"`
    CreatedAt     time.Time          `json:"createdAt"`
}

type OpenClawResponse struct {
    RequestID      string    `json:"requestId"`
    Analysis       string    `json:"analysis"`
    Recommendation string    `json:"recommendation"`
    Confidence     float64   `json:"confidence"`
    ToolCalls      []ToolCall `json:"toolCalls"`
    CompletedAt    time.Time  `json:"completedAt"`
}
```

### MCP tool servers

Owner: **SCH-23** | Consumer: OpenClaw agent

| Tool | Server | Description |
|------|--------|-------------|
| `get_positions` | `mcp/portfolio` | Current portfolio positions |
| `get_account_summary` | `mcp/portfolio` | Account-level metrics |
| `get_position_detail` | `mcp/portfolio` | Detail for a specific symbol |
| `get_news` | `mcp/market` | Recent news for a symbol (stub for P1) |
| `get_fundamentals` | `mcp/market` | Basic fundamentals (stub for P1) |

### Environment variables

All env vars documented in `.env.example`. Each service reads only what it needs:

| Variable | Used by | Epic |
|----------|---------|------|
| `DATABASE_URL` | portfolio-api | SCH-18 |
| `IBKR_GATEWAY_HOST/PORT` | ingest (via broker) | SCH-20 |
| `IBKR_MODE` | ingest (via broker) | SCH-20 |
| `AUTH_SECRET` | portfolio-api | SCH-18 |
| `AUTH_USERNAME/PASSWORD` | portfolio-api | SCH-18 |
| `INTERNAL_API_KEY` | portfolio-api, ingest | SCH-18, SCH-21 |
| `DISCORD_WEBHOOK_URL` | signals | SCH-16 |
| `INGEST_INTERVAL` | ingest | SCH-21 |
| `SIGNAL_RULES_PATH` | signals | SCH-16 |
| `SIGNAL_COOLDOWN` | signals | SCH-16 |
| `NEXT_PUBLIC_API_URL` | web | SCH-17 |
| `OPENCLAW_API_URL` | openclaw-proxy | SCH-23 |
| `OPENCLAW_API_KEY` | openclaw-proxy | SCH-23 |
| `OPENCLAW_TIMEOUT` | openclaw-proxy | SCH-23 |

### Docker Compose service names

Owner: **SCH-19** | Used by all epics for inter-service networking.

| Service | Internal hostname | Port |
|---------|-------------------|------|
| `web` | `web` | 3000 |
| `portfolio-api` | `portfolio-api` | 8080 |
| `ingest` | `ingest` | — |
| `signals` | `signals` | — |
| `db` | `db` | 5432 |
| `ib-gateway` | `ib-gateway` | 4001 |
| `openclaw-proxy` | `openclaw-proxy` | 8090 |

## Coding standards

### Go

- Use `internal/` for all non-public packages.
- Standard `cmd/` layout with thin `main.go` entry points.
- Interfaces defined by consumer, not implementer (except `Broker` which is shared).
- Error wrapping with `fmt.Errorf("context: %w", err)`.
- Context propagation for all I/O operations.
- Structured logging (slog or zerolog — pick one, use consistently).

### TypeScript / Next.js

- App Router (not Pages Router).
- Server Components by default; `"use client"` only when needed.
- Tailwind for styling; no CSS modules.
- shadcn/ui components; don't reinvent existing ones.

### General

- Every new package/module should have at least one test file.
- Commits reference the Linear issue ID (e.g., `SCH-19: scaffold repo layout`).
- PRs should target `main` and pass CI before merge.
- No secrets in the repo; all via `.env` and `.env.example`.

## Linear tracking

- **Project**: Portfolio platform
- **Team**: Schtvr (key: `SCH`)
- **Milestones**: P0: MVP → P1: First follow-up → P2: Auto-trade
- When starting work on an epic, update the Linear issue status to **In Progress**.
- When opening a PR, link it to the Linear issue.
- When done, move the issue to **Done**.
