# Skill: Create GitHub Issues (CLI)

## Purpose
Create well-formed issues quickly with `gh`.

## Prerequisites
- GitHub CLI authenticated: `gh auth status`
- Repo context set (run inside target repo)

## Minimal command
```bash
gh issue create \
  --title "<short title>" \
  --body "<problem, scope, acceptance criteria>" \
  --label "enhancement"
```

## Better template
Use this body structure:

- **Problem**: what is broken or missing
- **Goal**: desired user-visible outcome
- **Scope**: what to include/exclude
- **Acceptance Criteria**:
  - [ ] Criterion 1
  - [ ] Criterion 2

## Useful options
- Assign: `--assignee @me`
- Project: `--project "<project name>"`
- Milestone: `--milestone "<milestone>"`
