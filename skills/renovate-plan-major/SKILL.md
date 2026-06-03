---
name: renovate-plan-major
description: "For a specific major-tier Renovate PR or upgrade cluster, research the breaking changes, draft one or more migration issues in the Themis /issue format (Title, Context, Acceptance Criteria checkboxes, Notes with production constraints), present the draft here for review and refinement, and file the agreed issue(s) in the target Gitea repo. Produces issues ready for the /factory pipeline. Trigger on phrases like \"plan the nomad upgrade\", \"create migration issue for X\", \"draft an issue for the major tier\", or when the user selects a specific major PR to act on."
---
 
# Renovate Plan Major Skill
 
You are producing one or more migration planning issues for a major-tier
dependency upgrade. The issues must be detailed enough that the Themis
`/factory` pipeline (specifically `/issue`) can execute them without
asking questions mid-run. Production systems are in scope — plan
accordingly.
 
## The pipeline these issues feed
 
```
renovate-plan-major  (this skill — research + draft + file)
  → Gitea issue with label ready-for-agent (when agreed)
  → /issue or /factory in Claude Code (execution)
```
 
---
 
## Step 0: Identify the target
 
Ask if not already clear: which major PR or upgrade cluster is being planned?
If the user points at a specific PR number or package name, use that.
If triage output is available in context, pull from it directly.
 
Retrieve the PR details via `gitea:pull_request_read` (`method: get`) to
confirm: current version, target version, affected files, repo.
 
---
 
## Step 1: Research breaking changes
 
### 1a: Read the PR body
 
Renovate's PR body contains a merge-confidence table and links to changelogs.
Extract any links to release notes or changelogs. Read them via web fetch if
available.
 
### 1b: Identify the change category
 
Determine which of these applies (more than one may apply):
 
- **Docker image major** (e.g. `ubuntu:24 → 26`, `nomad:1.x → 2.x`):
  affects the build/runtime environment of the service
- **Infrastructure service major** (e.g. Nomad, Consul, Postgres):
  affects a running service; may require data migration, config changes,
  API contract changes between components
- **Go module major** (e.g. `pgx/v4 → v5`, `nomad/api v1 → v2`):
  import path changes, API surface changes
- **Frontend framework major** (e.g. React 18 → 19, react-router v6 → v7):
  component API changes, deprecated patterns, bundler compat
- **Coordinated cluster** (e.g. React + react-dom + react-router together):
  all packages must land in a single PR or sequential PRs with no gap
### 1c: Assess production impact
 
Check the repo against what is known about the system:
- Is this repo deployed to production?
- Does it expose an API that other services depend on?
- Does it manage persistent state (database, queue, message broker)?
- Is there a rollback path if the upgrade introduces a regression?
This shapes the issue's Notes and constraints.
 
---
 
## Step 2: Determine issue structure
 
A major upgrade may produce **one issue or several**, depending on complexity:
 
**Single issue** — when the upgrade is self-contained:
- Docker base image bump (no API changes, just rebuild)
- Go module with clear import-path migration and no state concerns
**Multiple issues** — when the upgrade has distinct phases that can be
reviewed and merged independently:
- Phase 1: preparatory refactoring (remove deprecated API usage before
  bumping the version)
- Phase 2: the version bump itself
- Phase 3: follow-up cleanup or feature adoption enabled by the new version
**Cluster issues** — when multiple packages must coordinate:
- One issue per package OR one combined issue, depending on whether they
  can be tested independently. State the dependency explicitly using
  "depends on #N" syntax so `/factory` respects ordering.
Present this structure to the user for agreement before drafting.
 
---
 
## Step 3: Draft the issue(s)
 
Each issue follows the Themis `/issue` format exactly. The `/issue` command
extracts these sections to drive TDD execution.
 
