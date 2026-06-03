---
name: renovate-triage
description: "Scan all Gitea repositories for open Renovate PRs and Dependency Dashboard issues, classify each update into safe/minor/major tiers using explicit rules, and produce a structured report as the foundation for merge or planning decisions. Pure read — no side effects. Always run this first before renovate-merge-safe or renovate-plan-major. Trigger on phrases like \"check renovate\", \"what renovate PRs are open\", \"triage dependencies\", \"what needs merging\", or any request to audit the state of Renovate across repos."
---
 
# Renovate Triage Skill
 
You are performing a read-only audit of all open Renovate activity across the
user's Gitea repositories. The output is a structured triage report that feeds
directly into `renovate-merge-safe` (for the safe tier) and
`renovate-plan-major` (for the major tier). Nothing is merged or filed here.
 
## The pipeline this skill feeds
 
```
renovate-triage  (this skill — read only)
  → renovate-merge-safe    (merge safe tier, batch with confirmation)
  → renovate-plan-major    (draft migration issues for major tier)
```
 
---
 
## Step 1: Collect open Renovate PRs
 
Search for all open PRs authored by `renovate` across all repos owned by the
current user. Use `gitea:search_issues` with `type: pulls`, `state: open`,
`query: renovate`, `owner: <user>`. Paginate if needed — fetch all results
before classifying anything.
 
For each PR record: repo, PR number, title, URL, created_at, updated_at.
 
---
 
## Step 2: Collect Dependency Dashboards
 
Search for open issues with `title: Dependency Dashboard` across all repos.
Use `gitea:search_issues` with `type: issues`, `state: open`,
`query: Dependency Dashboard`. For each dashboard found, read the full body
via `gitea:issue_read`. Extract:
 
- **Rate-limited** updates (not yet raised as PRs)
- **Pending approval** updates (held by Renovate config, not yet raised)
- Any **warnings or errors** Renovate has logged (lookup failures, config
  problems, etc.)
---
 
## Step 3: Classify every item
 
Apply these rules in order. The first matching rule wins.
 
### Safe tier — merge without hesitation
 
- **Docker digest bump**: title contains `digest to <sha>`, same image tag,
  new SHA. This is a security patch within the same version. Always safe.
- **Go pseudo-version digest**: title contains `digest to <sha>` for a Go
  module. Same logic — content-addressed, same semantic version.
- **Patch within same semver major**: e.g. `v5.9.1 → v5.9.2`, prefix `fix(deps)`.
  No API changes possible within a patch. Safe if the major is unchanged.
### Minor tier — needs review before merging
 
- **Minor version bump** of a third-party library: e.g. `v1.80 → v1.81`,
  `v0.53 → v0.55`. Flag with the ecosystem (Go stdlib, gRPC, frontend tooling,
  Hashicorp, etc.) since risk varies by domain.
- **Base image tag bump** (e.g. `golang:1.26-alpine` digest where the tag
  itself is the current target): technically a digest but the builder image
  deserves a sanity check that the Go version matches go.mod.
- **Frontend tooling minor** (TypeScript, Vite, ESLint, Tailwind, PostCSS):
  these have a higher breaking-change rate at minor versions than typical
  libraries. Flag individually, note ecosystem compatibility risks.
- Any update where **CI is absent** on the PR (no status check result):
  safe in classification but flag as "CI absent — confirm manually".
### Major tier — hold, requires migration planning
 
- **Major version increment**: e.g. `v1 → v2`, `v16 → v18`, `v24 → v26`.
  Applies to both Docker images and Go/npm packages. No exceptions.
- **Pending approval** in the Dependency Dashboard: Renovate itself has gated
  this — respect the gate.
- **Coordinated upgrade cluster**: a set of packages that must land together
  (e.g. React + react-dom + react-router + @types/react). Treat the cluster
  as a single major item even if individual packages are minor.
### Issues / no action
 
- **Renovate lookup failure** (visible in dashboard warnings): not a PR, needs
  a `renovate.json` datasource config fix. Note separately.
- **Already merged**: note for awareness, do not include in any action tier.
- **Rate-limited / not yet a PR**: classify by what it would be (safe/minor/
  major) and note it is not yet actionable.
---
 
## Step 4: Check CI status for safe-tier PRs
 
For each PR in the safe tier, call `gitea:pull_request_read` with
`method: get_status` to get the head commit CI result. Record:
 
- `passing` — CI ran and passed
- `failing` — CI ran and failed → move to minor tier with note "CI failing"
- `pending` — CI is running → note as pending, do not block triage
- `absent` — no CI configured or no run triggered → flag "CI absent"
Do not check CI for minor or major tier — that happens during review.
 
---
 
## Step 5: Produce the triage report
 
Structure the report exactly as follows. This format is what
`renovate-merge-safe` and `renovate-plan-major` consume.
 
```markdown
# Renovate Triage — <date>
 
## Summary
- Safe to merge: <n> PRs across <n> repos
- Needs review (minor): <n> PRs
- Hold (major): <n> PRs / clusters
- Issues (config/errors): <n>
 
---
 
## ✅ Safe tier — ready to merge
 
| Repo | PR | Title | Type | CI |
|------|----|-------|------|----|
| calliope | #26 | pgvector/pgvector:pg16 digest to 00ba258 | digest | passing |
| ... | | | | |
 
---
 
## ⚠️ Minor tier — needs review
 
For each item:
**<repo> #<n> — <title>**
- Type: <digest/patch/minor>
- Ecosystem: <Go stdlib / gRPC / frontend / Hashicorp / etc.>
- Risk note: <one sentence — what to check before merging>
- CI: <passing/absent/failing>
 
---
 
## 🔴 Major tier — hold, migration planning required
 
For each item or cluster:
**<repo> #<n> (or cluster name) — <title>**
- From: <current version>
- To: <target version>
- Production: <yes/no — is this repo in production?>
- Cluster: <list other PRs that must land together, or "standalone">
- Known risks: <what is known about breaking changes from this upgrade>
 
---
 
## ℹ️ Issues / no action
 
- <repo>: <description of config error or lookup failure>
- <repo> #<n>: already merged — noted for awareness
 
---
 
## Rate-limited / pending (not yet PRs)
 
- <repo>: <title> — would be <safe/minor/major> — <unlock or approval needed>
```
 
---
 
## Behaviour rules
 
- **Never merge, comment, or modify anything** during triage. Read only.
- If a repo's Dependency Dashboard cannot be fetched, note it as unavailable
  and continue — do not abort the whole triage.
- If the PR list is large (>30), process in pages and accumulate before
  classifying. Do not report partial results.
- When classification is ambiguous (e.g. a pseudo-version bump that crosses
  a breaking change boundary), default to the more conservative tier and add
  a note explaining the ambiguity.
- After producing the report, ask: "Shall we proceed with the safe tier, review
  the minor tier together, or start planning for a specific major?" Do not
  proceed autonomously.