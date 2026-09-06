# 1 · Install

You can't run the factory yet — it needs a configured project and a built sandbox image first (the next steps). This step just gets the `themis` binary and the prerequisites in place.

## Prerequisites

- **Go** — only if you build from source or use `go install`; not needed if you download a release binary.
- **Podman (rootless) or Docker** — the sandbox. `--userns=keep-id` is required for rootless Podman.
- **`CLAUDE_CODE_OAUTH_TOKEN`** — generated on your host with `claude setup-token`. Themis's one hard dependency.
- **A host token** — `GH_TOKEN` (GitHub) or `GITEA_TOKEN` (Gitea), with Issues + Pull-requests read/write.

## Get the `themis` binary

You need `themis` on your machine to run `themis init` and the factory launcher. This is the **host** binary for your own OS — the *sandbox* binary is built from source inside the container (see [Configure](02-configure.md)), so you don't build that yourself. Pick one:

- **Download a release (recommended).** Grab the binary for your OS/arch from the [GitHub releases](https://github.com/saaga0h/themis/releases), **verify it against `SHA256SUMS`**, then put it on your `PATH`:
  ```bash
  # adjust the filename to your platform (…-darwin-arm64, …-linux-amd64, …-windows-amd64.exe, …)
  curl -fsSLO https://github.com/saaga0h/themis/releases/latest/download/themis-linux-amd64
  curl -fsSLO https://github.com/saaga0h/themis/releases/latest/download/SHA256SUMS
  sha256sum -c --ignore-missing SHA256SUMS      # Linux; macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
  install -m755 themis-linux-amd64 /usr/local/bin/themis
  ```
  Binaries are static (`CGO_ENABLED=0`) — no libc dependency. **Always verify the checksum before trusting a downloaded binary.**

- **`go install`** (if you have Go): `go install github.com/saaga0h/themis/cmd/themis@latest`

- **Build from source:**
  ```bash
  go build -o themis ./cmd/themis   # for your current OS/arch
  make build                        # -> bin/themis (static linux/<host arch>)
  ```

## The sandbox

The factory runs Claude Code with `--dangerously-skip-permissions` so it can work unattended. **The container is the blast radius** — it bounds what the agent can touch to the mounted workspace, nothing else on the host. Never run the factory outside its container, and never on issues or repositories you don't trust to execute code (the green gate runs project-defined shell — see [`CODING_STANDARDS.md` → Trust Model](../../CODING_STANDARDS.md)).

The sandbox image needs: your project's **toolchain** (compiler, test runner) + **Claude Code** + the **`themis` binary** + your repo mounted. You don't assemble this by hand — `themis init` scaffolds the `Containerfile` (Claude Code and the `themis` binary are built in automatically; for a supported language the toolchain is filled in too), and you build the image from it in the next step. Store tokens in a `.env` at the repo root (gitignored) and pass them with `--env-file .env`. Every field and stanza is documented in the [configuration reference](../configuration-reference.md).

With the binary built and the prerequisites in place, you're installed. **Running the factory comes later** — after you configure a project (next), write a contract or two, and build your sandbox image. Provider setup (`gh` for GitHub, token + MCP for Gitea) is in [`docs/providers.md`](../providers.md); the full command/flag reference is in [`docs/development.md`](../development.md).

→ Next: [Configure a project](02-configure.md)
