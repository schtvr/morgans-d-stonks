# SCH-AG1: The Agent — shadow-mode decision loop

> **Status: Core shadow loop implemented** — verified 2026-05.
> Trade execution via `order_bridge.go` + `AGENT_TRADE_ENABLED` extends beyond pure shadow mode;
> live order placement is gated by `TRADING_ENABLED` and the policy envelope.
> See acceptance criteria below for remaining items.

> **Milestone**: P1.5 (post-MVP, replaces SCH-23 OpenClaw scope)
> **Wave**: 5 (parallel with SCH-22 rich alerts)
> **Depends on**: P0 complete — specifically SCH-16 (signals + gate), SCH-18 (portfolio API), SCH-21 (ingest snapshots)
> **Supersedes**: `phase_2/openclaw-mcp-alerts.md` (OpenClaw integration is dropped; this epic provides the agent runtime in-process)

## Objective

Replace the external OpenClaw integration with a **local LLM agent** ("The Agent") that runs inside `signals`, reasons over a **read-only MCP server**, and emits a `Decision { action, confidence, rationale }` per **trigger**. v0 is **shadow mode**: decisions persist + get scored against actual price drift + render on the dashboard. **No order placement** (paper or live) ships in this epic — that is a follow-up.

## Product decisions (locked in this conversation)

- **Trigger universe (v0)**: gate-passed `crypto_signal_v1` events **+** a daily timer at `12:00 UTC` so the agent reasons even on quiet days. Other triggers (portfolio drift, vol regime) deferred.
- **Action enum (v0)**: `buy | sell | ignore`. Sizing is a **hint** (`sizingHintNotional`), not authoritative; later phases reconcile with policy.
- **Learning primitive (v0)**: hybrid **A + B**:
  - **A**: human-curated `config/agent-prompt.md` checked into the repo; loaded into the agent's system prompt on startup.
  - **B**: `get_decision_outcomes()` MCP tool — the agent reads its **own** scored history (return @ +1h/+24h/+7d/+14d, excess vs BTC baseline) before deciding.
  - Agent-writable scratchpad (variant C) is deferred to v1.
- **Benchmark**: rolling **14-day excess return vs BTC buy-and-hold, net of fees**. Surfaced on the dashboard. See `## Benchmark math` below.
- **Provider abstraction**: `agent.Provider` interface. **Anthropic ships v0** (Claude Sonnet 4.5 default); Ollama/OpenAI slot in later without schema change.
- **MCP transport**: in-process **stdio** via `mark3labs/mcp-go` for v0. Streamable HTTP hook reserved for v1 (Cursor / Claude Desktop can use the same tools). **Concurrency note:** each agent worker gets its own isolated MCP server instance — `NewInProcessServer` is called once per worker at startup, not shared across workers. `mark3labs/mcp-go` stdio sessions are single-caller; sharing one instance across concurrent workers would corrupt the framing. N workers = N server instances (cheap: no subprocess, no network, just N independent Go objects).
- **No Discord post-back on decisions.** Existing signal-alert Discord behavior is unchanged. The dashboard is the only human surface for decisions.
- **No order placement of any kind** in this epic. Paper execution (`paper_orders` + virtual broker) is a separate v0.5 epic. Live execution is v1.
- **No data sent back to provider for fine-tuning.** Everything beyond the LLM API request itself is local.

## Architecture

