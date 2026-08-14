# Development Guide

<!-- @tier: 1 -->
<!-- @source: cmd/themis/, Makefile, internal/runner/, internal/checkpoint/, internal/agent/ -->
<!-- @see-also: docs/subsystems/ -->

## Overview

`themis` (`cmd/themis`) is a Go CLI that orchestrates an autonomous software factory pipeline. It fetches issues from GitHub or Gitea, drives Claude Code through a deterministic multi-step pipeline (test-red → implement → review → docs → ship), and opens a pull request on completion. Build with `make build`; run in a sandboxed container via `make factory` (the autonomous loop) or `make factory-issue ISSUE=N` (a single issue). See `README.md` for the full container walkthrough and Containerfile example.

## Prerequisites

| Prerequisite | Notes |
|---|---|
| Go 1.22 | Module: `github.com/saaga0h/themis` |
| Podman (rootless) or Docker | `--userns=keep-id` required for rootless Podman |
| `gh` CLI | Required when `--provider github` (default) |
| `CLAUDE_CODE_OAUTH_TOKEN` | Headless Claude Code auth; generated via `claude setup-token` on the host |
| `GH_TOKEN` | GitHub fine-grained token with Issues (read/write) and Pull requests (read/write) — GitHub provider only |
| `GITEA_TOKEN` | Gitea personal access token with Issues and Pull requests permissions — Gitea provider only |

## Build, Test, Lint

### Makefile targets

| Target | Purpose | Notes |
|---|---|---|
| `build` | Compile `themis` binary to `bin/themis` | `go build ./cmd/themis` |
| `test` | Run Go tests | `go test ./...`; add `-race` flag manually for race detection: `go test -race ./...` |
| `lint` | Static analysis | `go vet ./...` |
| `build-image` | Build the factory container image (`themis:dev`) | `podman build` with `--memory=16g` |
| `run-factory` | Run the autonomous factory loop | Pipes `/factory --provider $(PROVIDER)` into the container via `claude --verbose --dangerously-skip-permissions` |
| `dry-run` | Preview which issues would be processed | Pipes `/factory --provider $(PROVIDER) --dry-run`; no issues are executed |
| `run-issue` | Run the pipeline against a single issue | `make run-issue ISSUE=<number>`; uses the `themis:dev` image and `/issue` CC command |
| `run-v2-issue` | Build and run the `themis` v2 binary inside the container against a single issue | Compiles on-the-fly inside the container, then calls `themis issue <number> --provider gitea` directly (no CC command layer). Depends on `factory-cc`; overlays the curated `.claude/` onto the workspace |
| `factory-cc` | Materialize the curated `.claude/` the v2 container overlays | Copies only the skills/agents/commands listed in `factory/manifest.txt` into `.themis/factory-cc/.claude/` so each per-step `claude` indexes the pipeline subset, not the full interactive catalog. Regenerated from the tracked source each run |
| `shell` | Open an interactive bash shell inside the factory container | Useful for debugging |

Default values: `IMAGE=themis:dev`, `MAX_TURNS=600`, `PROVIDER=gitea`.

Override per-invocation: `make run-factory PROVIDER=github`, `make run-issue ISSUE=42 PROVIDER=gitea`.

## Commands

`themis <command> [flags]`

| Command | Synopsis | Description |
|---|---|---|
| `version` | `themis version` | Prints `0.1.0` and exits 0 |
| `issue` | `themis issue <number> [--provider github\|gitea] [--max-turns N]` | Fetches one issue and runs the full pipeline; creates a PR on success |
| `run` | `themis run [--provider github\|gitea] [--dry-run] [--max-turns N]` | Lists all open `ready-for-agent` issues and processes them sequentially |

### Flags

| Flag | Commands | Default | Description |
|---|---|---|---|
| `--provider github\|gitea` | `issue`, `run` | `github` | Issue tracker backend |
| `--max-turns N` | `issue`, `run` | `250` (`runner.DefaultMaxTurns`) | Per-agent invocation turn limit passed to Claude Code |
| `--dry-run` | `run` | off | Print which issues would be processed without executing them |

For full subcommand behavior (loop ordering, dependency skipping, blocking label logic, PR body construction), see `docs/subsystems/themis/README.md`.

## Configuration / Environment Variables

