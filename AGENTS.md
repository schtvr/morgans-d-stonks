# AGENTS.md — morgans-d-stonks

## Project context

**Portfolio Platform** — Homelab **crypto** portfolio and watchlist dashboard with Go services and Next.js, fed by **Coinbase** read APIs, deployed via Docker Compose.

- **Stack**: Go (services), Next.js + Tailwind + shadcn/ui (dashboard), Postgres, Docker Compose
- **Broker**: **Coinbase** (CDP read keys for ingest/signals); optional in-process `BROKER_PROVIDER=mock` for tests and local smoke without keys

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
│   ├── epics/
│   │   ├── phase_1/             # P0 (MVP) epics
│   │   │   ├── foundation-homelab.md
│   │   │   ├── coinbase-connectivity.md
│   │   │   ├── portfolio-service.md
│   │   │   ├── dashboard.md
│   │   │   ├── ingest-snapshots.md
│   │   │   ├── signals-discord.md
│   │   │   └── logging/         # P1 cross-cutting (stdout JSON for Loki)
│   │   │       ├── epic_P1_logging.md
│   │   │       └── stories/
│   │   └── phase_2/             # P1 (first follow-up) epics
│   │       ├── rich-alerts-dashboard-analytics.md
│   │       └── openclaw-mcp-alerts.md
│   └── skills/                  # Short agent checklists (read with assigned epic)
│       └── logging.md
├── apps/
│   └── web/                     # Next.js dashboard
├── cmd/
│   ├── portfolio-api/
│   ├── ingest/
│   ├── signals/
│   └── agent-worker/            # P1.5 (SCH-AG1, runs inside signals)
├── internal/
│   ├── broker/
│   ├── portfolio/
│   ├── auth/
│   ├── ingest/
│   ├── signal/
│   ├── discord/
│   ├── logging/                 # P1 shared slog setup (epic_P1_logging)
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

1. Read the instruction file for your assigned epic under `.agent/epics/phase_1/` or `phase_2/` (including nested dirs such as `phase_1/logging/`).
2. Check the **Wave** and **Depends on** fields to understand ordering.
3. Follow **Scope** for implementation details; respect **Do NOT** to avoid conflicts.
4. Verify every item in **Acceptance criteria** before marking done.
5. If a **Shared contract** must change, update all listed consuming epics.

### Optional skills (checklists)

For structured logging work, read `.agent/skills/logging.md` alongside `.agent/epics/phase_1/logging/epic_P1_logging.md`.

### Git workflow

- Branch name format: `cursor/<issue-id>-<short-description>-<4-char-suffix>` (e.g. `cursor/42-foundation-a4ba`).
- One logical change per commit; reference the GitHub issue in the message or PR when applicable.
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
│  Wave 2:  SCH-20  Coinbase connectivity
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
│           SCH-AG1 The Agent — shadow decisions
│           P1 logging (stdout JSON / Loki) — see `phase_1/logging/epic_P1_logging.md`
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
| `GET` | `/api/market/candles` | Session | Coinbase product candles (`symbol`, `range`) for charting |
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

### The Agent contract

Owner: **SCH-AG1** | Consumers: `signals` (writer), `portfolio-api` (storer), dashboard (reader)

```go
type Action string

const (
    ActionBuy    Action = "buy"
    ActionSell   Action = "sell"
    ActionIgnore Action = "ignore"
)

type TriggerKind string

const (
    TriggerSignal TriggerKind = "signal"
    TriggerDaily  TriggerKind = "daily"
)

type DecisionRequest struct {
    TriggerKind        TriggerKind          `json:"triggerKind"`
    TriggerAt          time.Time            `json:"triggerAt"`
    IdempotencyKey     string               `json:"idempotencyKey"` // signal.ID or "daily-YYYY-MM-DD"
    Signal             *signal.CryptoAlert  `json:"signal,omitempty"`
    EagerContext       EagerContext         `json:"eagerContext"`
    PromptVersion      string               `json:"promptVersion"` // sha256 prefix of agent-prompt.md
}

type EagerContext struct {
    PortfolioSummary      PortfolioSummaryLine `json:"portfolioSummary"`
    DecisionsForSymbol24h *int `json:"decisionsForSymbol24h,omitempty"`
}

type Decision struct {
    Action             Action     `json:"action"`
    Confidence         float64    `json:"confidence"`
    Rationale          string     `json:"rationale"`
    SizingHintNotional *float64   `json:"sizingHintNotional,omitempty"`
    ToolCalls          []ToolCall `json:"toolCalls"`
    Model              string     `json:"model"`
    LatencyMS          int64      `json:"latencyMs"`
    CostCents          int64      `json:"costCents"`
}

type ToolCall struct {
    Name       string          `json:"name"`
    Input      json.RawMessage `json:"input"`
    Output     json.RawMessage `json:"output"`
    DurationMS int64           `json:"durationMs"`
}
```

