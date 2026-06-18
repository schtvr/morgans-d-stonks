# Coinbase Trading Runbook

> **Backlog:** Coinbase order execution and `trading-worker` are not part of the default MVP compose stack. Enable only with `TRADING_ENABLED=true` and optional `docker-compose.coinbase.yml`. See [live-execution-broker plan](../plans/live-execution-broker.md) for live mode.

## Preflight

- Confirm `TRADING_ENABLED=true` only after `TRADING_ALLOWED_PROVIDERS`, `TRADING_ALLOWED_SYMBOLS`, and `TRADING_MAX_NOTIONAL` are set.
- Verify the broker selection matches the intended environment:
  - `TRADING_BROKER_ENV=paper` for simulated fills (`coinbase.NewPaperExecution`)
  - `TRADING_BROKER_ENV=live` for real Coinbase Advanced Trade orders
- **Live mode additionally requires:**
  - `COINBASE_TRADE_API_KEY` / `COINBASE_TRADE_API_SECRET` (trade permissions, separate from read keys)
  - `TRADING_LIVE_ACK=true` (break-glass; blocks MCP/internal order routes without it)
  - `TRADING_MAX_NOTIONAL` ≤ `TRADING_LIVE_MAX_NOTIONAL_CAP` (default 500 USD)
- Confirm the API and worker can both reach Postgres and that the trading migrations have been applied.
- Check `GET /metrics` for non-zero `trading_order_creates_total` and `trading_reconciliation_lag_seconds_*` values before enabling live traffic.
- Watch `trading_live_reconciliation_errors_total` after enabling live execution.

## Incident Playbooks

- **Reject storm**
  - Turn `TRADING_KILL_SWITCH=true`.
  - Inspect `trading_order_rejects_total` and the order logs for `reason_codes`.
  - Narrow the symbol allowlist or reduce `TRADING_MAX_NOTIONAL`.
- **Stale state**
  - Review `reconciliation` rows for the affected order IDs.
  - Restart the trading worker to force another reconciliation pass.
  - If needed, cancel the affected orders through `/internal/orders/{id}/cancel` (worker calls Coinbase batch cancel for live orders).
- **Live poll failures**
  - Check `trading_live_reconciliation_errors_total` and worker logs for `poll order` errors.
  - Verify trade API key permissions and Coinbase account status.
- **API outage**
  - Stop new order intake: `TRADING_KILL_SWITCH=true` or `TRADING_ENABLED=false`.
  - Keep the worker running only if reconciliation is needed; otherwise stop it to avoid noisy retries.

## Rollback

1. Set `TRADING_KILL_SWITCH=true` — blocks new accepted orders at policy layer.
2. Cancel open internal orders via `/internal/orders/{id}/cancel` or dashboard; live cancels propagate to Coinbase when `provider_order_id` is set.
3. Set `TRADING_ENABLED=false` and stop `trading-worker`.
4. For live mode, unset `TRADING_LIVE_ACK` before re-enabling to prevent accidental restarts.
5. Drop or archive the trading tables only after confirming there are no pending investigations tied to them.

## Ownership

- Primary: platform maintainer on call.
- Secondary: whoever owns the current trading rollout.
- Escalate to the broker integration owner if live execution behavior diverges from Coinbase UI state.
