# Crypto Alert MVP Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the repo so the default path is a local homelab crypto portfolio/watchlist dashboard that ingests Coinbase read data, fires deterministic `crypto_signal_v1` price-move alerts to Discord for OpenClaw consumption, and keeps IBKR/trading backlog code out of the MVP runtime path.

**Architecture:** Keep `portfolio-api` as the single backend service and DB owner. Keep Coinbase ingest, watchlist, alert settings, recent alerts, and Discord signal emission in the MVP path; preserve IBKR and trading/order execution code as backlog/shelved code with tests, but remove them from default compose, dashboard, and route registration.

**Tech Stack:** Go 1.25 backend with chi, pgx, slog; Next.js frontend with npm/Vitest; Postgres; Docker Compose; Discord webhook integration.

---

## Agreed Product Decisions

- MVP promise: local homelab crypto portfolio/watchlist dashboard with deterministic Coinbase price-move alerts.
- Coinbase read API credentials are assumed present for the product path.
- Mock/demo remains useful for CI and first boot, but is not the primary product path.
- IBKR is backlog only, not a planned follow-up phase.
- Trading/order execution and MCP trade endpoints are retained as shelved code, but removed from the default runtime path.
- OpenClaw consumes the Discord-visible signal payload externally; this repo does not host an OpenClaw MCP server for MVP.
- Signal payload is crypto-only `v1`.
- Persist exact JSON payload sent to Discord alongside normalized alert fields.
- Discord message is a human-readable one-line summary plus a fenced JSON code block.
- Dashboard shows persisted alert records only, not Discord/OpenClaw delivery status.
- Existing `.agent/epics` are historical context; current docs become the source of truth.

## Current Repo Facts

- `backend/cmd/portfolio-api/main.go` registers portfolio, watchlist, alert settings, recent alerts, order, MCP trade, and metrics routes.
- `backend/cmd/signals/main.go` already builds `signal.CryptoAlert` and posts via `discord.CryptoAlertWebhookContent`.
- `backend/internal/signal/types.go` has `CryptoAlert`, but no schema version or stable event ID.
- `backend/internal/discord/signal_message.go` currently returns raw compact JSON for crypto alerts.
- `backend/internal/portfolio/postgres/migrations/004_recent_alerts.sql` stores normalized recent-alert fields, but not exact payload JSON.
- `frontend/app/page.tsx` renders `TradeActivityCard`.
- `docker-compose.yml` includes `trading-worker` and `ib-gateway`.
- `README.md` and `.env.example` still present IBKR/trading too prominently for the agreed MVP.

## Parallelization Strategy

Run Task 1 first. It defines the shared `crypto_signal_v1` payload and persistence shape.

After Task 1 lands, Tasks 2, 3, 4, and 5 can run in parallel because they touch mostly disjoint files:

- Task 2: signal emission and Discord formatting.
- Task 3: runtime compose and route cleanup.
- Task 4: dashboard cleanup.
- Task 5: docs and env cleanup.

Task 6 is final integration and verification after all other tasks merge.

If using multiple implementation subagents at once, use isolated git worktrees or branches per task. Do not let parallel agents edit the same working tree.

## File Ownership Map

- Shared signal contract and persistence:
  - `backend/internal/signal/types.go`
  - `backend/internal/portfolio/types.go`
  - `backend/internal/portfolio/repository.go`
  - `backend/internal/portfolio/postgres/repository.go`
  - `backend/internal/portfolio/postgres/migrations/005_recent_alert_payload.sql`
  - relevant tests under `backend/internal/signal`, `backend/internal/portfolio`, `backend/internal/portfolio/postgres`

- Discord signal emission:
  - `backend/internal/discord/signal_message.go`
  - `backend/internal/discord/signal_message_test.go`
  - `backend/cmd/signals/main.go`
  - `backend/cmd/signals/signals_test.go`

- Default runtime cleanup:
  - `docker-compose.yml`
  - `docker-compose.coinbase.yml`
  - `backend/cmd/portfolio-api/main.go`
  - `backend/cmd/portfolio-api/trading_handlers_test.go`
  - `backend/internal/config/trading.go` if feature-gate naming needs clarification

