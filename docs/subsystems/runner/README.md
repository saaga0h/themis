# Runner

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/runner/ -->

## Overview

`internal/runner` is the pipeline orchestrator. `Run(ctx, Config) (*Result, error)` drives a single issue from fetch through PR creation. It owns state load/resume, step dispatch, agent invocation, checkpoint enforcement, review-cycle accounting, and Ship guards.

Infrastructure steps (Fetch, Scan, Branch) execute without an agent. Agent steps (TestRed → Implement → Refactor → Review → Fix → Docs) load a Markdown template, substitute runtime placeholders via `internal/prompt`, invoke the agent, run the checkpoint, and advance `PipelineState`. Ship is a hybrid: it runs Ship guards, optionally invokes an agent to write the PR body, then calls `IssueWriter.CreatePR`.

## Key Files

| File | Role |
|------|------|
| `internal/runner/runner.go` | `Run`, `Config`, `Result`, all step handlers, template helpers |
| `internal/pipeline/pipeline.go` | `PipelineState.Advance` — step transition and cycle-limit logic |
| `internal/checkpoint/step_checkpoint.go` | `NewStepCheckpoint` — `CheckpointFn` factory called after each agent step |

## Architecture

### Config Fields

| Field | Type | Purpose |
|-------|------|---------|
| `WorkDir` | `string` | Repository root; all file I/O is relative to this |
| `IssueNumber` | `int` | Gitea issue number driving this run |
| `Fetcher` | `tracker.Fetcher` | Reads issue title, body, labels, `Ref` |
| `Invoker` | `agent.Invoker` | Executes an agent turn sequence |
| `IssueWriter` | `IssueWriter` | `AddLabel`, `RemoveLabel`, `Comment`, `CreatePR` |
| `TemplateDir` | `string` | Directory containing `*.md` prompt templates |
| `CheckpointFn` | `func(ctx, Step, workDir) error` | Called after every agent step; nil skips checkpoint |
| `TestACKey` | `string` | Acceptance-criteria key for TestRed retry tracking; defaults to `"tests"` |
| `Git` | `GitOps` | Git operations interface (CheckoutNewBranch, Checkout, PushBranch, CurrentBranch, BranchCommitLog, CommitsAheadOfBase, ChangedFiles); nil skips all git operations |
| `ProfileLoader` | `func(dir string) (ProfileData, error)` | Loads per-project profile settings; nil uses zero-value `ProfileData` defaults |
| `ReviewResultsLoader` | `func(ctx context.Context, workDir string) ([]review.ReviewFinding, bool)` | Reads `.themis/review-results.json`; nil defaults to `review.ReadReviewResults` |
| `Logger` | `io.Writer` | Step log sink; defaults to `os.Stderr` |
| `CodeVersion` | `string` | Binary version; compared against `state.CodeVersion` on resume |
| `MaxTurns` | `int` | Per-agent turn limit; `DefaultMaxTurns = 250` when unset |

### GitOps Interface

Defined in `internal/runner/runner.go` at the point of use (consumer-side). Concrete implementations live in `cmd/themis/`; tests substitute `fakeGitOps` stubs.

```go
type GitOps interface {
    CheckoutNewBranch(ctx context.Context, dir, name string) error
    Checkout(ctx context.Context, dir, name string) error
    PushBranch(ctx context.Context, dir, branch string) error
    CurrentBranch(ctx context.Context, dir string) (string, error)
    BranchCommitLog(ctx context.Context, dir string) string
    CommitsAheadOfBase(ctx context.Context, dir, base string) (int, error)
    ChangedFiles(ctx context.Context, dir string) string
}
```

### ProfileData

`ProfileData` is the subset of profile settings the runner needs. It is populated by calling `Config.ProfileLoader`; the runner does not import `internal/profile` directly.

```go
type ProfileData struct {
    ImplementModel string
    ReviewModel    string
}
```

### Result Type

```go
type Result struct {
    PRURL string
}
```

`Run` returns a non-nil `*Result` only on successful PR creation. All other exits return an error.

### Step Sequence

```
Fetch → Scan → Branch → TestRed → Implement → Refactor → Review → Fix* → Docs → Ship
                                                               ↑___________|
```

`Fix` loops back to `Review`. The pipeline advances via `PipelineState.Advance`; the runner does not hard-code transitions — they are owned by `internal/pipeline`.

### Pipeline Loop (step dispatch)

Each iteration reads `state.CurrentStep` and branches:

