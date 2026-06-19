# Agent

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/agent/ -->

## Overview

`internal/agent` is the seam between deterministic pipeline control and LLM creative work. The pipeline side calls `Invoker.Invoke` with structured options; the agent side spawns an LLM subprocess, feeds it a prompt, and returns a structured result. Nothing in this package decides *what* to ask the agent — that is the caller's responsibility. This package decides *how* to spawn it, tag it for observability, and interpret what came back.

## Invoker Interface

```go
type Invoker interface {
    Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error)
}
```

`Invoker` is the only boundary the rest of the codebase crosses into this subsystem. Callers depend on the interface, not on `ClaudeCodeInvoker` directly.

## InvokeOptions

| Field | Type | Description |
|---|---|---|
| `Prompt` | `string` | Full prompt text fed to the agent via stdin |
| `Model` | `string` | Model identifier passed as `--model MODEL` |
| `MaxTurns` | `int` | Maximum agent turns passed as `--max-turns N` |
| `WorkDir` | `string` | Working directory for the subprocess and git snapshot |
| `AllowedTools` | `[]string` | Reserved. Declared on the struct but not currently read by `buildArgs` — no `--allowedTools` flag is emitted. No caller sets it. |
| `IssueNumber` | `int` | Issue number injected into `OTEL_RESOURCE_ATTRIBUTES` |
| `PipelineStep` | `string` | Pipeline step name injected into `OTEL_RESOURCE_ATTRIBUTES` |

## InvokeResult

| Field | Type | Description |
|---|---|---|
| `ExitCode` | `int` | Always `0` on success; error path returns `nil` result |
| `Stdout` | `string` | Raw captured stdout from the `claude` subprocess |
| `CommitsMade` | `[]string` | Commit SHAs made during the invocation (see Commit Detection) |
| `TestsPassed` | `bool` | `true` if stdout contains `"PASS"` or `"ok "` |
| `Completed` | `bool` | `true` if stdout contains any completion marker (see below) |

## ClaudeCodeInvoker

`ClaudeCodeInvoker` is the production implementation of `Invoker`. It spawns the `claude` CLI binary that must be on `PATH`.

Exact command line built by `buildArgs`:

```
claude --print --verbose --dangerously-skip-permissions --max-turns N --model MODEL
```

The prompt is written to the subprocess's stdin via `strings.NewReader(opts.Prompt)`. Stdout is captured into a `bytes.Buffer`; stderr is not captured (it inherits the parent process's stderr).

### Completed detection

`Completed` is set by scanning the full stdout (case-insensitively) for any of these markers:

- `"COMPLETED"`
- `"Task complete"`
- `"All ACs pass"`
- `"Implementation complete"`

If none of these strings appear in the output, `Completed` is `false` even if the process exits cleanly.

### TestsPassed detection

`TestsPassed` is set by scanning stdout for the literal substrings `"PASS"` or `"ok "` (case-sensitive). This matches standard Go test output; it does not parse structured test results.

## OTEL Tagging

Before spawning the subprocess, `buildCmdEnv` constructs the child environment by copying the parent environment and setting `OTEL_RESOURCE_ATTRIBUTES`. The value format is:

```
issue.number=<IssueNumber>,pipeline.step=<PipelineStep>[,<existing attrs>]
```

If `OTEL_RESOURCE_ATTRIBUTES` already exists in the parent environment, its value is preserved and appended after the new attributes. If it does not exist, the variable is appended to the environment. `buildCmdEnv` never sets `OTEL_EXPORTER_*` or `OTEL_TRACES_EXPORTER`.

This means every `claude` invocation is tagged with the issue number and pipeline step name in telemetry, regardless of what the calling code does.

## Commit Detection

Commit detection uses `internal/git` to snapshot the repository state before and after the invocation:

1. `git.CommitsBefore(ctx, workDir)` is called before spawning `claude`. If this fails, `Invoke` returns an error immediately — the subprocess is never started.
2. After `cmd.Run()` returns successfully, `git.CommitsAfter(ctx, workDir, before)` computes the set of new commits.
3. The resulting `[]string` (commit SHAs) is stored in `InvokeResult.CommitsMade`.