### Agent MCP tool catalog

Owner: **SCH-AG1** | Consumer: The Agent (via `mcp-go` stdio session)

| Tool | Input | Output (capped) | Backed by |
|------|-------|-----------------|-----------|
| `get_market_candles` | `{ symbol, window: "1h"\|"24h"\|"7d"\|"30d" }` | up to **200** OHLCV points | `coinbase.Client.FetchProductCandles` |
| `get_holdings` | `{}` | snapshot positions + summary + `stale` flag | `repo.LatestSnapshot` |
| `get_position` | `{ symbol }` | one position w/ qty, avgCost, marketValue, unrealizedPL | `repo.LatestSnapshot` |
| `get_recent_signals` | `{ symbol?, limit<=50, window<=72h }` | RecentAlert rows | `repo.ListRecentAlerts` |
| `get_recent_decisions` | `{ symbol?, limit<=20, window<=72h }` | AgentDecision rows w/o full toolCalls | `repo.ListAgentDecisions` |
| `get_decision_outcomes` | `{ symbol?, horizon: "24h"\|"7d"\|"14d", limit<=50 }` | scored outcomes (return %, excess vs BTC) | `repo.ListAgentDecisionOutcomes` |
| `get_correlated_symbols` | `{ symbol }` | `[symbol, "BTC-USD", "ETH-USD", ...top3MV...]` (deduped) | pure derivation from snapshot |

### MCP response contracts

All portfolio MCP tool responses share these rules.

#### Correlation (MUST)

Every request MUST supply `portfolioId`; v0 server default = env `PORTFOLIO_ID` or `"default"`. Every successful response MUST echo it back.

#### Error envelope (MUST)

Tool errors MUST return:

```json
{
  "error": {
    "code": "<string>",
    "message": "<human-readable string>"
  }
}
```

Standard codes:

| Code | Meaning |
|------|---------|
| `not_found` | No snapshot / symbol / policy exists |
| `snapshot_stale` | Snapshot age exceeds staleness threshold |
| `upstream_failure` | DB or broker call failed |
| `unauthorized` | Caller not permitted for this `portfolioId` |

#### Freshness (holdings, MUST)

Holdings responses MUST include:

- `snapshotAge` — integer seconds since `takenAt` at read time
- `stale` — boolean; `true` when `snapshotAge > INGEST_INTERVAL_SECONDS × 3`

If `stale: true`, the agent SHOULD log a warning and may abort execution. The MCP server MUST NOT silently substitute live Coinbase data — surface staleness and let the agent decide.

#### Response versioning (MUST)

- Holdings tools: `"schemaVersion": "holdings_v1"` in every successful response
- Policy tool: `"schemaVersion": "policy_v1"` in every successful response
- Increment the version suffix on any breaking field change

#### Policy provenance (SHOULD)

`get_policy` responses SHOULD include:

```json
"provenance": {
  "configHash": "sha256:<first-8-hex of sha256 of serialized TRADING_* block>",
  "asOf": "<RFC3339 process startup time>"
}
```

`asOf` = process startup time (env snapshot is immutable at runtime). Enables replay of "what policy was active when this trade happened."

#### Units and normalization (MUST)

