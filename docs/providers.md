# Providers — GitHub & Gitea

Themis is **provider-agnostic**. The pipeline, gates, and state machine are identical whichever host you use; only a thin adapter differs — a `Fetcher` (read the issue), an `IssueWriter` (comment, label, open the PR), and an `IssueQuerier` (list `ready-for-agent`). Two providers ship:

- **GitHub** — via the `gh` CLI.
- **Gitea** — via the REST API.

**Where you set it:** the `provider:` field in `.themis/workflow.yaml` (`github` or `gitea`) — a per-project fact you set once. The `--provider` flag on `themis run`/`issue` overrides it for a single run; if neither is set, it defaults to `github` (the public, hosted case; Gitea is the self-hosted one).

## The provider contract

Both adapters promise the same behaviour. Your repo, on either host, must satisfy the same contract:

- **The two factory labels must exist:** `ready-for-agent` (you apply it to signal an issue is ready) and `needs-review` (Ship applies it). These are factory infrastructure, like `.themis/workflow.yaml`. *Gitea auto-creates them if missing; GitHub requires them to pre-exist* — see setup below.
- **The PR targets the resolved base branch** — the run's base (the branch you launched from) → `main` if unset. Ship removes `ready-for-agent`, adds `needs-review`, and is idempotent if a PR for the branch already exists.
- **A per-issue target branch is optional and Gitea-only.** Gitea honours an issue's `ref` field as the PR base; GitHub issues have no such field, so on GitHub every PR targets the resolved base.

## GitHub setup

1. **Provider:** default — no flag needed, or `--provider github`.
2. **Auth:** the `gh` CLI must be **installed and authenticated** — `gh auth login`, or `GH_TOKEN` / `GITHUB_TOKEN` in the environment. Themis shells out to `gh` and relies on its ambient auth (there's no separate token plumbing).
3. **Labels:** create both factory labels in the repo before running — the `gh` path adds/removes labels by name and errors if they don't exist:
   ```bash
   gh label create ready-for-agent --description "Ready for the factory"
   gh label create needs-review    --description "Factory opened a PR; awaiting human review"
   ```
4. **Target branch:** GitHub issues can't declare one; PRs target the resolved base.

> **Status: not yet proven end-to-end.** The GitHub adapter is implemented but has only been exercised by unit tests, not a live run — Themis is dogfooded on Gitea. Two behaviours are pending live verification: that `gh pr create` infers the head branch correctly in the sandbox flow, and that the "PR already exists" detection matches the installed `gh` version's wording. The sandbox image also does not yet ship `gh` (it ships `tea`), so in-container GitHub runs wait on a packaging step. Track this under the GitHub end-to-end issue.

## Gitea setup

1. **Provider:** `--provider gitea` (the binary defaults to GitHub).
2. **Auth:** `GITEA_TOKEN` in the environment. `owner` / `repo` / API base are inferred from the `origin` remote (override with `GITEA_OWNER` / `GITEA_REPO` / `GITEA_API_URL`).
   - **Credential hygiene:** keep the token in the environment (or a git credential helper / SSH), **not embedded in the remote URL** (`https://user:token@host/…` in `.git/config`) — a URL-embedded token is stored in plaintext and prints in the clear on any `git remote -v`. The factory never writes credentials into `.git/config` (it authenticates each push in isolation); don't reintroduce them by cloning with a tokenised URL.
3. **Labels:** auto-created if missing — no manual step.
4. **Target branch:** an issue's `ref` field, when set, is used as the PR base.

## Why not a single code path

Because the transports are genuinely different — Gitea is an authenticated HTTP REST API (numeric label IDs, JSON bodies); GitHub is a `gh` subprocess. Merging them into one path would mean provider conditionals threaded through the logic, which is less clean, not more. The *single path* lives one level up: the runner and pipeline are already provider-agnostic, and this contract is the shared behaviour the thin adapters implement.
