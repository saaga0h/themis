# Claude Code Workflow: Architect → Implement → Review → Ship

A minimal workflow system for Claude Code that separates reasoning from execution and uses the right model for each job.

## Philosophy

- **Opus** reasons about architecture and complex decisions
- **Sonnet** implements features and maintains project context
- **Haiku** runs tests, scans codebases, reads plans — mechanical work

The user chooses the main model. The workflow handles delegation automatically.

## Components

### Agents (primitives)

| Agent | Default Model | Purpose |
|-------|--------------|---------|
| `codebase-scanner` | haiku | Maps project structure, reports facts |
| `test-runner` | haiku | Runs tests, reports pass/fail |
| `context-updater` | sonnet | Updates CLAUDE.md after changes |
| `doc-updater` | haiku | Updates README following existing style |
| `plan-reader` | haiku | Reads plan files, reports status |
| `architecture-reviewer` | sonnet | Reviews code against CLAUDE.md, flags drift |
| `security-reviewer` | sonnet | Finds vulnerabilities, credentials, injection paths |
| `complexity-reviewer` | haiku | Measures function length, nesting, dependencies |
| `convention-reviewer` | haiku | Checks naming, patterns, style against conventions |
| `coverage-reviewer` | haiku | Maps tested vs untested code, identifies gaps |

Agents are single-purpose. They don't reason about what to do — they execute one thing.

### Commands (compositions)

| Command | Purpose |
|---------|---------|
| `/architect <description>` | Analyze project, create implementation plan |
| `/implement [plan-name]` | Execute plan, run tests, update docs |
| `/review [scope] [--flags]` | Run code review with specialized agents |
| `/ship [plan-name]` | Create PR/MR from completed work |

Commands compose agents into workflows. They ask the user which model to use for the main work, then delegate subtasks to appropriate agents.

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
  → context-updater (sonnet): updates CLAUDE.md
  → doc-updater (haiku): updates README if needed
  → reports results

/review --last-plan
  → complexity-reviewer (haiku): structural scan
  → convention-reviewer (haiku): pattern check
  → coverage-reviewer (haiku): coverage map
  → security-reviewer (sonnet): vulnerability scan
  → architecture-reviewer (sonnet): alignment check
  → synthesizes findings into review report
  → saves to .claude/reviews/

/ship mqtt-retry
  → reads plan, review report, git diff
  → composes PR description
  → user confirms
  → creates PR via GitHub/GitLab MCP or CLI
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

## Plan files

Plans live in `.claude/plans/` within each project. They're self-contained — a fresh session with no context can pick one up and execute it.

Plans track progress with checkboxes. If a session dies, `/implement` picks up where it left off.

## Review reports

Reviews are saved to `.claude/reviews/` within each project. The `/ship` command reads them when composing PR descriptions. They're also useful for tracking code quality over time.

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
- [ ] `/consolidate` command for periodic CLAUDE.md cleanup (Opus)
- [ ] Plan composition (parent plans with child plans)
- [ ] Hooks for auto-running test-runner after edits
- [ ] Skills versions of commands (with supporting templates)
