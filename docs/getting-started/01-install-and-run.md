# 1 · Install

You can't run the factory yet — it needs a configured project and a built sandbox image first (the next steps). This step just gets the `themis` binary and the prerequisites in place.

## Prerequisites

- **Go** — to build the `themis` binary.
- **Podman (rootless) or Docker** — the sandbox. `--userns=keep-id` is required for rootless Podman.
- **`CLAUDE_CODE_OAUTH_TOKEN`** — generated on your host with `claude setup-token`. Themis's one hard dependency.
- **A host token** — `GH_TOKEN` (GitHub) or `GITEA_TOKEN` (Gitea), with Issues + Pull-requests read/write.

## Build the binary

```bash
make build   # -> bin/themis (static linux binary; FACTORY_ARCH defaults to your host arch,
             #    override e.g. FACTORY_ARCH=amd64 for a cross-arch sandbox)
```

## The sandbox

The factory runs Claude Code with `--dangerously-skip-permissions` so it can work unattended. **The container is the blast radius** — it bounds what the agent can touch to the mounted workspace, nothing else on the host. Never run the factory outside its container, and never on issues or repositories you don't trust to execute code (the green gate runs project-defined shell — see [`CODING_STANDARDS.md` → Trust Model](../../CODING_STANDARDS.md)).

The sandbox image needs: your project's **toolchain** (compiler, test runner) + **Claude Code** + the **`themis` binary** + your repo mounted. Store tokens in a `.env` at the repo root (gitignored) and pass them with `--env-file .env`.

With the binary built and the prerequisites in place, you're installed. **Running the factory comes later** — after you configure a project (next), write a contract or two, and build your sandbox image. Provider setup (`gh` for GitHub, token + MCP for Gitea) is in [`docs/providers.md`](../providers.md); the full command/flag reference is in [`docs/development.md`](../development.md).

→ Next: [Configure a project](02-configure.md)
