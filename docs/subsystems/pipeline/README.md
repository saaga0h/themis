# Pipeline State Machine

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/pipeline/ -->

## Overview

`internal/pipeline` is a pure, deterministic state machine. It defines the ten steps of the Themis workflow, the `PipelineState` struct that tracks all mutable run state, and `Advance(StepResult) (Step, error)` — the single function that encodes every transition rule. There is no I/O in `pipeline.go`; all persistence is isolated in `store.go`.

## Key Files & Entry Points

| File | Role |
|------|------|
| `internal/pipeline/pipeline.go` | `Step` const, `PipelineState`, `Advance`, `linearNext`, `checkReviewCycleLimit` |
| `internal/pipeline/store.go` | `SaveState` / `LoadState` — JSON persistence to `.themis/state.json` |

## The Ten Steps

Steps are declared as `iota` constants (`Step int`) in the order below. `String()` returns the name at the matching index of a fixed `[...]string` array.

| # | Constant | String | Kind |
|---|----------|--------|------|
| 0 | `StepFetch` | `"Fetch"` | infrastructure |
| 1 | `StepScan` | `"Scan"` | infrastructure |
| 2 | `StepBranch` | `"Branch"` | infrastructure |
| 3 | `StepTestRed` | `"TestRed"` | agent |
| 4 | `StepImplement` | `"Implement"` | agent |
| 5 | `StepRefactor` | `"Refactor"` | agent |
| 6 | `StepReview` | `"Review"` | agent |
| 7 | `StepFix` | `"Fix"` | agent |
| 8 | `StepDocs` | `"Docs"` | agent |
| 9 | `StepShip` | `"Ship"` | infrastructure |

"Infrastructure" steps succeed or fail based on git/API operations; "agent" steps are driven by an LLM agent whose result is reported back via `StepResult`.

## PipelineState

All fields are exported and JSON-serialised.

| Field | Type | Description |
|-------|------|-------------|
| `IssueNumber` | `int` | GitHub issue being worked |
| `CurrentStep` | `Step` | Step about to execute (not yet recorded in history) |
| `ReviewCycle` | `int` | Number of completed Review→Fix iterations so far |
| `MaxReviewCycles` | `int` | Ceiling override; 0 means use the default of 2 |
| `TestFixAttempts` | `map[string]int` | Per-acceptance-criterion retry counter; key is `StepResult.TestACKey` |
| `Commits` | `[]string` | Commit SHAs recorded by the runner |
| `StartedAt` | `time.Time` | Wall-clock start of the run |
| `StepHistory` | `[]StepResult` | Ordered record of completed step results |
| `CodeVersion` | `string` | Opaque version tag set by the caller |

## Advance() Transition Logic

`Advance` is the only function that mutates `CurrentStep`. It has three cases, matched by `ps.CurrentStep`:

### Case 1 — StepTestRed

```
if !result.Success && result.TestACKey != "":
    if TestFixAttempts[key] >= 3  →  error (terminal)
    else                          →  TestFixAttempts[key]++, return StepTestRed
else (success or no key):
    recordStep, CurrentStep = StepImplement, return StepImplement
```

A failing test result with a non-empty `TestACKey` causes a retry of `StepTestRed` up to three times per key. On the fourth failure the call returns an error; `CurrentStep` is not updated. A successful result (or a failure without a key) advances linearly to `StepImplement`.

### Case 2 — StepReview

```
if result.BlockingFindings:
    checkReviewCycleLimit(result.Round3Trigger)  →  error if limit hit
    ReviewCycle++, recordStep, CurrentStep = StepFix, return StepFix
else:
    recordStep, CurrentStep = StepDocs, return StepDocs
```

A clean review (no blocking findings) advances to `StepDocs`. A review with blocking findings increments `ReviewCycle` and routes to `StepFix`, unless the cycle limit has been reached (see below). `StepFix` transitions back to `StepReview` via `linearNext`.

### Case 3 — All other steps (linear default)

`linearNext` encodes the default sequence:

```
Fetch → Scan → Branch → TestRed
Implement → Refactor → Review
Fix → Review
Docs → Ship
```

`StepTestRed`, `StepReview`, and `StepShip` are absent from `linearNext`. Passing any of them to `linearNext` returns `"no linear transition defined for step <name>"`. `StepShip` is the terminal step; calling `Advance` from it will hit `linearNext` and return that error.

## Review-Cycle Gate & Round-3 Trigger

`checkReviewCycleLimit` is called before every `ReviewCycle++` increment.

| Condition | Error returned |
|-----------|---------------|
| `ReviewCycle >= 3` | `"review cycle 3 exhausted: blocking findings remain after maximum cycles"` |
| `ReviewCycle >= maxReviewCycles() && trigger == TriggerNone` | `"review cycle limit <N> reached with blocking findings and no round-3 trigger"` |

`maxReviewCycles()` returns `ps.MaxReviewCycles` if non-zero, otherwise `2`.

**Default ceiling is 2.** After two Review→Fix iterations (`ReviewCycle == 2`), a third review with blocking findings will error — unless `result.Round3Trigger` is set to a non-`TriggerNone` value:

