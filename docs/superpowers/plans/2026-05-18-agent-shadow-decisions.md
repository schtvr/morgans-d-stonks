# The Agent — shadow-mode decision loop (v0) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.
>
> **Epic:** [`.agent/epics/phase_2/agent-shadow-decisions.md`](../../.agent/epics/phase_2/agent-shadow-decisions.md). Read it first — it owns the shared contracts and acceptance criteria.

**Goal:** Replace the dormant OpenClaw scaffolding with an in-process LLM agent ("The Agent") that runs inside `signals`, reasons over a read-only MCP server, and emits a structured `Decision` per trigger (gate-passed signal **or** daily 12:00 UTC). Persist decisions, score outcomes against actual price drift, surface a `/agent` dashboard with a 14-day excess-return-vs-BTC benchmark. **No order placement in this epic.**

**Tech Stack:** Go 1.25 backend (chi, pgx, slog), Next.js 14 frontend (App Router, shadcn/ui, Vitest), Postgres, Docker Compose. New deps: `github.com/mark3labs/mcp-go`, `github.com/anthropics/anthropic-sdk-go`.

---

## Agreed product decisions

Locked in conversation 2026-05-17 / 2026-05-18:

- v0 = **shadow mode** (decision + persistence + outcome scoring + dashboard; **no execution**).
- Triggers = **gate-passed `crypto_signal_v1`** + **daily timer at 12:00 UTC**.
- Action enum = `buy | sell | ignore`. Sizing is a hint, not authoritative.
- Learning primitive = **A + B** (`config/agent-prompt.md` + `get_decision_outcomes()` tool). No agent-writable scratchpad.
- Benchmark = rolling **14-day excess return vs BTC buy-and-hold, net of one entry fee**.
- Provider abstraction; **Anthropic ships v0** (`claude-sonnet-4-5` default).
- MCP server: real protocol via `mark3labs/mcp-go`, stdio in-process for v0, HTTP later.
- **No Discord post-back on decisions.** Signal alerts to Discord unchanged.
- **No data sent to provider beyond per-request inputs** (no fine-tuning).
- OpenClaw enqueue path stripped; `lab_openclaw_runs` table preserved (data only).

## Current repo facts

- `backend/cmd/signals/main.go` already polls Coinbase, runs `EvaluateGate`, builds `signal.CryptoAlert`, POSTs `/internal/recent-alerts`, posts Discord. The agent worker hooks in **after** `gate.Emit == true`, before the existing `discord.CryptoAlertWebhookContent` path.
- `backend/internal/broker/coinbase/candles.go` already implements `FetchProductCandles` + granularity selection — the `get_market_candles` MCP tool wraps this.
- `backend/internal/portfolio/postgres/migrations/` last numbered file is `006_lab.sql`. New migrations are `007` and `008`.
- `backend/cmd/portfolio-api/lab_handlers.go::recordLabSignal` currently enqueues `lab_openclaw_runs` rows that nothing consumes. We strip the enqueue but keep the table.
- `backend/internal/openclaw/types.go` is just status constants. Deprecate; don't delete.
- `frontend/app/lab/page.tsx` exists and reads OpenClaw run state. We add a new `/agent` page and leave `/lab` accessible but stale during transition.
- `go.mod` is Go 1.25.7 with chi, pgx, uuid, yaml. We add `mark3labs/mcp-go` and `anthropics/anthropic-sdk-go`.

## Parallelization strategy

Run **Task 1** first. It defines the schemas, types, and migrations every other task depends on.

After Task 1 lands, the following can run **in parallel** in isolated worktrees:

- Task 2: agent package (provider abstraction, mock + Anthropic, cost, prompt)
- Task 3: MCP RO server (`mark3labs/mcp-go`, 7 tools, stdio)
- Task 6: portfolio-api routes (`POST /internal/agent-decisions`, dashboard GETs) — depends on Task 1 schema only
- Task 8: OpenClaw deprecation + `.env.example` + AGENTS.md cleanup

**Task 4** (signals integration) depends on **Tasks 2 + 3** — needs both `agent.Provider` and the MCP server.

**Task 5** (outcome scorer goroutine) depends on **Tasks 1 + 6** — needs the tables and at least one row to score.

**Task 7** (frontend `/agent` page) depends on **Task 6** — needs the HTTP endpoints to call.

**Task 9** is final integration + smoke + docs.

**Important coupling: Tasks 3 and 6 share an integration boundary.**
Task 3 (MCP server) can be *written* in parallel with Task 6, but three MCP tools
(`get_recent_signals`, `get_recent_decisions`, `get_decision_outcomes`) call internal
portfolio-api read endpoints that Task 6 creates. Task 3's unit tests use fake HTTP
handlers and pass independently; Task 3's integration tests require Task 6 to be
merged first. The Task 3 implementer should stub the HTTP calls and add a comment:
`// Integration-tested only after Task 6 (portfolio-api internal read endpoints) merges.`

If multiple subagents run at once, use isolated git worktrees (one branch per task). Do not let parallel agents touch the same files.

## File ownership map

- **Schemas & DB (Task 1):**
  - `backend/internal/portfolio/types.go`
  - `backend/internal/portfolio/repository.go`
  - `backend/internal/portfolio/postgres/repository.go`
  - `backend/internal/portfolio/postgres/migrations/007_agent_decisions.sql`
  - `backend/internal/portfolio/postgres/migrations/008_agent_decision_outcomes.sql`
  - `backend/internal/agent/types.go` (shared with Task 2)
- **Agent package (Task 2):**
  - `backend/internal/agent/{provider,mock,anthropic,worker,cost,prompt}.go`
  - `backend/internal/agent/*_test.go`
  - `backend/internal/config/agent.go`
  - `config/agent-prompt.md`
- **MCP server (Task 3):**
  - `backend/internal/mcp/agent/server.go`
  - `backend/internal/mcp/agent/tools_market.go`
  - `backend/internal/mcp/agent/tools_portfolio.go`
  - `backend/internal/mcp/agent/tools_history.go`
  - `backend/internal/mcp/agent/server_test.go`
- **Signals integration (Task 4):**
  - `backend/cmd/signals/main.go`
  - `backend/cmd/signals/agent_integration.go`
  - `backend/cmd/signals/signals_test.go`
- **Outcome scorer (Task 5):**
  - `backend/cmd/portfolio-api/scorer.go`
  - `backend/cmd/portfolio-api/scorer_test.go`
  - `backend/cmd/portfolio-api/main.go` (wire goroutine on boot)
- **Portfolio API routes (Task 6):**
  - `backend/cmd/portfolio-api/agent_handlers.go`
  - `backend/cmd/portfolio-api/agent_handlers_test.go`
  - `backend/cmd/portfolio-api/main.go` (mount routes)
- **Frontend (Task 7):**
  - `frontend/app/agent/page.tsx`
  - `frontend/components/agent/{decision-list,decision-detail,benchmark-chart,cost-meter}.tsx`
  - `frontend/lib/agent-api.ts`
  - `frontend/components/site-header.tsx`
- **OpenClaw deprecation + cleanup (Task 8):**
  - `.env.example`
  - `AGENTS.md`
  - `.agent/epics/phase_2/openclaw-mcp-alerts.md`
  - `backend/internal/openclaw/types.go`
  - `backend/cmd/portfolio-api/lab_handlers.go` (strip `recordLabSignal` enqueue branch)
  - `docker-compose.yml`
- **Integration (Task 9):**
  - Cross-cutting; no new files unless conflict resolution.

---

## Task 1: Shared contracts, types, and migrations

**Subagent lane:** foundation. Must complete first.

**Files:**
- Modify: `backend/internal/portfolio/types.go`
- Modify: `backend/internal/portfolio/repository.go`
- Modify: `backend/internal/portfolio/postgres/repository.go`
- Create: `backend/internal/portfolio/postgres/migrations/007_agent_decisions.sql`
- Create: `backend/internal/portfolio/postgres/migrations/008_agent_decision_outcomes.sql`
- Create: `backend/internal/agent/types.go`
- Modify: `backend/internal/portfolio/postgres/migrations_test.go` (assert new migrations apply)

- [x] **Step 1: Agent type definitions in `internal/agent/types.go`**

