# Themis — Claude Code Workflow System

A workflow system for Claude Code built around three principles: right model for the job, tests before architecture, and docs loaded by scope not by habit.

## Philosophy

- **Opus** reasons about architecture and complex decisions
- **Sonnet** implements features, writes docs, handles judgment calls
- **Haiku** runs tests, scans codebases, reads plans — mechanical work

The user chooses the main model. The workflow handles delegation automatically.

## On context

Context has a cost. Loading everything into every session degrades decisions and burns tokens on information that isn't relevant to the current task.

This workflow manages context at three layers:

**CLAUDE.md** — always loaded, so kept to the minimum that prevents hard failures: build quirks, environment constraints, things an agent will get wrong immediately without being told. If `docs/development.md` exists, CLAUDE.md holds a pointer to it rather than duplicating its content. Research ([Gloaguen et al., 2025](https://arxiv.org/abs/2602.11988)) shows that bloated context files reduce agent task success rates while increasing cost by 20%+.

**docs/ — tiered project documentation** — loaded on demand, never speculatively. Root docs (README, ARCHITECTURE, CONCEPTS), Tier 1 reference (development, data model, API, messaging), Tier 2 subsystem docs, Tier 3 module-level business rules. Commands resolve which docs are relevant to their scope and load only those.

**Scope-based doc resolver** — commands pass their scope (feature description, plan name, subsystem) to `codebase-scanner`, which greps `docs/content-plan.md` for matching tags and returns only the relevant paths. `/architect add retry to MQTT client` loads the MQTT subsystem doc. `/review --security` loads nothing from docs/. The resolution happens inside the Haiku scan call that already runs — no extra agent cost.

```markdown
# CLAUDE.md — Project Name

## Constraints
<!-- what an agent WILL get wrong without being told — hard failures only -->

## Gotchas
<!-- non-obvious operational issues, remove when fixed -->

## Docs
See `docs/development.md` for build commands, setup, env vars, and troubleshooting.
```

A good CLAUDE.md is under 20 lines. Everything else belongs in docs/.

## Components

### Agents (primitives)

| Agent | Default Model | Purpose |
|-------|--------------|---------|
| `codebase-scanner` | haiku | Maps project structure, reports facts |
| `test-runner` | haiku | Runs tests, reports pass/fail |
| `context-updater` | sonnet | Updates CLAUDE.md — operational info only |
| `doc-updater` | haiku | Updates README following existing style |
| `plan-reader` | haiku | Reads plan files, reports status |
| `architecture-reviewer` | sonnet | Checks internal consistency from code structure |
| `security-reviewer` | sonnet | Finds vulnerabilities, credentials, injection paths |
| `complexity-reviewer` | haiku | Measures function length, nesting, dependencies |
| `convention-reviewer` | haiku | Checks patterns against codebase majority style |
| `coverage-reviewer` | haiku | Maps tested vs untested code, identifies gaps |
| `doc-scanner` | haiku | Scans codebase for docs, maps drift between code and docs, produces structured drift report |
| `doc-writer` | sonnet | Writes or updates one documentation file per invocation from a specific assignment |
| `interview` | sonnet | Derives AC through earned questions; detects specification vs. exploration mode |
| `ac-drafter` | sonnet | Formats draft AC into structured criteria ready for test-architect |
| `test-architect` | sonnet | Decides test type, boundary, and what NOT to test; produces test skeleton |
| `test-writer` | sonnet | Writes failing tests from skeleton; confirms RED; presents Specification Review |

Agents are single-purpose. They don't reason about what to do — they execute one thing.

### Commands (compositions)

| Command | Purpose |
|---------|---------|
| `/architect <description>` | Analyze project, create implementation plan |
| `/implement [plan-name]` | Execute plan, run tests, update docs |
| `/review [scope] [--flags]` | Run code review with specialized agents |
| `/ship [plan-name]` | Create PR/MR from completed work |
| `/context [--dry-run]` | Rebuild CLAUDE.md from current repo state |
| `/feature <description>` | TDD-first entry point — interview, tests, then architect |
| `/concept <intent-doc>` | TDD-first entry point from a web conversation intent document |
| `/document [--dry-run] [--full] [--tier N]` | Audit and update project documentation |
| `/issue <number>` | Implement a single GitHub issue end-to-end, fully autonomously |
| `/factory [--max N] [--dry-run]` | Process all open `ready-for-agent` issues in order |

Commands compose agents into workflows. They ask the user which model to use for the main work, then delegate subtasks to appropriate agents.

## Software Factory — Running CC in Isolation

The `/issue` and `/factory` commands are designed to run fully autonomously inside
a sandboxed container. This gives you a software factory: GitHub issues as the work
queue, Claude Code as the implementer, a container as the blast radius boundary.

### Why a container

Running Claude Code directly on your machine with `--dangerously-skip-permissions`
means it has unrestricted access to your filesystem, credentials, and environment.
A container bounds what it can touch to the mounted workspace — nothing else on
the host is visible.

### Requirements

- Podman (rootless) or Docker
- A `Containerfile` / `Dockerfile` in your repo with Claude Code installed
- `CLAUDE_CODE_OAUTH_TOKEN` — generated via `claude setup-token` on your host
- `GH_TOKEN` — a GitHub fine-grained token with Issues (read/write) and Pull requests (read/write)

### Containerfile

A minimal Containerfile that installs Claude Code:

```dockerfile
FROM node:22-bookworm

RUN apt-get update && apt-get install -y git curl jq gh && rm -rf /var/lib/apt/lists/*

ARG AGENT_UID=1000
ARG AGENT_GID=1000
RUN groupmod -g $AGENT_GID node && \
    usermod -u $AGENT_UID -g $AGENT_GID -d /home/agent -m -l agent node
USER ${AGENT_UID}:${AGENT_GID}

RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/agent/.local/bin:$PATH"

WORKDIR /home/agent
ENTRYPOINT ["sleep", "infinity"]
```

Build once:

```bash
podman build -t myproject:dev .
```

### Auth token

Generate a token on your host (do this once):

```bash
claude setup-token
```

Store tokens in a `.env` file in your repo root (add to `.gitignore`):

```
CLAUDE_CODE_OAUTH_TOKEN=your_token_here
GH_TOKEN=your_github_token_here
```

### Running the factory

```bash
# Dry run — see what would be processed
echo '/factory --dry-run' | podman run -i \
  --userns=keep-id \
  --entrypoint claude \
  -v $(pwd):/home/agent/workspace \
  -v ~/.claude:/home/agent/.claude \
  --env-file .env \
  -w /home/agent/workspace \
  myproject:dev \
  --print \
  --dangerously-skip-permissions \
  --max-turns 300

# Full factory run
echo '/factory' | podman run -i \
  --userns=keep-id \
  --entrypoint claude \
  -v $(pwd):/home/agent/workspace \
  -v ~/.claude:/home/agent/.claude \
  --env-file .env \
  -w /home/agent/workspace \
  myproject:dev \
  --print \
  --dangerously-skip-permissions \
  --max-turns 300

# Single issue
echo '/issue 42' | podman run -i \
  --userns=keep-id \
  --entrypoint claude \
  -v $(pwd):/home/agent/workspace \
  -v ~/.claude:/home/agent/.claude \
  --env-file .env \
  -w /home/agent/workspace \
  myproject:dev \
  --print \
  --dangerously-skip-permissions \
  --max-turns 300
```

### Makefile shorthand

Wrap this in a Makefile to avoid typing the full command every time:

```makefile
IMAGE := myproject:dev
MAX_TURNS ?= 300

PODMAN_RUN := podman run -i \
	--userns=keep-id \
	--entrypoint claude \
	-v $(PWD):/home/agent/workspace \
	-v $(HOME)/.claude:/home/agent/.claude \
	--env-file .env \
	-w /home/agent/workspace \
	$(IMAGE) \
	--print \
	--dangerously-skip-permissions \
	--max-turns $(MAX_TURNS)

run-factory:
	echo '/factory' | $(PODMAN_RUN)

dry-run:
	echo '/factory --dry-run' | $(PODMAN_RUN)

run-issue:
	echo '/issue $(ISSUE)' | $(PODMAN_RUN)

shell:
	podman run -it --userns=keep-id --entrypoint /bin/bash \
		-v $(PWD):/home/agent/workspace \
		-v $(HOME)/.claude:/home/agent/.claude \
		--env-file .env -w /home/agent/workspace $(IMAGE)
```

Then: `make run-factory`, `make run-issue ISSUE=42`, `make dry-run`.

### Issue format

Issues picked up by `/factory` must be labeled `ready-for-agent` and follow this structure:

```markdown
## Context
<what this is and why>

## What to build
<implementation guidance>

## Acceptance Criteria
- [ ] AC one — verifiable, specific
- [ ] AC two — verifiable, specific

## Notes
<constraints, deferred scope, references>
```

Each AC becomes a failing test. The agent implements until all tests pass, runs a
full review, fixes blocking findings, and ships a PR. The human merges.

### Credential notes

- `--userns=keep-id` is required on rootless Podman — without it the container
  process runs as a different UID and cannot read `~/.claude` even with `:rw` mount
- `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token` works for headless use;
  the interactive OAuth flow does not work inside a container
- Store both tokens in `.env` and add `.env` to `.gitignore`

## TDD Workflow

The TDD-first entry points invert the default order: tests define the contract
before architecture begins. Acceptance criteria are derived through structured
interview, not assumed. The distinction between specification (you know what
correct looks like) and exploration (you're finding out) is made explicit before
any test or plan is written.

```
/feature add retry logic to MQTT client
  → asks: architecture model? implementation model?
  → codebase-scanner (haiku): maps project
  → interview (sonnet): specification or exploration?
      if exploration → writes breadcrumb to .claude/explorations/, stops
      if specification → drafts AC with explicit assumptions, human corrects
  → ac-drafter (sonnet): formats AC, human confirms [Touchpoint 1]
  → test-architect (sonnet): designs test structure, human reviews [Touchpoint 2]
  → test-writer (sonnet): writes failing tests, confirms RED
      → test-runner (haiku): all tests FAIL — this is correct
      → Specification Review: "does this specify the right thing?" [Touchpoint 3]
  → /architect: creates plan with **Test Spec** field referencing test files
      human reviews plan [Touchpoint 4]
  → /implement: makes failing tests pass
      → test-runner (haiku): confirms GREEN after each task

/concept path/to/intent-doc.md
  → same as /feature, but prepends intent-bridge if doc is not yet grounded
  → detects grounded status from "## Status: grounded" marker — skips if present
  → from interview onward: identical to /feature
```

The existing commands remain the default for work that doesn't need TDD
entry points, and as escape hatches from inside the pipeline.

## Workflow

```
/architect add retry logic to MQTT client
  → asks: which model? (opus/sonnet/haiku)
  → codebase-scanner (haiku): maps project
  → chosen model: reasons about approach
  → writes plan to .claude/plans/mqtt-retry.md
  → user reviews and confirms

/implement mqtt-retry
  → asks: which model? (sonnet/haiku/opus)
  → plan-reader (haiku): reads plan, finds first task
  → chosen model: implements each task
  → test-runner (haiku): tests after each task
  → context-updater (sonnet): only if build/tooling/constraints changed
  → doc-updater (haiku): updates README if needed
  → reports results

/review --last-plan
  → complexity-reviewer (haiku): structural scan
  → convention-reviewer (haiku): pattern check (from code, not CLAUDE.md)
  → coverage-reviewer (haiku): coverage map
  → security-reviewer (sonnet): vulnerability scan
  → architecture-reviewer (sonnet): internal consistency check
  → synthesizes findings into review report
  → saves to .claude/reviews/

/ship mqtt-retry
  → reads plan, review report, git diff
  → composes PR description
  → user confirms
  → creates PR via GitHub/GitLab MCP or CLI

/context
  → codebase-scanner (haiku): maps project
  → audits build commands, conventions, constraints, gotchas
  → rebuilds CLAUDE.md from scratch
  → user confirms before writing

/document
  → doc-scanner (haiku): identifies project type, maps codebase, inventories docs, reports drift
  → reviews scan: what exists, what drifted, what is missing entirely
  → reports audit grouped by document — stops here if --dry-run
  → updates root docs (README, ARCHITECTURE, CONCEPTS) — CONCEPTS written by orchestrator, not delegated
  → updates Tier 1 docs in docs/ (development, datamodel, api-reference, messaging, content-plan)
  → updates Tier 2 subsystem docs in docs/subsystems/<name>/
  → decides Tier 3 module docs — orchestrator judges incident surface area, not an agent
  → codebase-scanner (haiku): verifies cross-references, dead links, content-plan consistency
  → reports final coverage: root docs, Tier 1, Tier 2, Tier 3
```

## Review flags

Run all reviewers or select specific perspectives:

| Flag | Runs | Models |
|------|------|--------|
| (no flags) | All five reviewers | 2 sonnet + 3 haiku |
| `--quick` | complexity + conventions | 2 haiku |
| `--security` | security only | 1 sonnet |
| `--architecture` | architecture only | 1 sonnet |
| `--complexity` | complexity only | 1 haiku |
| `--conventions` | conventions only | 1 haiku |
| `--coverage` | coverage only | 1 haiku |

Scope with a directory path or `--last-plan`:
```
/review pkg/patterns/ --security
/review --last-plan --quick
/review                          # full project, all reviewers
```

## Document flags

Scope or change the behavior of the documentation audit:

| Flag | Behavior |
|------|----------|
| (no flags) | Update only what has drifted |
| `--full` | Rebuild all docs from scratch, treating code as sole source of truth |
| `--dry-run` | Report drift only — write nothing |
| `--tier 0` | Root docs only (README, ARCHITECTURE, CONCEPTS) |
| `--tier 1` | `docs/` tier only (development, datamodel, api-reference, etc.) |
| `--tier 2` | Subsystem docs only (`docs/subsystems/<name>/`) |
| `--tier 3` | Module docs only (`docs/subsystems/<name>/modules/`) |

Examples:
```
/document                    # audit everything, update what drifted
/document --dry-run          # see what would change without writing
/document --full             # rebuild all docs from code
/document --tier 0           # update root docs only
/document --tier 2 --full    # rebuild all subsystem docs from scratch
```

## Installation

```bash
chmod +x install.sh
./install.sh
```

Installs to `~/.claude/agents/` and `~/.claude/commands/` (global).

## Key design decisions

### Code is the source of truth, not CLAUDE.md

The `architecture-reviewer` derives architectural intent from package structure, import graphs, and naming conventions — not from prose descriptions in CLAUDE.md. The `convention-reviewer` infers conventions from the majority pattern in existing code, with CLAUDE.md only for explicit overrides. This avoids the drift problem where agents argue with stale documentation instead of reading the code.

### Context updates are conditional

`/implement` only calls `context-updater` when build commands, tooling, protocol contracts, or environment requirements actually changed. Most implementation runs don't touch operational constraints, so most runs skip the CLAUDE.md update entirely.

### /context is explicit, not automatic

CLAUDE.md regeneration is a user-triggered action (`/context`), not something that happens automatically. Use it before major refactoring, after large merges, or when things feel stale. It rebuilds from scratch by auditing the repo, preserving only user-added custom sections.

## Plan files

Plans live in `.claude/plans/` within each project. They're self-contained — a fresh session with no context can pick one up and execute it.

Plans track progress with checkboxes. If a session dies, `/implement` picks up where it left off.

## Review reports

Reviews are saved to `.claude/reviews/` within each project. The `/ship` command reads them when composing PR descriptions.

## Designed for messy projects

This workflow assumes projects are imperfect:
- Documentation might be stale
- Code might not match docs
- Tests might be missing or broken
- There might be leftover code from refactors

The `/architect` command handles this by scanning what actually exists before planning, and the `/implement` command stops and asks when reality doesn't match the plan.

## Evolving this

- [x] `/review` command for code review workflow
- [x] `/ship` command for PR/MR creation
- [x] `/context` command for explicit CLAUDE.md rebuild
- [x] Paper-informed CLAUDE.md philosophy (minimal operational context)
- [x] TDD-first workflow (`/feature`, `/concept`) with structured AC derivation
- [x] Exploration mode — explicit first-class alternative to TDD for hypothesis work
- [x] `/document` command for structured, tiered documentation auditing and generation
- [x] `/issue` command — fully autonomous GitHub issue implementation
- [x] `/factory` command — autonomous backlog processing loop
- [x] Sandboxed execution via Podman/Docker with `--userns=keep-id`
- [ ] Plan composition (parent plans with child plans)
- [ ] Hooks for auto-running test-runner after edits
- [ ] Skills versions of commands (with supporting templates)
