# 1 · Install & run

## Prerequisites

- **Go** — to build the `themis` binary.
- **Podman (rootless) or Docker** — the sandbox. `--userns=keep-id` is required for rootless Podman.
- **`CLAUDE_CODE_OAUTH_TOKEN`** — generated on your host with `claude setup-token`. Themis's one hard dependency.
- **A host token** — `GH_TOKEN` (GitHub) or `GITEA_TOKEN` (Gitea), with Issues + Pull-requests read/write.

## Build the binary

```bash
make build          # -> bin/themis (static linux binary the sandbox mounts)
# or: CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/themis ./cmd/themis
```

## The sandbox

The factory runs Claude Code with `--dangerously-skip-permissions` so it can work unattended. **The container is the blast radius** — it bounds what the agent can touch to the mounted workspace, nothing else on the host. Never run the factory outside its container, and never on issues or repositories you don't trust to execute code (the green gate runs project-defined shell — see [`CODING_STANDARDS.md` → Trust Model](../../CODING_STANDARDS.md)).

The sandbox image needs: your project's **toolchain** (compiler, test runner) + **Claude Code** + the **`themis` binary** + your repo mounted. Store tokens in a `.env` at the repo root (gitignored) and pass them with `--env-file .env`.

## Running

Themis processes issues two ways:

- `themis run` — the loop: every open `ready-for-agent` issue, in order.
- `themis issue <number>` — a single issue.

Both take `--provider github|gitea` (default `github` — pass `--provider gitea` for a Gitea host).

> **Status — packaging in progress.** Themis currently dogfoods on *itself* via its `Makefile` (`make factory`, `make factory-issue ISSUE=N`), which builds `themis` from the mounted source inside the container. A **portable wrapper to run the binary against *your own* project** (a prebuilt image + a `--sandbox`-style launcher) is a beta prerequisite being finalized — see [`docs/beta-readiness.md`](../beta-readiness.md). Until then, adapt the `Makefile`'s `FACTORY_RUN` block: mount `bin/themis` + your repo into a container that has your toolchain + Claude Code, and run `themis run --provider …`.

See [`docs/development.md`](../development.md) for the full command/flag reference and troubleshooting.

→ Next: [Configure a project](02-configure.md)
