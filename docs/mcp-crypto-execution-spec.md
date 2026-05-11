# MCP Crypto Alerts & Coinbase Execution Spec (MVP)

## Goal

Enable deterministic crypto alerting to Discord for watchlisted products and allow OpenClaw to initiate Coinbase trades through MCP HTTP calls into `portfolio-api`.

## Architecture

1. **Market collector (`signals`)**
   - Consume Coinbase market data via WebSocket in steady state.
   - Maintain short rolling windows per product for return and volatility features.
2. **Deterministic filter (`signals`)**
   - Run health and significance gates.
   - Emit candidate event packets only when conditions are unusual.
3. **OpenClaw loop (Discord)**
   - OpenClaw consumes candidate event packets from Discord.
   - OpenClaw decides trade action and posts explicit trade payloads back through MCP HTTP.
4. **Execution gateway (`portfolio-api`)**
   - Enforce trading policy and risk controls.
   - Execute through Coinbase adapter (paper mode first).
   - Emit trade result notifications to Discord.

## Decisions Locked

- Trading starts in **paper mode**.
- Strategy direction is externalized to OpenClaw (this service is execution/risk only).
- MCP auth uses `X-Internal-Key`.
- MCP endpoints are separate (validate/create/get/cancel).
- `schema_version` is required (`v1`).
- `idempotency_key` is required for create.
- Order type support for MVP is market-only.
- Position sizing supports `quote_size` and `base_size`.
- Initial allowlisted products: `BTC-USD`, `ETH-USD`, `SOL-USD`.
- No shorting: sell without an available position is rejected.
- Mandatory controls: allowlist, max notional/order, per-symbol cooldown, kill switch, global max exposure.
- Failures are logged and posted to Discord.
- Store lifecycle trade records and logs, but not full raw MCP request/response blobs.
- One Discord channel for alerts + trade results.

## MCP API Contract

Base path: `/mcp/v1/trades`

### POST `/mcp/v1/trades/validate`

Validate policy/risk only; no execution side effects.

```json
{
  "schema_version": "v1",
  "idempotency_key": "optional-for-validate",
  "order": {
    "product_id": "BTC-USD",
    "side": "buy",
    "type": "market",
    "quote_size": "250.00",
    "base_size": null
  },
  "risk_context": {
    "requested_by": "openclaw",
    "reason_flags": ["5m_breakout", "spread_ok"]
  }
}
```

### POST `/mcp/v1/trades/create`

Creates an order in configured trading environment.

- `idempotency_key` required.
- Exactly one of `quote_size` or `base_size` required.

```json
{
  "schema_version": "v1",
  "idempotency_key": "abcd-1234",
  "order": {
    "product_id": "ETH-USD",
    "side": "sell",
    "type": "market",
    "quote_size": null,
    "base_size": "0.15"
  },
  "risk_context": {
    "requested_by": "openclaw",
    "reason_flags": ["vol_expansion", "5m_breakout"]
  }
}
```

### GET `/mcp/v1/trades/{id}`

Returns normalized order state and policy metadata.

### POST `/mcp/v1/trades/{id}/cancel`

Attempts cancellation where order state permits it.

## Candidate Event Packet (signals -> Discord/OpenClaw)

```json
{
  "schema_version": "v1",
  "event_type": "candidate_crypto_move",
  "product_id": "SOL-USD",
  "ts": "2026-05-08T12:34:56Z",
  "last_price": 172.31,
  "return_1m_pct": 0.82,
  "return_5m_pct": 1.93,
  "return_15m_pct": 2.4,
  "rolling_vol_5m_pct": 0.68,
  "spread_bps": 9.2,
  "quote_volume_24h": 128300000,
  "cooldown_active": false,
  "prior_alert_ago_sec": 860,
  "reason_flags": ["5m_breakout", "vol_expansion", "spread_ok"]
}
```

## Deterministic Gating

### Health gate

Drop product if any is true:

- Product is restricted/disabled.
- Spread exceeds configured cap.
- Liquidity is below configured floor.
- Product is in cooldown.

### Significance gate

Alert candidate when all hold:

- `abs(return_5m) >= max(floorPct, 2.5 * rollingVol_5m)`
- Condition persists for 2 of last 3 snapshots.
- Spread remains under cap.
- Cooldown is not active.

### Initial floors

- BTC/ETH: `1.0%` over 5m
- Liquid alts: `1.5%` over 5m
- Smaller alts: `2.5%+` over 5m

## Risk Policy Checks (validate/create)

1. Internal key auth passes.
2. `schema_version == v1`.
3. Symbol in allowlist.
4. `type == market` for MVP.
5. Exactly one size field present.
6. Max notional per order within policy.
7. Per-symbol cooldown not violated.
8. Kill switch is false.
9. Sell orders require available position (no shorting).
10. `current_exposure + proposed_notional <= global_max_exposure`.
11. Idempotency key behavior is deterministic and conflict-safe.

## Config Surface

### Trading

- `TRADING_ENABLED`
- `TRADING_KILL_SWITCH`
- `TRADING_ALLOWED_SYMBOLS`
- `TRADING_MAX_NOTIONAL`
- `TRADING_GLOBAL_MAX_EXPOSURE`
- `TRADING_SYMBOL_COOLDOWN`

### Signals

- `SIGNALS_WS_ENABLED`
- `SIGNALS_SPREAD_BPS_MAX`
- `SIGNALS_LIQUIDITY_MIN_24H_QUOTE_VOL`
- `SIGNALS_FLOOR_PCT_BTC_ETH`
- `SIGNALS_FLOOR_PCT_LIQUID_ALTS`
- `SIGNALS_FLOOR_PCT_SMALL_ALTS`

### Shared

- `INTERNAL_API_KEY`
- `MCP_SCHEMA_VERSION=v1`
- `DISCORD_WEBHOOK_URL`

## Implementation Order

1. Extend config parsing + validation.
2. Add signal gate core and tests.
3. Integrate WebSocket market feed in signals.
4. Add MCP route group and handlers.
5. Enforce risk policy updates and tests.
6. Add Discord trade success/failure notifications.
7. Surface trade activity in dashboard.
8. Run compose smoke tests.

## Implementation Status

- [x] MCP route group and handlers (`/mcp/v1/trades/validate|create|{id}|{id}/cancel`) behind internal key + trading gate.
- [x] MCP request envelope validation + mapping (`schema_version`, market-only, xor size fields, symbol normalization).
- [x] MCP schema version config (`MCP_SCHEMA_VERSION`) wired into portfolio-api config.
- [x] Unit tests for MCP mapper and decode path.
- [x] Signal deterministic gate core + unit tests (`internal/signal/gate.go`).
- [x] Signal pipeline integration of deterministic gate (candidate alerts pass through gate + reason flags).
- [ ] Coinbase WebSocket collector integration for signal pipeline (config surface added: `SIGNALS_WS_ENABLED`, `COINBASE_WS_URL`; runtime collector still pending).
- [x] Trading risk policy extensions: no-shorting + global max exposure + cooldown path enforcement (open-order based guardrails).
- [x] Discord trade success/failure notifications (MCP create path).
- [x] Dashboard trade activity surface for MCP-generated orders (`/api/trading/orders/open` + `TradeActivityCard`).
- [x] Compose smoke test script for candidate-alert -> MCP trade flow (`scripts/smoke_mcp_flow.sh`).
