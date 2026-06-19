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
| Agent invoker | `internal/agent/` | `Invoker` interface and `ClaudeCodeInvoker` for spawning Claude Code |
| Git helpers | `internal/git/` | Context-aware git subprocess helpers for commit snapshot and branch queries |
| Issue tracker integration | `internal/tracker/` | Fetcher interface and implementations for GitHub (gh CLI) and Gitea (REST API); AC checkbox parser |
| Checkpoint verification | `internal/checkpoint/` | Verifies commit message prefixes and working-tree cleanliness after each agent step |
| Pipeline runner | `internal/runner/` | Orchestrates the full pipeline: loads profile, fetches issue, invokes agents per step, enforces limits, creates PR |

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
`hearth-sync`, `issue-writer`, `pr-composition`, `pr-review`, `renovate-merge-safe`,
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
### Agent Invoker (`internal/agent/`)

`Invoker` interface with `Invoke(ctx, InvokeOptions) (*InvokeResult, error)` as
the seam between deterministic pipeline control and LLM creative work.
`InvokeOptions` carries `Prompt`, `Model`, `MaxTurns`, `WorkDir`, `AllowedTools`,
`IssueNumber int`, and `PipelineStep string`. `ClaudeCodeInvoker` implements the
interface by spawning `claude --print --dangerously-skip-permissions --max-turns N
--model MODEL`, feeding the prompt via stdin, and capturing stdout. Before spawning,
it injects `OTEL_RESOURCE_ATTRIBUTES=issue.number=N,pipeline.step=S` into the
subprocess environment (prepended to any existing `OTEL_RESOURCE_ATTRIBUTES` value),
so every Claude Code invocation is tagged with the current issue and pipeline step in
observability telemetry. Detects commits made during invocation by snapshotting
`git log` before/after via `internal/git`. Tests use a `fakeInvoker` — no real
`claude` process required for unit tests.

### Git Helpers (`internal/git/`)