1. **Fetch** — auto-advances (best-effort pre-run sync is handled by the caller before `Run` is invoked).
2. **Scan** — auto-advances (no-op placeholder for future scanner integration).
3. **Branch** — calls `cfg.Git.CheckoutNewBranch` with branch name `issue/<N>-<slug>` (skipped when `cfg.Git` is nil). On a fresh start, seeds `.themis/review-results.json` with `{"findings":[]}` so the Review step starts non-blocking. Advances.
4. **Agent steps** (`agentSteps` map: TestRed, Implement, Refactor, Review, Fix, Docs) — template load → substitute → `Invoker.Invoke` → `CheckpointFn` → `deriveStepResult` → `state.Advance`.
5. **Ship** — see Ship Step & Guards below.
6. **Unknown steps** — auto-advance with a success result.

### `modelForStep`

| Step | Model source |
|------|-------------|
| `Review` | `prof.Review.Agents.Security` |
| All others | `prof.Implement.Model` |

## Blocking & Review Cycles

### Blocking determination

After the Review agent runs, the runner calls `cfg.ReviewResultsLoader(ctx, workDir)` (defaulting to `review.ReadReviewResults`) to parse `.themis/review-results.json`.

- **File absent** → treated as blocking (fail-safe). Warning logged: `"review-results.json not found after review step — treating as blocking"`. `deriveStepResult` returns `StepResult{Success: false, BlockingFindings: true}`.
- **File present** → `review.CountFindingsBySeverity` counts findings at or above `review.BlockingThreshold` (`"medium"`). `low` and any other severity are non-blocking.

`lastBlockingFindings` is updated only when `stepResult.BlockingFindings` is true; it carries the `review.FormatBlockingFindings` output (critical → high → medium list) into subsequent template substitutions as `{{BLOCKING_FINDINGS}}`.

### Review cycle limit and "continue to ship"

When `state.Advance` returns an error containing `"review cycle"`:

- If `state.ReviewCycle > initialReviewCycle` (cycle advanced during this run): the runner logs the error, sets `state.CurrentStep = pipeline.StepDocs`, saves state, and continues. The issue proceeds to Docs → Ship so the human can decide via the PR.
- If `state.ReviewCycle == initialReviewCycle` (resumed into an already-exhausted state): falls through to `blockIssue`.

### `blockIssue`

On any other `state.Advance` error: posts a comment `"Pipeline blocked on issue #N: <error>"` and adds the `blocked` label via `IssueWriter`. The runner then returns the original `advErr`.

## Ship Step & Guards

Guards execute before any network I/O:

| Condition | Error returned |
|-----------|---------------|
| `branch == base` | `"current branch is the base branch — no issue branch was created"` |
| `git.CommitsAheadOfBase == 0` | `"no commits on branch <branch> — nothing to ship"` |

Both guards skip `CreatePR` entirely.

`base` is `issue.Ref`; if `issue.Ref` is empty, defaults to `"main"`.

**PR body fallback:** The runner invokes the agent with `ship.md` to compose the PR body. If the ship template is missing, the agent invocation fails, or the agent returns empty stdout, the runner falls back to `buildPRBody()` — a minimal body containing `Closes #N`, the issue title, and the acceptance-criteria checklist. The fallback is logged as a warning; it does not fail the run.

Agent stdout is passed through `stripCodeFences` before use, which removes leading triple-backtick fences that Claude Code's `--print` mode can emit.

On PR creation success, `.themis/review-results.json` is deleted.

## Template Args

`buildTemplateArgs` constructs a map of all possible placeholders. `filterArgs` then scans the template with `{{([A-Z0-9_]+)}}` and retains only keys that appear in that specific template, so unused substitutions never reach `prompt.Substitute`.

| Placeholder | Source |
|-------------|--------|
| `{{ISSUE_NUMBER}}` | `cfg.IssueNumber` |
| `{{ISSUE_TITLE}}` | `issue.Title` |
| `{{ACCEPTANCE_CRITERIA}}` | `tracker.ParseCheckboxes(issue.Body)` formatted as `- [ ] …` lines |
| `{{AC_STATUS}}` | Same value as `ACCEPTANCE_CRITERIA`; `ship.md` uses this key, agent-step templates use `ACCEPTANCE_CRITERIA` |
| `{{CODING_STANDARDS}}` | `CODING_STANDARDS.md` at `WorkDir`; empty string if absent |
| `{{UBIQUITOUS_LANGUAGE}}` | `UBIQUITOUS_LANGUAGE.md` at `WorkDir`; empty string if absent |
| `{{BRANCH_NAME}}` | `cfg.Git.CurrentBranch`; falls back to `"main"` when `cfg.Git` is nil or on error |
| `{{CHANGED_FILES}}` | `cfg.Git.ChangedFiles`; empty string when `cfg.Git` is nil |
| `{{REVIEW_CYCLE}}` | `state.ReviewCycle + 1` (1-based for templates) |
| `{{BLOCKING_FINDINGS}}` | Formatted critical/high/medium findings from last Review; empty on first cycle |
| `{{REVIEW_OUTPUT}}` | `invokeResult.Stdout` from last Review agent invocation |
| `{{PIPELINE_SHAPE}}` | Deduplicated conventional-commit prefixes from `cfg.Git.BranchCommitLog` (e.g. `feat, fix, test`); empty when `cfg.Git` is nil |
| `{{COMMIT_LOG}}` | Full `cfg.Git.BranchCommitLog` output; empty when `cfg.Git` is nil |

