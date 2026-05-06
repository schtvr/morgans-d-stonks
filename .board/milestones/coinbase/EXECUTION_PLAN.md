# Coinbase Milestone Execution Plan

_Last updated: 2026-05-06T00:00:00Z_

## Topological order (dependency aware)
1. CB-01
2. CB-02
3. CB-04
4. CB-03
5. CB-06
6. CB-05
7. CB-07
8. CB-08
9. CB-09
10. CB-10
11. CB-11
12. CB-12
13. CB-13
14. CB-14
15. CB-15

## Critical path
CB-01 → CB-02 → CB-06 → CB-07 → CB-09 → CB-10 → CB-12 → CB-13 → CB-15

## Parallelizable sets (after dependencies are met)
- Wave 1: CB-01
- Wave 2: CB-02, CB-04
- Wave 3: CB-03, CB-06
- Wave 4: CB-05, CB-07
- Wave 5: CB-08, CB-09
- Wave 6: CB-10, CB-11
- Wave 7: CB-12, CB-13, CB-14
- Wave 8: CB-15

## Ticket status board
| Ticket | Status | Owner role | Last update (UTC) | Notes |
|---|---|---|---|---|
| CB-01 | done | implementer/tester/reviewer | 2026-05-02T05:05:18Z | Completed and committed |
| CB-02 | done | implementer/tester/reviewer | 2026-05-02T05:47:48Z | Domain models + tests complete |
| CB-03 | done | implementer/tester/reviewer | 2026-05-04T00:00:00Z | Coinbase read adapter + metadata cache complete |
| CB-04 | done | implementer/tester/reviewer | 2026-05-02T05:47:48Z | Broker config redesign complete |
| CB-05 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Canonical symbol mapping and read-path wiring complete |
| CB-06 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Trading persistence schema, repo, and rollback doc complete |
| CB-07 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Internal order API, idempotency, and replay-safe responses complete |
| CB-08 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Deterministic policy engine and allow/deny checks complete |
| CB-09 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Order lifecycle state machine and guards complete |
| CB-10 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Trading worker reconciliation loop complete |
| CB-11 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Coinbase paper execution adapter complete |
| CB-12 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Trading metrics, logging, and alert rule bundle complete |
| CB-13 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Rollout controls, allowlists, and startup validation complete |
| CB-14 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Failure-injection coverage for reject / partial / replay flows complete |
| CB-15 | done | implementer/tester/reviewer | 2026-05-06T00:00:00Z | Go-live runbook and rollback checklist complete |
