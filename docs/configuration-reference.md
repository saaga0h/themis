# Configuration reference

This is the reference for every knob `themis init` sets up: the `themis init`
command itself, the fields of `.themis/workflow.yaml`, the stanzas of the
scaffolded `Containerfile`, and how to configure a language that has no built-in
preset.

You usually don't hand-write any of this — for a supported language (`go`,
`python`, `node`, `rust`) `themis init` **generates a working config**. Reach for
this page when you picked **`other`** (a language without a preset), or when you
want to customize what `init` produced. For the guided walkthrough, see
[Configure a project](getting-started/02-configure.md).

---

## `themis init`

`themis init` scaffolds a project's config: `.themis/workflow.yaml`, a
`Containerfile`, a `.env.example`, and `.gitignore` entries for secrets and
per-run artifacts. It **produces a working config or it refuses** — it never
writes a plausible-but-broken skeleton by default.

**How it dispatches:**

- **No flags, in a terminal** → the **interactive wizard** (the common path).
- **No flags, no terminal** (CI, a pipe) → it **refuses** with a message telling
  you to pass `--language`. It will not emit an unconfigured skeleton that would
  fail on first run.
- **Any content flag** (below) → the **headless** path, driven entirely by flags.

**Flags:**

| Flag | Meaning |
|---|---|
| `--language` | `go`, `python`, `node`, `rust`, or `other`. **Required** for a headless init — it is the floor. A known language composes a working config; `other` writes the intentionally-incomplete skeleton (see below). |
| `--image` | The sandbox image tag written to `image:`. Defaults to `themis-<dir>:latest` (lowercased). |
| `--provider` | `github` or `gitea`. Written uncommented when set; omitted leaves the line commented so it is auto-detected from the git remote at run time. |
| `--runtime` | `podman` or `docker`. Written uncommented when set; omitted leaves it commented (autodetect). |
| `--stack` | Overrides the informational `stack:` label. Defaults to the language. |
| `--force` | Overwrite existing files. Without it, an existing file is left untouched and reported as skipped. |

A content flag is any of `--language/--image/--provider/--runtime/--stack`;
`--force` alone is not one (bare `themis init --force` still runs the wizard).

Headless examples:

```bash
themis init --language go                       # working Go config, defaults for the rest
themis init --language python --image themis-svc:latest --provider github
themis init --language other                    # intentionally-incomplete; you fill it in
```

---

## `.themis/workflow.yaml`

The per-project pipeline descriptor. The factory binary is stack-agnostic:
everything stack-specific is declared here, never hardcoded.

| Field | Type | Meaning |
|---|---|---|
| `stack` | string | Informational label only (`go`, `node`, …). Selects no behavior. |
| `verify` | list of strings | **The Green Gate.** Shell commands run in order via `bash -c` inside the sandbox image; **each must exit 0** for a run to ship. This is your build/lint/test. |
| `docs.standards` | path | Your coding-standards contract, read during review. Unset → skipped. |
| `docs.glossary` | path | Your ubiquitous-language contract, read during review. Unset → skipped. |
| `docs.architecture` | path | Optional architecture doc read during review. |
| `docs.surfaces` | list of globs | When set, the Docs step runs only for changes touching a matching path; when empty, Docs always runs. |
| `provider` | `github` \| `gitea` | The host the factory reads issues from and opens PRs on. Omit to auto-detect from the git remote (github.com → github, any other host → gitea); with no remote it defaults to `github`. The `--provider` flag overrides. |
| `image` | string | The sandbox image tag the factory runs in — built locally from the `Containerfile`. **Required to run.** |
| `runtime` | `podman` \| `docker` | Pins the container runtime. Omit to autodetect (podman, then docker). |
| `footprint_exempt` | list of paths | Files a footprint gate always allows regardless of an issue's declared packages — manifests/lockfiles a legitimate change touches (e.g. `go.mod`/`go.sum`, `package.json`/`package-lock.json`). `themis init` seeds these per language; add your own (e.g. `yarn.lock`) as needed. |

