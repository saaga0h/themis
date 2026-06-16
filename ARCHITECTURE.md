# Architecture

<!-- @tier: 1 -->
<!-- @see-also: docs/subsystems/ -->

## Overview

Themis is a software factory system: a set of AI agent definitions, slash command
prompts, and reusable skills that orchestrate autonomous implementation pipelines
over a codebase. Starting with v2.0, deterministic pipeline control is moving out
of LLM instructions and into a compiled Go binary (`cmd/themis`), which will
coordinate agent execution, enforce cycle limits, and manage pipeline state.

## Component Inventory

| Component | Path | Role |
|-----------|------|------|
| Agents | `agents/` | AI agent definitions invoked as subagents during pipeline runs |
| Commands | `commands/` | Slash command prompts that drive top-level user workflows |
| Skills | `skills/` | Reusable skill prompts composed into agents and commands |
| Go binary | `cmd/themis/` | v2.0 pipeline orchestrator (deterministic, compiled) |
| Pipeline state machine | `internal/pipeline/` | Step definitions, state transitions, persistence |
| Prompt templates | `templates/` | Per-step markdown prompt templates with `{{KEY}}` placeholders |
| Prompt substitution | `internal/prompt/` | `{{KEY}}` placeholder substitution for template rendering |
| Project profile | `internal/profile/` | Per-project YAML configuration schema and loader |

### Agents (`agents/`)

Named subagent definitions. Each file is a markdown prompt with a YAML front
matter block declaring `name`, `description`, `tools`, and `model`. Agents are
delegated to via `Task` calls from commands or other agents. Current agents:

- `ac-drafter` — drafts acceptance criteria
- `architecture-reviewer` — reviews architectural concerns
- `codebase-scanner` — scans the repo for scope-relevant files before implementation
- `complexity-reviewer` — reviews numerical or algorithmic complexity
- `context-updater` — updates context artifacts
- `convention-reviewer` — reviews code style and naming conventions
- `coverage-reviewer` — reviews test coverage
- `depth-reviewer` — reviews implementation depth
- `doc-scanner` — identifies documentation drift
- `doc-updater` — updates documentation sections
- `doc-writer` — writes new documentation files
- `interview` — elicits requirements from the user
- `plan-reader` — reads and summarises a plan file
- `pr-composer` — composes the AC verification table and review notes for a PR body
- `security-reviewer` — reviews security concerns
- `test-architect` — designs the test strategy for an issue
- `test-runner` — runs the test suite and reports results
- `test-writer` — writes test files from acceptance criteria

### Commands (`commands/`)

Slash command prompts executed directly by the user in a Claude Code session.
Each file is a markdown prompt with YAML front matter declaring `description`,
`argument-hint`, and `allowed-tools`. Current commands:

- `/architect` — design sessions and ADR authoring
- `/concept` — explore and document a concept
- `/context` — build or refresh codebase context artifacts
- `/document` — trigger targeted documentation updates
- `/factory` — run the full software factory pipeline
- `/feature` — implement a feature from a plan
- `/implement` — implement a plan step
- `/intent-bridge` — capture and persist user intent
- `/issue` — implement a single issue fully autonomously (fetch → tests → implement → review → docs → PR)
- `/review` — run the review pipeline against current branch
- `/ship` — create a pull request for completed work

### Skills (`skills/`)

Reusable prompt fragments. Each skill lives in its own directory as `SKILL.md`.
Current skills: `deepening`, `deepen`, `design-it-twice`, `grill-me`,
`hearth-sync`, `issue-writer`, `pr-review`, `renovate-merge-safe`,
`renovate-plan-major`, `renovate-triage`, `review-walker`, `split-walker`,
`ui-reader`.

### Prompt Templates (`templates/`)

Seven markdown files, one per pipeline step, containing the creative-work
instructions that get rendered and passed to Claude Code as prompts. Templates
use `{{KEY}}` placeholders (uppercase letters, digits, underscores) substituted
at runtime by the pipeline orchestrator. Files: `test-red.md`, `implement.md`,
`refactor.md`, `review.md`, `fix-findings.md`, `update-docs.md`, `ship.md`.

### Prompt Substitution (`internal/prompt/`)

`Substitute(template string, args map[string]string) (string, error)` renders a
prompt template by replacing all `{{KEY}}` placeholders. Returns an error if any
placeholder lacks a corresponding arg (prevents silent empty substitutions) or if
any arg lacks a corresponding placeholder (catches caller-side typos).
### Project Profile (`internal/profile/`)

Per-project YAML configuration at `.themis/profile.yaml`. Controls model
assignments per review agent, round-3 gate behaviour, test-fix attempt limits,
and whether docs and refactor steps are enabled. `Load(dir string) (*Profile, error)`
returns sensible defaults (sonnet/haiku mix, round3=auto, 3 test-fix attempts,
both steps enabled) when the file is absent. Uses `yaml.v3` with `KnownFields(true)`
strict mode and validates model names and round3 values at load time.

### Go Binary (`cmd/themis/`)

Entry point for the v2.0 deterministic orchestration layer. Currently implements
the `version` subcommand (reports `0.1.0`). Future subcommands — `issue` and
`run` — will move pipeline coordination out of LLM prompt instructions and into
compiled, testable Go code.

### Pipeline State Machine (`internal/pipeline/`)

Deterministic skeleton for pipeline orchestration. Defines the ten pipeline steps
(`Fetch → Scan → Branch → TestRed → Implement → Refactor → Review → Fix → Docs → Ship`)
and encodes all transition logic in Go:

- `pipeline.go` — `Step` type, `PipelineState` struct, `Advance()` method, review-cycle
  gate and round-3 logic. No I/O dependencies (stdlib: `errors`, `fmt`, `time` only).
- `store.go` — `SaveState` and `LoadState` functions, JSON persistence to `.themis/state.json`.

`Advance(StepResult) (Step, error)` is the single transition entry point. It handles
the happy path, the Review→Fix→Review cycle (default max 2, extendable to 3 via a
`Round3Trigger`), and test-fix attempt counting per acceptance criterion. State is
persisted after each step so interrupted runs can resume.

## Go Module

| Property | Value |
|----------|-------|
| Module path | `git.home.federation.fi/lavernea/themis` |
| Go version | 1.22 |
| Entry point | `cmd/themis/main.go` |

The module is the foundation for Themis v2.0. The current binary is minimal by
design: it establishes the module structure and versioning so that subsequent
issues can add subcommands (`issue`, `run`) incrementally without restructuring
the repository.

## Related Documents

- `docs/subsystems/` — per-subsystem architecture detail (to be created as Go subsystems grow)
- `UBIQUITOUS_LANGUAGE.md` — canonical terminology used across all agents, commands, and code
- `CODING_STANDARDS.md` — language standards and reviewer checklist
