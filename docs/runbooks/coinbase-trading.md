# Coinbase Trading Runbook

## Preflight

- Confirm `TRADING_ENABLED=true` only after `TRADING_ALLOWED_PROVIDERS`, `TRADING_ALLOWED_SYMBOLS`, and `TRADING_MAX_NOTIONAL` are set.
- Verify the broker selection matches the intended environment:
  - `BROKER_PROVIDER=coinbase`
  - `BROKER_ENV=paper` for safe simulation
- Confirm the API and worker can both reach Postgres and that the trading migrations have been applied.
- Check `GET /metrics` for non-zero `trading_order_creates_total` and `trading_reconciliation_lag_seconds_*` values before enabling live traffic.

## Incident Playbooks

- **Reject storm**
  - Turn `TRADING_KILL_SWITCH=true`.
  - Inspect `trading_order_rejects_total` and the order logs for `reason_codes`.
  - Narrow the symbol allowlist or reduce `TRADING_MAX_NOTIONAL`.
- **Stale state**
  - Review `reconciliation` rows for the affected order IDs.
  - Restart the trading worker to force another reconciliation pass.
  - If needed, cancel the affected orders through `/internal/orders/{id}/cancel`.
- **API outage**
  - Stop new order intake by leaving `TRADING_ENABLED=false`.
  - Keep the worker running only if reconciliation is needed; otherwise stop it to avoid noisy retries.

## Rollback

- Disable `TRADING_ENABLED`.
- Set `TRADING_KILL_SWITCH=true` if the API must remain up but all placement should halt.
- Stop `trading-worker`.
- Drop or archive the trading tables only after confirming there are no pending investigations tied to them.

## Ownership

- Primary: platform maintainer on call.
- Secondary: whoever owns the current trading rollout.
- Escalate to the broker integration owner if paper execution behavior diverges from the expected simulation mode.