- Dashboard MVP cleanup:
  - `frontend/app/page.tsx`
  - `frontend/components/trade-activity-card.tsx` only if deleting or moving to backlog; prefer leaving unused rather than deleting in first PR.

- Docs and environment:
  - `README.md`
  - `.env.example`
  - `docs/crypto-mvp-refactor.md`
  - optionally `docs/runbooks/coinbase-trading.md` with an upfront backlog notice
  - optionally `docs/mcp-crypto-execution-spec.md` with an upfront backlog notice

---

## Task 1: Define `crypto_signal_v1` Payload and Persist Exact JSON

**Subagent lane:** shared contract. Must complete before Task 2.

**Files:**
- Modify: `backend/internal/signal/types.go`
- Modify: `backend/internal/portfolio/types.go`
- Modify: `backend/internal/portfolio/repository.go`
- Modify: `backend/internal/portfolio/postgres/repository.go`
- Create: `backend/internal/portfolio/postgres/migrations/005_recent_alert_payload.sql`
- Test: `backend/internal/signal/types_test.go` or existing signal tests
- Test: `backend/internal/portfolio/postgres/repository_test.go` or existing postgres tests

- [ ] **Step 1: Add explicit crypto signal schema fields**

In `backend/internal/signal/types.go`, extend `CryptoAlert` with stable schema and identity fields while preserving existing JSON names:

```go
type CryptoAlert struct {
	SchemaVersion   string    `json:"schemaVersion"`
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	ReasonFlags     []string  `json:"reasonFlags,omitempty"`
	Symbol          string    `json:"symbol"`
	ProductID       string    `json:"productId,omitempty"`
	Source          string    `json:"source,omitempty"`
	CurrentPrice    float64   `json:"currentPrice"`
	PreviousPrice   *float64  `json:"previousPrice,omitempty"`
	DeltaAmount     *float64  `json:"deltaAmount,omitempty"`
	DeltaPct        float64   `json:"deltaPct"`
	ThresholdPct    float64   `json:"thresholdPct"`
	Quantity        *float64  `json:"quantity,omitempty"`
	AvgCost         *float64  `json:"avgCost,omitempty"`
	CostBasis       *float64  `json:"costBasis,omitempty"`
	UnrealizedPL    *float64  `json:"unrealizedPl,omitempty"`
	UnrealizedPLPct *float64  `json:"unrealizedPlPct,omitempty"`
	FiredAt         time.Time `json:"firedAt"`
}

const CryptoSignalSchemaVersion = "crypto_signal_v1"
```

Use `SchemaVersion` value `crypto_signal_v1`. Use `Type` value `crypto_price_move`.

- [ ] **Step 2: Add persisted payload field to portfolio recent alerts**

In `backend/internal/portfolio/types.go`, add the exact payload field:

```go
PayloadJSON json.RawMessage `json:"payloadJson,omitempty"`
```

Import `encoding/json` in that file.

- [ ] **Step 3: Add database migration for exact payload**

Create `backend/internal/portfolio/postgres/migrations/005_recent_alert_payload.sql`:

```sql
ALTER TABLE recent_alerts
ADD COLUMN IF NOT EXISTS payload_json JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_recent_alerts_payload_schema
    ON recent_alerts ((payload_json->>'schemaVersion'));
```

- [ ] **Step 4: Wire repository reads and writes**

Update `ListRecentAlerts` in `backend/internal/portfolio/postgres/repository.go` to select `payload_json` after `created_at` and scan it into `item.PayloadJSON`.

Update `InsertRecentAlert` to insert `payload_json`. If `alert.PayloadJSON` is empty, store `{}`.

Expected insert columns include:

```sql
payload_json
```

Expected exec argument includes:

```go
payloadJSON(alert.PayloadJSON)
```

Add a small helper:

```go
func payloadJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}
```

Import `encoding/json`.

- [ ] **Step 5: Update tests**

Add or update a repository test that inserts a recent alert with:

```json
{"schemaVersion":"crypto_signal_v1","id":"test-alert","type":"crypto_price_move","symbol":"BTC-USD"}
```

Then list recent alerts and assert:

