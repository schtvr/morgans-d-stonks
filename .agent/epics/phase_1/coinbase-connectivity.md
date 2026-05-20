# SCH-20: Coinbase connectivity & domain model

> **Linear**: [SCH-20](https://linear.app/schtvr/issue/SCH-20) (repurposed from equities gateway work)

## Goal

Reliable **read-only** connectivity to **Coinbase** (CDP / Advanced Trade REST) with a clean internal domain model. Ingest, signals, and dashboard consume the shared `internal/broker.Broker` interface — not raw exchange wire types outside the Coinbase adapter.

## Canonical types

See `internal/broker/types.go` (`Position`, `AccountSummary`, `Quote`). Adapters map Coinbase responses into these structs.

## Environment

See `.env.example`: `BROKER_PROVIDER`, `COINBASE_READ_API_KEY`, `COINBASE_READ_API_SECRET`, optional `BROKER_ENV`.

## Scope

- `internal/broker/coinbase/` — REST client, JWT auth, mapping to domain types.
- `BROKER_PROVIDER=mock` — in-process mock broker for tests / local smoke (no network).

## Out of scope (other epics)

- Ingest scheduling & snapshots: **SCH-21**
- Signals & Discord: **SCH-16**

## Acceptance criteria (high level)

- [ ] Ingest can persist snapshots sourced from Coinbase read API when credentials are set.
- [ ] `BROKER_PROVIDER=mock` works without live keys (tests / CI).
