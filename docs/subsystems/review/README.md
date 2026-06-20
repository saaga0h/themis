# Review

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/review/ -->

## Overview

`internal/review` is a domain type package that owns the structured output of the Review pipeline step. It defines the types written by the review agent to `.themis/review-results.json`, the exported `BlockingThreshold` constant, and pure analysis functions that operate on `ReviewFinding` slices. The single filesystem I/O function, `ReadReviewResults`, lives here but is injected into the runner via `Config.ReviewResultsLoader` to keep the runner's filesystem coupling explicit and testable.

## Types

### `ReviewFinding`

```go
type ReviewFinding struct {
    Severity    string `json:"severity"`
    Description string `json:"description"`
    File        string `json:"file,omitempty"`
    Line        int    `json:"line,omitempty"`
}
```

One entry in `.themis/review-results.json`. `Severity` is one of `"critical"`, `"high"`, `"medium"`, `"low"`. `File` and `Line` are optional location annotations.

### `ReviewResults`

```go
type ReviewResults struct {
    Findings []ReviewFinding `json:"findings"`
}
```

Top-level shape of `.themis/review-results.json`. The review agent writes this file; `ReadReviewResults` reads it.

## Constants

```go
const BlockingThreshold = "medium"
```

Findings with severity `"critical"`, `"high"`, or `"medium"` are blocking. `"low"` is not. This is the single source of truth used by `CountFindingsBySeverity`, `DetermineBlockingStatus`, and `FormatBlockingFindings`.

## Functions

### Pure analysis (no I/O)

| Function | Signature | Purpose |
|----------|-----------|---------|
| `CountFindingsBySeverity` | `(findings []ReviewFinding) (blocking, nonBlocking int)` | Counts findings at or above `BlockingThreshold` vs. below |
| `DetermineBlockingStatus` | `(findings []ReviewFinding) bool` | Returns `true` when any blocking finding exists |
| `FormatBlockingFindings` | `(findings []ReviewFinding) string` | Human-readable list ordered `critical → high → medium`; `low` omitted. Format: `- SEVERITY: description (file:line)\n` |

### I/O

| Function | Signature | Purpose |
|----------|-----------|---------|
| `ReadReviewResults` | `(ctx context.Context, workDir string) ([]ReviewFinding, bool)` | Reads `.themis/review-results.json`; returns `(nil, false)` on missing file or parse error |

`ReadReviewResults` accepts `context.Context` per convention; `os.ReadFile` has no context-aware variant so the context is not used. It is the only filesystem-touching function in the package; all others are pure.

## Dependency injection

`ReadReviewResults` is injected into `runner.Config` as `ReviewResultsLoader`:

```go
// In runner.Config:
ReviewResultsLoader func(ctx context.Context, workDir string) ([]review.ReviewFinding, bool)
```

The runner normalizes a `nil` `ReviewResultsLoader` to `review.ReadReviewResults` at startup, so callers (including tests) that omit the field still get the default behavior. Tests substitute a stub that controls which findings are returned without touching the filesystem.

## Dependency direction

`internal/review` is a leaf package — it imports only stdlib (`context`, `encoding/json`, `fmt`, `os`, `path/filepath`, `strings`). No other internal package imports it except `internal/runner`, which treats it as a domain type package.

## Related Documents

- `docs/datamodel.md` — `.themis/review-results.json` schema, lifecycle, and severity mapping
- `docs/subsystems/runner/README.md` — how `ReviewResultsLoader` is normalized and used in the Review step
- `ARCHITECTURE.md` — system-level design and subsystem map