Context-aware wrappers around git subprocess calls. All functions accept
`context.Context` so callers can cancel in-flight git operations. Validates that
`dir` is an absolute path before constructing subprocesses. Functions:
`CommitsBefore`, `CommitsAfter`, `WorkingTreeClean`, `CurrentBranch`, `LastCommitMessage`,
`CheckoutNewBranch`, `Checkout`, `Fetch`, `PushBranch`, `ChangedFiles`, `BranchCommitLog`,
`CommitsAheadOfBase`.
`InferGiteaConfig(ctx, dir)` parses the `origin` remote URL (https:// or ssh:// formats)
and returns a `GiteaConfig` with `Owner`, `Repo`, and `APIBase` fields inferred from the URL.
SCP-style SSH remotes (`git@host:path`) are not supported.
`ChangedFiles` returns newline-separated file paths changed on HEAD relative to the nearest
remote tracking branch (using merge-base diff); returns empty string when no remote tracking
branch exists. `BranchCommitLog` returns `git log --oneline` output for commits on the current
branch relative to the nearest remote tracking branch (same merge-base logic); returns empty
string when no remote exists or any git command fails. Tested against real temporary git
repositories (no mocking).

### Go Binary (`cmd/themis/`)

Entry point for the v2.0 deterministic orchestration layer. Implements three
subcommands: `version` (reports `0.1.0`), `issue`, and `run`.

The `issue` subcommand runs the full pipeline for a single issue number. It
resolves the repo root, selects a tracker fetcher based on provider (from args),
wires the production `Config` via `newIssueConfig` (which calls
`checkpoint.NewStepCheckpoint` and sets it as `CheckpointFn`), and delegates to
`runner.Run`. Gitea connection details (`owner`, `repo`, `apiBase`) are inferred
from the `origin` git remote via `resolveGiteaConfig` (which calls
`git.InferGiteaConfig`); `GITEA_OWNER`, `GITEA_REPO`, and `GITEA_API_URL`
environment variables override individual fields when set. `GITEA_TOKEN` is always
read from the environment and is required for Gitea.

The `run` subcommand processes all open `ready-for-agent` issues sequentially.
It lists issues via `IssueQuerier` (`GiteaQuerier` or `GitHubQuerier` depending
on provider), sorts them by number, and runs the `issue` pipeline for each one
in order. Before each issue it checks out `main`. A failure on one issue is
logged and the loop continues. The loop stops early if `TurnTracker.RemainingFraction()`
drops below 0.10; `TurnTracker` reads `THEMIS_TURNS_REMAINING_FRACTION` from the
environment (defaults to 1.0 when unset). Issues with a `depends on #N` body
pattern are skipped when issue N is still open; if the dependency check itself fails (network error,
API error), the issue is also skipped rather than aborting the loop. `--dry-run` prints the plan
without executing. After all issues are processed, a summary is written to stderr: non-zero blocked and skipped counts appear first on one line (`run summary: N blocked, N skipped (dependency), N skipped (turns)`), followed by `run summary: N processed` (always printed, even when zero). When the issue list is empty, `no ready-for-agent issues found` is printed instead.

### Issue Tracker Integration (`internal/tracker/`)

`Fetcher` interface with `Fetch(ctx context.Context, number int) (*IssueData, error)` as the seam between pipeline orchestration and the issue tracker. `IssueData` carries `Number`, `Title`, `Body`, `Labels`, `URL`, and `Ref` — the target branch specified on the issue (populated from Gitea's `ref` field or GitHub's `ref` field when present; empty string when absent). `GitHubFetcher` implements the interface using `gh issue view --json`; `GiteaFetcher` uses the Gitea REST API via `NewGiteaFetcher(owner, repo, apiBase, token string, timeout time.Duration)` — the caller must supply a non-zero timeout to prevent unbounded blocking under network partition. `ParseCheckboxes(body string) []string` extracts both checked and unchecked `- [ ]`/`- [x]` items from markdown. `NewFetcher(provider, owner, repo, apiBase, token string, timeout time.Duration) (Fetcher, error)` is the factory; `provider` must be `"github"` or `"gitea"`.

### Checkpoint Verification (`internal/checkpoint/`)

Three functions that the pipeline runner calls after each agent step:

- `NewStepCheckpoint(ctx context.Context, dir string) (func(context.Context, pipeline.Step, string) error, error)` — snapshots the current git HEAD, then returns a stateful checkpoint function. On each call the function verifies the working tree is clean, checks that required steps produced a new commit with the expected prefix (see table below), and advances the snapshot so the next call only sees commits from that step.
- `VerifyCommitPrefix(ctx context.Context, dir, prefix string) error` — confirms the last commit message starts with the expected conventional-commit prefix (e.g., `test(`, `feat(`). Delegates to `git.LastCommitMessage`.
- `VerifyCleanWorkingTree(ctx context.Context, dir string) error` — confirms no uncommitted changes remain. Delegates to `git.WorkingTreeClean`.

| Step | Required prefix | Commit required? |
|------|-----------------|-----------------|
| TestRed | `test(` | yes |
| Implement | `feat(` | yes |
| Refactor | `refactor(` | no (optional) |
| Fix | `fix(` | yes |
| Docs | `docs(` | no (optional) |

All git subprocess calls are delegated to `internal/git`. `NewStepCheckpoint` is wired into the production runner by `cmd/themis/main.go::newIssueConfig`.

### Pipeline Runner (`internal/runner/`)

`Run(ctx context.Context, cfg Config) (*Result, error)` is the main orchestration loop. `Config` accepts the work directory, issue number, a `tracker.Fetcher`, an `agent.Invoker`, an `IssueWriter` interface (`AddLabel`, `RemoveLabel`, `Comment`, `CreatePR`), a template directory, an optional `CheckpointFn`, optional `GitBranchFn`/`GitPushFn` for branch creation and push (both injectable — `nil` skips the operation; production implementations provided by `cmd/themis`), and a `CodeVersion string` embedded in fresh state and compared on resume. The runner loads or resumes `pipeline.PipelineState`; on resume it emits three safety warnings to stderr: (1) if the state file belongs to a different issue number, it resets to fresh; (2) if `CodeVersion` in the state differs from `cfg.CodeVersion`, it warns of a version mismatch; (3) after the Branch step, if the current branch name does not contain the issue number, it warns of a branch mismatch. After loading state, the runner advances through infrastructure steps (Fetch/Scan/Branch) without agent invocation, invokes agents for creative steps (TestRed through Docs) using `internal/prompt` substitution of per-step templates, enforces cycle limits (blocking the issue and commenting when limits are hit). After the Review step, blocking status is determined by reading `.themis/review-results.json` written by the review agent (instructed via `review.md`); findings with severity `critical`, `high`, or `medium` are blocking (`blockingThreshold = "medium"`), `low` findings are not. If the JSON is absent, the runner logs a warning and falls back to stdout-based detection. On a fresh start (no prior state, or prior state for a different issue), the runner deletes any stale `.themis/review-results.json` before the pipeline runs. On the Ship step runs a pre-push guard before any network I/O: if the current branch equals the base branch, `Run` returns an error (`"current branch is the base branch — no issue branch was created"`); if `git.CommitsAheadOfBase` reports zero commits, `Run` returns an error (`"no commits on branch <name> — nothing to ship"`). Both guards skip `CreatePR` entirely. When the guard passes, the runner pushes the branch, invokes the agent with the `ship.md` template to compose the PR body (using the `pr-composition` skill), and calls `IssueWriter.CreatePR` with the agent's stdout as the PR body. If the ship agent invocation fails, the runner falls back to `buildPRBody()` (a minimal "Closes #N" body) and logs a warning to stderr. `issue.Ref` is used as the PR base branch (falling back to `"main"` when empty). Template placeholders are filtered to only those actually present in the template before substitution, preventing spurious errors. Six placeholders are populated at runtime from pipeline state and git: `{{REVIEW_OUTPUT}}` carries the full stdout of the most recent Review step (runtime-only — not persisted to state.json; substituted as empty string when the runner resumes past Review without running it this session); `{{BLOCKING_FINDINGS}}` carries the blocking findings from `.themis/review-results.json` formatted as a human-readable severity-ordered list (critical → high → medium, low omitted) for substitution into `fix-findings.md`; `{{REVIEW_CYCLE}}` is the current review cycle number (1-indexed) used in `fix-findings.md`; `{{PIPELINE_SHAPE}}` is a comma-separated summary of distinct conventional-commit type prefixes found in branch commits (e.g. `test, feat, refactor`); `{{COMMIT_LOG}}` is the `git log --oneline` output for commits on the branch relative to the base; `{{AC_STATUS}}` renders the same acceptance-criteria checkbox list as `{{ACCEPTANCE_CRITERIA}}` but is the key used in `ship.md` (both are populated from the same source — agent-step templates use `ACCEPTANCE_CRITERIA`, the ship template uses `AC_STATUS`).

### Pipeline State Machine (`internal/pipeline/`)

Deterministic skeleton for pipeline orchestration. Defines the ten pipeline steps
(`Fetch → Scan → Branch → TestRed → Implement → Refactor → Review → Fix → Docs → Ship`)
and encodes all transition logic in Go:

- `pipeline.go` — `Step` type, `PipelineState` struct (fields include `IssueNumber`, `CurrentStep`, `ReviewCycle`, `MaxReviewCycles`, `TestFixAttempts`, `Commits`, `StartedAt`, `StepHistory`, and `CodeVersion` — the binary version string embedded at fresh-start for version-mismatch detection on resume), `Advance()` method, review-cycle gate and round-3 logic. No I/O dependencies (stdlib: `errors`, `fmt`, `time` only).
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