```go
if !bytes.Contains(got[0].PayloadJSON, []byte(`"schemaVersion":"crypto_signal_v1"`)) {
	t.Fatalf("payload json not persisted: %s", got[0].PayloadJSON)
}
```

- [ ] **Step 6: Run backend tests**

Run:

```bash
cd backend && go test ./internal/signal ./internal/portfolio ./internal/portfolio/postgres
```

Expected: all selected packages pass.

**Return from subagent:** status, files changed, tests run, and any migration compatibility concerns.

---

## Task 2: Emit Discord Summary Plus Fenced `crypto_signal_v1` JSON

**Subagent lane:** signal emission. Depends on Task 1.

**Files:**
- Modify: `backend/internal/discord/signal_message.go`
- Modify: `backend/internal/discord/signal_message_test.go`
- Modify: `backend/cmd/signals/main.go`
- Modify: `backend/cmd/signals/signals_test.go`

- [ ] **Step 1: Make Discord crypto alert content human-readable plus JSON**

In `backend/internal/discord/signal_message.go`, update `CryptoAlertWebhookContent` so it returns a string shaped like this:

````text
crypto_price_move BTC-USD delta=1.23% threshold=1.00%
```json
{"schemaVersion":"crypto_signal_v1","id":"btc_usd-20260513T120000Z","type":"crypto_price_move","symbol":"BTC-USD"}
```
````

The code should use `json.Marshal` for compact machine-readable JSON. The first line should include symbol, delta pct, threshold pct, and current price. Keep output under Discord limits for normal payload size.

- [ ] **Step 2: Add deterministic tests for Discord content**

In `backend/internal/discord/signal_message_test.go`, assert:

```go
content, err := CryptoAlertWebhookContent(signal.CryptoAlert{
	SchemaVersion: signal.CryptoSignalSchemaVersion,
	ID:            "alert-1",
	Type:          "crypto_price_move",
	Symbol:        "BTC-USD",
	CurrentPrice:  100,
	DeltaPct:      1.25,
	ThresholdPct:  1,
	FiredAt:       fixedTime,
})
```

Assertions:

```go
strings.Contains(content, "crypto_price_move BTC-USD")
strings.Contains(content, "```json")
strings.Contains(content, `"schemaVersion":"crypto_signal_v1"`)
strings.Contains(content, `"id":"alert-1"`)
```

- [ ] **Step 3: Build stable payload in signals**

In `backend/cmd/signals/main.go`, update `buildCryptoAlert`:

```go
alert := sigpkg.CryptoAlert{
	SchemaVersion: sigpkg.CryptoSignalSchemaVersion,
	ID:            stableCryptoAlertID(symbol, firedAt),
	Type:          "crypto_price_move",
	ReasonFlags:   reasonFlags,
	Symbol:        symbol,
	ProductID:     symbol,
	Source:        item.Source,
	CurrentPrice:  currentPrice,
	DeltaPct:      decision.DeltaPct,
	ThresholdPct:  thresholdPct,
	FiredAt:       firedAt,
}
```

Add:

```go
func stableCryptoAlertID(symbol string, firedAt time.Time) string {
	return fmt.Sprintf("%s-%s", strings.ToLower(strings.ReplaceAll(symbol, "-", "_")), firedAt.UTC().Format("20060102T150405Z"))
}
```

This is deterministic and avoids adding a new UUID dependency to `cmd/signals`.

- [ ] **Step 4: Persist exact JSON payload**

In `runOnce`, after building `alert`, marshal `alert` to compact JSON before persisting:

```go
payloadJSON, err := json.Marshal(alert)
if err != nil {
	return err
}
```

Because `signal.CryptoAlert` should not import portfolio, do not put `PayloadJSON` on `signal.CryptoAlert`. Instead, ensure the portfolio API sets `portfolio.RecentAlert.PayloadJSON` from the compact JSON produced from the alert.

Recommended implementation: leave `persistRecentAlert` sending the `signal.CryptoAlert` body as it already does, and update `portfolio-api` `decodeCryptoAlert` in `backend/cmd/portfolio-api/internal_handler.go` to marshal the received `signal.CryptoAlert` and set `PayloadJSON` on `portfolio.RecentAlert`.

The local `payloadJSON` variable in `runOnce` is optional if `decodeCryptoAlert` handles this centrally. Keep one source of truth for the compact JSON.

- [ ] **Step 5: Preserve exact Discord-visible artifact**

The exact JSON in the Discord fenced block must match `payload_json` in `recent_alerts`. Both should come from `json.Marshal(alert)`.

- [ ] **Step 6: Run focused tests**

Run:

```bash
cd backend && go test ./internal/discord ./cmd/signals ./cmd/portfolio-api
```

Expected: all selected packages pass.

**Return from subagent:** status, Discord output example, tests run, and any payload compatibility notes.

---

## Task 3: Remove IBKR and Trading from Default Runtime Path

**Subagent lane:** runtime cleanup. Can run after Task 1; does not depend on Task 2.

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker-compose.coinbase.yml`
- Modify: `backend/cmd/portfolio-api/main.go`
- Modify: `backend/cmd/portfolio-api/trading_handlers_test.go`
- Optionally modify: `backend/internal/config/trading.go`

