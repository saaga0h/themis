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

`main()` selects the subcommand via `os.Args[1]` switch. The `issue` subcommand delegates to `runIssue()`, which parses args and wires together the `tracker.Fetcher`, `agent.Invoker`, and `runner.IssueWriter` implementations before calling `runner.Run`.

Unknown subcommands print `unknown command: <name>` to stderr and exit 1. Invocation with no arguments prints a usage line to stderr and exits 1.

## Subcommands

| Subcommand | Status | Behavior |
|------------|--------|----------|
| `version` | Implemented | Prints `0.1.0` to stdout, exits 0 |
| `issue` | Implemented (issue #11) | Fetches issue from GitHub/Gitea, runs full pipeline, creates PR |
| `run` | Planned | <!-- TODO: document when implemented --> |

### `themis issue <number> [--provider github\|gitea]`

Fetches issue `<number>` from the configured tracker, loads (or resumes) pipeline state, and runs the pipeline to completion. Creates a PR on success; adds the `blocked` label and comments on the issue when a cycle limit is reached.

- `--provider github` (default): fetches via `gh issue view --json`
- `--provider gitea`: fetches from Gitea REST API (`GITEA_OWNER`, `GITEA_REPO`, `GITEA_API_URL`, `GITEA_TOKEN` env vars)

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

## Dependencies

- `internal/runner` — pipeline orchestration
- `internal/tracker` — issue fetching
- `internal/agent` — agent invocation interface

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/` — other subsystem READMEs