| Variable | Required | Description |
|---|---|---|
| `CLAUDE_CODE_OAUTH_TOKEN` | Yes | OAuth token for Claude Code headless invocations; generated with `claude setup-token` |
| `GH_TOKEN` | GitHub only | GitHub fine-grained PAT; consumed by the `gh` CLI for issue and PR operations |
| `GITEA_TOKEN` | Gitea only | Gitea PAT; used by `GiteaQuerier`, `GiteaFetcher`, and `giteaIssueWriter` for all Gitea API calls |
| `GITEA_OWNER` | Gitea only* | Repository owner; overrides the value inferred from the `origin` git remote |
| `GITEA_REPO` | Gitea only* | Repository name; overrides the value inferred from the `origin` git remote |
| `GITEA_API_URL` | Gitea only* | Gitea API base URL (e.g. `https://git.example.com`); overrides the value inferred from the `origin` git remote |
| `THEMIS_TURNS_REMAINING_FRACTION` | No | Float 0.0–1.0 representing the fraction of Claude Code turns remaining in the current session. When below `0.10`, `themis run` stops processing further issues. Unset = `1.0` (unrestricted). |
| `OTEL_RESOURCE_ATTRIBUTES` | No | If set, `issue.number=N,pipeline.step=S` is prepended to the existing value before being passed to each Claude Code subprocess. Not read by `themis` itself — it is forwarded to child processes. |

\* `GITEA_OWNER`, `GITEA_REPO`, and `GITEA_API_URL` are only required when inference from the `origin` remote fails (e.g. SCP-style SSH remotes — see troubleshooting below). If inference succeeds, these vars override individual fields only when set.

All tokens are expected in a `.env` file at the repo root (loaded by `--env-file .env` in every Makefile container target). Add `.env` to `.gitignore`.

## When Things Look Wrong

| Symptom | Check | Fix |
|---|---|---|
| `warning: state file is for issue #X, not #Y — starting fresh` | `.themis/state.json` in the work directory belongs to a different issue | Normal when switching issues; state is reset automatically. If unexpected, delete `.themis/state.json` manually before re-running. |
| `current branch is the base branch — no issue branch was created` | Pipeline reached Ship step but HEAD is still on `main` (or the issue's base branch) | The branch step did not execute or the branch was deleted. Delete `.themis/state.json` to force a fresh start. |
| `no commits on branch <branch> — nothing to ship` | The issue branch exists but has zero commits ahead of base | Earlier pipeline steps produced no commits. Resume from scratch (delete `.themis/state.json`) or inspect the agent's work. |
| `warning: review-results.json not found after review step — treating as blocking` | `.themis/review-results.json` absent or not written by the review agent | The review agent did not write structured output. The pipeline treats missing JSON as blocking — it will enter the fix cycle. Check agent logs; ensure the review template writes `{"findings": [...]}` to `.themis/review-results.json`. |
| `insufficient turns remaining` printed; remaining issues skipped | `THEMIS_TURNS_REMAINING_FRACTION` is below `0.10` | Run `themis run` in a fresh Claude Code session with a full turn budget, or unset `THEMIS_TURNS_REMAINING_FRACTION`. |
| `checkpoint failed after step <step>` | Last commit on the branch does not have the expected conventional commit prefix | Step-to-prefix map: `test-red→test(`, `implement→feat(`, `refactor→refactor(`, `fix→fix(`, `docs→docs(`. Agent made a commit with the wrong prefix. Either amend the commit message or delete `.themis/state.json` and re-run. |
| `cannot determine Gitea config (remote: ...); set GITEA_OWNER, GITEA_REPO, and GITEA_API_URL` | `git.InferGiteaConfig` could not parse the `origin` remote URL | SCP-style SSH remotes (`git@host:owner/repo.git`) are not supported by inference. Set `GITEA_OWNER`, `GITEA_REPO`, and `GITEA_API_URL` explicitly in `.env`. |
| `gh issue list` or `gh pr create` fails with auth error | `GH_TOKEN` not set or not passed into the container | Verify `.env` contains `GH_TOKEN` and that `--env-file .env` is present in the `podman run` invocation. |

## Secrets & Deployment

The following secrets are required at runtime:

- `CLAUDE_CODE_OAUTH_TOKEN` — Claude Code headless auth token
- `GH_TOKEN` — GitHub PAT (GitHub provider only)
- `GITEA_TOKEN` — Gitea PAT (Gitea provider only)

Store them in a `.env` file at the repo root. The Makefile passes this file to `podman run` via `--env-file .env`. `--userns=keep-id` is mandatory for rootless Podman — without it the container process runs as a different UID and cannot read the `~/.claude` mount.

`CLAUDE_CODE_OAUTH_TOKEN` must come from `claude setup-token` run on the host; the interactive OAuth flow does not work inside a container.

## Related Documents

- `README.md` — container setup, Containerfile example, full factory walkthrough
- `docs/subsystems/themis/README.md` — subcommand architecture, loop behavior, provider wiring
- `ARCHITECTURE.md` — system-level design and OTEL tracing contract
