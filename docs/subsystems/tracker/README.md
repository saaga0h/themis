# Issue Tracker Abstraction

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/tracker/ -->

## Overview

`internal/tracker` provides a provider-agnostic interface for fetching issue data from GitHub or Gitea. It defines the `Fetcher` interface, two concrete implementations, supporting types for list-issues API shapes, and helper functions for parsing raw issue item types.

The package is consumed by `cmd/themis` (wiring and Gitea configuration resolution) and `internal/runner` (issue fetch at pipeline start). Directive parsing (checkboxes, check blocks, footprint, exports, destructive-AC validation) is handled by `internal/issuespec`.

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

No internal package dependencies.

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/issuespec/README.md` — directive parsing (checkboxes, check blocks, footprint, exports, destructive-AC validation)
- `docs/subsystems/runner/README.md` — consumes `Fetcher` at pipeline start
- `docs/subsystems/themis/README.md` — wires `NewFetcher` and handles provider-specific configuration
- `skills/issue-writer/SKILL.md` — user-facing guide for check block syntax and destructive AC rules
