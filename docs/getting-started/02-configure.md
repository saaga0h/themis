# 2 · Configure a project

Every project the factory works on declares how to build and run it. `themis init`
sets that up for you — for a supported language it produces a **working** config
you can build on immediately, rather than a form to fill in. Themis hardcodes no
language; this is where *your* project declares its stack, checks, and sandbox.

For the meaning of every field and flag, see the
[configuration reference](../configuration-reference.md). This page is the
walkthrough.

## Run it (interactive)

From your project root, in a terminal:

```bash
themis init
```

It asks three things — and only the first is a real decision:

1. **Language** — `Go`, `JavaScript / TypeScript`, `Python`, `Rust`, or `Other`.
2. **Sandbox image tag** — pre-filled with `themis-<dir>:latest`; press Enter to accept.
3. **Provider** — pre-selected from your git remote (GitHub by default); press Enter to confirm.

Then it offers to **install the interactive skills** (into this project, globally,
or skip). For a supported language, that's it — everything is generated and ready.

`themis init` writes four things:

- **`.themis/workflow.yaml`** — the green gate (`verify`) + run config, filled in for your language.
- **`Containerfile`** — the sandbox image, with your language's toolchain already installed.
- **`.env.example`** — the credential names to copy into `.env`.
- **`.gitignore`** entries — so `.env` and per-run artifacts never get committed.

Existing files are left untouched (reported as *skipped*) unless you pass `--force`.

## Or headless (scripts / CI)

Pass flags instead and `init` runs without prompting:

```bash
themis init --language go
themis init --language python --image themis-svc:latest --provider github
```

`--language` is required in headless mode — it's the floor. `init` produces a
working config or **refuses**; it never writes a broken skeleton by default (a
no-terminal run with no flags errors and tells you to pass `--language`). The full
flag list is in the [configuration reference](../configuration-reference.md#themis-init).

## The `Other` language

If your language isn't one of the built-in presets, choose **Other** (or
`--language other`). This is the **one** case `init` leaves incomplete on purpose:
`verify` fails until you set it, and the Containerfile toolchain is a TODO. It
tells you so, and points at the
[custom-configuration section](../configuration-reference.md#custom-configuration--a-language-without-a-preset-other),
which walks through writing your own `verify` commands and toolchain.

## Build the sandbox image

`init` scaffolds the `Containerfile`; you build the image locally (there is no
registry image — the `themis` binary is compiled inside the image for your
architecture). Build with whichever engine you use:

```bash
# Podman (the default) — auto-finds the Containerfile:
podman build -t themis-myproject:latest .

# Docker — point it at the Containerfile with -f:
docker build -f Containerfile -t themis-myproject:latest .
```

`-t` names the **image you're building** — a lowercase tag *you choose* — not the
`Containerfile` filename. (`podman build -t Containerfile .` fails with
*"repository name must be lowercase"*.) Use the same tag you chose for `image:` in
`workflow.yaml`. To build `themis` from your own git host instead of the public
repo, pass `--build-arg THEMIS_REPO=…` — see the
[Containerfile reference](../configuration-reference.md#the-containerfile).

## Credentials

Copy `.env.example` to `.env` (gitignored — never commit it) and fill in two secrets:

- **`CLAUDE_CODE_OAUTH_TOKEN`** — Claude Code auth. Generate it with `claude setup-token`.
- **Your provider token** — `GH_TOKEN` (GitHub) or `GITEA_TOKEN` (Gitea), with Issues + Pull-requests read/write.

Provider specifics (the `gh` CLI, the Gitea MCP, the factory labels) are in
[`docs/providers.md`](../providers.md).

## Install the interactive skills

The rest of the workflow — shaping the project, writing contracts, authoring
issues, reviewing PRs — is driven by Themis's **skills**, which Claude Code loads
from `.claude`. The `init` wizard offers to install them; you can also do it
anytime:

```bash
themis skills install            # into this project's .claude/
themis skills install --global   # into ~/.claude, shared across projects
```

**Reload Claude Code** afterward so it picks them up.

→ Next: [A green baseline](03-project-baseline.md)
