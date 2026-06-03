---
name: renovate-merge-safe
description: "Merge the safe tier of Renovate PRs identified by renovate-triage. Presents the batch with enough detail for a go/no-go decision, waits for explicit confirmation, then merges sequentially with a per-PR safety check before each merge. Conservative by design — any ambiguity skips the PR and flags it. Never touches minor or major tier. Trigger on phrases like \"merge the safe ones\", \"merge safe renovate PRs\", \"go ahead with the safe tier\", or after triage output when the user says to proceed."
---
 
# Renovate Merge Safe Skill
 
You are merging only the PRs classified as **safe** by `renovate-triage`. You
do not touch minor or major tier items under any circumstances. The design
principle is: it is always better to skip a safe PR than to accidentally merge
a questionable one.
 
## Precondition
 
This skill requires triage output. If triage has not been run in this session,
run `renovate-triage` first and wait for the report before proceeding.
 
---
 
## Step 1: Present the batch for confirmation
 
From the triage report, extract all safe-tier PRs. Present them to the user as
a single list with enough information to make a decision:
 
```
Ready to merge — <n> PRs:
 
  calliope #26   pgvector/pgvector:pg16 digest to 00ba258    CI: passing
  calliope #27   python:3.14-bookworm digest to d155200      CI: passing
  jeeves   #20   pgvector/pgvector:pg16 digest to 00ba258    CI: passing
  minerva  #25   pgvector/pgvector:pg16 digest to 00ba258    CI: passing
  journal  #32   pgvector/pgvector:pg16 digest to 00ba258    CI: passing
  forge    #29   ubuntu:24.04 docker digest to c4a8d55       CI: passing
  calliope #25   genproto/rpc digest to 0a33c5d              CI: passing
  journal  #31   pgx/v5 v5.9.1 → v5.9.2                     CI: passing
  minerva  #23   pgx/v5 v5.9.1 → v5.9.2                     CI: passing
  forge    #30   nomad/api digest to 2d76421                 CI: passing
 
PRs flagged CI absent (will be skipped unless you confirm):
  (none)
 
Shall I merge all of the above? Or list any you want to exclude.
```
 
**Wait for explicit confirmation before touching anything.** A vague "yes" or
"go ahead" is sufficient. If the user excludes specific PRs, remove them from
the batch and confirm the revised list.
 
If any PR is flagged CI absent, call it out explicitly and ask whether to
include or skip it. Default is skip.
 
---
 
## Step 2: Pre-merge safety check (per PR, in order)
 
Before merging each PR, run a fresh safety check. Do not rely on the triage
snapshot — state may have changed.
 
For each PR call `gitea:pull_request_read` with `method: get`:
 
**Abort this PR (skip, do not merge) if any of these are true:**
- `state` is not `open` — already merged or closed
- `mergeable` is `false` — merge conflict
- PR title or body has changed to indicate a version bump (not just a digest)
  since triage — re-classify on the fly
**Flag and ask before merging if:**
- CI status changed to `failing` since triage
- CI was `absent` at triage and is still absent
**Proceed if:**
- `state: open`, `mergeable: true`, CI passing or pending
For CI pending: note it in the merge result but proceed — digest bumps do not
depend on CI for safety.
 
---
 
## Step 3: Merge
 
Use `gitea:pull_request_write` with `method: merge`. Use merge method
`merge` (not squash, not rebase) unless the repo's default is known to be
different. Do not modify the commit message — keep Renovate's.
 
After each merge, log the result immediately:
 
```
✅ calliope #26 — merged (pgvector digest)
✅ jeeves #20   — merged (pgvector digest)
⏭️  forge #29   — skipped: CI failing (started passing after triage)
✅ journal #31  — merged (pgx/v5 patch)
```
 
If a merge call fails (API error, permission error, conflict race), log it
as `❌ <repo> #<n> — failed: <reason>`, skip to the next PR, and continue.
Do not abort the whole batch on a single failure.
 
---
 
## Step 4: Final report
 
After the batch completes:
 
```
## Merge complete
 
Merged:   <n> PRs
Skipped:  <n> PRs
Failed:   <n> PRs
 
### Merged
✅ calliope #26 — pgvector digest
✅ ...
 
### Skipped (reason)
⏭️  forge #29  — CI failing at merge time
⏭️  ...
 
### Failed (action needed)
❌ ...
 
### Next steps
- <n> PRs in minor tier await review — run renovate-triage to see them,
  or ask to go through them now.
- <n> PRs in major tier await migration planning.
```
 
Then stop. Do not proceed to minor or major tier autonomously.
 
---
 
## Hard rules
 
- **Never merge a PR not in the safe tier.** If a PR was reclassified during
  the pre-merge check (e.g. discovered to be a version tag change not a digest),
  skip it immediately and note the reclassification.
- **Never merge without the confirmation step.** Even if the user said "just
  do it all" — present the list first so they can see exactly what will happen.
- **One PR at a time, sequentially.** Do not attempt parallel merges.
- **Stop on repeated API failures.** If 3 or more consecutive merges fail with
  the same error pattern, stop the batch and report — something systemic is
  wrong.
- **Do not modify PRs, add comments, or change labels** as part of this flow.
  Merge only.