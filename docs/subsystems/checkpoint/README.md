# Checkpoint Verification

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/checkpoint/ -->

## Overview

`internal/checkpoint` verifies that each pipeline step left the repository in the expected state — clean working tree and a new commit with the correct conventional-commit prefix.

## Key Files

| File | Role |
|------|------|
| `internal/checkpoint/checkpoint.go` | `VerifyCommitPrefix` and `VerifyCleanWorkingTree` |
| `internal/checkpoint/step_checkpoint.go` | `NewStepCheckpoint` — stateful per-step checkpoint factory |

## Public API

### `NewStepCheckpoint`

```go
func NewStepCheckpoint(ctx context.Context, dir string) (func(context.Context, pipeline.Step, string) error, error)
```

Snapshots the current git HEAD at construction time, then returns a closure that enforces the following contract on every call:

1. Working tree must be clean (delegates to `VerifyCleanWorkingTree`).
2. If the step requires a commit (see table below) and no new commit was found, returns an error.
3. If a new commit was found and the step has an expected prefix, verifies the last commit message starts with that prefix (delegates to `VerifyCommitPrefix`).
4. Advances the internal snapshot so the next call only sees commits produced during that step.

**Step → prefix mapping:**

| Step | Required prefix | Commit required? |
|------|-----------------|-----------------|
| TestRed | `test(` | yes |
| Implement | `feat(` | yes |
| Refactor | `refactor(` | no (optional) |
| Fix | `fix(` | yes |
| Docs | `docs(` | no (optional) |

### `VerifyCommitPrefix`

```go
func VerifyCommitPrefix(ctx context.Context, dir, prefix string) error
```

Reads the last commit message via `git.LastCommitMessage` and returns an error if it does not start with `prefix`.

### `VerifyCleanWorkingTree`

```go
func VerifyCleanWorkingTree(ctx context.Context, dir string) error
```

Checks `git.WorkingTreeClean` and returns an error if any uncommitted changes remain.

## Wiring

`NewStepCheckpoint` is instantiated by `cmd/themis/main.go::newIssueConfig` and passed as `runner.Config.CheckpointFn`. The runner calls it after every agent step.

## Dependencies

- `internal/git` — all git subprocess calls
- `internal/pipeline` — `Step` type for the prefix/optional-commit tables

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/runner/README.md` — how `CheckpointFn` is called by the runner