```markdown
# <title>
 
## Context
 
<Why this upgrade is happening. What Renovate flagged. What the current
version is and what the target version is. What this service does and
why the upgrade matters. 2–4 sentences — enough for the factory to
understand the domain without re-reading the PR.>
 
<For infrastructure services: note that this is a production system
and what the blast radius of a regression would be.>
 
<For coordinated clusters: note which other issues this depends on
or must precede.>
 
## Breaking Changes
 
<Enumerate the specific breaking changes relevant to this codebase.
Be concrete — name the API, config key, or behaviour that changes.
Do not list breaking changes that do not apply here.>
 
- `<old API / config>` → `<new API / config>`
- `<deprecated pattern removed>` — replace with `<new pattern>`
- `<behaviour change>` — affects `<what in this repo>`
 
## Acceptance Criteria
 
- [ ] Dependency version updated to <target> in <go.mod / package.json /
      docker-compose.yml / Dockerfile — list all affected files>
- [ ] All existing tests pass with the new version
- [ ] <specific behaviour that must still work after upgrade>
- [ ] <migration step that must be verified — e.g. "database schema
      compatible with Postgres 18">
- [ ] <any new capability that should be exercised to confirm the upgrade
      is functional, not just present>
- [ ] Build succeeds and produces a deployable artifact
- [ ] No deprecated API usage from <old version> remains in the codebase
 
## Notes
 
**Production status**: <yes/no — if yes, note deployment process and
whether a rollback plan is needed>
 
**Migration constraints**:
<Any constraint that shapes implementation — ordering requirements,
data migration steps, config file changes needed outside the repo,
environment variable changes, etc.>
 
**Do not**:
<Explicit prohibitions — things the factory must not do during this
upgrade. Examples: "do not upgrade postgres beyond 18.x", "do not
change the Nomad job spec format during this PR", "do not adopt
React 19 concurrent features — upgrade only, no API migration">
 
**Depends on**: <#N — other issue that must be merged first, or "none">
 
**Rollback path**: <how to revert if the upgrade introduces a regression
in production — e.g. "revert this PR; no data migration required" or
"requires postgres dump before deployment">
```
 
---
 
## Step 4: Present draft for review
 
Present the full draft issue(s) here in chat. Do not file anything yet.
 
Explicitly ask:
1. Is the breaking changes section accurate and complete?
2. Are the ACs specific enough for the factory to verify them as tests?
3. Are the Notes constraints correct — especially "do not" and rollback path?
4. Should this be one issue or multiple? Is the phasing right?
5. Which repo should the issue be filed in?
Incorporate feedback. Re-present if substantive changes were requested.
Only proceed to Step 5 when the user explicitly says the draft is ready.
 
---
 
## Step 5: File the issue(s)
 
Once the draft is agreed:
 
Use `gitea:issue_write` with `method: create` to file each issue in the
correct repo. Set:
- `title`: the issue title from the draft
- `body`: the full markdown from the draft
- Do **not** set `ready-for-agent` label at filing time — the user applies
  that label when they are ready to run the factory. This prevents the
  factory from picking it up before the user is ready.
After filing, report:
 
```
Filed: <repo> #<n> — <title>
URL: <issue url>
 
This issue is NOT yet labeled ready-for-agent.
When you are ready to run /factory or /issue against it, add that label.
```
 
If multiple issues were filed with a dependency chain, list them in order
and note the dependency relationships.
 
---
 
## Hard rules
 
- **Never file an issue without the user explicitly approving the draft.**
  "Looks good" or "file it" is sufficient. Do not interpret enthusiasm as
  approval — ask directly if unclear.
- **Never add `ready-for-agent` at filing time.** That is a deliberate
  human gate before factory execution.
- **Never merge the Renovate PR directly.** This skill produces planning
  issues. The factory closes the Renovate PR as part of implementing the
  migration issue, or the user merges it manually after the migration is done.
- **If breaking changes cannot be determined** (changelog unavailable, no
  release notes, ambiguous PR body): say so explicitly in the issue's
  Breaking Changes section and add an AC: "[ ] Breaking changes researched
  and addressed — see <link>". Do not fabricate specifics.
- **Production systems get a rollback path in Notes.** No exceptions.
  If the rollback path is genuinely unknown, that is itself a blocker to
  label — note it and surface it to the user before filing.