## How Do I Add / Diagnose / Failure Behavior

### How do I add a new template placeholder?

1. Add the key-value pair to the `return` map in `buildTemplateArgs` (`runner.go`).
2. Reference `{{YOUR_KEY}}` in one or more `*.md` templates under `TemplateDir`.
3. No further wiring needed. `filterArgs` automatically limits substitution to keys present in each template, so the new placeholder is silently ignored by templates that do not reference it.

To add a new `Config` dependency (e.g. a new service client): add the field to `Config`, thread it through the caller in `cmd/themis/main.go::newIssueConfig`, and use it inside the relevant step handler in `Run`.

### How do I diagnose problems?

1. **Read the step logs on stderr.** Each step emits `<Step>: start` and `<Step>: done (<ms>ms)`. A missing `done` line identifies the failing step. Agent steps additionally log `invoking agent model=… maxTurns=…` and `agent result: N commits, completed|not completed`.
2. **Inspect `.themis/state.json`.** `CurrentStep` shows where the pipeline stopped. `ReviewCycle` and `TestFixAttempts` reveal how many retries have been consumed.
3. **Inspect `.themis/review-results.json`.** If absent after a Review step, the runner treated it as blocking. If present, check the `findings` array for severity values at or above `medium`.

### What is the failure behavior?

| Scenario | Behavior |
|----------|----------|
| Git fetch fails (caller-side, before `Run`) | Warning logged by caller; pipeline continues from Fetch step which auto-advances |
| Template file missing | `Run` returns error immediately |
| `Invoker.Invoke` fails (agent step) | `Run` returns error immediately; no `blockIssue` |
| Checkpoint fails after agent step | `Run` returns error immediately; no `blockIssue` |
| `review-results.json` absent after Review | Treated as blocking (fail-safe); warn logged |
| `state.Advance` returns review-cycle error, cycle advanced this run | Runner skips to `StepDocs` and continues to Ship; human reviews via PR |
| `state.Advance` returns review-cycle error, already-exhausted state resumed | `blockIssue` (comment + `blocked` label), then return error |
| `state.Advance` returns any other error | `blockIssue`, then return error |
| Ship guard: current branch == base | Return error before any network I/O; no PR created |
| Ship guard: zero commits ahead of base | Return error before any network I/O; no PR created |
| Ship agent invocation fails | Warning logged; `buildPRBody()` minimal body used; PR still created |
| `IssueWriter.CreatePR` fails | `Run` returns error |
| Issue-number mismatch on resume | State discarded; fresh start |
| `CodeVersion` mismatch on resume | Warning logged; run continues with loaded state |
| Post-Branch branch name lacks issue number | Warning logged; run continues |

## Dependencies

| Dependency | Role |
|------------|------|
| `internal/pipeline` | `PipelineState`, `Step`, `StepResult`, `Advance` |
| `internal/agent` | `Invoker`, `InvokeOptions`, `InvokeResult` |
| `internal/prompt` | `Substitute` — placeholder substitution in templates |
| `internal/tracker` | `Fetcher`, `IssueData`, `ParseCheckboxes` |
| `internal/review` | `ReviewFinding`, `BlockingThreshold`, `CountFindingsBySeverity`, `DetermineBlockingStatus`, `FormatBlockingFindings`; `ReadReviewResults` is the default for `Config.ReviewResultsLoader` |
| `internal/checkpoint` | Provides `CheckpointFn` via `NewStepCheckpoint` (wired externally) |

`internal/git` and `internal/profile` are **not** imported directly. Git operations are injected via the `GitOps` interface (`Config.Git`); profile settings are injected via `Config.ProfileLoader`; review-results filesystem I/O is injected via `Config.ReviewResultsLoader`. Concrete implementations are provided by `cmd/themis/`.

## Related Documents

- `ARCHITECTURE.md` — system-level design and subsystem map
- `docs/subsystems/pipeline/` — `PipelineState.Advance`, step transitions, cycle-limit logic
- `docs/subsystems/checkpoint/README.md` — `CheckpointFn` contract and step-prefix table
- `docs/subsystems/tracker/` — `Fetcher`, `IssueData`, `ParseCheckboxes`
