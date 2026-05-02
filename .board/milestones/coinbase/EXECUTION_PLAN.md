# Coinbase Milestone Execution Plan

_Last updated: 2026-05-02T00:00:00Z_

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
| CB-03 | in_progress | planner | 2026-05-02T05:47:48Z | Ready after CB-02 |
| CB-04 | done | implementer/tester/reviewer | 2026-05-02T05:47:48Z | Broker config redesign complete |
| CB-05 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-02, CB-03 |
| CB-06 | in_progress | planner | 2026-05-02T05:47:48Z | Ready after CB-02 |
| CB-07 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-04, CB-06 |
| CB-08 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-02, CB-07 |
| CB-09 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-06, CB-07 |
| CB-10 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-06, CB-09 |
| CB-11 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-08, CB-09 |
| CB-12 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-07, CB-09, CB-10 |
| CB-13 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-08, CB-11, CB-12 |
| CB-14 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-11, CB-12 |
| CB-15 | todo | planner | 2026-05-02T00:00:00Z | Waits on CB-12, CB-13, CB-14 |
