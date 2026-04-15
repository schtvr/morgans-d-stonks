# SCH-16: Deterministic Signals & Discord (Minimal)

> **Linear**: [SCH-16](https://linear.app/schtvr/issue/SCH-16/epic-p0-deterministic-signals-and-discord-minimal)
> **Milestone**: P0: MVP
> **Wave**: 4 (depends on SCH-21 for snapshots + SCH-18 for snapshot reads)
> **Depends on**: SCH-19, SCH-18, SCH-21

## Objective

After each ingest cycle, evaluate versioned deterministic price/indicator rules against the latest snapshot and emit minimal Discord notifications (`ticker | signal`).

## Scope

### Signal engine (`cmd/signals/`)

- Can run as:
  - A standalone binary triggered by cron/scheduler after ingest, OR
  - Invoked directly by the ingest job (design for the interface, let implementation decide).
- On each run:
  1. Fetch latest snapshot from portfolio service (`GET /api/portfolio/positions` or an internal endpoint).
  2. Load signal rules from config.
  3. Evaluate each rule against current snapshot data.
  4. Emit `SignalEvent` for each triggered rule.
  5. Deduplicate: don't re-fire the same signal if conditions haven't changed.
  6. Send to Discord.

### Signal rules (`internal/signal/`)

Versioned rule configuration — use a YAML/JSON file in the repo for P0:

```yaml
version: 1
rules:
  - id: "price-drop-5pct"
    name: "5% Price Drop"
    description: "Fires when a position drops 5% or more from avg cost"
    condition:
      type: "price_change_pct"
      field: "unrealizedPLPct"
      operator: "lte"
      threshold: -5.0

  - id: "large-position"
    name: "Large Position Alert"
    description: "Fires when a single position exceeds 20% of portfolio"
    condition:
      type: "concentration"
      field: "marketValuePct"
      operator: "gte"
      threshold: 20.0
```

- Parse rules at startup; validate schema.
- Rule evaluation is pure: `func Evaluate(rule Rule, snapshot Snapshot) ([]SignalEvent, error)`.
- Unit-testable with fixture snapshots.

### SignalEvent type (`internal/signal/types.go`)

```go
type SignalEvent struct {
    ID        string    `json:"id"`        // unique event ID (UUID)
    RuleID    string    `json:"ruleId"`
    RuleName  string    `json:"ruleName"`
    Symbol    string    `json:"symbol"`
    Signal    string    `json:"signal"`    // human-readable signal text
    Value     float64   `json:"value"`     // the value that triggered
    Threshold float64   `json:"threshold"` // the threshold from the rule
    FiredAt   time.Time `json:"firedAt"`
}
```

This shape is intentionally stable — P1 will forward it to OpenClaw.

### Deduplication

- Track fired signals with a state file or DB table (key: `ruleId + symbol`).
- Strategy: cooldown-based — once a signal fires for a `ruleId + symbol`, suppress for a configurable duration (default 1 hour).
- On service restart, reload state from persistent store.

### Discord client (`internal/discord/`)

- Use Discord webhook (simplest for P0).
- Message format: `**AAPL** | 5% Price Drop` (minimal, per P0 spec).
- Respect Discord rate limits (5 requests per 2 seconds per webhook).
- Env: `DISCORD_WEBHOOK_URL`.

### Configuration

```env
SIGNAL_RULES_PATH=./config/signals.yaml
SIGNAL_COOLDOWN=1h
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
PORTFOLIO_API_URL=http://portfolio-api:8080
```

## Do NOT

- Implement OpenClaw, MCP, or rich message formatting (P1, SCH-23).
- Add ML/LLM-based signals.
- Build a signal management UI.
- Implement order placement based on signals (P2).

## Acceptance criteria

- [ ] Example rule fires against a test snapshot and produces a `SignalEvent`.
- [ ] Unit tests for rule evaluation with fixture data.
- [ ] Discord webhook sends message in `ticker | signal` format.
- [ ] Duplicate signals are suppressed within the cooldown window.
- [ ] Rules are loaded from a versioned config file.
- [ ] `SignalEvent` JSON shape is documented and stable.

## Shared contracts

This epic **consumes**:

- **SCH-18** portfolio API — reads latest snapshot/positions.
- **SCH-21** — depends on snapshots being written regularly.

This epic **produces**:

- `SignalEvent` — will be consumed by SCH-23 (OpenClaw bridge) in P1.
- Discord messages — end-user notifications.

## Files to create/modify

| File | Action |
|------|--------|
| `cmd/signals/main.go` | Implement |
| `internal/signal/types.go` | Implement (SignalEvent, Rule types) |
| `internal/signal/engine.go` | Implement (rule evaluation) |
| `internal/signal/engine_test.go` | Implement (with fixtures) |
| `internal/signal/config.go` | Implement (YAML rule loader) |
| `internal/signal/dedup.go` | Implement (cooldown tracker) |
| `internal/discord/client.go` | Implement (webhook sender) |
| `internal/discord/client_test.go` | Implement |
| `config/signals.yaml` | Create (example rules) |