```
signals service (in-process)
┌────────────────────────────────────────────────────────────────┐
│                                                                │
│  ticker ─► fetchFollowedSymbols / fetchSnapshot                │
│              │                                                 │
│              ▼                                                 │
│  per-symbol gate (EvaluateGate.Emit == true)                   │
│              │                                                 │
│              ▼                                                 │
│  agent worker pool ──► agent.Provider.Decide(req)              │
│        ▲     │              │                                  │
│        │     │              │ tool_use (loop)                  │
│        │     │              ▼                                  │
│        │     │       MCP RO server (mark3labs/mcp-go, stdio)   │
│        │     │       ┌──────────────────────────────┐          │
│        │     │       │ get_market_candles           │ ──► coinbase REST
│        │     │       │ get_holdings                 │ ──► portfolio-api
│        │     │       │ get_position                 │ ──► portfolio-api
│        │     │       │ get_recent_signals           │ ──► portfolio-api
│        │     │       │ get_recent_decisions         │ ──► portfolio-api
│        │     │       │ get_decision_outcomes        │ ──► portfolio-api
│        │     │       │ get_correlated_symbols       │ ──► pure derivation
│        │     │       └──────────────────────────────┘          │
│        │     ▼                                                 │
│        │  Decision { action, confidence, rationale, toolCalls }│
│        │     │                                                 │
│        └─────┴─► POST /internal/agent-decisions                │
│                                                                │
│  daily timer (12:00 UTC) ──► same worker pool, trigger=daily   │
└────────────────────────────────────────────────────────────────┘

portfolio-api
┌────────────────────────────────────────────────────────────────┐
│  POST /internal/agent-decisions      (signals → here)          │
│  GET  /api/agent/decisions           (dashboard list)          │
│  GET  /api/agent/decisions/{id}      (dashboard drilldown)     │
│  GET  /api/agent/benchmark           (excess return series)    │
│  GET  /api/agent/cost                (daily LLM spend)         │
│                                                                │
│  scorer goroutine (in-process, runs every 1m)                  │
│    └─► for each unscored (decision, horizon) where now>=t+H:   │
│         compute return + BTC baseline + excess, persist        │
└────────────────────────────────────────────────────────────────┘
```

## Shared contracts

### Decision schema (`agent.Decision`)

Owner: this epic | Consumers: `signals` (writer), portfolio-api (storer), dashboard (reader)

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
    // DecisionsForSymbol24h is nil for daily triggers (no symbol). For signal
    // triggers it is the count of prior agent decisions on this symbol in the
    // last 24h — a flip-flopping guard passed eagerly to avoid a tool round-trip.
    DecisionsForSymbol24h *int `json:"decisionsForSymbol24h,omitempty"`
}

type PortfolioSummaryLine struct {
    NetLiquidation float64        `json:"netLiquidation"`
    TotalCash      float64        `json:"totalCash"`
    TopPositions   []PositionLine `json:"topPositions"` // top 3 by MV
}

type Decision struct {
    Action             Action     `json:"action"`
    Confidence         float64    `json:"confidence"`             // 0..1
    Rationale          string     `json:"rationale"`              // <= 1000 chars; bound for cost
    SizingHintNotional *float64   `json:"sizingHintNotional,omitempty"`
    ToolCalls          []ToolCall `json:"toolCalls"`
    Model              string     `json:"model"`                  // e.g. "claude-sonnet-4-5"
    LatencyMS          int64      `json:"latencyMs"`
    CostCents          int64      `json:"costCents"`              // best-effort from provider response
}

