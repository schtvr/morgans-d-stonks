# P1 logging — agent stories

Stories derived from [epic_P1_logging.md](../epic_P1_logging.md). Execute **in order** (01 → 05) to minimize merge conflicts.

| # | File | PR branch (suggested) |
|---|------|------------------------|
| 01 | [story-01-internal-logging-package.md](./story-01-internal-logging-package.md) | `cursor/p1-logging-story-01-internal-logging` |
| 02 | [story-02-wire-cmd-loggers.md](./story-02-wire-cmd-loggers.md) | `cursor/p1-logging-story-02-wire-cmd` |
| 03 | [story-03-portfolio-api-access-logs.md](./story-03-portfolio-api-access-logs.md) | `cursor/p1-logging-story-03-access-logs` |
| 04 | [story-04-ingest-tick-summary.md](./story-04-ingest-tick-summary.md) | `cursor/p1-logging-story-04-ingest-tick` |
| 05 | [story-05-signals-tick-summary.md](./story-05-signals-tick-summary.md) | `cursor/p1-logging-story-05-signals-tick` |

**Integration branch**: `cursor/p1-logging` (stories land first; each story PR targets this branch).

Before coding, read [.agent/skills/logging.md](../../../../skills/logging.md).
