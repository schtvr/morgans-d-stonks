# Skill: GitHub Branch Management

## Purpose
Create consistent feature branches and keep them clean.

## Quick workflow
1. Update local main:
   - `git checkout main`
   - `git pull origin main`
2. Create branch with standard naming:
   - `git checkout -b cursor/<issue-id>-<short-description>-<suffix>`
3. Commit in logical chunks:
   - `git add -p`
   - `git commit -m "<ISSUE-ID>: <concise change summary>"`
4. Sync with main before PR if needed:
   - `git fetch origin`
   - `git rebase origin/main`
5. Push and set upstream:
   - `git push -u origin <branch-name>`

## Conventions
- One logical change per commit.
- Never commit secrets.
- Keep commit subjects short and imperative.
