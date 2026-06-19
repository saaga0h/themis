# Git Subsystem

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/git/ -->

## Overview

`internal/git` provides context-aware wrappers around `git` subprocess calls. Every exported function accepts a `context.Context` for cancellation and deadline propagation. All functions route through the internal `runGit` helper, which validates that `dir` is an absolute path before constructing any subprocess.

The package has two files with distinct responsibilities:

- `git.go` — repository inspection and mutation helpers (commits, branches, working tree, push)
- `gitea_infer.go` — parses the `origin` remote URL to derive Gitea API coordinates

There is no mocking layer. Tests spin up real temporary git repositories using `os/exec` directly.

## Key Files & Entry Points

| File | Role |
|---|---|
| `internal/git/git.go` | All git operations; exports 11 functions + `runGit` + `parseLines` helpers |
| `internal/git/gitea_infer.go` | `InferGiteaConfig` + `GiteaConfig` struct |
| `internal/git/git_test.go` | Tests for git.go; uses real repos via `initTestRepo` |
| `internal/git/gitea_infer_test.go` | Tests for gitea_infer.go; uses `initTestRepoWithRemote` |

## Architecture

All exported functions share a single call pattern:

```
ExportedFunc(ctx context.Context, dir string, ...) → (result, error)
```

`runGit` enforces the absolute-path contract (see [Absolute-Path Validation](#absolute-path-validation)) and captures both stdout and stderr. Stderr is appended to the error string on failure so callers receive the full git diagnostic without needing to re-run.

Two functions — `ChangedFiles` and `BranchCommitLog` — deliberately suppress errors and return empty strings instead. They both delegate to the unexported `branchMergeBase`, which iterates `refs/remotes/` to find the nearest remote tracking branch. When no remote exists, the entire call chain short-circuits to `""` without logging or propagating an error. This is intentional: callers treat an empty result as "no remote context available" rather than a failure.

`CommitsAheadOfBase` tries two ref spellings in sequence (`origin/<base>` then `<base>`) and returns an error only if neither resolves.

## Data Flow

```
caller → ExportedFunc(ctx, dir, ...)
           └─ runGit(ctx, dir, gitArgs...)
                ├─ filepath.IsAbs(dir) check → error if relative
                ├─ exec.CommandContext(ctx, "git", args...) with cmd.Dir = dir
                └─ stdout / (stderr appended to error on failure)
```

For merge-base operations:

```
ChangedFiles / BranchCommitLog
  └─ branchMergeBase(ctx, dir)
       └─ git for-each-ref refs/remotes/   → iterate, skip */HEAD entries
            └─ git merge-base HEAD <ref>   → first success wins
  └─ git diff --name-only <base>  (ChangedFiles)
  └─ git log --oneline <base>..HEAD  (BranchCommitLog)
```

## Function Reference

| Function | Signature Summary | Returns on Success | Failure Mode |
|---|---|---|---|
| `CommitsBefore` | `(ctx, dir) ([]string, error)` | All commit SHAs reachable from HEAD (`git log --format=%H`) | Propagates error |
| `CommitsAfter` | `(ctx, dir, before []string) ([]string, error)` | SHAs in current HEAD not present in `before` snapshot | Propagates error |
| `WorkingTreeClean` | `(ctx, dir) (bool, error)` | `true` when `git status --porcelain` is empty | Propagates error |
| `CurrentBranch` | `(ctx, dir) (string, error)` | Branch name from `git rev-parse --abbrev-ref HEAD` | Propagates error |
| `LastCommitMessage` | `(ctx, dir) (string, error)` | Subject line (`%s` format) of the most recent commit | Propagates error |
| `CheckoutNewBranch` | `(ctx, dir, name string) error` | — (side effect: creates and checks out branch) | Propagates error |
| `Checkout` | `(ctx, dir, name string) error` | — (side effect: switches to existing branch) | Propagates error |
| `Fetch` | `(ctx, dir) error` | — (side effect: `git fetch origin`) | Propagates error |
| `PushBranch` | `(ctx, dir, branch string) error` | — (side effect: `git push -u origin <branch>`) | Propagates error |
| `ChangedFiles` | `(ctx, dir) string` | Newline-separated paths changed on HEAD vs merge-base with nearest remote tracking branch | Returns `""` — never errors |
| `BranchCommitLog` | `(ctx, dir) string` | `git log --oneline` output for commits on current branch since merge-base | Returns `""` — never errors |
| `CommitsAheadOfBase` | `(ctx, dir, base string) (int, error)` | Count of commits reachable from HEAD but not from `origin/<base>` or `<base>` | Propagates error if neither ref resolves |

### Non-obvious behavior: `ChangedFiles`

`ChangedFiles` returns paths changed on HEAD relative to the **merge-base** with the nearest remote tracking branch, not relative to the tracking branch tip. It calls `git diff --name-only <merge-base-sha>`. When no remote tracking refs exist under `refs/remotes/` (or every `merge-base` call fails), it returns `""` without error.

### Non-obvious behavior: `BranchCommitLog`

`BranchCommitLog` uses the same `branchMergeBase` logic and runs `git log --oneline <merge-base-sha>..HEAD`. Any failure in either the merge-base lookup or the log command yields `""`. Callers cannot distinguish "no remote" from "git command failed" via the return value.

### Non-obvious behavior: `CommitsAfter`

`CommitsAfter` re-runs `git log --format=%H` at call time and computes the set difference against the caller-supplied `before` snapshot. Order within the returned slice follows the order git log returns commits, not chronological insertion order of the new commits.

## Gitea Config Inference

### `InferGiteaConfig(ctx context.Context, dir string) (*GiteaConfig, error)`

Runs `git remote get-url origin`, parses the result with `net/url.Parse`, and returns a `GiteaConfig`:

```go
type GiteaConfig struct {
    Owner   string  // first path segment
    Repo    string  // second path segment, .git suffix stripped
    APIBase string  // always https://<hostname>, even for ssh:// remotes
}
```

### Supported URL formats

| Format | Example | Notes |
|---|---|---|
| `https://` | `https://gitea.example.com/owner/repo.git` | Direct parse |
| `ssh://` | `ssh://git@gitea.example.com:2222/owner/repo.git` | Port is discarded; `APIBase` uses hostname only with `https://` scheme |

`.git` suffix on the repo name is stripped automatically. URLs without a `.git` suffix also parse correctly.

### UNSUPPORTED: SCP-style SSH remotes

> **`git@host:path` remotes are not supported and return an error.**

`net/url.Parse` does not recognize the SCP colon syntax as a scheme, so `u.Scheme` will not be `https` or `ssh`. `InferGiteaConfig` returns:

```
unrecognized remote URL format "git@github.com:owner/repo.git" (expected https:// or ssh://)
```

Callers that may encounter SCP-style remotes must either convert the remote URL to `ssh://` format before calling this function or handle the error explicitly.

## Absolute-Path Validation

`runGit` calls `filepath.IsAbs(dir)` before constructing any subprocess. A relative path returns:

```
dir must be an absolute path, got "<value>"
```

This check applies to every exported function because all of them route through `runGit`. There is no fallback to the process working directory.

## Testing

Tests use real temporary git repositories. There is no mocking of `exec` or the filesystem.

`initTestRepo(t)` (in `git_test.go`) creates a `t.TempDir()` repo with an initial commit on branch `main`. It sets `--initial-branch=main` explicitly to be portable across environments where `init.defaultBranch` may differ. Three tests (`TestInitTestRepo_*`) verify this portability directly.

`initTestRepoWithRemote(t, remoteURL)` (in `gitea_infer_test.go`) wraps `initTestRepo` and adds an `origin` remote with the given URL. The URL does not need to be reachable; `InferGiteaConfig` only reads it with `git remote get-url`.

Tests do not cover `ChangedFiles` or `BranchCommitLog` with a live remote because setting up a remote tracking ref in a temp repo would require a second bare repo clone. The empty-string failure paths are exercised implicitly when no remote is configured.

## How to Add a New Git Helper

Follow this pattern from `git.go`:

1. Accept `ctx context.Context` as the first parameter and `dir string` as the second.
2. Call `runGit(ctx, dir, ...)` — do not construct `exec.Command` directly in the new function.
3. Decide on a failure contract before writing: propagate the error (standard), or return `""` / zero value (only when "no remote context" is a legitimate non-error state for callers).
4. Wrap errors with `fmt.Errorf("git <subcommand>: %w", err)` to preserve the stderr diagnostic.
5. Add a test that calls `initTestRepo(t)` and exercises the new function against a real repo.

Do not add fallbacks that accept relative paths or silently resolve against the process working directory.

## How to Diagnose Problems

| Symptom | Check | Fix |
|---|---|---|
| `dir must be an absolute path` error | Caller is passing a relative path | Pass an absolute path (use `filepath.Abs` at the call site) |
| `ChangedFiles` or `BranchCommitLog` returns `""` unexpectedly | No remote tracking refs under `refs/remotes/` in the repo | Run `git fetch origin` first, or check that an `origin` remote exists |
| `InferGiteaConfig` returns `unrecognized remote URL format` | Remote is SCP-style (`git@host:path`) | Convert remote to `ssh://git@host/path` or `https://host/path` before calling |
| `InferGiteaConfig` returns `no origin remote` | Repo has no `origin` remote configured | Add an origin remote with `git remote add origin <url>` |
| `CommitsAheadOfBase` returns error | Neither `origin/<base>` nor `<base>` resolves as a ref | Verify the base branch name and that `git fetch origin` has been run |
| Context cancellation causes unexpected `""` from `ChangedFiles`/`BranchCommitLog` | `runGit` fails due to cancelled context; both functions suppress errors | Ensure context is not cancelled before calling; cancelled context errors are silently swallowed by these two functions |

## What is the Failure Behavior

**Functions that propagate errors:** `CommitsBefore`, `CommitsAfter`, `WorkingTreeClean`, `CurrentBranch`, `LastCommitMessage`, `CheckoutNewBranch`, `Checkout`, `Fetch`, `PushBranch`, `CommitsAheadOfBase`, `InferGiteaConfig`. All wrap the underlying error with context via `fmt.Errorf`.

**Functions that suppress errors and return empty string:** `ChangedFiles`, `BranchCommitLog`. Every failure path — no remote, `git for-each-ref` error, `git merge-base` error, `git diff`/`git log` error, context cancellation — produces `""`. Callers cannot distinguish failure from "no remote tracking branch exists".

**`CommitsAheadOfBase`** is a middle case: it silently retries with a second ref spelling before surfacing an error. Callers always get either a valid count or an explicit error.

## Dependencies

| Dependency | Usage |
|---|---|
| `os/exec` | Spawns `git` subprocess via `exec.CommandContext` |
| `context` | Passed to `exec.CommandContext`; honours cancellation and deadlines |
| `path/filepath` | `filepath.IsAbs` for absolute-path validation |
| `net/url` | `url.Parse` for remote URL parsing in `InferGiteaConfig` |
| `strconv` | `strconv.Atoi` to parse commit count in `CommitsAheadOfBase` |

The package has no dependency on other `internal/` packages.

## Related Documents

- [ARCHITECTURE.md](/Users/saaga.helin/projects/themis/ARCHITECTURE.md) — system-wide architecture
- [docs/subsystems/checkpoint/README.md](/Users/saaga.helin/projects/themis/docs/subsystems/checkpoint/README.md) — checkpoint subsystem, which uses `CommitsBefore`/`CommitsAfter` to track commit snapshots
- [docs/subsystems/themis/README.md](/Users/saaga.helin/projects/themis/docs/subsystems/themis/README.md) — top-level themis subsystem, which uses `InferGiteaConfig`, `CurrentBranch`, `PushBranch`, and related helpers
