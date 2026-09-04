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

`themis init` also scaffolds a **`Containerfile`** with a **TODO for your toolchain** (the compiler/test tools your `verify` commands need — the file has an inline Go example, and there are more per-stack examples in the docs). Fill the TODO; the rest is pre-filled.

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

→ Next: [A green baseline](03-project-baseline.md)
