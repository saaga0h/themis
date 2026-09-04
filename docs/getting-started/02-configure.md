# 2 · Configure a project

Every project the factory works on declares how to build and run it. `themis init` scaffolds that; you fill in the parts only you can know (your verify commands, your toolchain). Themis hardcodes no language — this is where *your* project declares its stack, checks, and sandbox.

## Scaffold it

From your project root:

```bash
themis init          # scaffolds .themis/workflow.yaml, a Containerfile, and .env.example
                     # (existing files are skipped; --force to overwrite)
```

It drops three stack-neutral starting points and prints a next-steps checklist. Nothing is baked to a language — the toolchain is a TODO you fill.

## `.themis/workflow.yaml` — the green gate + run config

```yaml
stack: "unconfigured"          # informational label: go, rust, node, python, ...

verify:
  # The Green Gate: shell commands run in order via `bash -c`; each must exit 0
  # for a run to ship. Replace this with YOUR project's real build/lint/test.
  # The default below fails on purpose so an unconfigured project can't green-gate.
  - "echo 'themis: configure verify' && exit 1"

docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md

image: ""                      # the sandbox image tag you build (below). Required to run.
# runtime: podman              # podman | docker; omit to autodetect
```

- **`verify`** is the heart of it — the commands that decide "done". A Go project might use `go build ./...`, `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test ./...`; a Node project `npm run build`, `npm run lint`, `npm test`; anything else, whatever proves *your* code. Until you replace the default, the gate fails deliberately — the factory won't run on an unconfigured project.
- **`docs`** points at your [contract docs](04-contracts.md) — the authoritative standards the review reads. A fresh repo has none yet: you create `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md` in [step 4](04-contracts.md) — the `contract-drafter` skill drafts a minimal pair, or write them by hand. `themis init` does **not** generate them (they're judgment, not scaffolding); until a `docs:` path exists, review simply skips it.
- **`image`** names the sandbox container image the factory runs in — the tag you build from the `Containerfile` (next section). Required to launch a run.
- **`runtime`** picks the container engine. Omit it to **autodetect — Podman first, Docker as fallback**; set it to `podman` or `docker` to pin one.

Themis's own [`.themis/workflow.yaml`](../../.themis/workflow.yaml) is a worked example.

## The sandbox image (Podman or Docker)

`themis init` also scaffolds a **`Containerfile`**. It's mostly pre-filled; the one part you complete is the **toolchain**, and here's exactly what that means:

> The factory runs your `verify` commands (from `workflow.yaml`) **inside this image**. So the image must contain every tool those commands call — your compiler/interpreter, your test runner, any linter/formatter you verify with. Read your `verify:` list and add a `RUN` line installing each. (git and Claude Code are already in the image; you add the language-specific tools.)

### Per-stack toolchain examples

Drop one of these into the `# TODO: install your project's toolchain` section of the scaffolded Containerfile (the rest of the file — the themis builder stage, the base, Claude Code — stays as generated). The base is `node:22-bookworm` (Debian), so `apt` and prebuilt tarballs both work.

**Go**
```dockerfile
ARG GO_VERSION=1.24.3
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz \
      | tar -C /usr/local -xz \
    && ln -s /usr/local/go/bin/go /usr/local/bin/go
```
Covers `verify` like `go build ./...`, `go vet ./...`, `test -z "$(gofmt -l .)"`, `go test ./...`.

**Python**
```dockerfile
RUN apt-get update && apt-get install -y python3 python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/*
RUN pip3 install --no-cache-dir --break-system-packages pytest ruff
```
Covers `ruff check .`, `pytest`.

**Node / TypeScript**
```dockerfile
# Node is already in the base image. Add global build tools only if your verify needs them:
RUN npm install -g typescript
```
Covers `npm ci`, `npm run build`, `npm test`.

**Rust**
```dockerfile
RUN apt-get update && apt-get install -y cargo rustc \
    && rm -rf /var/lib/apt/lists/*
```
Covers `cargo build`, `cargo test`. (For `cargo clippy`, install via `rustup` instead of `apt`.)

> **Changing the base image?** You can (`FROM your-preferred-base`), but keep **bash** and **git**, and make sure **Node/npm** is available — Claude Code is an npm package, so a base without npm needs a `RUN` to install Node first.

**You don't supply the `themis` binary — the image builds it.** A throwaway `golang` builder stage in the scaffolded Containerfile clones and compiles themis *for this image's architecture* (amd64, arm64, riscv, …), then copies just the binary into the final image (Go stays in the builder — your image doesn't carry it). So there's **no prebuilt binary to fetch and no registry image** — nothing arch-specific to get right, and it builds **once, at image-build time**, not per run.

By default it builds from the public repo (`THEMIS_REPO=https://github.com/saaga0h/themis.git`, `THEMIS_REF=main`). To build from your own host (e.g. a Gitea mirror) or pin a ref, pass build-args:

```bash
podman build --build-arg THEMIS_REPO=<your-git-url> --build-arg THEMIS_REF=<branch-or-tag> -t themis-myproject:latest .
```

Then build the image and point `image:` at the tag. Themis supports **both container engines** — it autodetects, preferring Podman. Build with whichever you use:

```bash
# Podman (the default) — auto-finds the Containerfile:
podman build -t themis-myproject:latest .

# Docker — only auto-finds a file named "Dockerfile", so point it at the Containerfile with -f:
docker build -f Containerfile -t themis-myproject:latest .
```

**Read `-t` carefully:** it names the **image you're building** — a lowercase tag *you choose* (here `themis-myproject:latest`), **not** the `Containerfile` filename. (`podman build -t Containerfile .` fails with *"repository name must be lowercase"* — that's passing the filename as the tag.) The trailing `.` is the build context (the current directory).

Then set that exact tag as `image:` in `workflow.yaml` — e.g. `image: themis-myproject:latest`. If you have both engines and want to force one, set `runtime:`. There's **no registry** — the image is built and stays local, and the `Containerfile` is **yours to customise per project** (add whatever system libs or CLIs your build needs).

## Credentials

`themis init` also writes **`.env.example`** (token *names* only). Copy it to `.env` (gitignored — never commit it) and fill in `CLAUDE_CODE_OAUTH_TOKEN` (Claude Code auth — Themis's one hard dependency) and your provider token (`GH_TOKEN` for GitHub, `GITEA_TOKEN` for Gitea). Provider specifics are in [`docs/providers.md`](../providers.md).

## Factory labels

The factory keys off two labels on your repo's issues: **`ready-for-agent`** (you add it to an issue to hand it to the factory) and **`needs-review`** (the factory adds it when it opens the PR). Create them once:

- **Gitea** — nothing to do; the factory auto-creates them on first use.
- **GitHub** — create them yourself (the `gh` path adds labels by name and errors if they don't exist):
  ```bash
  gh label create ready-for-agent --description "Ready for the factory"
  gh label create needs-review    --description "Factory opened a PR; awaiting review"
  ```

→ Next: [A green baseline](03-project-baseline.md)