- [ ] **Step 1: Remove backlog services from base compose**

In `docker-compose.yml`, remove the `trading-worker` service and the `ib-gateway` service from the default stack.

Update `ingest` environment defaults to Coinbase-first:

```yaml
BROKER_PROVIDER: ${INGEST_BROKER_PROVIDER:-coinbase}
BROKER_ENV: ${INGEST_BROKER_ENV:-paper}
COINBASE_READ_API_KEY: ${COINBASE_READ_API_KEY:-}
COINBASE_READ_API_SECRET: ${COINBASE_READ_API_SECRET:-}
```

Remove `IBKR_*` env vars from the default `ingest` service. Keep `portfolio-api`, `web`, `ingest`, `signals`, and `db`.

- [ ] **Step 2: Turn `docker-compose.coinbase.yml` into a backlog/trading override**

Either rename is out of scope for first PR, or update comments clearly. Keep file if tests/docs reference it, but make it explicitly trading/backlog:

```yaml
# Optional backlog override for future Coinbase paper execution experiments.
# Not part of the MVP default runtime path.
```

Keep `trading-worker` only in this override if needed for future re-entry. If keeping it, define the full `trading-worker` service there because it no longer exists in base compose.

- [ ] **Step 3: Stop registering order/MCP trade routes by default**

In `backend/cmd/portfolio-api/main.go`, keep constructing trading dependencies only if necessary for tests, but do not register these routes unless an explicit backlog flag is enabled.

Recommended minimal approach:

```go
if app.tradingCfg.Enabled {
	r.Route("/internal/orders", func(r chi.Router) {
		r.Use(app.tradingGate)
		r.Post("/validate", app.handleOrderValidate)
		r.Post("/", app.handleOrderCreate)
		r.Get("/{id}", app.handleOrderGet)
		r.Post("/{id}/cancel", app.handleOrderCancel)
	})
	r.Route("/mcp/v1/trades", func(r chi.Router) {
		r.Use(app.tradingGate)
		r.Post("/validate", app.handleMCPOrderValidate)
		r.Post("/create", app.handleMCPOrderCreate)
		r.Get("/{id}", app.handleOrderGet)
		r.Post("/{id}/cancel", app.handleOrderCancel)
	})
}
```

Do not expose order routes when `TRADING_ENABLED=false`.

- [ ] **Step 4: Keep public trading UI route out of default session routes**

In `backend/cmd/portfolio-api/main.go`, remove this from the authenticated route group unless `TRADING_ENABLED=true`:

```go
r.Get("/api/trading/orders/open", app.handleOpenOrdersList)
```

Register it only inside the same `if app.tradingCfg.Enabled` block or a helper that clearly gates trading routes.

- [ ] **Step 5: Update tests for disabled trading routes**

In `backend/cmd/portfolio-api/trading_handlers_test.go`, assert order routes are not found when `TRADING_ENABLED=false` and remain available when enabled.

Expected disabled behavior:

```go
if rr.Code != http.StatusNotFound {
	t.Fatalf("expected disabled trading route to be hidden, got %d", rr.Code)
}
```

- [ ] **Step 6: Validate compose and backend tests**

Run:

