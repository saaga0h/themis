# Themis CLI

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: cmd/themis/ -->

## Overview

`themis` is the CLI entry point for the Themis v2.0 deterministic orchestration system. It dispatches subcommands and exits non-zero on any error.

## Key Files & Entry Points

| File | Role |
|------|------|
| `cmd/themis/main.go` | `package main`; argument parsing and subcommand dispatch |
| `cmd/themis/main_test.go` | Integration tests that compile the binary and exec it |

## Architecture

`main()` selects the subcommand via `os.Args[1]` switch. The `issue` subcommand delegates to `runIssue()`, which parses args then calls `newIssueConfig` to wire together the `tracker.Fetcher`, `agent.Invoker`, `runner.IssueWriter`, and `checkpoint.CheckpointFn` implementations before calling `runner.Run`.

Unknown subcommands print `unknown command: <name>` to stderr and exit 1. Invocation with no arguments prints a usage line to stderr and exits 1.

## Subcommands

| Subcommand | Status | Behavior |
|------------|--------|----------|
| `version` | Implemented | Prints `0.1.0` to stdout, exits 0 |
| `issue` | Implemented (issue #11) | Fetches issue from GitHub/Gitea, runs full pipeline, creates PR |
| `run` | Implemented (issue #37) | Lists all `ready-for-agent` issues and processes them sequentially |

### `themis issue <number> [--provider github\|gitea]`

Fetches issue `<number>` from the configured tracker, loads (or resumes) pipeline state, and runs the pipeline to completion. Creates a PR on success; adds the `blocked` label and comments on the issue when a cycle limit is reached.

- `--provider github` (default): fetches via `gh issue view --json`
- `--provider gitea`: fetches from Gitea REST API; `owner`, `repo`, and `apiBase` are inferred from the `origin` git remote (`GITEA_OWNER`, `GITEA_REPO`, `GITEA_API_URL` override individual fields); `GITEA_TOKEN` is required

### `themis run [--provider github|gitea] [--dry-run]`

Lists all open issues labelled `ready-for-agent`, sorts them by issue number (ascending), and runs the full `issue` pipeline for each one in sequence.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--provider github\|gitea` | `github` | Issue tracker backend |
| `--dry-run` | off | Print which issues would be processed; skip actual execution |

**Loop behaviour**

- Issues are processed lowest-number-first.
- If fewer than 10 % of agentic turns remain (`THEMIS_TURNS_REMAINING_FRACTION < 0.10`), the loop stops early and prints `insufficient turns remaining` to stderr. When `THEMIS_TURNS_REMAINING_FRACTION` is unset the loop runs unrestricted.
- If an issue body contains `depends on #N` and issue `N` is still open, that issue is skipped for this run.
- A failure on one issue (non-zero exit from the pipeline) is logged to stderr and the loop continues with the next issue.
- Before each issue the runner checks out `main` to avoid branch-state contamination between issues.

**Provider wiring**

| Provider | Issue listing | Dependency check |
|----------|--------------|-----------------|
| `github` | `gh issue list --label ready-for-agent` (via `GitHubQuerier`) | `gh issue view N --json state` |
| `gitea` | Gitea REST API paginated at 50 issues/page (via `GiteaQuerier`) | Gitea REST `GET /api/v1/repos/{owner}/{repo}/issues/{N}` |

Gitea connection details are resolved via `resolveGiteaConfig` (same as `themis issue`) and can be overridden with `GITEA_OWNER`, `GITEA_REPO`, `GITEA_API_URL`, and `GITEA_TOKEN`.

## Build, Test, and Lint

```
make build   # go build ./cmd/themis/
make test    # go test ./...
make lint    # go vet ./...
```

Module: `git.home.federation.fi/lavernea/themis`, Go 1.22.

## Key Files

| File | Role |
|------|------|
| `cmd/themis/main.go` | Subcommand dispatch and `runIssue` wiring |
| `cmd/themis/issue.go` | `parseIssueArgs` — parses `<number>` and `--provider` |
| `cmd/themis/invoker.go` | `claudeInvoker` — bridges `agent.Invoker` for the binary |
| `cmd/themis/issue_writer.go` | `ghIssueWriter` and `giteaIssueWriter` — label, comment, PR creation |
| `cmd/themis/gitea_config.go` | `resolveGiteaConfig` — infers Gitea owner/repo/apiBase from git remote, overridable via env vars |

## Dependencies

- `internal/runner` — pipeline orchestration
- `internal/tracker` — issue fetching
- `internal/agent` — agent invocation interface
- `internal/checkpoint` — step checkpoint wired as `CheckpointFn`

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/` — other subsystem READMEs
