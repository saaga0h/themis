# Issue Tracker Abstraction

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/tracker/ -->

## Overview

`internal/tracker` provides a provider-agnostic interface for fetching issue data from GitHub or Gitea. It defines the `Fetcher` interface, two concrete implementations, supporting types for list-issues API shapes, and helpers for parsing issue bodies: `ParseCheckboxes` for acceptance criteria, `ParseCheckBlocks` for issue-declared verification commands, `IsDestructiveAC` for identifying destructive criteria, and `ValidateDestructiveChecks` for enforcing the pairing rule.

The package is consumed by `cmd/themis` (wiring, issue validation at fetch time) and `internal/runner` (issue fetch at pipeline start).

## Key Files & Entry Points

| File | Role |
|------|------|
| `internal/tracker/tracker.go` | All types, implementations, and helpers — the entire package |

## IssueData

`IssueData` is the normalized issue representation returned by every `Fetcher`.

| Field | Type | Source |
|-------|------|--------|
| `Number` | `int` | Issue number |
| `Title` | `string` | Issue title |
| `Body` | `string` | Issue body (markdown) |
| `Labels` | `[]string` | Label names (extracted from provider-specific label objects) |
| `URL` | `string` | Web URL of the issue |
| `Ref` | `string` | Target branch from the provider's `ref` field; empty string when the field is absent from the API response |

`Ref` maps to Gitea's `ref` JSON field and GitHub's `ref` JSON field (passed through `ParseGitHubJSON`). Both providers set it to empty string when the field is not present in the response — this is tested explicitly in `TestGiteaFetcher_RefIsEmptyWhenAbsentFromAPI` and `TestParseGitHubJSON_RefIsEmptyWhenAbsent`.

## Fetcher Interface

```go
type Fetcher interface {
    Fetch(ctx context.Context, number int) (*IssueData, error)
}
```

Retrieves a single issue by number. The caller is responsible for cancellation via `ctx`. Errors from `Fetch` propagate upward wrapped with issue context (e.g., `fetching issue #N`).

## GitHubFetcher

`GitHubFetcher` invokes the `gh` CLI:

```
gh issue view <number> --json number,title,body,labels,state,url
```

- Requires the `gh` binary to be installed and authenticated.
- Does not accept configuration parameters — all GitHub context comes from the repository's git remote as resolved by `gh`.
- JSON is parsed by the exported `ParseGitHubJSON(data []byte) (*IssueData, error)` function, which is independently testable.
- The `gh` command does not include `ref` in the `--json` field list; `Ref` will therefore be empty for GitHub issues in the current implementation.

## GiteaFetcher

`GiteaFetcher` calls the Gitea REST API:

```
GET {apiBase}/api/v1/repos/{owner}/{repo}/issues/{number}
Authorization: token {token}
```

### Construction

```go
func NewGiteaFetcher(owner, repo, apiBase, token string, timeout time.Duration) *GiteaFetcher
```

- `apiBase` trailing slashes are stripped.
- `token` is optional; when empty, no `Authorization` header is sent.
- `timeout` is passed directly to `http.Client{Timeout: timeout}`. There is no guard against a zero value inside the package — **a zero timeout means no HTTP-level timeout** and risks unbounded blocking under network partition. Passing a non-zero timeout is a caller obligation. `cmd/themis` enforces this with a package-level constant `giteaClientTimeout = 30 * time.Second`.

### Failure behavior

| Condition | Error returned |
|-----------|---------------|
| Request construction fails | `"building request: ..."` |
| Network/transport error or timeout | `"GET {url}: ..."` |
| Non-200 HTTP status | `"Gitea API returned {status}: {body}"` |
| Malformed JSON response | `"decoding Gitea response: ..."` |

## NewFetcher Factory

```go
func NewFetcher(provider, owner, repo, apiBase, token string, timeout time.Duration) (Fetcher, error)
```

| `provider` | Returns | Notes |
|------------|---------|-------|
| `"github"` | `*GitHubFetcher` | `owner`, `repo`, `apiBase`, `token`, `timeout` are ignored |
| `"gitea"` | `*GiteaFetcher` | All parameters forwarded to `NewGiteaFetcher` |
| anything else | `nil, error` | `unknown provider "X" (must be github or gitea)` |

`NewFetcher` itself never returns a non-nil error for a known provider; the error path is solely the unknown-provider case.

## ParseCheckboxes

```go
func ParseCheckboxes(body string) []string
```

Extracts the text of all markdown checkbox items from `body` using the pattern `^- \[[ xX]\] (.+)$` (multiline).