| Dimension | Canonical form |
|-----------|---------------|
| `symbol` | Coinbase product_id format: `BASE-QUOTE`, e.g. `BTC-USD` |
| Prices / values | `float64`, quoted currency (USD unless `currency` field differs) |
| Quantities | `float64`, base asset units |
| `currency` | ISO 4217 string, e.g. `"USD"` |
| Timestamps | RFC3339 UTC, e.g. `"2026-05-15T12:00:00Z"` |
| Durations | Go duration string, e.g. `"5m"`, `"1h"` |

#### Pagination / size limits

Holdings tools MUST cap responses at **200 positions**. v0 has no cursor; if the cap is hit, include `"truncated": true`. Homelab portfolios are expected well under this limit.

### MCP trade tool: `place_trade`

Owner: **SCH-23 / SCH-24** | Consumer: OpenClaw agent | Gated by: `TRADING_ENABLED=true`

This tool is the **agent-facing execution entry point**. It maps directly to `POST /mcp/v1/trades/create` on `portfolio-api` (internal network, `X-Internal-Key` auth). Policy runs synchronously inside the service before the order is persisted.

#### Request (`trade_request_v1`)

```json
{
  "schema_version": "v1",
  "idempotency_key": "<signal-id>-<portfolioId>",
  "order": {
    "product_id": "BTC-USD",
    "side": "buy",
    "type": "market",
    "base_size": 0.001
  }
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `schema_version` | string | ✓ | Must be `"v1"` (`MCP_SCHEMA_VERSION` env) |
| `idempotency_key` | string | ✓ | Stable per intent; recommend `{signal.id}-{portfolioId}`. Server rejects reuse with different payload (409). |
| `order.product_id` | string | ✓ | Coinbase product_id format: `BTC-USD` |
| `order.side` | string | ✓ | `"buy"` or `"sell"` |
| `order.type` | string | ✓ | Only `"market"` supported in v0 |
| `order.base_size` | float64 | — | Quantity in base asset units (e.g. BTC). Exactly one of `base_size` or `quote_size` required. |
| `order.quote_size` | float64 | — | Quantity in quote currency (e.g. USD). |

#### Success response (`201 Created`)

```json
{
  "order": {
    "id": "<uuid>",
    "symbol": "BTC-USD",
    "side": "buy",
    "quantity": 0.001,
    "notional": 0.001,
    "status": "accepted",
    "idempotencyKey": "<signal-id>-default",
    "provider": "coinbase",
    "createdAt": "2026-05-15T12:00:00Z",
    "updatedAt": "2026-05-15T12:00:00Z"
  },
  "decision": {
    "allowed": true,
    "reasonCodes": [],
    "notional": 0.001,
    "checkedAt": "2026-05-15T12:00:00Z"
  }
}
```

#### Rejected by policy (`201 Created`, `status: "rejected"`)

When policy blocks the order it is still persisted (audit trail) but `status = "rejected"` and `decision.allowed = false`:

```json
{
  "order": { "status": "rejected", "reason": "kill_switch,max_notional", ... },
  "decision": { "allowed": false, "reasonCodes": ["kill_switch"], ... }
}
```

**The agent MUST check `decision.allowed` — HTTP 201 does not mean the order was sent to the broker.**

#### Policy reason codes

| Code | Cause |
|------|-------|
| `kill_switch` | `TRADING_KILL_SWITCH=true` |
| `provider_not_allowed` | Broker not in `TRADING_ALLOWED_PROVIDERS` |
| `symbol_not_allowed` | Symbol not in `TRADING_ALLOWED_SYMBOLS` |
| `symbol_denied` | Symbol in `TRADING_DENIED_SYMBOLS` |
| `max_notional` | Order size exceeds `TRADING_MAX_NOTIONAL` |
| `reserve` | Trade would breach `TRADING_RESERVE` cash floor |
| `global_max_exposure` | Open buy notional would exceed `TRADING_GLOBAL_MAX_EXPOSURE` |
| `symbol_cooldown` | Recent open order for same symbol within `TRADING_SYMBOL_COOLDOWN` |
| `no_shorting` | Sell quantity exceeds net long position |

#### HTTP error codes

| Status | Meaning |
|--------|---------|
| `400` | Validation failure (bad schema_version, missing field, invalid side) |
| `409` | `idempotency_key` reused with different payload |
| `404` | Route not registered (`TRADING_ENABLED=false`) |

#### Validation-only endpoint

`POST /mcp/v1/trades/validate` — same request shape, returns `decision` without persisting or touching the broker. Agent SHOULD call this before `create` when it wants a dry-run policy check.

#### Order lifecycle

`new` → (`accept` / `reject`) → `accepted` / `rejected` → (`fill` / `cancel`) → `filled` / `canceled`

The trading-worker polls `accepted` orders and submits them to Coinbase; status transitions are written as immutable `OrderEvent` records.

### Environment variables

| Variable | Service | Epic |
|----------|---------|------|
| `DATABASE_URL` | portfolio-api | SCH-18 |
| `COINBASE_READ_API_KEY` / `COINBASE_READ_API_SECRET` | ingest, signals | SCH-20 |
| `BROKER_PROVIDER` / `BROKER_ENV` | ingest, portfolio-api, signals | SCH-20 |
| `AUTH_SECRET` | portfolio-api | SCH-18 |
| `AUTH_USERNAME/PASSWORD` | portfolio-api | SCH-18 |
| `INTERNAL_API_KEY` | portfolio-api, ingest | SCH-18, SCH-21 |
| `DISCORD_WEBHOOK_URL` | signals | SCH-16 |
| `INGEST_INTERVAL` | ingest | SCH-21 |
| `SIGNAL_RULES_PATH` | signals | SCH-16 |
| `SIGNAL_COOLDOWN` | signals | SCH-16 |
| `TRADING_ENABLED`, `TRADING_KILL_SWITCH`, `TRADING_MAX_NOTIONAL`, `TRADING_RESERVE`, `TRADING_GLOBAL_MAX_EXPOSURE`, `TRADING_SYMBOL_COOLDOWN`, `TRADING_ALLOWED_PROVIDERS`, `TRADING_ALLOWED_SYMBOLS`, `TRADING_DENIED_SYMBOLS` | portfolio-api, trading-worker | SCH-23 policy projection (`get_policy` v0) |
| `NEXT_PUBLIC_API_URL` | web | SCH-17 |
| `PORTFOLIO_ID` | portfolio-api, mcp/portfolio | SCH-23 (correlation; defaults to `"default"`) |
| `AGENT_ENABLED` | signals | SCH-AG1 |
| `AGENT_PROVIDER` | signals | SCH-AG1 |
| `AGENT_MODEL` | signals | SCH-AG1 |
| `ANTHROPIC_API_KEY` | signals | SCH-AG1 |
| `AGENT_DAILY_COST_CAP_USD` | signals | SCH-AG1 |
| `AGENT_DAILY_TIMER_UTC` | signals | SCH-AG1 |
| `AGENT_CONCURRENCY` | signals | SCH-AG1 |
| `AGENT_PROMPT_PATH` | signals | SCH-AG1 |
| `LOG_LEVEL` | all Go services | P1 logging epic |
| `APP_VERSION` | all Go services (optional) | P1 logging epic |

### Docker Compose service names

| Service | Internal hostname | Port |
|---------|-------------------|------|
| `web` | `web` | 3000 |
| `portfolio-api` | `portfolio-api` | 8080 |
| `ingest` | `ingest` | — |
| `signals` | `signals` | — |
| `db` | `db` | 5432 |

## Coding standards

### Go

- `internal/` for all non-public packages; thin `main.go` entry points in `cmd/`.
- Interfaces defined by the consumer (except the shared `Broker`).
- Error wrapping: `fmt.Errorf("context: %w", err)`.
- Context propagation for all I/O.
- Structured logging via `slog` (standard library) — use consistently across all services; shared root logger setup lives in `internal/logging` (see `.agent/epics/phase_1/logging/epic_P1_logging.md`).

### TypeScript / Next.js

- App Router; Server Components by default — `"use client"` only when needed.
- Tailwind for styling; no CSS modules.
- shadcn/ui components; don't reinvent existing ones.