```bash
docker compose config --quiet
cd backend && go test ./cmd/portfolio-api ./internal/config ./internal/trading ./internal/brokerwire
```

Expected: compose config succeeds, selected Go packages pass.

**Return from subagent:** status, service list after `docker compose config --services`, tests run, and any route compatibility notes.

---

## Task 4: Remove Trade Activity from MVP Dashboard

**Subagent lane:** frontend cleanup. Can run after Task 1; independent of backend route cleanup.

**Files:**
- Modify: `frontend/app/page.tsx`
- Optionally modify/delete: `frontend/components/trade-activity-card.tsx`

- [ ] **Step 1: Remove TradeActivityCard from main dashboard**

In `frontend/app/page.tsx`, remove:

```tsx
import { TradeActivityCard } from "@/components/trade-activity-card";
```

Remove this render:

```tsx
<TradeActivityCard />
```

- [ ] **Step 2: Preserve component as backlog code**

Prefer leaving `frontend/components/trade-activity-card.tsx` in place but unused for now. Do not delete unless lint/build fails due to unused code, which it should not.

- [ ] **Step 3: Verify frontend**

Use Node from `frontend/.nvmrc` before build. If `nvm` is available:

```bash
cd frontend && nvm use && npm test && npm run build
```

If `nvm` is not available, report that local build needs Node `22.22.0`.

Expected:

- `npm test` passes.
- `npm run build` passes under Node `22.22.0`.

**Return from subagent:** status, tests/build run, Node version used, and whether the trade activity component was preserved.

---

## Task 5: Update Docs and Environment Source of Truth

**Subagent lane:** docs/env. Can run after Task 1; coordinate with Task 3 if both edit compose docs.

**Files:**
- Modify: `README.md`
- Modify: `.env.example`
- Create: `docs/crypto-mvp-refactor.md`
- Modify: `docs/runbooks/coinbase-trading.md`
- Modify: `docs/mcp-crypto-execution-spec.md`

- [ ] **Step 1: Rewrite README around crypto MVP**

`README.md` should state:

- Default product: local homelab crypto portfolio/watchlist dashboard.
- Coinbase read API credentials are required for real portfolio snapshots.
- Signals post a one-line summary plus fenced `crypto_signal_v1` JSON to Discord.
- OpenClaw consumes Discord payload externally.
- Trading/order execution and IBKR are backlog, not default runtime.
- `docker compose up` starts `web`, `portfolio-api`, `ingest`, `signals`, `db`.

Remove or demote language that says IBKR is the primary broker.

- [ ] **Step 2: Clean `.env.example`**

Make Coinbase read path prominent:

```env
BROKER_PROVIDER=coinbase
BROKER_ENV=paper
INGEST_BROKER_PROVIDER=coinbase
INGEST_BROKER_ENV=paper
COINBASE_READ_API_KEY=
COINBASE_READ_API_SECRET=
```

Move IBKR variables under a clearly labeled backlog section:

```env
# Backlog / not part of MVP default runtime: IBKR
```

Move trading variables under a clearly labeled backlog section:

```env
# Backlog / not part of MVP default runtime: Coinbase order execution
TRADING_ENABLED=false
```

- [ ] **Step 3: Add current MVP/refactor doc**

Create `docs/crypto-mvp-refactor.md` with:

- MVP scope.
- Out of scope.
- Runtime services.
- Signal payload contract.
- OpenClaw consumption model.
- Verification checklist for the user:

```bash
cp .env.example .env
# fill Coinbase read creds and Discord webhook
docker compose up --build
```

- [ ] **Step 4: Mark backlog docs clearly**

At the top of `docs/runbooks/coinbase-trading.md`, add:

```markdown
> Backlog note: Coinbase order execution is not part of the MVP default runtime path. This runbook is preserved for future re-entry.
```

At the top of `docs/mcp-crypto-execution-spec.md`, add:

```markdown
> Backlog note: MCP trade execution is not part of the MVP. OpenClaw consumes `crypto_signal_v1` payloads from Discord for MVP.
```

- [ ] **Step 5: Verify docs mention no default IBKR path**

Search:

```bash
rg "IBKR|ib-gateway|trading-worker|MCP trade|order execution" README.md .env.example docs
```