**`verify` is the field that matters most.** It is what "done" means for your
project. Write the commands that prove your code: for a compiled language a
build + vet + test; for anything else, whatever gives you confidence. Every
command runs inside the sandbox image, so the image must contain the tools those
commands call (see the toolchain section below). If `verify` is empty the green
gate is a no-op and the factory warns — an unconfigured project cannot ship.

---

## The `Containerfile`

The sandbox image the factory runs in. `themis init` scaffolds it; you build it
locally (there is no registry image). Its stanzas, top to bottom:

- **Builder stage** — a throwaway `golang` stage clones and compiles the `themis`
  binary *for this image's architecture* (amd64, arm64, riscv, …), so there is
  no prebuilt binary to fetch and nothing arch-specific to get right. Two
  build-args control the source:
  - `THEMIS_REPO` (default `https://github.com/saaga0h/themis.git`) — override to
    build from your own host, e.g. a Gitea mirror.
  - `THEMIS_REF` (default `main`) — pin a branch or tag.

  ```bash
  podman build --build-arg THEMIS_REPO=<git-url> --build-arg THEMIS_REF=<ref> -t <tag> .
  ```

- **Base image** — `node:22-bookworm` (Debian + Node). Node is present because
  Claude Code is an npm package. You may change the base, but keep **bash** and
  **git**, and ensure **npm** is available.
- **Toolchain section** — for a supported language, `init` fills this with the
  install commands your `verify` needs. For `other` it is a **TODO** you complete
  (see the next section). This is the one part that is yours to own.
- **Agent layer** — copies the `themis` binary from the builder stage and
  installs Claude Code (`npm install -g @anthropic-ai/claude-code`). Keep this.
  GitHub projects also need the `gh` CLI; the scaffold leaves a commented line
  for it.

**Building it.** `-t` names the **image you are building** — a lowercase tag *you
choose* — not the Containerfile's filename:

```bash
# Podman (default) — auto-finds the Containerfile:
podman build -t themis-myproject:latest .

# Docker — point it at the file with -f (it only auto-finds "Dockerfile"):
docker build -f Containerfile -t themis-myproject:latest .
```

`podman build -t Containerfile .` fails with *"repository name must be
lowercase"* — that is passing the filename as the tag. Set the tag you build as
`image:` in `workflow.yaml`.

---

## Custom configuration — a language without a preset (`other`)

`themis init --language other` (or picking **Other** in the wizard) writes a
deliberately-incomplete config: `verify` fails on purpose, and the Containerfile
toolchain section is a TODO. This is the **only** path `init` leaves incomplete,
and it is by explicit request. Two things to fill in:

**1. `verify` in `.themis/workflow.yaml`** — replace the failing placeholder with
the commands that prove your code, in order, each exiting 0 on success. The shape
is a YAML list:

```yaml
verify:
  - <build command>
  - <lint/format check>
  - <test command>
```

Keep them fast and deterministic — they run on every factory attempt. A check
that only *reports* (e.g. a formatter that rewrites files) should be turned into
one that *fails* on a violation (e.g. "formatting differs"), so the gate can
catch it.

**2. The toolchain in the `Containerfile`** — the factory runs your `verify`
commands *inside the image*, so it must contain every tool they call: your
compiler or interpreter, your test runner, any linter/formatter. Add a `RUN` line
per tool in the toolchain TODO section. The base is Debian, so `apt-get install`
and prebuilt tarballs both work.

To see a complete, working example of both files, run `themis init --language go`
in a scratch directory and read what it generates — the supported-language
presets are the reference implementation of this shape.

---

## Credentials (`.env`)

`themis init` writes `.env.example` with the token **names** only. Copy it to
`.env` (gitignored — never commit it) and fill in:

- **`CLAUDE_CODE_OAUTH_TOKEN`** — Claude Code auth (Themis's one hard
  dependency). Generate it with `claude setup-token`.
- **Your provider token** — `GH_TOKEN` (GitHub) or `GITEA_TOKEN` (Gitea), with
  Issues + Pull-requests read/write.

Provider-specific setup (the `gh` CLI, the Gitea MCP, labels) is in
[`docs/providers.md`](providers.md).