type ToolCall struct {
    Name     string          `json:"name"`
    Input    json.RawMessage `json:"input"`
    Output   json.RawMessage `json:"output"`
    DurationMS int64         `json:"durationMs"`
}
```

### MCP tool catalog (RO)

Owner: this epic | Consumer: The Agent (via `mcp-go` stdio session)

| Tool | Input | Output (capped) | Backed by |
|---|---|---|---|
| `get_market_candles` | `{ symbol, window: "1h"\|"24h"\|"7d"\|"30d" }` | up to **200** OHLCV points (dashboard uses 480; LLM cap is tighter for token economy) | `coinbase.Client.FetchProductCandles` (existing) |
| `get_holdings` | `{}` | snapshot positions + summary + `stale` flag (snapshot age > ingest interval × 3) | repo.LatestSnapshot |
| `get_position` | `{ symbol }` | one position w/ qty, avgCost, marketValue, unrealizedPL | repo.LatestSnapshot |
| `get_recent_signals` | `{ symbol?, limit<=50, window<=72h }` | RecentAlert rows | repo.ListRecentAlerts (filtered) |
| `get_recent_decisions` | `{ symbol?, limit<=20, window<=72h }` | AgentDecision rows w/o full toolCalls | repo.ListAgentDecisions |
| `get_decision_outcomes` | `{ symbol?, horizon: "24h"\|"7d"\|"14d", limit<=50 }` | scored outcomes (return %, excess vs BTC) | repo.ListAgentDecisionOutcomes |
| `get_correlated_symbols` | `{ symbol }` | `[symbol, "BTC-USD", "ETH-USD", ...top3MV...]` (deduped) | pure derivation from snapshot |

### Decision persistence endpoint

Owner: portfolio-api | Consumer: signals worker

| Method | Path | Auth | Body | Behavior |
|---|---|---|---|---|
| `POST` | `/internal/agent-decisions` | `X-Internal-Key` | `{ request: DecisionRequest, decision: Decision }` | Idempotent on `request.idempotencyKey`. Duplicate → 200 + existing row. New → 201. |

### Dashboard endpoints

| Method | Path | Auth | Returns |
|---|---|---|---|
| `GET` | `/api/agent/decisions?symbol&action&limit&from&to` | Session | paginated decisions (no tool-call bodies) |
| `GET` | `/api/agent/decisions/{id}` | Session | full decision incl. tool-call payloads |
| `GET` | `/api/agent/benchmark?window=14d` | Session | daily series of paper / hypothetical-action return vs BTC baseline; `excessReturnPct` |
| `GET` | `/api/agent/cost?window=7d` | Session | daily LLM spend in cents + cap |

## Environment variables (new)

| Variable | Service | Default | Purpose |
|---|---|---|---|
| `AGENT_ENABLED` | signals | `false` | Master switch. `false` → trigger fires but worker no-ops. |
| `AGENT_PROVIDER` | signals | `mock` | One of `mock`, `anthropic`. (`ollama`, `openai` slot later.) |
| `AGENT_MODEL` | signals | `claude-sonnet-4-5` | Provider-specific model id. |
| `ANTHROPIC_API_KEY` | signals | — | Required when `AGENT_PROVIDER=anthropic`. |
| `AGENT_DAILY_COST_CAP_USD` | signals | `5` | Hard daily spend cap. Decisions skip with `action=ignore`, reason logged, when exceeded. |
| `AGENT_DAILY_TIMER_UTC` | signals | `12:00` | HH:MM 24h UTC daily trigger. |
| `AGENT_CONCURRENCY` | signals | `2` | Worker pool size. |
| `AGENT_PROMPT_PATH` | signals | `config/agent-prompt.md` | Path to system prompt. SHA256 prefix → `prompt_version`. |

## Benchmark math

### Per-horizon scoring

For each `(decision d, horizon H)` once `now >= d.triggerAt + H`:

**Always record (all actions, all triggers):**
- `symbol_return_pct = (symbol.close(t+H) - symbol.close(t)) / symbol.close(t) * 100`
  - For `daily` triggers without a symbol: use portfolio NAV change between the two nearest ingest snapshots bracketing `[t, t+H]` — requires at least one snapshot on each side; if not available, mark `deferred` and retry next scorer tick.
- `btc_return_pct = (btc.close(t+H) - btc.close(t)) / btc.close(t) * 100`
  - **Fee deduction applies only at `H = 14d`:** subtract `0.6%` (Coinbase Advanced Trade tier-0 taker, one entry). Short horizons (1h, 24h, 7d) record raw BTC return with no fee adjustment — deducting a taker fee from a 1h window creates artificial drag that distorts the comparison.

**`buy` decisions:**
- `realized_return_pct = symbol_return_pct` (we're long: price up is good).
- `excess_return_pct = realized_return_pct - btc_return_pct`.

**`sell` decisions:**
- `realized_return_pct = -symbol_return_pct` (we're short/exiting: price down is good).
- `excess_return_pct = realized_return_pct - btc_return_pct`.

**`ignore` decisions:**
- `realized_return_pct = NULL`. Shadow mode cannot claim a P&L for inaction — we don't know what "ignoring" meant for the actual portfolio without paper execution tracking positions.
- `excess_return_pct = NULL`.
- `symbol_return_pct` and `btc_return_pct` are still recorded so the dashboard can display "symbol moved X% / BTC moved Y% — agent said ignore."
- `ignore` decisions are **excluded from the headline excess-return metric**.

**`daily` triggers (portfolio health check):**
- Portfolio NAV change is recorded as `symbol_return_pct` (reusing the field; `symbol = "_portfolio"`).
- `excess_return_pct = portfolio_return_pct - btc_return_pct` for `H = 7d` and `H = 14d` (enough data for meaningful comparison). For `H = 1h` and `H = 24h`, record price data only; exclude from excess metric.
- Daily triggers are **excluded from the headline excess-return metric** (they're a portfolio health signal, not a directional call, and conflating them with symbol-level buy/sell decisions would pollute the signal quality score).

### Headline metric (dashboard)

`headline_excess_return_pct` = rolling mean of `excess_return_pct` for `horizon = 14d` over the requested window, **restricted to `buy` and `sell` decisions only**.

Two companion charts:
1. `excess_return_pct` per buy/sell decision at horizon=14d, plotted by decision date.
2. `symbol_return_pct` vs `btc_return_pct` for ignore decisions — opportunity cost view, no excess claimed.

### Caveats (published on chart)

- **Shadow mode:** fees on hypothetical buy/sell trades are not modeled. `realized_return_pct` is an upper bound on actual execution performance.
- **First 14 days:** no rolling-14d data exists; chart shows absolute symbol/BTC returns only until the window fills.
- **`ignore` decisions:** no P&L is claimed. Opportunity cost is displayed informatively, not attributed to agent performance.

## Scope (in this epic)

- `internal/agent/` package — provider abstraction, Anthropic impl, mock impl, worker, idempotency, cost.
- `internal/mcp/agent/` package — MCP RO server, all 7 tools, stdio bootstrap, HTTP-ready interface.
- `agent_decisions` + `agent_decision_outcomes` tables; migrations 007 + 008.
- Repository methods: `InsertAgentDecision`, `GetAgentDecision`, `ListAgentDecisions`, `ListAgentDecisionOutcomes`, `InsertAgentDecisionOutcome`, `ListUnscoredDecisionHorizons`.
- portfolio-api routes: `POST /internal/agent-decisions`, `GET /api/agent/*` (×4).
- portfolio-api **scorer goroutine** — periodic (every 1m) outcome computation. Idempotent on (decision_id, horizon).
- `signals` integration — gate-pass enqueue + daily 12:00 UTC timer enqueue + bounded worker pool + per-symbol serialization + POST decision.
- `config/agent-prompt.md` — checked in, versioned, hashed for `prompt_version`.
- Frontend `/agent` route — decision list, decision drilldown (tool-call timeline), benchmark chart, cost meter.
- OpenClaw deprecation: strip `recordLabSignal`'s `lab_openclaw_runs` enqueue; mark `internal/openclaw/types.go` + `lab_openclaw_runs` table as deprecated (kept for data, not enqueued); remove `OPENCLAW_*` env vars from `.env.example`; mark `phase_2/openclaw-mcp-alerts.md` as superseded.
- AGENTS.md updates: replace OpenClaw contract sections with The Agent contract.

## Do NOT (in this epic)

- Place orders (paper or live). Paper execution is a separate v0.5 epic.
- Add news / fundamentals / orderbook / on-chain MCP tools.
- Build an agent-writable scratchpad. Decisions are append-only; rules are human-curated.
- Send any data to the model provider beyond per-request inputs (no fine-tuning).
- Post agent decisions to Discord.
- Delete `lab_openclaw_runs` data — only stop writing new rows. Existing rows preserved.
- Modify the deterministic signal pipeline (SCH-16 owns it).

## Acceptance criteria

- [x] With `AGENT_ENABLED=true` and `AGENT_PROVIDER=mock`, a gate-passed signal results in an `agent_decisions` row within ~1s.
- [x] Daily timer at `AGENT_DAILY_TIMER_UTC` enqueues exactly one trigger per UTC day; idempotency key `daily-YYYY-MM-DD` rejects duplicates.
- [x] With `AGENT_PROVIDER=anthropic` + valid key, the mock can be swapped end-to-end with no schema change; tool-call trace is persisted per decision.
- [x] `AGENT_DAILY_COST_CAP_USD` exceeded → next trigger short-circuits to `action=ignore`, `rationale="daily cost cap reached"`, cost=0; restores at UTC midnight.
- [x] Scorer fills `agent_decision_outcomes` rows for all horizons whose deadline has passed; rerunning scorer is idempotent.
- [x] `GET /api/agent/benchmark?window=14d` returns a daily series of `excessReturnPct` plus headline rolling-14d mean.
- [x] Frontend `/agent` page renders: decisions list, drilldown with tool-call timeline, benchmark chart, cost meter.
- [x] OpenClaw enqueue path (`recordLabSignal` → `lab_openclaw_runs`) is removed. Existing rows untouched.
- [x] Per-symbol serialization: two concurrent triggers on the same symbol cannot produce two in-flight decisions.
- [x] Provider/MCP failures log + skip; never fall back to a heuristic action. No partial decisions persisted.
- [x] Mock provider deterministic in CI; no live API calls in default `go test ./...`.

## File impact map

| Path | Action |
|---|---|
| `backend/internal/agent/provider.go` | Create |
| `backend/internal/agent/types.go` | Create |
| `backend/internal/agent/mock.go` | Create |
| `backend/internal/agent/anthropic.go` | Create |
| `backend/internal/agent/worker.go` | Create |
| `backend/internal/agent/cost.go` | Create |
| `backend/internal/agent/prompt.go` | Create |
| `backend/internal/agent/*_test.go` | Create |
| `backend/internal/mcp/agent/server.go` | Create |
| `backend/internal/mcp/agent/tools_*.go` | Create (one file per tool family) |
| `backend/internal/mcp/agent/server_test.go` | Create |
| `backend/internal/portfolio/types.go` | Modify (add `AgentDecision`, `AgentDecisionOutcome`) |
| `backend/internal/portfolio/repository.go` | Modify (add agent CRUD signatures) |
| `backend/internal/portfolio/postgres/repository.go` | Modify (impl) |
| `backend/internal/portfolio/postgres/migrations/007_agent_decisions.sql` | Create |
| `backend/internal/portfolio/postgres/migrations/008_agent_decision_outcomes.sql` | Create |
| `backend/cmd/portfolio-api/main.go` | Modify (mount routes + scorer goroutine) |
| `backend/cmd/portfolio-api/agent_handlers.go` | Create |
| `backend/cmd/portfolio-api/agent_handlers_test.go` | Create |
| `backend/cmd/portfolio-api/scorer.go` | Create |
| `backend/cmd/portfolio-api/scorer_test.go` | Create |
| `backend/cmd/portfolio-api/lab_handlers.go` | Modify (strip `recordLabSignal` enqueue path; keep handlers for legacy data) |
| `backend/cmd/signals/main.go` | Modify (worker pool, daily timer, decision POST) |
| `backend/cmd/signals/agent_integration.go` | Create |
| `backend/cmd/signals/*_test.go` | Modify / create |
| `backend/internal/config/agent.go` | Create (load `AGENT_*` env) |
| `backend/internal/openclaw/types.go` | Add deprecation comment; do not delete |
| `config/agent-prompt.md` | Create |
| `.env.example` | Modify (remove `OPENCLAW_*`; add `AGENT_*` block) |
| `.agent/epics/phase_2/openclaw-mcp-alerts.md` | Modify (top-of-file `> SUPERSEDED by agent-shadow-decisions.md` banner) |
| `AGENTS.md` | Modify (replace OpenClaw contract section with The Agent contract) |
| `docker-compose.yml` | Modify (add `AGENT_*` env wiring to signals; ensure `ANTHROPIC_API_KEY` propagates) |
| `frontend/app/agent/page.tsx` | Create |
| `frontend/components/agent/decision-list.tsx` | Create |
| `frontend/components/agent/decision-detail.tsx` | Create |
| `frontend/components/agent/benchmark-chart.tsx` | Create |
| `frontend/components/agent/cost-meter.tsx` | Create |
| `frontend/lib/agent-api.ts` | Create |
| `frontend/components/site-header.tsx` | Modify (add `/agent` nav link; mark `/lab` deprecated or hide) |
| `go.mod` / `go.sum` | Modify (add `mark3labs/mcp-go`, `anthropics/anthropic-sdk-go`) |

## Out of scope (future epics)

- **v0.5 paper execution**: `paper_orders`, `paper_positions`, `paper_cash`, fee/fill model, paper portfolio benchmarking. Same agent + same decision schema; new sink.
- **v1 live execution**: small-notional live trading, full reuse of the policy envelope.
- **v1+ learning loop**: agent-writable scratchpad, automated rule mining from outcomes.
- **News / fundamentals / orderbook / on-chain** MCP tools.
- **Multi-portfolio / multi-account.**
- **Cross-exchange.**
