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
| Agent invoker | `internal/agent/` | `Invoker` interface and `ClaudeCodeInvoker` for spawning Claude Code |
| Git helpers | `internal/git/` | Context-aware git subprocess helpers for commit snapshot and branch queries |

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

### Agent Invoker (`internal/agent/`)

`Invoker` interface with `Invoke(ctx, InvokeOptions) (*InvokeResult, error)` as
the seam between deterministic pipeline control and LLM creative work.
`ClaudeCodeInvoker` implements the interface by spawning `claude --print
--dangerously-skip-permissions --max-turns N --model MODEL`, feeding the prompt
via stdin, and capturing stdout. Detects commits made during invocation by
snapshotting `git log` before/after via `internal/git`. Tests use a
`fakeInvoker` — no real `claude` process required for unit tests.

### Git Helpers (`internal/git/`)

Context-aware wrappers around git subprocess calls. All functions accept
`context.Context` so callers can cancel in-flight git operations. Validates that
`dir` is an absolute path before constructing subprocesses. Functions:
`CommitsBefore`, `CommitsAfter`, `WorkingTreeClean`, `CurrentBranch`. Tested
against real temporary git repositories (no mocking).

### Go Binary (`cmd/themis/`)

Entry point for the v2.0 deterministic orchestration layer. Currently implements
the `version` subcommand (reports `0.1.0`). Future subcommands — `issue` and
`run` — will move pipeline coordination out of LLM prompt instructions and into
compiled, testable Go code.

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