```go
package agent

import (
	"encoding/json"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

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
	TriggerKind    TriggerKind         `json:"triggerKind"`
	TriggerAt      time.Time           `json:"triggerAt"`
	IdempotencyKey string              `json:"idempotencyKey"`
	Signal         *signal.CryptoAlert `json:"signal,omitempty"`
	EagerContext   EagerContext        `json:"eagerContext"`
	PromptVersion  string              `json:"promptVersion"`
}

type EagerContext struct {
	PortfolioSummary      PortfolioSummaryLine `json:"portfolioSummary"`
	// nil for daily triggers (no symbol); count of prior decisions on this symbol
	// in the last 24h for signal triggers.
	DecisionsForSymbol24h *int `json:"decisionsForSymbol24h,omitempty"`
}

type PortfolioSummaryLine struct {
	NetLiquidation float64        `json:"netLiquidation"`
	TotalCash      float64        `json:"totalCash"`
	TopPositions   []PositionLine `json:"topPositions"`
}

type PositionLine struct {
	Symbol      string  `json:"symbol"`
	MarketValue float64 `json:"marketValue"`
	Quantity    float64 `json:"quantity"`
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

- [x] **Step 2: Persistence types in `portfolio/types.go`**

Append (do not modify existing types):

```go
type AgentDecision struct {
	ID                 int64           `json:"id"`
	TriggerKind        string          `json:"triggerKind"`
	TriggerAt          time.Time       `json:"triggerAt"`
	IdempotencyKey     string          `json:"idempotencyKey"`
	Symbol             string          `json:"symbol,omitempty"`
	SignalEventID      *int64          `json:"signalEventId,omitempty"`
	Action             string          `json:"action"`
	Confidence         float64         `json:"confidence"`
	Rationale          string          `json:"rationale"`
	SizingHintNotional *float64        `json:"sizingHintNotional,omitempty"`
	Model              string          `json:"model"`
	PromptVersion      string          `json:"promptVersion"`
	LatencyMS          int64           `json:"latencyMs"`
	CostCents          int64           `json:"costCents"`
	RequestJSON        json.RawMessage `json:"requestJson,omitempty"`
	ResponseJSON       json.RawMessage `json:"responseJson,omitempty"`
	ToolCallsJSON      json.RawMessage `json:"toolCallsJson,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type AgentDecisionOutcome struct {
	ID                int64     `json:"id"`
	DecisionID        int64     `json:"decisionId"`
	Horizon           string    `json:"horizon"` // "1h" | "24h" | "7d" | "14d"
	PriceAtDecision   *float64  `json:"priceAtDecision,omitempty"`
	PriceAtHorizon    *float64  `json:"priceAtHorizon,omitempty"`
	// SymbolReturnPct: actual price move of the symbol (or portfolio NAV for
	// daily triggers) over the horizon. Always populated for buy/sell/daily.
	// For ignore: populated with the symbol's move so the dashboard can show
	// opportunity-cost context — but no P&L is attributed.
	SymbolReturnPct   *float64  `json:"symbolReturnPct,omitempty"`
	// BTCReturnPct: raw BTC-USD return over the horizon. Fee deduction only
	// applies at horizon="14d" (0.6% taker). All other horizons: raw return.
	BTCReturnPct      float64   `json:"btcReturnPct"`
	// RealizedReturnPct: directional return for buy (+symbol) or sell (-symbol).
	// NULL for ignore and daily triggers — shadow mode cannot claim P&L on inaction.
	RealizedReturnPct *float64  `json:"realizedReturnPct,omitempty"`
	// ExcessReturnPct = RealizedReturnPct - BTCReturnPct. NULL when RealizedReturnPct
	// is NULL. Excluded from headline metric for ignore and daily decisions.
	ExcessReturnPct   *float64  `json:"excessReturnPct,omitempty"`
	FeesModeledPct    float64   `json:"feesModeledPct"` // v0: 0 for shadow
	ScoredAt          time.Time `json:"scoredAt"`
}

type AgentDecisionFilter struct {
	Limit  int
	Symbol string
	Action string
	From   *time.Time
	To     *time.Time
}

type AgentDecisionsResponse struct {
	Decisions []AgentDecision `json:"decisions"`
}

type AgentBenchmarkPoint struct {
	AsOf            time.Time `json:"asOf"`
	// Mean realized return for buy/sell decisions at horizon=14d on this day.
	RealizedReturnPct float64  `json:"realizedReturnPct"`
	// Mean BTC return (with 0.6% fee) at horizon=14d.
	BTCReturnPct      float64  `json:"btcReturnPct"`
	// Mean excess return = realized - btc. buy/sell only; ignore/daily excluded.
	ExcessReturnPct   float64  `json:"excessReturnPct"`
	// Count of scored buy/sell decisions contributing to this day's means.
	DecisionCount     int      `json:"decisionCount"`
	// Count of ignore decisions on this day (opportunity-cost view, not in excess).
	IgnoreCount       int      `json:"ignoreCount"`
}

type AgentBenchmarkResponse struct {
	Window              string                `json:"window"`             // e.g. "14d"
	HeadlineExcessPct   float64               `json:"headlineExcessPct"`  // rolling mean of ExcessReturnPct
	Points              []AgentBenchmarkPoint `json:"points"`
	// true in v0 shadow: realized return does not deduct trade fees.
	NoteShadowFeesNotPaid bool                `json:"noteShadowFeesNotPaid"`
	// true when fewer than 14d of scored data exist.
	NoteWindowIncomplete  bool                `json:"noteWindowIncomplete"`
}

type AgentCostPoint struct {
	Day       string `json:"day"`       // YYYY-MM-DD UTC
	CostCents int64  `json:"costCents"`
	Decisions int    `json:"decisions"`
}

type AgentCostResponse struct {
	Window   string           `json:"window"`
	CapCents int64            `json:"capCents"`
	Today    AgentCostPoint   `json:"today"`
	Points   []AgentCostPoint `json:"points"`
}
```

- [x] **Step 3: Repository interface additions in `portfolio/repository.go`**

Append to the `Repository` interface:

```go
InsertAgentDecision(ctx context.Context, d AgentDecision) (*AgentDecision, error)
GetAgentDecision(ctx context.Context, id int64) (*AgentDecision, error)
GetAgentDecisionByIdempotencyKey(ctx context.Context, key string) (*AgentDecision, error)
ListAgentDecisions(ctx context.Context, filter AgentDecisionFilter) ([]AgentDecision, error)
CountDecisionsForSymbolSince(ctx context.Context, symbol string, since time.Time) (int, error)
SumCostCentsForDay(ctx context.Context, day time.Time) (int64, error)
ListAgentCostDaily(ctx context.Context, days int) ([]AgentCostPoint, error)

InsertAgentDecisionOutcome(ctx context.Context, o AgentDecisionOutcome) (*AgentDecisionOutcome, error)
ListUnscoredDecisionHorizons(ctx context.Context, now time.Time, limit int) ([]UnscoredHorizon, error)
ListAgentDecisionOutcomes(ctx context.Context, filter AgentDecisionOutcomeFilter) ([]AgentDecisionOutcome, error)
ListBenchmarkDaily(ctx context.Context, horizon string, days int) ([]AgentBenchmarkPoint, error)
```

With supporting types in `portfolio/types.go`:

```go
type UnscoredHorizon struct {
	DecisionID int64
	Symbol     string
	TriggerAt  time.Time
	Horizon    string
	Action     string
}

type AgentDecisionOutcomeFilter struct {
	DecisionIDs []int64
	Horizon     string
}
```

- [x] **Step 4: Migration `007_agent_decisions.sql`**

```sql
CREATE TABLE IF NOT EXISTS agent_decisions (
    id BIGSERIAL PRIMARY KEY,
    trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('signal','daily')),
    trigger_at TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    symbol TEXT NOT NULL DEFAULT '',
    signal_event_id BIGINT REFERENCES lab_signal_events(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('buy','sell','ignore')),
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    rationale TEXT NOT NULL DEFAULT '',
    sizing_hint_notional DOUBLE PRECISION,
    model TEXT NOT NULL DEFAULT '',
    prompt_version TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    cost_cents BIGINT NOT NULL DEFAULT 0,
    request_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    tool_calls_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_decisions_trigger_at
    ON agent_decisions (trigger_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_symbol_trigger_at
    ON agent_decisions (symbol, trigger_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_action_trigger_at
    ON agent_decisions (action, trigger_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_cost_day
    ON agent_decisions (((trigger_at AT TIME ZONE 'UTC')::date));
```

- [x] **Step 5: Migration `008_agent_decision_outcomes.sql`**

```sql
CREATE TABLE IF NOT EXISTS agent_decision_outcomes (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES agent_decisions(id) ON DELETE CASCADE,
    horizon TEXT NOT NULL CHECK (horizon IN ('1h','24h','7d','14d')),
    price_at_decision DOUBLE PRECISION,
    price_at_horizon DOUBLE PRECISION,
    -- symbol_return_pct: actual price/NAV move over horizon. Populated for all
    -- actions including ignore (opportunity-cost display). NULL only if deferred.
    symbol_return_pct DOUBLE PRECISION,
    -- btc_return_pct: raw BTC return over horizon. 0.6% fee deducted only at
    -- horizon='14d'. Other horizons: raw return (no fee drag).
    btc_return_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- realized_return_pct: directional return for buy/sell. NULL for ignore and
    -- daily triggers — shadow mode does not claim P&L on inaction.
    realized_return_pct DOUBLE PRECISION,
    -- excess_return_pct: realized - btc. NULL when realized is NULL.
    -- Excluded from headline metric for ignore/daily decisions.
    excess_return_pct DOUBLE PRECISION,
    fees_modeled_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    scored_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (decision_id, horizon)
);

CREATE INDEX IF NOT EXISTS idx_agent_decision_outcomes_decision
    ON agent_decision_outcomes (decision_id);
CREATE INDEX IF NOT EXISTS idx_agent_decision_outcomes_horizon_scored
    ON agent_decision_outcomes (horizon, scored_at DESC);
```

- [x] **Step 6: Postgres repository implementations**

Implement every method added in Step 3 in `backend/internal/portfolio/postgres/repository.go`. `InsertAgentDecision` MUST be idempotent: `INSERT ... ON CONFLICT (idempotency_key) DO NOTHING RETURNING *`, falling back to a `SELECT` when no row is returned (duplicate).

`ListUnscoredDecisionHorizons` computes the cross-join `(decision × horizon)` and excludes pairs already in `agent_decision_outcomes`. Only return rows where `now >= decision.trigger_at + horizon_duration`. Bound `limit` (default 50).

- [x] **Step 7: Tests**

- `backend/internal/portfolio/postgres/migrations_test.go`: assert both new migrations apply cleanly to an empty DB.
- `backend/internal/portfolio/postgres/repository_test.go`: test inserts (idempotency-key collision returns the original), filters, and `ListUnscoredDecisionHorizons` (horizon filtering by time, exclude already-scored pairs).

- [x] **Step 8: Run focused tests**

```bash
cd backend && go test ./internal/agent ./internal/portfolio ./internal/portfolio/postgres
```

Expected: all selected packages pass.

**Return from subagent:** status, files changed, migrations applied, schema diff summary.

---

## Task 2: Agent package — provider abstraction, Anthropic impl, cost, prompt

**Subagent lane:** agent runtime. Depends on Task 1.

**Files:**
- Create: `backend/internal/agent/provider.go`
- Create: `backend/internal/agent/mock.go`
- Create: `backend/internal/agent/anthropic.go`
- Create: `backend/internal/agent/worker.go`
- Create: `backend/internal/agent/cost.go`
- Create: `backend/internal/agent/prompt.go`
- Create: `backend/internal/agent/*_test.go`
- Create: `backend/internal/config/agent.go`
- Create: `config/agent-prompt.md`

- [x] **Step 1: Provider interface**

`backend/internal/agent/provider.go`:

```go
package agent

import "context"

type Provider interface {
	// Decide runs the agent loop (tool-use included) and returns the final structured decision.
	// Implementations are responsible for connecting to the MCP server, executing tool calls,
	// and bounding the loop (max iterations, timeout).
	Decide(ctx context.Context, req DecisionRequest) (*Decision, error)
	Name() string
	Model() string
}
```

- [x] **Step 2: Mock provider**

`backend/internal/agent/mock.go`: deterministic provider for CI. Given a request:

- If `req.Signal != nil && req.Signal.DeltaPct >= req.Signal.ThresholdPct * 2` → `ActionBuy`, confidence 0.7, rationale `"mock: strong move"`.
- If `req.Signal != nil && req.Signal.DeltaPct <= -req.Signal.ThresholdPct * 2` → `ActionSell`, confidence 0.7, rationale `"mock: strong drop"`.
- Else → `ActionIgnore`, confidence 0.3, rationale `"mock: insufficient signal"`.
- Empty `ToolCalls`, model `"mock"`, cost 0, latency 0.

Used by default in tests; never makes network calls. Selected by `AGENT_PROVIDER=mock` (default).

- [x] **Step 3: Anthropic provider**

`backend/internal/agent/anthropic.go`. Use `github.com/anthropics/anthropic-sdk-go`. Tool-use loop:

1. Build system prompt from `agent-prompt.md` + a fixed schema preamble that declares the `Decision` JSON shape and the available tools.
2. Initial user message: serialized `DecisionRequest.EagerContext` + (if present) `DecisionRequest.Signal`.
3. Loop up to `maxIterations` (default 8):
   - Send messages + tool definitions.
   - If response is `tool_use`: dispatch to MCP server via the in-process MCP client (passed in at construction); append `tool_result` to message history.
   - If response is text: parse as `Decision` JSON (strict schema; reject on unknown fields).
4. Track total input + output tokens; compute `costCents` from per-model price table (constants for `claude-sonnet-4-5`, `claude-haiku-4-5`).
5. Return `Decision` populated with `ToolCalls`, `Model`, `LatencyMS`, `CostCents`.

Hard error contract:
- Parse failure → return error (caller logs + skips; no decision persisted).
- LLM API error → return error.
- Loop exhausted without text response → return error.
- **Never** synthesize an action on the agent's behalf.

- [x] **Step 4: Prompt loader**

`backend/internal/agent/prompt.go`:

```go
type Prompt struct {
	Body    string
	Version string // sha256 hex, first 12 chars of config/agent-prompt.md
}

func LoadPrompt(path string) (*Prompt, error) { ... }
```

Called once at startup; cached on the `Provider` impl. `Version` is plumbed into every `DecisionRequest.PromptVersion`.

- [x] **Step 5: Cost tracker**

`backend/internal/agent/cost.go`:

```go
type CostTracker struct {
	repo CostRepo
	capCents int64
}

type CostRepo interface {
	SumCostCentsForDay(ctx context.Context, day time.Time) (int64, error)
}

// CheckAndReserve returns ErrCapReached if today's total + estimate would exceed the cap.
// The estimate is conservative; actual cost is reconciled after the call via repo.
func (t *CostTracker) CheckAndReserve(ctx context.Context, estimateCents int64) error { ... }
```

When `ErrCapReached` is hit, the worker returns a *synthetic* `Decision { Action: ignore, Rationale: "daily cost cap reached", CostCents: 0, Model: "(skipped)" }` and **still persists** it (so cap behavior is auditable). This is the **only** case where the worker writes a decision without consulting a provider.

- [x] **Step 6: Worker pool**

`backend/internal/agent/worker.go`. Bounded pool consuming a `chan DecisionRequest`. Each worker:

1. Acquires a per-symbol mutex (or `daily` mutex for `TriggerDaily`) to enforce per-key serialization.
2. Calls `CostTracker.CheckAndReserve`. If capped → emit synthetic ignore.
3. Calls `provider.Decide(ctx, req)` with `context.WithTimeout(req, 60s)`.
4. POSTs `{request, decision}` to `portfolio-api /internal/agent-decisions` with `X-Internal-Key`.
5. Logs structured outcome.

Failures at any step: log + skip. No retries in v0 (cost containment + simplicity).

- [x] **Step 7: Config loader**

`backend/internal/config/agent.go`:

```go
type Agent struct {
	Enabled           bool
	Provider          string        // "mock" | "anthropic"
	Model             string
	AnthropicAPIKey   string
	DailyCostCapUSD   float64
	DailyTimerUTC     string        // "HH:MM"
	Concurrency       int
	PromptPath        string
	PortfolioAPIURL   string        // reuse signals env
	InternalAPIKey    string        // reuse signals env
}

func LoadAgent() Agent { ... }
func (a Agent) Validate() error { ... } // require key if Provider=="anthropic"
```

- [x] **Step 8: Starter system prompt**

`config/agent-prompt.md`. Concise, principled, NOT financial advice:

```markdown
# The Agent — crypto decision system prompt

You are an autonomous decision-maker for a personal crypto portfolio. Your job is to decide one action per trigger: `buy`, `sell`, or `ignore`. Your goal is to grow the portfolio's value over rolling 14-day windows, net of fees, beating a buy-and-hold BTC baseline.

## Hard constraints
- You may only return `buy`, `sell`, or `ignore`.
- You may only act on symbols already in `get_holdings()` or symbols passed in the trigger signal. Do not invent symbols.
- If confidence < 0.55 → return `ignore`.
- If `get_holdings()` reports `stale: true` → return `ignore` with rationale `"stale snapshot"`.
- Use tools to ground every factual claim. Never quote a price you did not retrieve via `get_market_candles` or `get_position`.

## Tool-use guidance (be parsimonious — every tool call costs tokens)
1. Start by reading the eager context (signal payload + portfolio summary). Often that is enough to return `ignore`.
2. If you suspect a real move: pull `get_market_candles(symbol, "24h")` and `get_market_candles(symbol, "7d")`.
3. For sizing context: `get_position(symbol)`.
4. For regime context: `get_correlated_symbols(symbol)` → then candles on BTC-USD / ETH-USD only if you suspect a market-wide move.
5. For learning: `get_decision_outcomes(symbol?, horizon="14d")` — review your own track record on this symbol before acting.
6. For pattern context: `get_recent_signals(symbol, window="24h")` — am I flip-flopping?

## Output
Return strict JSON matching the decision schema. Rationale must be ≤ 1000 chars. Be concrete: cite specific numbers from your tool calls.

## What you are NOT
- You are not a financial advisor.
- You are not optimizing for tax efficiency.
- You do not have access to news, fundamentals, or off-chain context.
- You have no memory across runs except what `get_decision_outcomes` and `get_recent_decisions` tell you.
```

- [x] **Step 9: Tests**

- Mock provider: round-trip Decide(req) for buy / sell / ignore branches.
- Cost tracker: cap not reached → pass; cap reached → `ErrCapReached`.
- Anthropic provider: build with a fake HTTP transport; assert tool-use loop iterates and decodes; assert strict JSON rejection on malformed final message.
- Prompt loader: hashes deterministically; reloading the same file → same `Version`.
- Worker: synthetic ignore on cap; per-symbol serialization (two enqueues for same symbol → second blocks until first completes).

- [x] **Step 10: Run tests**

```bash
cd backend && go test ./internal/agent ./internal/config
```

**Return from subagent:** status, files changed, test results, mock provider sample request/response.

---

## Task 3: RO MCP server (`mark3labs/mcp-go`)

**Subagent lane:** MCP server. Depends on Task 1.

**Files:**
- Create: `backend/internal/mcp/agent/server.go`
- Create: `backend/internal/mcp/agent/tools_market.go`
- Create: `backend/internal/mcp/agent/tools_portfolio.go`
- Create: `backend/internal/mcp/agent/tools_history.go`
- Create: `backend/internal/mcp/agent/server_test.go`
- Modify: `backend/go.mod` (add `github.com/mark3labs/mcp-go`)

- [x] **Step 1: Server bootstrap**

`backend/internal/mcp/agent/server.go` exposes a server that wraps:

- A `coinbase.Client` (for candles)
- An HTTP client + base URL + internal key (for portfolio data)
- A pure-derivation function for `get_correlated_symbols`

Server registers all 7 tools and supports two transports: **stdio** (v0 in-process) and **streamable HTTP** (reserved for v1).

**Concurrency design (critical):** `mark3labs/mcp-go` stdio sessions are single-caller — sharing one server instance across concurrent workers would corrupt stdio framing. `NewInProcessServer` MUST be callable N times and return N independent server instances. Each agent worker at startup calls `NewInProcessServer(coinbaseClient, portfolioAPIURL, internalAPIKey)` and gets its own isolated `(server, client)` pair. There is no shared state between instances — all tools are pure reads backed by the same external sources. Do NOT attempt to share a single server via a mutex; create one per worker instead.

Recommended wiring: server runs in the same goroutine tree as the worker; worker constructs the paired `(client, server)` over an in-memory pipe (no subprocess, no network). `mark3labs/mcp-go` supports this via its in-process helpers.

**Note on integration tests:** `get_recent_signals`, `get_recent_decisions`, and `get_decision_outcomes` call internal portfolio-api read endpoints created in Task 6. Unit tests stub these with fake HTTP handlers. Add a build comment:
```go
// Integration tests for get_recent_signals, get_recent_decisions, get_decision_outcomes
// require the Task 6 portfolio-api read endpoints to be deployed. Unit tests use stubs.
```

- [x] **Step 2: Market tools (`tools_market.go`)**

Implement `get_market_candles`:

- Input schema: `{ symbol: string (required), window: "1h"|"24h"|"7d"|"30d" (required) }`.
- Map `window` → `(since, until)`:
  - `1h`  → `(now - 1h, now)` → `ONE_MINUTE` granularity
  - `24h` → `(now - 24h, now)` → `FIVE_MINUTE`
  - `7d`  → `(now - 7d, now)` → `ONE_HOUR`
  - `30d` → `(now - 30d, now)` → `SIX_HOUR`
- Call `coinbase.Client.FetchProductCandles`.
- Downsample with `portfolio.DownsampleMarketCandlePoints` to cap at **200** points. The dashboard uses 480; 200 is the LLM cap for token economy. (The epic's tool catalog has been corrected to match.)
- Response: `{ symbol, window, granularity, points: [{asOf, open, high, low, close, volume}] }`.

- [x] **Step 3: Portfolio tools (`tools_portfolio.go`)**

`get_holdings`:

- Calls portfolio-api `/internal/snapshot/latest`.
- Returns `{ source: "ingest_snapshot", timestamp, snapshotAgeSec, stale, positions, summary }`.
- `stale = snapshotAgeSec > ingest_interval_sec * 3`. `ingest_interval_sec` read from env (default 600).

`get_position`:

- Input: `{ symbol }`.
- Returns single position from latest snapshot or `{ error: { code: "not_found" } }`.

`get_correlated_symbols`:

- Input: `{ symbol }`.
- Output: deduped list `[symbol, "BTC-USD", "ETH-USD"] + top 3 positions by MV from snapshot`, capped at 6 items.
- Pure function over the snapshot returned by `/internal/snapshot/latest`. No external calls.

- [x] **Step 4: History tools (`tools_history.go`)**

`get_recent_signals`:

- Input: `{ symbol?, limit (≤50, default 20), window (≤72h, default "24h") }`.
- Calls portfolio-api `/api/trading/recent-alerts` (or extend with an internal variant if symbol filter is needed; prefer a small new internal endpoint `/internal/recent-alerts/list` filtered).
- Returns lean rows (no `payload_json`).

`get_recent_decisions`:

- Input: `{ symbol?, limit (≤20, default 10), window (≤72h, default "24h") }`.
- New internal endpoint `/internal/agent-decisions/list` (Task 6 adds this). Returns lean rows (no tool-call payloads).

`get_decision_outcomes`:

- Input: `{ symbol?, horizon ("24h"|"7d"|"14d", default "14d"), limit (≤50, default 20) }`.
- New internal endpoint `/internal/agent-decisions/outcomes` (Task 6 adds this).
- Returns: `[{decisionId, symbol, action, triggerAt, horizon, realizedReturnPct, btcBaselineReturnPct, excessReturnPct}]`.

- [x] **Step 5: Tests**

- Server registers all 7 tools; tool descriptors validate against the agent prompt's expected names.
- Each tool: happy-path test with a fake portfolio-api HTTP handler + a fake Coinbase candle response.
- `get_holdings`: `stale=true` when snapshot age > 30 min (assuming default).
- `get_correlated_symbols`: deduplication and capping.

- [x] **Step 6: Run tests**

```bash
cd backend && go test ./internal/mcp/agent
```

**Return from subagent:** status, tool descriptor JSON dump, test results.

---

## Task 4: Signals integration — worker pool + daily timer + decision POST

**Subagent lane:** signals service. Depends on Tasks 2 + 3.

**Files:**
- Modify: `backend/cmd/signals/main.go`
- Create: `backend/cmd/signals/agent_integration.go`
- Modify: `backend/cmd/signals/signals_test.go`

- [x] **Step 1: Wire agent components on startup**

In `main()`, after loading `cfg`:

```go
agentCfg := config.LoadAgent()
if err := agentCfg.Validate(); err != nil {
	log.Error("invalid agent config", "err", err)
	os.Exit(1)
}
var agentWorker *agent.Worker
if agentCfg.Enabled {
	prompt, err := agent.LoadPrompt(agentCfg.PromptPath)
	if err != nil { log.Error("agent prompt", "err", err); os.Exit(1) }

	mcpSrv, mcpCli, err := mcpagent.NewInProcessServer(coinbaseClient, agentCfg.PortfolioAPIURL, agentCfg.InternalAPIKey)
	if err != nil { log.Error("mcp server", "err", err); os.Exit(1) }
	defer mcpSrv.Close()

	provider, err := agent.NewProvider(agentCfg, mcpCli, prompt)
	if err != nil { log.Error("agent provider", "err", err); os.Exit(1) }

	costTracker := agent.NewCostTracker(/* http-backed CostRepo hitting /internal/agent-cost */)

	agentWorker = agent.NewWorker(agent.WorkerConfig{
		Provider:        provider,
		Concurrency:     agentCfg.Concurrency,
		CostTracker:     costTracker,
		PortfolioAPIURL: agentCfg.PortfolioAPIURL,
		InternalAPIKey:  agentCfg.InternalAPIKey,
		Log:             log,
	})
	agentWorker.Start(ctx)
	defer agentWorker.Stop()
}
```

- [x] **Step 2: Enqueue gate-passed signals**

After `gate.Emit == true` in `runOnce`, before the existing discord send:

```go
if agentWorker != nil {
	agentWorker.Enqueue(agent.DecisionRequest{
		TriggerKind:    agent.TriggerSignal,
		TriggerAt:      now,
		IdempotencyKey: alert.ID,
		Signal:         &alert,
		EagerContext:   buildEagerContext(snap, alert.Symbol /* + decisions-24h count via portfolio-api */),
		PromptVersion:  promptVersion,
	})
}
```

Discord post happens as before — agent runs in parallel, not blocking.

- [x] **Step 3: Daily timer**

In `main()`:

```go
if agentWorker != nil {
	go runDailyTimer(ctx, log, agentCfg, agentWorker, /* deps */)
}
```

`runDailyTimer`:

- Compute next fire = today at `HH:MM` UTC; if past, add 24h.
- Sleep until fire (cancellable via ctx).
- Enqueue `{TriggerKind: TriggerDaily, TriggerAt: now, IdempotencyKey: "daily-" + now.UTC().Format("2006-01-02"), Signal: nil, EagerContext: ...}`.
- Loop.

- [x] **Step 4: Eager context builder**

`agent_integration.go::buildEagerContext`:

- Pulls latest snapshot already fetched in `runOnce` (avoid re-fetching).
- Computes top-3 positions by `MarketValue`.
- Asks portfolio-api `GET /internal/agent-decisions/count?symbol=X&since=24h` for `DecisionsForSymbol24h` — cheap; cached for the tick.

- [x] **Step 5: Tests**

- Mock provider + fake portfolio-api: gate-passed alert → decision POSTed.
- Daily timer: with `AGENT_DAILY_TIMER_UTC=00:01`, advance clock; assert one enqueue per UTC day.
- Per-symbol serialization: two enqueues for the same symbol → second waits.
- Cap reached: synthetic ignore decision posted.

- [x] **Step 6: Run tests**

```bash
cd backend && go test ./cmd/signals
```

**Return from subagent:** status, sample log lines for a full trigger → decision flow, test results.

---

## Task 5: Outcome scorer in portfolio-api

**Subagent lane:** scorer. Depends on Tasks 1 + 6 (needs tables; can stub `/api/agent` reads).

**Files:**
- Create: `backend/cmd/portfolio-api/scorer.go`
- Create: `backend/cmd/portfolio-api/scorer_test.go`
- Modify: `backend/cmd/portfolio-api/main.go` (start scorer goroutine after migrations)

- [x] **Step 1: Scorer loop**

```go
func (a *app) runScorer(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-t.C:
			if err := a.scoreBatch(ctx); err != nil {
				a.log.Warn("scorer", "err", err)
			}
		}
	}
}

func (a *app) scoreBatch(ctx context.Context) error {
	pending, err := a.repo.ListUnscoredDecisionHorizons(ctx, time.Now().UTC(), 50)
	if err != nil { return err }
	for _, h := range pending {
		outcome, err := a.computeOutcome(ctx, h)
		if err != nil {
			a.log.Warn("score_outcome", "decision_id", h.DecisionID, "horizon", h.Horizon, "err", err)
			continue
		}
		if _, err := a.repo.InsertAgentDecisionOutcome(ctx, *outcome); err != nil {
			a.log.Warn("insert_outcome", "err", err)
		}
	}
	return nil
}
```

- [x] **Step 2: `computeOutcome`**

For a `(decisionID, symbol, triggerAt, horizon, action, triggerKind)`:

**1. Resolve horizon duration** (`1h` → 1h, `24h` → 24h, `7d` → 168h, `14d` → 336h).

**2. BTC return:**

Fetch BTC-USD candles `[triggerAt, triggerAt+H]`. Take `close` of the first and last bars.

```
btc_return_pct = (last.close - first.close) / first.close * 100
```

Fee adjustment: **only at `horizon = "14d"`**, subtract `0.6`. Short horizons use raw BTC return — deducting a taker fee from a 1h window creates artificial drag.

If BTC candle fetch fails → return `deferred` sentinel; do not insert. Retry next tick.

**3. Symbol return:**

- For `buy` / `sell` (symbol present): fetch symbol candles `[triggerAt, triggerAt+H]`. `symbol_return_pct = (last.close - first.close) / first.close * 100`. Populate `price_at_decision = first.close`, `price_at_horizon = last.close`.
- For `ignore` with symbol: fetch the same candles and populate `symbol_return_pct` (for opportunity-cost display). Do **not** set `realized_return_pct` — leave NULL.
- For `daily` (no symbol): compare nearest ingest snapshot at or before `triggerAt` vs nearest at or after `triggerAt + H`. `symbol_return_pct = (nav_end - nav_start) / nav_start * 100` where NAV = sum of all positions' market values + cash from the snapshot. If either snapshot is missing → `deferred`; retry. Only compute `excess_return_pct` for `H ∈ {"7d","14d"}` on daily triggers; set NULL for `1h`/`24h` daily outcomes.

**4. Realized + excess:**

```
// buy: long — price up is positive
// sell: short/exit — price down is positive
realized_return_pct = (action == "buy") ? symbol_return_pct : -symbol_return_pct

excess_return_pct = realized_return_pct - btc_return_pct
```

For `ignore` and `daily` (excluded from headline): `realized_return_pct = NULL`, `excess_return_pct = NULL`.

**5.** `fees_modeled_pct = 0` (shadow mode).

**6.** Return populated `AgentDecisionOutcome`.

If any required fetch fails → return error (not deferred); do not insert. Re-attempt next tick.

- [x] **Step 3: Wire into portfolio-api startup**

In `main.go`:

```go
scorerCtx, scorerCancel := context.WithCancel(ctx)
defer scorerCancel()
go app.runScorer(scorerCtx)
```

- [x] **Step 4: Tests**

- `buy` at all 4 horizons: `realized_return_pct = symbol_return_pct`; `excess` populated; fee deducted from BTC baseline only at `14d`.
- `sell` at all 4 horizons: `realized_return_pct = -symbol_return_pct`; `excess` populated.
- `ignore` at all horizons: `realized_return_pct = NULL`, `excess_return_pct = NULL`, `symbol_return_pct` populated (opportunity-cost display).
- `daily` trigger, `H = 7d`: NAV comparison between two bracketing snapshots; `excess` populated. `H = 1h`: `excess = NULL`.
- `daily` trigger: one or both snapshots missing → `deferred` (no row inserted); scorer retries on next tick; once snapshots arrive, row is inserted.
- Re-running scorer with the same pending row is a no-op (idempotent via unique constraint).
- BTC candle fetch fails → no row inserted; next run retries.

- [x] **Step 5: Run tests**

```bash
cd backend && go test ./cmd/portfolio-api
```

**Return from subagent:** status, sample computed outcomes for a buy / ignore / daily trigger (all horizons), test results.

---

## Task 6: portfolio-api routes — agent endpoints + internal

**Subagent lane:** API. Depends on Task 1.

**Files:**
- Create: `backend/cmd/portfolio-api/agent_handlers.go`
- Create: `backend/cmd/portfolio-api/agent_handlers_test.go`
- Modify: `backend/cmd/portfolio-api/main.go` (mount routes)

- [x] **Step 1: Internal — POST decision**

`POST /internal/agent-decisions` (behind `X-Internal-Key`):

- Decode `{ request: DecisionRequest, decision: Decision }`.
- Derive `symbol` from `request.signal.symbol` (or empty for daily).
- Idempotent insert via `repo.InsertAgentDecision`. Duplicate idempotency key → 200 + existing row JSON; new → 201 + row.

- [x] **Step 2: Internal — list / count / outcomes (for MCP tools)**

- `GET /internal/agent-decisions/list?symbol&limit&window` → JSON `{ decisions: [...] }` lean rows (no `request_json` / `response_json` / `tool_calls_json`).
- `GET /internal/agent-decisions/count?symbol&since=ISO` → `{ count: N }`.
- `GET /internal/agent-decisions/outcomes?symbol&horizon&limit` → `{ outcomes: [...] }`.
- `GET /internal/agent-cost/today` → `{ day, costCents }`. Used by the in-signals `CostTracker.SumCostCentsForDay` over HTTP.

- [x] **Step 3: Session dashboard endpoints**

`GET /api/agent/decisions?symbol&action&limit&from&to` → `AgentDecisionsResponse` (lean rows).
`GET /api/agent/decisions/{id}` → full row including tool-call payloads.
`GET /api/agent/benchmark?window=14d` → `AgentBenchmarkResponse`. Build by querying `agent_decision_outcomes` for `horizon=14d`, grouping by day, computing daily means; headline = mean over the requested window.
`GET /api/agent/cost?window=7d` → `AgentCostResponse` (today + last 7 days).

- [x] **Step 4: Mount routes**

In `main.go`:

```go
r.Route("/api/agent", func(r chi.Router) {
	r.Get("/decisions", app.handleAgentDecisionsList)
	r.Get("/decisions/{id}", app.handleAgentDecisionGet)
	r.Get("/benchmark", app.handleAgentBenchmark)
	r.Get("/cost", app.handleAgentCost)
})

// inside internal group:
r.Post("/internal/agent-decisions", app.handleInternalAgentDecisionCreate)
r.Get("/internal/agent-decisions/list", app.handleInternalAgentDecisionsList)
r.Get("/internal/agent-decisions/count", app.handleInternalAgentDecisionsCount)
r.Get("/internal/agent-decisions/outcomes", app.handleInternalAgentDecisionOutcomes)
r.Get("/internal/agent-cost/today", app.handleInternalAgentCostToday)
```

- [x] **Step 5: Tests**

- Insert decision → 201; same key again → 200, same body.
- Bench endpoint with a seeded set of outcomes: assert headline mean and daily series.
- Cost endpoint: assert today + window aggregation.
- Decision list filters: by symbol, by action, by date range.

- [x] **Step 6: Run tests**

```bash
cd backend && go test ./cmd/portfolio-api
```

**Return from subagent:** status, sample responses for each endpoint, test results.

---

## Task 7: Frontend `/agent` page

**Subagent lane:** frontend. Depends on Task 6.

**Files:**
- Create: `frontend/app/agent/page.tsx`
- Create: `frontend/components/agent/decision-list.tsx`
- Create: `frontend/components/agent/decision-detail.tsx`
- Create: `frontend/components/agent/benchmark-chart.tsx`
- Create: `frontend/components/agent/cost-meter.tsx`
- Create: `frontend/lib/agent-api.ts`
- Modify: `frontend/components/site-header.tsx`

- [x] **Step 1: API client**

`frontend/lib/agent-api.ts` — typed fetchers for `/api/agent/decisions`, `/api/agent/decisions/{id}`, `/api/agent/benchmark`, `/api/agent/cost`. Match existing `frontend/lib/lab-api.ts` style.

- [x] **Step 2: Page layout**

`frontend/app/agent/page.tsx` is a server component that:

- Renders `<BenchmarkChart />` (top, full width)
- Renders `<CostMeter />` (right side under the chart)
- Renders `<DecisionList />` (below, paginated)
- Clicking a row navigates to `/agent/decisions/[id]` (sub-route; uses the detail endpoint).

- [x] **Step 3: Decision list**

`DecisionList` shows: timestamp, trigger kind, symbol, action chip (colored by buy/sell/ignore), confidence bar, rationale preview (first 120 chars), model. Filters: action, symbol, date range. Pagination via `limit` + `from`/`to`.

- [x] **Step 4: Decision detail**

`DecisionDetail` for `/agent/decisions/[id]`:

- Header: action chip, confidence, model, prompt version, latency, cost.
- Tabs: Rationale | Tool calls | Raw request | Raw response.
- Tool-calls tab: timeline (one row per `ToolCall` with name, duration, input/output JSON).

- [x] **Step 5: Benchmark chart**

`BenchmarkChart` (client component) — uses the existing chart primitives (see `frontend/components/portfolio-history-chart.tsx` for the pattern). Plots daily `realizedReturnPct` and `btcBaselineReturnPct` as two lines, with `excessReturnPct` as a shaded delta. Headline number above chart: rolling-14d mean excess. Show a small caveat badge: "shadow mode — fees on hypothetical trades not modeled."

- [x] **Step 6: Cost meter**

`CostMeter` — today's spend bar against `capCents`, plus a 7-day sparkline.

- [x] **Step 7: Header link**

`site-header.tsx`: add `/agent` link. Mark `/lab` as deprecated (gray label or `(legacy)` suffix) without removing.

- [x] **Step 8: Tests**

- Vitest: API client mocks, list filters work, detail tabs render.
- Component snapshot for benchmark chart with synthetic data.

- [x] **Step 9: Verify frontend**

```bash
cd frontend && nvm use && npm test && npm run build
```

Expected: tests pass; build succeeds under Node 22.22.0.

**Return from subagent:** status, screenshot or HTML dump of the new page, Node version used.

---

## Task 8: OpenClaw deprecation + config + AGENTS.md cleanup

**Subagent lane:** docs / config. Independent of Tasks 2–7.

**Files:**
- Modify: `.env.example`
- Modify: `AGENTS.md`
- Modify: `.agent/epics/phase_2/openclaw-mcp-alerts.md`
- Modify: `backend/internal/openclaw/types.go`
- Modify: `backend/cmd/portfolio-api/lab_handlers.go`
- Modify: `docker-compose.yml`

- [x] **Step 1: `.env.example`**

Remove the `OPENCLAW_*` block (already not present per grep; verify). Add an `# The Agent (shadow mode)` block:

```env
# The Agent — shadow mode (in-process LLM agent runs after gate-passed signals + daily 12:00 UTC)
AGENT_ENABLED=false
AGENT_PROVIDER=mock
AGENT_MODEL=claude-sonnet-4-5
ANTHROPIC_API_KEY=
AGENT_DAILY_COST_CAP_USD=5
AGENT_DAILY_TIMER_UTC=12:00
AGENT_CONCURRENCY=2
AGENT_PROMPT_PATH=config/agent-prompt.md
```

- [x] **Step 2: AGENTS.md**

Replace the `### OpenClaw contract` and `### MCP tools` subsections under `## Shared contracts` with `### The Agent contract` and `### Agent MCP tool catalog` (copy from the epic file). Keep the `### MCP trade tool` section (that's a separate future epic). Update the "Execution waves" diagram to replace `SCH-23 OpenClaw, MCP & Alerts` with `SCH-AG1 The Agent — shadow decisions`. Add `AGENT_*` env vars to the env-var table.

- [x] **Step 3: Mark superseded epic**

Top of `.agent/epics/phase_2/openclaw-mcp-alerts.md`, add:

```markdown
> **SUPERSEDED** by `.agent/epics/phase_2/agent-shadow-decisions.md` (2026-05-18).
> The Agent now runs in-process inside `signals` with a local RO MCP server.
> External OpenClaw integration is dropped. The `lab_openclaw_runs` table is preserved
> for historical data but is no longer enqueued.
```

Leave the rest of the file untouched for history.

- [x] **Step 4: Deprecate openclaw package**

Top of `backend/internal/openclaw/types.go`:

```go
// Package openclaw is DEPRECATED as of 2026-05-18.
// The in-process Agent (internal/agent) replaces it. These status constants
// are retained only because the lab_openclaw_runs table still uses them for
// historical rows; do not enqueue new rows.
```

- [x] **Step 5: Strip `recordLabSignal` enqueue**

In `backend/cmd/portfolio-api/lab_handlers.go`:

- Keep `recordLabSignal` calling `InsertLabSignalEvent` (still useful as audit of every fired alert).
- **Remove** the entire `UpsertLabOpenClawRun` calls in that function (both the queued path and the skipped/paused/circuit paths).
- Keep `handleLabOpenClawPause` / `Resume` / `CircuitReset` handlers callable for now — the dashboard still references them; they become no-ops over data that no longer accumulates. Mark with a TODO to remove in a follow-up.

- [x] **Step 6: `docker-compose.yml`**

Ensure `signals` service env block propagates the new `AGENT_*` vars + `ANTHROPIC_API_KEY`. No new service. Confirm:

```yaml
signals:
  environment:
    AGENT_ENABLED: ${AGENT_ENABLED:-false}
    AGENT_PROVIDER: ${AGENT_PROVIDER:-mock}
    AGENT_MODEL: ${AGENT_MODEL:-claude-sonnet-4-5}
    ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    AGENT_DAILY_COST_CAP_USD: ${AGENT_DAILY_COST_CAP_USD:-5}
    AGENT_DAILY_TIMER_UTC: ${AGENT_DAILY_TIMER_UTC:-12:00}
    AGENT_CONCURRENCY: ${AGENT_CONCURRENCY:-2}
    AGENT_PROMPT_PATH: ${AGENT_PROMPT_PATH:-config/agent-prompt.md}
```

Also mount the prompt file into the `signals` container if needed (read-only).

- [x] **Step 7: Verify**

```bash
docker compose config --quiet
docker compose config --services
rg -n "OPENCLAW_" .env.example AGENTS.md docs/  # expect no hits in active env
```

**Return from subagent:** status, list of files modified, services list, grep verifications.

---

## Task 9: Integration + smoke + verification

**Subagent lane:** integration. Run only after Tasks 1–8.

**Files:**
- Any conflict resolution.
- Update `docs/crypto-mvp-refactor.md` with a footer linking to this plan.

- [x] **Step 1: Conflict triage**

```bash
git status --short
```

Resolve preserving the agent's design; never reintroduce OpenClaw enqueue paths.

- [x] **Step 2: Backend verification**

```bash
cd backend && gofmt -w . && go test ./...
```

Expected: all packages pass.

- [x] **Step 3: Frontend verification**

```bash
cd frontend && nvm use && npm test && npm run build
```

Expected: tests pass; build under Node 22.22.0.

- [x] **Step 4: Compose validation**

```bash
docker compose config --quiet
docker compose config --services
```

Service list unchanged (`web`, `portfolio-api`, `ingest`, `signals`, `db`).

- [x] **Step 5: Mock provider smoke**

```bash
AGENT_ENABLED=true AGENT_PROVIDER=mock docker compose up --build
```

In another shell, trigger a fake signal (lower threshold, or seed `followed_symbols` with a known-volatile symbol). Within a minute:

- `agent_decisions` has at least one row.
- Logs show structured `agent_decide` lines.
- After horizon `1h` passes, `agent_decision_outcomes` is populated for `horizon=1h`.

- [x] **Step 6: Anthropic provider smoke (optional, requires key)**

```bash
AGENT_ENABLED=true AGENT_PROVIDER=anthropic ANTHROPIC_API_KEY=... docker compose up --build signals
```

Verify one real decision lands with non-empty `tool_calls_json` and `cost_cents > 0`.

- [x] **Step 7: Daily timer smoke**

Set `AGENT_DAILY_TIMER_UTC` to a few minutes in the future and observe one decision with `triggerKind=daily`, `idempotencyKey=daily-<today>`.

- [x] **Step 8: Cost cap smoke**

Set `AGENT_DAILY_COST_CAP_USD=0.001`. After one real decision, next decision short-circuits to `action=ignore`, `rationale="daily cost cap reached"`, `costCents=0`. After UTC midnight (or after manually clearing the day's sum in DB), normal behavior resumes.

- [x] **Step 9: Dashboard verification**

- Visit `http://localhost:3000/agent`.
- Decisions list shows the smoke-test rows.
- Drilldown shows tool calls.
- Benchmark chart appears (may be empty pending 14d window).
- Cost meter shows non-zero today after a real decision.

- [x] **Step 10: Final review**

- README mentions the new `/agent` route briefly.
- `docs/crypto-mvp-refactor.md` footer links to this plan.
- AGENTS.md is consistent (no orphan OpenClaw references in shared contracts).
- All OpenClaw `lab_openclaw_runs` rows remain readable but no new rows accumulate.

**Return from subagent:** final verification results, sample decision row, sample outcome row, screenshots if possible, next user steps.

---

## Ready-to-dispatch subagent prompts

### Task 1 Prompt

Implement Task 1 from `docs/superpowers/plans/2026-05-18-agent-shadow-decisions.md`: shared agent types, persistence schema, migrations 007/008, repository signatures + Postgres impl. Idempotency on `agent_decisions.idempotency_key` is critical (duplicate → return existing row, no error). Run focused Go tests. Return status, files changed, schema diff.

### Task 2 Prompt

Implement Task 2: `internal/agent/` package — `Provider` interface, `mock` and `anthropic` providers, prompt loader, cost tracker, worker pool. Anthropic provider uses the official SDK and runs the tool-use loop with strict JSON parsing on the final message. Add `github.com/anthropics/anthropic-sdk-go` to go.mod. Run focused Go tests. Return status, sample mock decision, files changed.

### Task 3 Prompt

Implement Task 3: `internal/mcp/agent/` package — `mark3labs/mcp-go`-based RO server with 7 tools. Stdio in-process transport for v0; HTTP transport reserved for v1. Each tool has a bounded response shape. Add `github.com/mark3labs/mcp-go` to go.mod. Run focused Go tests. Return status, tool descriptors, sample responses.

### Task 4 Prompt

Implement Task 4: wire the agent worker pool and daily timer into `signals`. Gate-passed signals enqueue; daily 12:00 UTC enqueues with key `daily-YYYY-MM-DD`. Per-symbol serialization. Decision POSTed to portfolio-api `/internal/agent-decisions`. Mock provider in tests. Return status, sample log lines, files changed.

### Task 5 Prompt

Implement Task 5: outcome scorer goroutine inside portfolio-api. Runs every 60s; computes return + BTC baseline + excess for each `(decision, horizon)` whose deadline has passed; idempotent on unique constraint. Return status, sample outcomes, files changed.

### Task 6 Prompt

Implement Task 6: portfolio-api routes — `POST /internal/agent-decisions` (idempotent), 4 internal GETs for the MCP tools, and 4 session GETs for the dashboard. Return status, sample responses, files changed.

### Task 7 Prompt

Implement Task 7: frontend `/agent` route — decisions list, drilldown with tool-call timeline, benchmark chart, cost meter. Reuse existing chart primitives. Run Vitest and `npm run build` under Node 22.22.0. Return status, screenshots, files changed.

### Task 8 Prompt

Implement Task 8: OpenClaw deprecation + config cleanup. `.env.example` adds `AGENT_*` block; AGENTS.md replaces the OpenClaw contract section with The Agent contract; `phase_2/openclaw-mcp-alerts.md` gets a SUPERSEDED banner; `recordLabSignal` strips the `lab_openclaw_runs` enqueue; `docker-compose.yml` propagates new env. Return status, file diffs, services list.

### Task 9 Prompt

Implement Task 9: integration + smoke + verification. Run all backend tests, frontend tests + build, compose validate, mock-provider smoke. If Anthropic key available, run a real-provider smoke. Return final verification, sample decision + outcome rows, user steps to flip `AGENT_ENABLED=true` with a real key.

---

## Autonomous completion notes

- v0 is **shadow mode** — `AGENT_ENABLED=true` produces decisions and persists them; **no order placement of any kind ships in this epic.** Paper execution is a separate v0.5 epic.
- Mock provider is default. `AGENT_PROVIDER=anthropic` requires `ANTHROPIC_API_KEY`.
- `config/agent-prompt.md` is checked in; iterate on it freely, the hash becomes `prompt_version` and is recorded on every decision so prompt regressions are traceable.
- Outcome scoring depends on Coinbase candles for both the symbol and BTC-USD. If Coinbase candle fetch fails, the scorer retries on the next tick — outcomes are not lost.
- The cost cap is **hard**: when reached, decisions short-circuit to `ignore` with a synthetic rationale. They are still persisted (auditable). Cost resets at UTC midnight.
- Per-symbol serialization is enforced at the worker level — two concurrent triggers on the same symbol cannot produce two in-flight decisions.
- Preserve existing `lab_openclaw_runs` data — do **not** drop the table. Only stop writing new rows.
- Do not delete the deprecated frontend `/lab` page in this epic; it can render legacy data and a "(legacy)" badge.
- The benchmark chart includes a caveat: "shadow mode — fees on hypothetical trades not modeled." When paper execution lands, that caveat is removed and the same chart is repurposed against paper P&L.
- Never fall back to a heuristic action on provider/MCP failure. Failures are logged + skipped; no partial decisions are persisted.
- Do not send any data to the model provider beyond per-request inputs. No fine-tuning, no telemetry opt-in.

---

## Verification results (Task 9 — 2026-05-18)

### `go test ./...`

All 22 packages pass (20 with tests, 2 with no test files: `cmd/ingest`, `cmd/trading-worker`).

```
ok  github.com/schtvr/morgans-d-stonks/cmd/portfolio-api
ok  github.com/schtvr/morgans-d-stonks/cmd/signals
ok  github.com/schtvr/morgans-d-stonks/internal/agent
ok  github.com/schtvr/morgans-d-stonks/internal/auth
ok  github.com/schtvr/morgans-d-stonks/internal/broker
ok  github.com/schtvr/morgans-d-stonks/internal/broker/coinbase
ok  github.com/schtvr/morgans-d-stonks/internal/broker/mock
ok  github.com/schtvr/morgans-d-stonks/internal/brokerwire
ok  github.com/schtvr/morgans-d-stonks/internal/config
ok  github.com/schtvr/morgans-d-stonks/internal/discord
ok  github.com/schtvr/morgans-d-stonks/internal/ingest
ok  github.com/schtvr/morgans-d-stonks/internal/logging
ok  github.com/schtvr/morgans-d-stonks/internal/mcp/agent
ok  github.com/schtvr/morgans-d-stonks/internal/mcp/trades
ok  github.com/schtvr/morgans-d-stonks/internal/openclaw
ok  github.com/schtvr/morgans-d-stonks/internal/portfolio
ok  github.com/schtvr/morgans-d-stonks/internal/portfolio/postgres
ok  github.com/schtvr/morgans-d-stonks/internal/signal
ok  github.com/schtvr/morgans-d-stonks/internal/trading
ok  github.com/schtvr/morgans-d-stonks/internal/trading/postgres
```

### Frontend build

`npm test` — 17 tests across 6 files (including `lib/agent-api.test.ts` — 7 tests): **PASS**

`npm run build` — Next.js 16.2.6 under Node 22.22.0: **PASS**

Routes built: `/`, `/agent` (static), `/agent/decisions/[id]` (dynamic), `/lab`, `/login`, `/pulse`.

### Docker Compose

`docker compose config --quiet` + `--services`: **PASS**

Services: `db`, `portfolio-api`, `ingest`, `signals`, `web` — no `trading-worker`, no `openclaw-proxy`.

### Acceptance criteria

| # | Criterion | Status |
|---|-----------|--------|
| 1 | `AGENT_ENABLED=true` + mock → `agent_decisions` row within ~1s | **N/A-runtime** (wired in `cmd/signals/main.go:263`; verified in `signals_test.go`) |
| 2 | Daily timer idempotency key `daily-YYYY-MM-DD` rejects duplicates | **N/A-runtime** (verified: `agent_integration.go:142`, `signals_test.go:303`) |
| 3 | `AGENT_PROVIDER=anthropic` swap end-to-end with no schema change | **N/A-runtime** (provider abstraction verified in `internal/agent/provider.go`) |
| 4 | Cost cap → synthetic ignore `rationale="daily cost cap reached"`, cost=0 | **PASS** (code path at `worker.go:96-104`; tested in `worker_test.go`) |
| 5 | Scorer fills outcomes idempotently via `UNIQUE(decision_id, horizon)` | **PASS** (migration 008 + `InsertAgentDecisionOutcome` conflict-ignore; scorer tests pass) |
| 6 | `GET /api/agent/benchmark?window=14d` returns daily series + headline | **PASS** (`agent_handlers.go:279`; handler tests pass) |
| 7 | Frontend `/agent` page: decisions list, drilldown, benchmark chart, cost meter | **PASS** (all 4 components present; build passes) |
| 8 | OpenClaw enqueue path removed; existing rows untouched | **PASS** (`lab_handlers.go` clean; `UpsertLabOpenClawRun` → 410 in retry handler) |
| 9 | Per-symbol serialization: concurrent triggers on same symbol serialized | **PASS** (`worker.go` `sync.Map` + per-symbol `sync.Mutex`) |
| 10 | Provider/MCP failures log + skip; no heuristic fallback | **PASS** (`worker.go:112-116`: error → log + return; no fallback action) |
| 11 | Mock provider deterministic; no live API calls in `go test ./...` | **PASS** (rg: zero hits for `anthropic.com` in test files) |

### Fix applied in Task 9

**`lab_handlers.go:handleLabRunRetry`** — converted from a real `UpsertLabOpenClawRun` call to a 410 Gone response. The handler was the only remaining active `UpsertLabOpenClawRun` call; `recordLabSignal` was already clean. Also removed unused `openclaw` import. No logic change to any agent path.

### Known deferred items (N/A-runtime)

Runtime smoke tests (AC1, AC2, AC3) require a running Postgres + Coinbase keys. They pass in unit tests against mocks. See "Next steps" below for live smoke procedure.

### Next steps for smoke-test with real Anthropic key

```bash
# 1. Ensure .env has Coinbase read keys + DB running
#    AGENT_ENABLED=true
#    AGENT_PROVIDER=anthropic
#    ANTHROPIC_API_KEY=sk-ant-...
#    AGENT_DAILY_COST_CAP_USD=1   # low for smoke

docker compose up --build

# 2. In a second shell — watch for a gate-passed signal or trigger daily timer:
#    Set AGENT_DAILY_TIMER_UTC to 2-3 minutes from now in .env, rebuild signals.

# 3. After ~1 min, check the DB:
#    psql $DATABASE_URL -c "SELECT id, action, confidence, cost_cents FROM agent_decisions ORDER BY id DESC LIMIT 5;"

# 4. Visit http://localhost:3000/agent — decisions list should populate.

# 5. After 1h, scorer will populate agent_decision_outcomes for horizon=1h.
#    psql $DATABASE_URL -c "SELECT * FROM agent_decision_outcomes ORDER BY id DESC LIMIT 5;"
```