If `CommitsAfter` fails, `Invoke` returns an error even though the subprocess succeeded.

## Failure Behavior

| Scenario | Behavior |
|---|---|
| Context already cancelled on entry | Returns `ctx.Err()` immediately; subprocess never spawned |
| `git.CommitsBefore` fails | Returns wrapped error; subprocess never spawned |
| `claude` not found on PATH | `exec.CommandContext` call succeeds but `cmd.Run()` returns an `exec.ErrNotFound`-wrapped error; `Invoke` returns `fmt.Errorf("claude exited with error: %w", err)` |
| `claude` exits non-zero | `cmd.Run()` returns error; `Invoke` returns `fmt.Errorf("claude exited with error: %w", err)` |
| Context cancelled during run | `Invoke` returns `fmt.Errorf("agent killed by context: %w", ctx.Err())` |
| `git.CommitsAfter` fails | Returns wrapped error; `InvokeResult` is discarded |
| All clean | Returns `*InvokeResult` with `ExitCode: 0` |

On any error path, `Invoke` returns `(nil, error)` — callers must always check the error before dereferencing the result.

## Testing

The test file defines `fakeInvoker`, a minimal implementation of `Invoker` used by callers that need to unit-test pipeline logic without spawning a real subprocess:

```go
type fakeInvoker struct {
    result *InvokeResult
    err    error
}

func (f *fakeInvoker) Invoke(_ context.Context, _ InvokeOptions) (*InvokeResult, error) {
    return f.result, f.err
}
```

To test code that depends on `Invoker`, inject a `*fakeInvoker` with a pre-configured `result` or `err`. The `fakeInvoker` is defined in `agent_test.go` (package `agent`); callers in other packages must define their own stub or use a test double that satisfies the `Invoker` interface.

## How do I add a new Invoker implementation?

Implement the `Invoker` interface:

```go
type Invoker interface {
    Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error)
}
```

Return a `*InvokeResult` on success and `nil, error` on failure. There are no required constructor patterns — `ClaudeCodeInvoker` is a zero-value struct. Register the implementation where the invoker is wired in (caller's initialization, not in this package). For unit tests, use `fakeInvoker` as the model for a minimal stub.

## How do I diagnose problems?

| Symptom | Check | Fix |
|---|---|---|
| `claude exited with error: exec: "claude" executable file not found in $PATH` | `which claude` in the environment where the process runs | Install `claude` CLI and ensure it is on `PATH` |
| `Completed: false` despite apparent success | Inspect raw `Stdout`; none of the four completion markers were present | Have the agent output one of the recognized markers, or add a new marker to `completionMarkers` in `agent.go` |
| `TestsPassed: false` | Inspect `Stdout` for `"PASS"` or `"ok "` | Confirm the agent runs tests and their output reaches stdout |
| Wrong model or too few turns | Check `InvokeOptions.Model` and `InvokeOptions.MaxTurns` at the call site | Pass the correct values; they are forwarded verbatim to `--model` and `--max-turns` |
| OTEL attributes missing in telemetry | Confirm `IssueNumber` and `PipelineStep` are set on `InvokeOptions` | Non-zero/non-empty values are required; zero/empty are passed through and will produce `issue.number=0,pipeline.step=` |
| Agent killed unexpectedly | Check for context deadline/cancellation upstream | Look for `"agent killed by context"` in the error; extend the deadline or check what cancels the context |

## Dependencies

| Package | Role |
|---|---|
| `internal/git` | `CommitsBefore` / `CommitsAfter` for commit detection snapshot |
| `os/exec` | Spawns the `claude` subprocess via `exec.CommandContext` |
| `os` | Reads the parent environment via `os.Environ()` |

## Related Documents

- [ARCHITECTURE.md](/Users/saaga.helin/projects/themis/ARCHITECTURE.md) — system-level context for where the agent subsystem sits
- [docs/subsystems/runner/](../runner/) — the runner subsystem that calls `Invoker.Invoke` as part of pipeline execution
- [CONCEPTS.md](/Users/saaga.helin/projects/themis/CONCEPTS.md) — §2 "Deterministic Shell, Creative Core" explains the `Invoker` seam this subsystem implements
