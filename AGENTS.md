# AGENTS.md — morgans-d-stonks

## Project context

**Portfolio Platform** — US equities portfolio tracker built with Go services and a Next.js dashboard, connected to Interactive Brokers, deployed via Docker Compose on a homelab.

- **Linear project**: [Portfolio platform](https://linear.app/schtvr/project/portfolio-platform-1e44112535d4)
- **Stack**: Go (services), Next.js + Tailwind + shadcn/ui (dashboard), Postgres, Docker Compose
- **Broker**: Interactive Brokers (IBKR) — paper mode default, live via config flag

## Repository layout (target)

```
morgans-d-stonks/
├── AGENTS.md
├── README.md
├── go.mod / go.sum
├── docker-compose.yml
├── docker-compose.override.yml
├── Dockerfile                   # multi-stage Go build
├── .env.example
├── .gitignore
├── .github/workflows/ci.yml
├── .agent/
│   └── epics/
│       ├── phase_1/             # P0 (MVP) epics
│       │   ├── SCH-19-foundation-homelab.md
│       │   ├── SCH-20-ibkr-connectivity.md
│       │   ├── SCH-18-portfolio-service.md
│       │   ├── SCH-17-dashboard.md
│       │   ├── SCH-21-ingest-snapshots.md
│       │   └── SCH-16-signals-discord.md
│       └── phase_2/             # P1 (first follow-up) epics
│           ├── SCH-22-rich-alerts-dashboard-analytics.md
│           └── SCH-23-openclaw-mcp-alerts.md
├── apps/
│   └── web/                     # Next.js dashboard
├── cmd/
│   ├── portfolio-api/
│   ├── ingest/
│   ├── signals/
│   └── openclaw-proxy/          # P1
├── internal/
│   ├── broker/
│   ├── portfolio/
│   ├── auth/
│   ├── ingest/
│   ├── signal/
│   ├── discord/
│   ├── openclaw/                # P1
│   ├── mcp/                     # P1
│   │   ├── portfolio/
│   │   └── market/
│   └── config/
├── config/
│   └── signals.yaml
└── pkg/
```

## Agent skills

### How to read the epic files

1. Read the instruction file for your assigned epic under `.agent/epics/phase_1/` or `phase_2/`.
2. Check the **Wave** and **Depends on** fields to understand ordering.
3. Follow **Scope** for implementation details; respect **Do NOT** to avoid conflicts.
4. Verify every item in **Acceptance criteria** before marking done.
5. If a **Shared contract** must change, update all listed consuming epics.

### Git workflow

- Branch name format: `cursor/<issue-id>-<short-description>-<4-char-suffix>` (e.g. `cursor/SCH-19-foundation-a4ba`).
- One logical change per commit; reference the Linear issue ID in the message (e.g. `SCH-19: scaffold repo layout`).
- Push and open a PR targeting `main`; PRs must pass CI before merge.
- No force-pushes or amended commits unless explicitly asked.

### Secrets & environment

- Never commit secrets. All config via `.env` (gitignored) and `.env.example`.
- See `.env.example` for the full list of required variables.
- Use the `config` package for loading env; never read `os.Getenv` directly in business logic.

### Testing

- Every new package must have at least one `_test.go` file (Go) or `*.test.ts` file (TS).
- Run `go test ./...` before pushing Go changes.
- Run `pnpm test` (or `npm test`) before pushing dashboard changes.
- CI runs the same checks; a failing CI blocks merge.

### Parallelism

- Same-wave epics can be worked in parallel without conflicts.
- Cross-wave: code against the interface contracts in the instruction files if an earlier epic hasn't merged yet — contracts are the source of truth.
- P1 epics (phase_2) require all P0 epics merged.

## Execution waves

```
Phase 1 (P0) ──────────────────────────────────────────────
│
│  Wave 1:  SCH-19  Foundation & Homelab
│
│  Wave 2:  SCH-20  IBKR Connectivity
│           SCH-18  Portfolio Service        (parallel)
│           SCH-17  Dashboard — stylekit
│
│  Wave 3:  SCH-21  Ingest & Snapshots
│           SCH-17  Dashboard — data integration
│
│  Wave 4:  SCH-16  Signals & Discord
│
Phase 2 (P1) ──────────────────────────────────────────────
│
│  Wave 5:  SCH-22  Rich Alerts & Analytics  (parallel)
│           SCH-23  OpenClaw, MCP & Alerts
│
```

## Shared contracts

Changes to any contract require updating all listed consuming epics.

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

P1 extensions (owner: **SCH-22**):

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/portfolio/history` | Session | Portfolio value over time |
| `GET` | `/api/portfolio/positions/:symbol/history` | Session | Per-position history |
| `GET` | `/api/portfolio/metrics` | Session | Period returns + drawdown |

### SignalEvent type

Owner: **SCH-16** | Consumers: SCH-22, SCH-23

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

### OpenClaw contract

Owner: **SCH-23** | Future consumer: SCH-24 (P2)

```go
type OpenClawRequest struct {
    RequestID    string             `json:"requestId"`
    Signal       signal.SignalEvent `json:"signal"`
    PortfolioCtx PortfolioContext    `json:"portfolioCtx"`
    MCPTools     []string           `json:"mcpTools"`
    CreatedAt    time.Time          `json:"createdAt"`
}

type OpenClawResponse struct {
    RequestID      string     `json:"requestId"`
    Analysis       string     `json:"analysis"`
    Recommendation string     `json:"recommendation"`
    Confidence     float64    `json:"confidence"`
    ToolCalls      []ToolCall `json:"toolCalls"`
    CompletedAt    time.Time  `json:"completedAt"`
}
```

### MCP tools

Owner: **SCH-23** | Consumer: OpenClaw agent

| Tool | Server | Description |
|------|--------|-------------|
| `get_positions` | `mcp/portfolio` | Current positions |
| `get_account_summary` | `mcp/portfolio` | Account metrics |
| `get_position_detail` | `mcp/portfolio` | Detail for a symbol |
| `get_news` | `mcp/market` | Recent news (stub) |
| `get_fundamentals` | `mcp/market` | Basic fundamentals (stub) |

### Environment variables

| Variable | Service | Epic |
|----------|---------|------|
| `DATABASE_URL` | portfolio-api | SCH-18 |
| `IBKR_GATEWAY_HOST/PORT` | ingest | SCH-20 |
| `IBKR_MODE` | ingest | SCH-20 |
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

- `internal/` for all non-public packages; thin `main.go` entry points in `cmd/`.
- Interfaces defined by the consumer (except the shared `Broker`).
- Error wrapping: `fmt.Errorf("context: %w", err)`.
- Context propagation for all I/O.
- Structured logging via `slog` (standard library) — use consistently across all services.

### TypeScript / Next.js

- App Router; Server Components by default — `"use client"` only when needed.
- Tailwind for styling; no CSS modules.
- shadcn/ui components; don't reinvent existing ones.

## Linear tracking

- **Team**: Schtvr (`SCH`) | **Milestones**: P0 MVP → P1 Follow-up → P2 Auto-trade
- Mark issues **In Progress** when starting, **Done** when the PR merges.
- Link PRs to their Linear issue.
