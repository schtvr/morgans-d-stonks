# SCH-20: IBKR Connectivity & Domain Model

> **Linear**: [SCH-20](https://linear.app/schtvr/issue/SCH-20/epic-p0-ibkr-connectivity-and-domain-model)
> **Milestone**: P0: MVP
> **Wave**: 2 (depends on SCH-19 for repo layout)
> **Depends on**: SCH-19

## Objective

Establish reliable connectivity to Interactive Brokers in paper mode with a clean internal domain model. Upstream consumers (ingest, signals, dashboard) depend on the `Broker` interface — not on raw IBKR types.

## Scope

### Broker interface (`internal/broker/`)

Define the primary Go interface that all consumers depend on:

```go
package broker

type Broker interface {
    // Positions returns current account positions.
    Positions(ctx context.Context) ([]Position, error)

    // AccountSummary returns account-level metrics.
    AccountSummary(ctx context.Context) (*AccountSummary, error)

    // Quotes returns latest quotes for the given symbols.
    Quotes(ctx context.Context, symbols []string) ([]Quote, error)

    // IsMarketOpen checks if the equity market session is active.
    IsMarketOpen(ctx context.Context) (bool, error)

    // Close cleans up connections.
    Close() error
}
```

### Domain types (`internal/broker/types.go`)

```go
type Position struct {
    Symbol      string
    ConID       int64    // IBKR contract ID, preserved for debugging
    Quantity    float64
    AvgCost     float64
    MarketValue float64
    UnrealizedPL float64
    RealizedPL  float64
    Currency    string
    UpdatedAt   time.Time
}

type AccountSummary struct {
    AccountID     string
    NetLiquidation float64
    TotalCash      float64
    BuyingPower    float64
    Currency       string
    UpdatedAt      time.Time
}

type Quote struct {
    Symbol    string
    ConID     int64
    Last      float64
    Bid       float64
    Ask       float64
    Volume    int64
    UpdatedAt time.Time
}
```

These types are the **canonical internal model**. All downstream code uses these, never raw IBKR wire types.

### Mock implementation (`internal/broker/mock/`)

Create a `MockBroker` that implements the `Broker` interface with static or randomized test data. This is used when `IBKR_MODE=mock` and in CI.

### IBKR implementation (`internal/broker/ibkr/`)

- Implement against **IB Gateway** using the **TWS API** (socket protocol on port 4001/4002) or **Client Portal Web API** (REST on port 5000) — document the choice and trade-offs in a `DECISION.md` in this directory.
- Handle connection lifecycle: connect, heartbeat, reconnect on disconnect.
- Map IBKR-specific types to the canonical domain types above.
- Rate limiting / pacing: respect IBKR's documented pacing rules.
- Error taxonomy: distinguish auth failures, connection drops, data errors.

### Configuration

Read from environment:

```env
IBKR_GATEWAY_HOST=ib-gateway   # or host.docker.internal
IBKR_GATEWAY_PORT=4001
IBKR_MODE=paper                # paper | live | mock
```

Factory function:

```go
func NewBroker(cfg Config) (Broker, error)
```

Returns `MockBroker` when `IBKR_MODE=mock`, real client otherwise.

## Do NOT

- Implement order placement (P2 scope, SCH-24).
- Handle options, futures, or non-US equities.
- Create persistence logic (owned by SCH-18).
- Build the ingest scheduler (owned by SCH-21).

## Acceptance criteria

- [ ] `Broker` interface defined and exported from `internal/broker`.
- [ ] `MockBroker` returns realistic test data; passes unit tests.
- [ ] IBKR client connects to Gateway in paper mode and fetches positions.
- [ ] All IBKR wire types are mapped to canonical domain types; no leakage.
- [ ] `IBKR_MODE=mock` works without a running Gateway.
- [ ] Decision doc explains TWS API vs Client Portal choice.
- [ ] Error handling: reconnect on transient failures; log auth failures clearly.

## Shared contracts

The `Broker` interface and domain types are consumed by:

- **SCH-21** (Ingest) — calls `Positions()`, `Quotes()`, `IsMarketOpen()`
- **SCH-16** (Signals) — consumes snapshots built from `Positions()` + `Quotes()`
- **SCH-18** (Portfolio Service) — maps domain types to DB schema

Changes to the interface require coordination with those epics.

## Files to create/modify

| File | Action |
|------|--------|
| `internal/broker/broker.go` | Implement (interface + factory) |
| `internal/broker/types.go` | Implement (domain types) |
| `internal/broker/config.go` | Implement (config struct) |
| `internal/broker/mock/mock.go` | Implement |
| `internal/broker/mock/mock_test.go` | Implement |
| `internal/broker/ibkr/client.go` | Implement |
| `internal/broker/ibkr/mapper.go` | Implement (wire → domain) |
| `internal/broker/ibkr/DECISION.md` | Create |