| Trigger constant | Meaning |
|-----------------|---------|
| `TriggerNone` (0) | No extension; cycle 2 is the last allowed |
| `TriggerSecurity` | Security-related finding warrants a third cycle |
| `TriggerPublicAPI` | Public API contract issue warrants a third cycle |
| `TriggerNumerical` | Numerical correctness issue warrants a third cycle |
| `TriggerContextArtifact` | Context-artifact concern warrants a third cycle |

Any non-`TriggerNone` value raises the effective ceiling from 2 to 3. The absolute hard stop at `ReviewCycle >= 3` cannot be extended by any trigger — cycle 3 is the unconditional maximum.

**Sequence example:**

| ReviewCycle before check | trigger | Outcome |
|--------------------------|---------|---------|
| 0 | any | allowed, increment to 1 |
| 1 | any | allowed, increment to 2 |
| 2 | `TriggerNone` | error: cycle limit 2 reached |
| 2 | `TriggerSecurity` (or any non-None) | allowed, increment to 3 |
| 3 | any | error: cycle 3 exhausted (hard stop) |

## Test-Fix Attempt Limit

`TestFixAttempts` is a `map[string]int` keyed by `StepResult.TestACKey`. The key identifies a specific acceptance criterion (AC); the value is the number of retries consumed for that criterion.

The check is `>= 3` before incrementing, meaning the map value ranges 1–3 on successful retries and the error fires when the value would reach 4 (i.e. after three failed attempts). Each AC key has an independent counter; failing one criterion does not affect the counter for another.

## Persistence

`store.go` provides two functions with no other dependencies:

| Function | Signature | Behaviour |
|----------|-----------|-----------|
| `SaveState` | `SaveState(dir string, state *PipelineState) error` | Marshals `PipelineState` to indented JSON, writes to `<dir>/.themis/state.json`. Creates `.themis/` with mode `0700` if absent. File written with mode `0600`. |
| `LoadState` | `LoadState(dir string) (*PipelineState, error)` | Reads and unmarshals `<dir>/.themis/state.json`. Returns `nil, nil` if the file does not exist (fresh start). |

The state file path is the package-level constant `stateFile = ".themis/state.json"`.

## How Do I Add a New Step?

1. Add a new `Step` constant in the `iota` block in `pipeline.go`. Insert it at the position that reflects its place in the sequence — all subsequent constants shift by one.
2. Extend the `names` array inside `String()` to include the new step's display name at the matching index position.
3. Add a `case` to `linearNext` mapping the predecessor step to the new step, and the new step to its successor. If the new step has non-linear transition logic (like `StepTestRed` or `StepReview`), add a dedicated `case` in `Advance` instead and omit it from `linearNext`.
4. If the new step needs a checkpoint prefix, update `internal/checkpoint/step_checkpoint.go` (see `docs/subsystems/checkpoint/README.md`).

## How Do I Diagnose Problems?

Inspect `.themis/state.json` in the working directory of the affected run. The most useful fields:

| Field | What it tells you |
|-------|------------------|
| `CurrentStep` | The step that was about to execute when the run stopped |
| `ReviewCycle` | How many Review→Fix loops have completed |
| `TestFixAttempts` | Map of AC key → retry count; a value of 3 is the last allowed attempt |
| `StepHistory` | Ordered list of `StepResult` values for every completed step |

The error returned by `Advance` is the canonical failure message. It is not wrapped; read it verbatim to identify which threshold was hit.

## What Is the Failure Behavior?

`Advance` returns an error and leaves `CurrentStep` unchanged when a limit is exceeded. It never panics. There are three terminal error conditions:

| Error message (verbatim) | Trigger |
|--------------------------|---------|
| `test-fix attempts for "<key>" exceeded maximum of 3` | `TestFixAttempts[key]` was already 3 when `Advance` was called with `!result.Success && result.TestACKey == key` |
| `review cycle 3 exhausted: blocking findings remain after maximum cycles` | `ReviewCycle >= 3` at the start of a blocking-findings review |
| `review cycle limit <N> reached with blocking findings and no round-3 trigger` | `ReviewCycle >= maxReviewCycles()` and `trigger == TriggerNone` |

A fourth case, `no linear transition defined for step <name>`, is a programming error — it fires if `linearNext` is called for a step that should be handled by a dedicated `Advance` case.

## Architecture

`pipeline.go` is a pure computation unit. It has no init side-effects, no goroutines, and no external calls. The runner (see `docs/subsystems/runner/`) is responsible for:

- constructing `PipelineState` (or loading it via `LoadState`)
- calling `Advance` after each step
- persisting state via `SaveState` after each transition

This separation means the transition logic can be unit-tested without any filesystem or network access.

## Dependencies

`pipeline.go` imports only the Go standard library:

- `errors` — sentinel error construction in `checkReviewCycleLimit`
- `fmt` — formatted error strings and `Step.String()`
- `time` — `time.Time` field on `PipelineState`

`store.go` additionally imports `encoding/json`, `os`, and `path/filepath`.

No third-party dependencies.

## Related Documents

- `ARCHITECTURE.md` — system-level design and subsystem relationships
- `docs/subsystems/runner/README.md` — how the runner drives `Advance` and calls `SaveState`
- `docs/subsystems/checkpoint/README.md` — per-step commit verification keyed on `Step` constants defined here
