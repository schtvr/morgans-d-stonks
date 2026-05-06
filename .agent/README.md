# Agent Workspace

This directory stores local agent configuration and reusable playbooks.

## Structure

- `rules/` — global behavior rules and response constraints.
- `hooks/` — hook specs/scripts you can wire into your agent runtime.
- `skills/` — repeatable procedures for common tasks.

## Included starter content

- `rules/minimize-response-size.md`
- `hooks/pre-response-checklist.md`
- `skills/github-branch-management.md`
- `skills/github-issue-creation.md`

You can add more rule/skill files over time without changing application code.
