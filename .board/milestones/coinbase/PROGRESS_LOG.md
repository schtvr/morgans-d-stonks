# Coinbase Milestone Progress Log

## 2026-05-02T00:00:00Z — Iteration 1 (CB-01 start)
- Attempted:
  - Parsed CB-01..CB-15 tickets and built dependency DAG.
  - Generated initial topological execution order and wave plan.
  - Started CB-01 implementation (capability model + interface split).
- Ticket Brief (CB-01):
  - Scope summary: add broker capability flags and split read vs execution contracts, preserving read consumers.
  - Acceptance criteria:
    - [ ] Capability flags added (read/quote/place/cancel/stream).
    - [ ] Existing read consumers continue to work unchanged.
    - [ ] Provider factory returns interface support safely.
    - [ ] Unit tests for capability and provider selection.
  - Expected touched components: `internal/broker`, `internal/brokerwire`, tests.
  - Risk notes: broad interface changes could break ingest wiring if compatibility not preserved.
- Changes made so far:
  - Planning artifacts created: EXECUTION_PLAN/DECISIONS/PROGRESS_LOG/BLOCKERS/HANDOFF.
- Tests run: none yet.
- Next actions:
  - Complete broker interface/capability changes.
  - Add provider-selection safety APIs + tests.


## 2026-05-02T05:05:18Z — Iteration 2 (CB-01 complete)
- Attempted:
  - Implemented broker capability model and read/execution interface split.
  - Added provider-factory execution safety function.
  - Added unit tests for capability checks and provider selection behavior.
- Changed:
  - `internal/broker/broker.go`, `internal/broker/execution_types.go`
  - `internal/broker/mock/mock.go`
  - `internal/broker/broker_capabilities_test.go`
  - `internal/brokerwire/wire.go`, `internal/brokerwire/wire_test.go`
- Acceptance criteria evidence (CB-01):
  - [x] Capability flags added via `Capability` constants + `CapabilitiesBroker`.
  - [x] Existing read consumers remain compatible via `type Broker = ReadBroker`.
  - [x] Provider factory returns interface support safely via `NewExecution` type assertion guard.
  - [x] Unit tests added for capabilities and provider selection errors.
- Tests:
  - `go test ./internal/broker ./internal/brokerwire` ✅ pass
- Commit:
  - Pending commit step in current iteration.
- Next actions:
  - Commit CB-01 with ticket-formatted message.
  - Start CB-02 + CB-04 planning briefs.


## 2026-05-02T05:47:48Z — Iteration 3 (CB-02 + CB-04 complete)
- Attempted:
  - Implemented canonical broker domain types for instrument/money/quantity/fill with normalization + decimal validation.
  - Implemented broker config redesign in `internal/config` with `BROKER_PROVIDER` / `BROKER_ENV` and Coinbase read/trade env split.
  - Wired ingest startup to load/validate broker config via config package and map into brokerwire legacy config.
- Acceptance criteria evidence:
  - CB-02: decimal-safe string-backed amount/quantity constructors using rational parsing; normalization behavior + tests.
  - CB-04: config package owns new vars, provider-aware startup validation, `.env.example` and README updated.
- Tests:
  - `go test ./internal/broker ./internal/config ./internal/brokerwire ./cmd/ingest` ✅ pass
- Next actions:
  - Begin CB-03 and CB-06 (wave 3) in parallel tracks.

## 2026-05-04T00:00:00Z — Iteration 4 (CB-03 complete)
- Attempted:
  - Implemented Coinbase read-only broker adapter with positions/summary/quotes methods.
  - Added retry/backoff behavior for 429/5xx responses with context-aware cancellation.
  - Added product metadata cache keyed by product id and quote normalization helper.
  - Wired broker factory to return Coinbase adapter for `BROKER_PROVIDER=coinbase`.
- Acceptance criteria evidence (CB-03):
  - [x] Broker read interface implemented.
  - [x] Retries/backoff on 429/5xx with context timeouts.
  - [x] Product metadata cache (tick/min size/status).
  - [x] Integration-style test with mocked Coinbase API server.
- Tests:
  - `go test ./internal/broker/... ./internal/brokerwire` ✅ pass
- Next actions:
  - Start CB-06 persistence schema implementation.


## 2026-05-06T00:00:00Z — Iteration 5 (CB-05 through CB-15 complete)
- Attempted:
  - Implemented Coinbase symbol canonicalization and read-path mapping.
  - Added trading persistence schema, repository, append-only event triggers, and rollback guidance.
  - Added internal order API scaffolding with idempotency replay and request-hash validation.
  - Added deterministic policy engine, order state machine, reconciliation worker, and Coinbase paper execution adapter.
  - Added trading metrics, rollout controls, alert rules, README/runbook documentation, and Compose wiring for the worker.
  - Added failure-injection and replay/idempotency tests for the trading flow.
- Acceptance criteria evidence:
  - CB-05 through CB-15 marked done in the execution plan.
  - `GET /metrics` now exposes trading counters and lag summaries.
  - `go test ./...` ✅ pass
- Tests:
  - `go test ./internal/trading/... ./internal/broker/coinbase ./internal/brokerwire ./internal/config ./cmd/portfolio-api ./cmd/trading-worker` ✅ pass
  - `go test ./...` ✅ pass
- Next actions:
  - None for this milestone; proceed to review / merge.
