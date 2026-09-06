# Providers — GitHub & Gitea

Themis is **provider-agnostic**. The pipeline, gates, and state machine are identical whichever host you use; only a thin adapter differs — a `Fetcher` (read the issue), an `IssueWriter` (comment, label, open the PR), and an `IssueQuerier` (list `ready-for-agent`). Two providers ship:

- **GitHub** — via the `gh` CLI.
- **Gitea** — via the REST API.

**Where it comes from:** the provider is **auto-detected from your git `origin` remote** — `github.com` → GitHub, any other host → Gitea — so you usually set nothing. To force it, set `provider: github|gitea` in `.themis/workflow.yaml`, or pass `--provider` on `themis run`/`issue` (flag > config > inferred). With no remote it defaults to `github`.

## The provider contract

Both adapters promise the same behaviour. Your repo, on either host, must satisfy the same contract:

- **The two factory labels must exist:** `ready-for-agent` (you apply it to signal an issue is ready) and `needs-review` (Ship applies it). These are factory infrastructure, like `.themis/workflow.yaml`. *Gitea auto-creates them if missing; GitHub requires them to pre-exist* — see setup below.
- **The PR targets the resolved base branch** — the run's base (the branch you launched from) → `main` if unset. Ship removes `ready-for-agent`, adds `needs-review`, and is idempotent if a PR for the branch already exists.
- **A per-issue target branch is optional and Gitea-only.** Gitea honours an issue's `ref` field as the PR base; GitHub issues have no such field, so on GitHub every PR targets the resolved base.

## GitHub setup

1. **Provider:** default — no flag needed, or `--provider github`.
2. **Auth:** the factory runs `gh` **inside the sandbox**, so it authenticates from **`GITHUB_TOKEN`** in the environment, passed via `.env` / `--env-file` — and the factory's own `git push` uses the same token over https. A host `gh auth login` — **including a browser/web login** — does **not** cross into the container, and neither does your host's `gh` binary. Put a token in `.env`:
   ```bash
   echo "GITHUB_TOKEN=$(gh auth token)" >> .env   # writes the token without printing it
   ```
   `gh auth token` prints the token your terminal login already uses (works even after a browser login) — **convenient, but it carries your account's full access** (every scope your `gh` login was granted). For least privilege, instead create a **fine-grained PAT** limited to this one repo, with **Issues** and **Pull requests** read/write (plus **Contents** read/write for branches/commits), and use that as `GITHUB_TOKEN`. Either way, `.env` is gitignored — keep it that way; a token in a file is as sensitive as the access it carries.

   `gh` must also be present in the sandbox image — `themis init` scaffolds its install for a GitHub project.
3. **Labels:** create both factory labels in the repo before running — the `gh` path adds/removes labels by name and errors if they don't exist:
   ```bash
   gh label create ready-for-agent --description "Ready for the factory"
   gh label create needs-review    --description "Factory opened a PR; awaiting human review"
   ```
4. **Target branch:** GitHub issues can't declare one; PRs target the resolved base.

> **Status: proven end-to-end.** A live sandbox run took a GitHub issue through the full pipeline to an opened PR — init → build image → run → https token push → `gh pr create`. Reaching that fixed three sandbox-auth gaps, all now resolved: the image ships `gh` (the scaffold installs it for GitHub projects); the factory pushes over https with `GITHUB_TOKEN` rather than the repo's own remote auth (a host SSH key or `gh` login does not cross the sandbox boundary); and `gh pr create` is passed `--head` explicitly, because the out-of-band token push sets no upstream tracking. One behaviour remains only lightly exercised: the "PR already exists" idempotency check matches on gh's stderr wording, which can vary by gh version — re-verify if a re-run misbehaves.

## Gitea setup

1. **Provider:** `--provider gitea` (the binary defaults to GitHub).
2. **Auth:** `GITEA_TOKEN` in the environment. `owner` / `repo` / API base are inferred from the `origin` remote (override with `GITEA_OWNER` / `GITEA_REPO` / `GITEA_API_URL`).
   - **Credential hygiene:** keep the token in the environment (or a git credential helper / SSH), **not embedded in the remote URL** (`https://user:token@host/…` in `.git/config`) — a URL-embedded token is stored in plaintext and prints in the clear on any `git remote -v`. The factory never writes credentials into `.git/config` (it authenticates each push in isolation); don't reintroduce them by cloning with a tokenised URL.
3. **Labels:** auto-created if missing — no manual step.
4. **Target branch:** an issue's `ref` field, when set, is used as the PR base.

## Why not a single code path

Because the transports are genuinely different — Gitea is an authenticated HTTP REST API (numeric label IDs, JSON bodies); GitHub is a `gh` subprocess. Merging them into one path would mean provider conditionals threaded through the logic, which is less clean, not more. The *single path* lives one level up: the runner and pipeline are already provider-agnostic, and this contract is the shared behaviour the thin adapters implement.
