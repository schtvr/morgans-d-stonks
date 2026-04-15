# SCH-23: OpenClaw, MCP & Alert Intelligence

> **Linear**: [SCH-23](https://linear.app/schtvr/issue/SCH-23/epic-p1-openclaw-mcp-and-alert-intelligence)
> **Milestone**: P1: First follow-up
> **Wave**: 5 (all P0 complete; parallel with SCH-22)
> **Depends on**: P0 complete — specifically SCH-16 (SignalEvent), SCH-18 (portfolio API), SCH-21 (snapshots)

## Objective

Route structured `SignalEvent` traffic into OpenClaw with relevant MCP tools (portfolio snapshot, fundamentals/news), hardening the alert proxy path. Default to human-in-the-loop — no broker orders from this epic.

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
                             Human approval
                             (Discord / dashboard)
```

### OpenClaw proxy service (`cmd/openclaw-proxy/`)

A new Go service that:

1. **Receives** `SignalEvent` payloads from the signal engine.
2. **Enriches** with portfolio context (positions, account summary) via the portfolio API.
3. **Forwards** to OpenClaw with MCP tool definitions attached.
4. **Receives** the agent's analysis/recommendation.
5. **Routes** the recommendation to the human-in-the-loop channel (Discord rich alert via SCH-22, or a dashboard notification).

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

Exposes portfolio data as MCP tools:

| Tool | Description | Input | Output |
|------|-------------|-------|--------|
| `get_positions` | Current portfolio positions | `{}` | `{ positions: [...] }` |
| `get_account_summary` | Account-level metrics | `{}` | `{ summary: {...} }` |
| `get_position_detail` | Detail for a specific symbol | `{ symbol: "AAPL" }` | `{ position: {...}, history: [...] }` |

Implementation: thin HTTP wrapper around the portfolio API (SCH-18).

#### News/fundamentals MCP (`internal/mcp/market/`)

Stub for P1 — document the interface, implement with mock data or a simple external API:

| Tool | Description | Input | Output |
|------|-------------|-------|--------|
| `get_news` | Recent news for a symbol | `{ symbol: "AAPL", limit: 5 }` | `{ articles: [...] }` |
| `get_fundamentals` | Basic fundamentals | `{ symbol: "AAPL" }` | `{ pe: 28.5, marketCap: "2.8T", ... }` |

Implementation options for P1:
- Mock data (sufficient for testing the pipeline).
- Free API integration (Alpha Vantage, Finnhub — document choice if real).

### Human-in-the-loop

- **Default mode**: all OpenClaw recommendations are for human review only.
- Route agent output to Discord (using SCH-22's rich alert format) or a dashboard notification endpoint.
- No automated order placement — that's P2 (SCH-24).
- Log the full request/response cycle for audit.

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

- Implement auto-trade execution or order placement (P2, SCH-24).
- Replace or modify P0's deterministic signal rules (SCH-16 owns those).
- Build a full ML/LLM signal pipeline — OpenClaw is the agent runtime.
- Expose MCP servers to the public internet.

## Acceptance criteria

- [ ] A synthetic/replayed `SignalEvent` triggers an OpenClaw run with MCP tools attached.
- [ ] Logs prove MCP tool calls occurred (portfolio snapshot retrieved by agent).
- [ ] OpenClaw failures do not block or wedge the signal pipeline.
- [ ] Circuit breaker activates after consecutive failures; resumes after reset period.
- [ ] Idempotency: duplicate request IDs are handled gracefully.
- [ ] Human-in-the-loop: agent recommendations routed to Discord or dashboard, not to broker.
- [ ] All secrets redacted from logs.
- [ ] Decision doc: async vs sync delivery mechanism.

## Shared contracts

This epic **consumes**:

- **SCH-16** `SignalEvent` — the trigger for OpenClaw runs.
- **SCH-18** Portfolio API — fetches positions/summary for context + MCP tools.
- **SCH-20** Broker domain types — used in `PortfolioContext`.
- **SCH-22** Rich Discord alerts — routes recommendations to human.

This epic **produces**:

- `OpenClawRequest` / `OpenClawResponse` — the integration contract with OpenClaw.
- MCP tool servers — consumed by OpenClaw's agent.
- Audit logs of agent invocations.
- Foundation for P2 auto-trade (SCH-24 will extend recommendation routing to order placement).

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
