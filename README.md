# Claude Code Workflow: Architect → Implement → Review → Ship

A minimal workflow system for Claude Code that separates reasoning from execution and uses the right model for each job.

## Philosophy

- **Opus** reasons about architecture and complex decisions
- **Sonnet** implements features and handles judgment calls
- **Haiku** runs tests, scans codebases, reads plans — mechanical work

The user chooses the main model. The workflow handles delegation automatically.

## On CLAUDE.md

This workflow treats CLAUDE.md as **minimal operational context** — not project documentation.

Research ([Gloaguen et al., 2025](https://arxiv.org/abs/2602.11988)) shows that LLM-generated context files with project descriptions, architecture overviews, and directory trees **reduce** agent task success rates while increasing cost by 20%+. Information the agent can discover from code is redundant at best, actively harmful at worst — the agent spends tokens reading and complying with descriptions that may have drifted from reality.

CLAUDE.md should contain only what an agent **cannot figure out** from reading the code:

```markdown
# CLAUDE.md — Project Name

## Build & Test
<!-- exact commands, versions, only what's not obvious from Makefile/config -->

## Conventions
<!-- only non-obvious, non-linter-enforceable patterns -->

## Constraints
<!-- what an agent WILL get wrong without being told -->

## Gotchas
<!-- non-obvious operational issues, remove when fixed -->
```

**What belongs**: build commands, toolchain versions, protocol contracts, external dependencies, environment quirks, things learned the hard way.

**What doesn't**: project descriptions, architecture overviews, directory trees, changelogs, current state. The code already says all of that.

A good CLAUDE.md is 20–50 lines. If yours is longer, it's probably duplicating the codebase.

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

Commands compose agents into workflows. They ask the user which model to use for the main work, then delegate subtasks to appropriate agents.

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
- [ ] Plan composition (parent plans with child plans)
- [ ] Hooks for auto-running test-runner after edits
- [ ] Skills versions of commands (with supporting templates)
