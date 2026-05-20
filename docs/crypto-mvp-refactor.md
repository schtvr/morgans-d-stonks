# Crypto alert MVP — verification

Short companion to the root [README.md](../README.md). Implementation task checklist: [docs/superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md](superpowers/plans/2026-05-13-crypto-alert-mvp-refactor.md).

## MVP scope

- Dashboard: portfolio summary, positions (from latest snapshot), crypto **watchlist**, **alert settings**, **recent alerts**.
- **ingest**: Coinbase read → `POST /internal/snapshots`.
- **signals**: Coinbase quotes → threshold → Discord (summary + fenced JSON) + `POST /internal/recent-alerts` with **`crypto_signal_v1`** payload stored in **`payload_json`**.

## `crypto_signal_v1` (summary)

Stable fields on the alert JSON include: `schemaVersion` (`crypto_signal_v1`), `id`, `type` (`crypto_price_move`), `symbol`, `currentPrice`, `deltaPct`, `thresholdPct`, `firedAt`, optional position context, `reasonFlags`.

## Verification

1. `cp .env.example .env` — set DB, internal key, **Coinbase read** keys, optional **Discord** webhook.
2. `docker compose config --services` — expect: `web`, `portfolio-api`, `ingest`, `signals`, `db` (no `trading-worker` in the default stack).
3. `docker compose up --build`.
4. Open http://localhost:3000 — login, confirm watchlist and alerts UI.
5. With Discord configured, confirm messages show a one-line summary and a ```json fenced block.
6. **OpenClaw** consumes the Discord message externally (no MCP server required for MVP).

## Backlog

- **trading-worker**, **`/internal/orders`**, **`/mcp/v1/trades`**: enable via `TRADING_ENABLED=true` and optional `docker-compose.coinbase.yml` merge.

## The Agent (shadow mode)

Added in May 2026. See `docs/superpowers/plans/2026-05-18-agent-shadow-decisions.md`.
Enable with `AGENT_ENABLED=true AGENT_PROVIDER=anthropic ANTHROPIC_API_KEY=...` in `.env`.
Decisions visible at `http://localhost:3000/agent`.