- Returns both unchecked (`- [ ]`) and checked (`- [x]`, `- [X]`) items.
- Ignores checkbox lines inside fenced code blocks (``` or ~~~); only top-level checkboxes are extracted.
- Text is trimmed of leading/trailing whitespace.
- Returns `nil` (not an empty slice) when no matches are found.
- Preserves document order.

Used by `internal/runner` to derive the acceptance criteria list from `IssueData.Body`.

## ParseCheckBlocks

```go
func ParseCheckBlocks(body string) []string
```

Extracts each fenced ` ```check ... ``` ` block's inner command from a markdown issue body. These are issue-declared verification commands the factory appends to the green gate for per-run verification only — never committed to `.themis/workflow.yaml`.

- Returns the inner text of each top-level fenced block (ignores check blocks nested inside other fenced blocks per CommonMark fence-length rules).
- Text is trimmed of leading/trailing whitespace.
- Returns `nil` (not an empty slice) when no blocks are found.
- Preserves document order.
- Used by `cmd/themis` to extend the `.themis/workflow.yaml` `verify` contract for each run.

## IsDestructiveAC

```go
func IsDestructiveAC(ac string) bool
```

Reports whether an acceptance criterion is destructive/negative — asserting that something must NO LONGER exist after the change. Matches the phrasings: "No X remains", "X no longer exists", "Removed X" (case-insensitive, anchored to the start after trimming).

- Destructive ACs must be paired with a `check` block (enforced by `ValidateDestructiveChecks`).
- Examples: "No GiteaQuerier struct remains in cmd/themis", "Removed hardcoded Finna config from GetSources handler".

## ValidateDestructiveChecks

```go
func ValidateDestructiveChecks(body string) error
```

Deterministic meta-check that enforces every destructive AC has an accompanying `check` block positioned in its own span. Returns an error naming the offending AC when the rule is violated. Destructive-looking checkbox lines inside fenced code blocks are ignored.

- Returns `nil` when all destructive ACs have check blocks in their own spans, or when the body has no destructive ACs.
- Pairing is by position, not by count: each destructive AC must have a check block between its checkbox and the next checkbox (or end of body). Check blocks cannot be shared between destructive ACs. (per `skills/issue-writer/SKILL.md`).
- Called by `cmd/themis/main.go` at issue fetch time to fail fast if the issue structure is invalid.

## List-Issue Types

These types exist to support paginated list-issues responses consumed by `cmd/themis` queriers.

### `IssueItemLabel`

```go
type IssueItemLabel struct {
    Name string `json:"name"`
}
```

Raw label entry in a list-issues API response (both GitHub and Gitea use this shape).

### `IssueItem`

```go
type IssueItem struct {
    Number int              `json:"number"`
    Title  string           `json:"title"`
    Body   string           `json:"body"`
    Labels []IssueItemLabel `json:"labels"`
}
```

Raw API shape for a single entry in a list-issues response. `URL` and `Ref` are not populated by list endpoints and are absent from this type.

### `ParseIssueItems`

```go
func ParseIssueItems(items []IssueItem) []*IssueData
```

Converts a `[]IssueItem` to `[]*IssueData`, extracting label names via the internal `extractLabelNames` helper. Input order is preserved. `URL` and `Ref` on the resulting `IssueData` values are always empty string (not available from list endpoints).

## Architecture

### How to add a new tracker provider

1. Implement the `Fetcher` interface:
   ```go
   type MyFetcher struct{ /* config fields */ }
   func (f *MyFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) { ... }
   ```
2. Add a constructor (follow `NewGiteaFetcher` as the pattern for HTTP-based providers).
3. Add a `case "myprovider":` branch to `NewFetcher` that wires the constructor.
4. Update `cmd/themis` argument parsing and any querier needed for `themis run` label listing.

### How to diagnose fetch problems

| Symptom | Check | Likely cause |
|---------|-------|-------------|
| `unknown provider` error | `--provider` flag value | Must be exactly `"github"` or `"gitea"` |
| GitHub fetch fails silently or with auth error | `gh auth status` | `gh` CLI not authenticated |
| Gitea fetch returns 401 | Token env var presence | `GITEA_TOKEN` not set or not exported |
| Gitea fetch hangs indefinitely | Timeout value passed to `NewGiteaFetcher` | Zero timeout was passed — caller bug |
| Gitea fetch returns 404 | `apiBase`, `owner`, `repo` values | Wrong API base URL or repo path |
| Gitea fetch returns non-200 | Error message includes HTTP body | Inspect body for Gitea error detail |

### Failure behavior

- `NewFetcher` with an unknown provider returns an error immediately; no Fetcher is constructed.
- `GiteaFetcher` with a zero `timeout` creates an `http.Client` with no HTTP-level deadline. The `ctx` deadline still applies, but if the caller passes `context.Background()` (unbounded), the fetch will block until the OS TCP timeout or the server responds. **This is a caller bug, not a package guard.**
- All `Fetch` errors are returned unwrapped from the method. `cmd/themis` wraps them as `fmt.Errorf("fetching issue #%d: %w", number, err)` before propagating to the runner.
- A non-200 HTTP response from Gitea is an error; the response body is read and included in the error message to aid diagnosis.

## Dependencies

- `encoding/json` — JSON parsing for both providers
- `net/http` — Gitea HTTP client
- `os/exec` — `gh` CLI invocation for GitHub
- `regexp` — checkbox extraction

No internal package dependencies.

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/runner/README.md` — consumes `Fetcher` and `ParseCheckboxes` at pipeline start
- `docs/subsystems/themis/README.md` — wires `NewFetcher`, handles provider-specific configuration, and calls `ValidateDestructiveChecks` at fetch time
- `skills/issue-writer/SKILL.md` — user-facing guide for check block syntax and destructive AC rules
