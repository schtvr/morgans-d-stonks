# SCH-23: OpenClaw, MCP & Alert Intelligence

> **SUPERSEDED** by [`agent-shadow-decisions.md`](./agent-shadow-decisions.md) (2026-05-18).
> The Agent now runs **in-process** inside `signals` with a local RO MCP server.
> External OpenClaw integration is dropped. The `lab_openclaw_runs` table is preserved
> for historical data but is no longer enqueued. Read the new epic + the plan at
> `docs/superpowers/plans/2026-05-18-agent-shadow-decisions.md` before doing any
> work in this area. The content below is retained for history only.

> **Linear**: [SCH-23](https://linear.app/schtvr/issue/SCH-23/epic-p1-openclaw-mcp-and-alert-intelligence)
> **Milestone**: P1: First follow-up
> **Wave**: 5 (all P0 complete; parallel with SCH-22)
> **Depends on**: P0 complete — specifically SCH-16 (SignalEvent), SCH-18 (portfolio API), SCH-21 (snapshots)

## Objective

Route structured `SignalEvent` traffic into OpenClaw with relevant MCP tools (portfolio snapshot, policy, fundamentals/news), hardening the proxy path. **Discord is the machine channel** (OpenClaw consumes fenced JSON); humans **audit via dashboard/logs only**. **Product target:** **no human-in-the-loop** for signal handling or trading — **closed loop** `signal → OpenClaw → execution` gated only by **software** policy (`TRADING_*`, kill switch, limits). SCH-23 owns proxy + MCP; **order submission** may live in **SCH-24** / `trading-worker` but MUST honor the same policy envelope and MUST NOT require human approval in Discord.

## Scope

### Architecture overview

```
Signal Engine (P0)          OpenClaw Proxy           OpenClaw
┌──────────┐    SignalEvent   ┌──────────────┐        ┌──────────┐
│ SCH-16   │ ──────────────► │  SCH-23      │ ────►  │ OpenClaw │
│ signals  │                 │  proxy svc   │ ◄────  │ agent    │
└──────────┘                 │              │        │          │
                             │  MCP servers │        │ MCP      │
                             │  ┌─────────┐ │        │ client   │
                             │  │portfolio│ │ ◄──────│          │
                             │  │snapshot │ │        └──────────┘
                             │  ├─────────┤ │
                             │  │news/    │ │
                             │  │fundmtls │ │
                             │  └─────────┘ │
                             └──────────────┘
                                    │
                                    ▼
                             Observability & audit
                             (Discord transcript, logs, dashboard — not an approval gate)
```

### OpenClaw proxy service (`cmd/openclaw-proxy/`)

A new Go service that:

1. **Receives** `SignalEvent` payloads from the signal engine.
2. **Enriches** with portfolio context (positions, account summary) via the portfolio API.
3. **Forwards** to OpenClaw with MCP tool definitions attached.
4. **Receives** the agent's analysis/recommendation.
5. **Emits** structured artifacts for OpenClaw (Discord fenced JSON as the default bus) and **forwards** agent output toward **execution** when enabled — **no human approval step**; humans observe via logs/dashboard/Discord history only.

**Delivery mechanism** — pick one and document:

- **Option A**: HTTP endpoint that the signal engine calls directly (synchronous, simpler).
- **Option B**: Message queue (Redis pub/sub, NATS, or a simple Go channel-based async boundary) so the signal pipeline is never blocked by OpenClaw latency.

Recommendation: **Option B** (async) for resilience. If OpenClaw is slow or down, the signal pipeline keeps running. Implement with a simple in-process queue for P1; can swap to external queue later.

### Integration contract

**Request to OpenClaw**:

```go
type OpenClawRequest struct {
    RequestID     string            `json:"requestId"`     // idempotency key (UUID)
    Signal        signal.SignalEvent `json:"signal"`
    PortfolioCtx  PortfolioContext   `json:"portfolioCtx"`
    MCPTools      []string           `json:"mcpTools"`      // tool names available
    CreatedAt     time.Time          `json:"createdAt"`
}

type PortfolioContext struct {
    Positions      []broker.Position      `json:"positions"`
    AccountSummary *broker.AccountSummary `json:"accountSummary"`
    SnapshotAt     time.Time              `json:"snapshotAt"`
}
```

**Response from OpenClaw**:

```go
type OpenClawResponse struct {
    RequestID     string    `json:"requestId"`
    Analysis      string    `json:"analysis"`      // agent's reasoning
    Recommendation string  `json:"recommendation"` // action suggestion
    Confidence    float64   `json:"confidence"`     // 0.0–1.0
    ToolCalls     []ToolCall `json:"toolCalls"`     // MCP tools the agent used
    CompletedAt   time.Time  `json:"completedAt"`
}

type ToolCall struct {
    Tool     string          `json:"tool"`
    Input    json.RawMessage `json:"input"`
    Output   json.RawMessage `json:"output"`
    Duration time.Duration   `json:"duration"`
}
```

**Timeouts & retries**:

- Request timeout: 30 seconds (configurable).
- Retry: 1 retry with exponential backoff on transient errors (5xx, timeout).
- Circuit breaker: after 3 consecutive failures, stop forwarding for 5 minutes (log warnings).

### MCP servers

Implement MCP tool servers that OpenClaw's agent can call:

#### Portfolio snapshot MCP (`internal/mcp/portfolio/`)

Exposes portfolio data as MCP tools. **v0 semantics:** tools return the **latest ingest snapshot** from Postgres (same backing data as `GET /api/portfolio/positions` / summary), not a live Coinbase call per invocation. Each response includes `source: "ingest_snapshot"` and `timestamp` (snapshot `takenAt`). Live exchange reads are a future opt-in with a distinct `source`.

| Tool | Description | Input | Output |
|------|-------------|-------|--------|
| `get_positions` | Holdings as of last ingest | `{ portfolioId }` (required once multi-account exists; v0 may omit if single-tenant) | `{ source, timestamp, positions: [...] }` — positions use broker domain fields (`symbol`, `quantity`, `avgCost`, `marketValue`, …) |
| `get_account_summary` | Account-level metrics from same snapshot | `{}` or `{ portfolioId }` | `{ source, timestamp, summary: {...} }` |
| `get_position_detail` | Detail for a specific symbol | `{ symbol: "BTC-USD" }` | `{ source, timestamp, position: {...}, history: [...] }` |
| `get_policy` | **Execution-only:** trading limits + rollout flags (**env-derived v0**) | `{}` or `{ portfolioId }` | `{ source: "env_derived", freshness: { asOf }, tradingEnabled, killSwitch, maxNotional, reserve, globalMaxExposure, symbolCooldown, allowedProviders, allowedSymbols, deniedSymbols, constraints[] }` — **exclude** `SIGNAL_*` / ingest / alert-rule config |

Implementation: thin HTTP wrapper around the portfolio API (SCH-18) or shared repository read — **must not** call Coinbase REST on each MCP tool tick in v0. **`get_policy`** reads the same **`TRADING_*`** env as `internal/config/trading.go` (plus broker provider context); **no DB policy store in v0**. **`get_policy` is execution-only:** do not surface signals/ingest env (`SIGNAL_*`, `INGEST_*`, rules files). `freshness.asOf` = wall time at read.

#### News/fundamentals MCP (`internal/mcp/market/`)

Stub for P1 — document the interface, implement with mock data or a simple external API:

| Tool | Description | Input | Output |
|------|-------------|-------|--------|
| `get_news` | Recent news for a symbol | `{ symbol: "AAPL", limit: 5 }` | `{ articles: [...] }` |
| `get_fundamentals` | Basic fundamentals | `{ symbol: "AAPL" }` | `{ pe: 28.5, marketCap: "2.8T", ... }` |

Implementation options for P1:
- Mock data (sufficient for testing the pipeline).
- Free API integration (Alpha Vantage, Finnhub — document choice if real).

### Automation, execution handoff, and audit

- **No HITL:** Do not require a human click/emoji/reply in Discord for trades or signal handling to proceed.
- **Software gates only:** `TRADING_KILL_SWITCH`, `get_policy` / `TRADING_*` limits, symbol allow/deny lists, and worker-side validation are the **sole** hard stops before orders hit the broker.
- **Discord:** Treat as **durable machine-readable telemetry** for OpenClaw plus human **read-only** audit (replay, correlation IDs).
- **Execution:** SCH-23 may stop at “normalized intent” toward SCH-24 / `trading-worker`, but the **architecture must not** insert a human approval queue between agent decision and policy-checked execution.

### Observability

- Structured logs for every OpenClaw invocation: request ID, signal ID, duration, tool calls made, success/failure.
- Redact secrets (API keys, account IDs) from logs.
- Metrics (if instrumented): invocation count, latency p50/p99, error rate, tool call frequency.

### Configuration

```env
OPENCLAW_API_URL=http://openclaw:8090
OPENCLAW_API_KEY=changeme
OPENCLAW_TIMEOUT=30s
OPENCLAW_RETRY_MAX=1
OPENCLAW_CIRCUIT_BREAKER_THRESHOLD=3
OPENCLAW_CIRCUIT_BREAKER_RESET=5m
PORTFOLIO_API_URL=http://portfolio-api:8080
INTERNAL_API_KEY=changeme
```

### Compose wiring

Add to `docker-compose.yml`:

| Service | Image / Build | Ports | Notes |
|---------|---------------|-------|-------|
| `openclaw-proxy` | `./` (target cmd) | `8090:8090` | Proxy service |

Depends on: `portfolio-api`, `signals`.

## Do NOT

- Insert **human-in-the-loop** approvals (Discord reactions, manual dashboard confirms) on the hot path for signals or trades.
- **Bypass** `TRADING_KILL_SWITCH`, `get_policy` limits, or execution-worker validation when submitting orders.
- Replace or modify P0's deterministic signal rules (SCH-16 owns those).
- Build a full ML/LLM signal pipeline — OpenClaw is the agent runtime.
- Expose MCP servers to the public internet.

## Acceptance criteria

- [ ] A synthetic/replayed `SignalEvent` triggers an OpenClaw run with MCP tools attached.
- [ ] Logs prove MCP tool calls occurred (portfolio snapshot retrieved by agent).
- [ ] OpenClaw failures do not block or wedge the signal pipeline.
- [ ] Circuit breaker activates after consecutive failures; resumes after reset period.
- [ ] Idempotency: duplicate request IDs are handled gracefully.
- [ ] **Audit-only humans:** no approval gate in Discord or dashboard; operators can still inspect payloads, logs, and persisted alerts.
- [ ] All secrets redacted from logs.
- [ ] Decision doc: async vs sync delivery mechanism.

## Shared contracts

This epic **consumes**:

- **SCH-16** `SignalEvent` — the trigger for OpenClaw runs.
- **SCH-18** Portfolio API — fetches positions/summary for context + MCP tools.
- **SCH-20** Broker domain types — used in `PortfolioContext`.
- **SCH-22** Rich Discord alerts — **machine channel + audit** (structured payloads for OpenClaw; humans read-only).

This epic **produces**:

- `OpenClawRequest` / `OpenClawResponse` — the integration contract with OpenClaw.
- MCP tool servers — consumed by OpenClaw's agent.
- Audit logs of agent invocations.
- **Closed-loop handoff** to SCH-24 / execution surfaces when `TRADING_ENABLED` — policy-checked, **no human approval** in the path.

## Files to create/modify

| File | Action |
|------|--------|
| `cmd/openclaw-proxy/main.go` | Create |
| `internal/openclaw/client.go` | Create (OpenClaw HTTP client) |
| `internal/openclaw/client_test.go` | Create |
| `internal/openclaw/types.go` | Create (request/response types) |
| `internal/openclaw/proxy.go` | Create (orchestration: enrich → forward → route) |
| `internal/openclaw/proxy_test.go` | Create |
| `internal/openclaw/circuit.go` | Create (circuit breaker) |
| `internal/openclaw/queue.go` | Create (async boundary) |
| `internal/mcp/portfolio/server.go` | Create (MCP tool server) |
| `internal/mcp/portfolio/server_test.go` | Create |
| `internal/mcp/market/server.go` | Create (stub/mock) |
| `internal/mcp/market/server_test.go` | Create |
| `docker-compose.yml` | Modify (add openclaw-proxy service) |
| `.env.example` | Modify (add OPENCLAW_* vars) |