Expected: any matches are clearly labeled as backlog or historical.

**Return from subagent:** status, docs changed, exact user verification path, and any remaining legacy references.

---

## Task 6: Final Integration and Verification

**Subagent lane:** integration owner. Run only after Tasks 1-5.

**Files:**
- Any conflicted files from previous tasks.
- No new product scope.

- [ ] **Step 1: Check worktree and conflicts**

Run:

```bash
git status --short
```

Confirm only planned files changed. Resolve conflicts by preserving the agreed MVP scope.

- [ ] **Step 2: Run full backend verification**

Run:

```bash
cd backend && gofmt -w . && go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Run frontend verification**

Use Node `22.22.0`:

```bash
cd frontend && nvm use && npm test && npm run build
```

If `nvm` is unavailable, use any local Node manager that provides `22.22.0`. Do not claim frontend build passes under Node 18.

- [ ] **Step 4: Validate compose**

Run:

```bash
docker compose config --quiet
docker compose config --services
```

Expected service list:

```text
web
portfolio-api
ingest
signals
db
```

No `ib-gateway` or `trading-worker` in the default service list.

- [ ] **Step 5: Optional smoke boot**

If `.env` has Coinbase read creds and Discord webhook:

```bash
docker compose up --build
```

Smoke expectations:

- `portfolio-api` starts and applies migrations.
- `ingest` posts snapshots using Coinbase read path.
- dashboard is available at `http://localhost:3000`.
- `signals` can post a Discord message with a human summary and fenced `crypto_signal_v1` JSON.

If creds are not present, do not block the PR; report that user verification requires filling `.env`.

- [ ] **Step 6: Final review checklist**

Verify:

- README default path matches MVP.
- `.env.example` defaults to Coinbase read path.
- `crypto_signal_v1` schema is documented and tested.
- exact payload JSON is persisted.
- order/MCP trade routes are not available by default.
- dashboard has no trade activity card in default view.
- old epics remain historical and are not rewritten.

**Return from subagent:** final verification results, commands run, failures/blockers, and exact next steps for user `.env` verification.

---

## Ready-to-Dispatch Subagent Prompts

### Task 1 Prompt

Implement Task 1 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: define `crypto_signal_v1` payload fields and persist exact recent-alert payload JSON. Touch only the Task 1 files. Follow TDD where practical. Run the focused Go tests listed in Task 1. Return status, files changed, tests run, and migration compatibility notes.

### Task 2 Prompt

Implement Task 2 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: Discord signal content should be a one-line human summary plus fenced compact JSON, and the exact JSON payload should be persisted through the existing recent-alert path. Assume Task 1 is complete. Touch only Task 2 files unless a small integration change is required; explain any extra file. Run focused Go tests. Return a sample Discord message and test results.

### Task 3 Prompt

Implement Task 3 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: remove IBKR and trading from the default runtime path while preserving backlog code/tests. Update compose and route registration. Run compose config and focused backend tests. Return the default `docker compose config --services` list and route behavior notes.

### Task 4 Prompt

Implement Task 4 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: remove trade activity from the MVP dashboard while preserving the component as backlog code unless build requires otherwise. Run frontend tests and build with Node 22.22.0. Return Node version, tests/build results, and files changed.

### Task 5 Prompt

Implement Task 5 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: update README, `.env.example`, and docs so the crypto alert MVP is the current source of truth and IBKR/trading/MCP trade execution are backlog. Run the docs search command and report remaining legacy references.

### Task 6 Prompt

Implement Task 6 from `docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md`: integrate all prior task outputs, resolve conflicts, run full backend/frontend/compose verification, and produce the final user handoff. Do not add new product scope.

---

## Autonomous Completion Notes

- The user expects to come back, fill `.env`, and verify. Do not require further product decisions unless a task uncovers a true blocker.
- Preserve backlog code when possible. The first refactor should remove default runtime exposure, not delete future work.
- Do not rewrite historical `.agent/epics` files.
- Do not enable order execution by default.
- Do not introduce an OpenClaw MCP server for MVP.
- If local frontend build fails under Node 18, switch to Node `22.22.0`; do not treat that as an app failure.
