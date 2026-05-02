# Coinbase Milestone Decisions

| Decision ID | Date (UTC) | Ticket(s) | Context | Options considered | Selected | Rationale | Rollback/change path |
|---|---|---|---|---|---|---|---|
| DEC-001 | 2026-05-02 | ALL | Need deterministic local execution without GitHub API | (A) ad-hoc notes, (B) durable board docs | B | Auditable progress and resumability | Replace files with updated format if team standard changes |
| DEC-002 | 2026-05-02 | CB-01 | Interface split could break existing read consumers | (A) hard replace Broker, (B) preserve Broker as read-compatible alias and add execution interfaces | B | Meets ticket AC while minimizing churn | In later tickets, migrate consumers to narrower interfaces progressively |
