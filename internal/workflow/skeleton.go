package workflow

import (
	"os"
	"path/filepath"
)

// skeleton is the scaffolded .themis/workflow.yaml written by `themis init`.
// The stack value and verify command are deliberately unconfigured — a fresh
// project must not green-gate until someone edits this file with real
// commands. Language names appear only in the comment, never as the live
// stack: value, so the factory stays stack-agnostic.
const skeleton = `# .themis/workflow.yaml — this project's pipeline descriptor.
# The factory binary is stack-agnostic: everything stack-specific (the Green
# Gate commands, the authoritative docs) is declared here, not hardcoded.

# stack is an informational label only — it selects no behavior. Examples:
# go, node, python, rust, java, ruby, javascript, typescript, c++.
stack: "unconfigured"

# verify is the Green Gate: shell commands run in order via ` + "`bash -c`" + `;
# each must exit 0 for a run to ship. The command below fails on purpose so
# an unconfigured project cannot green-gate by accident — replace it with
# your project's real build/lint/test commands, e.g.:
#   - go build ./... && go vet ./... && go test ./...
#   - npm ci && npm run build && npm test
#   - pip install -e . && pytest
verify:
  - "echo 'themis: no verify commands configured in .themis/workflow.yaml' && exit 1"

# docs names the authoritative documents agents read during review. Paths are
# relative to the project root; unset fields are skipped.
docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md
  architecture: ""

# image is the sandbox container image the factory runs in — built locally from
# the Containerfile in this project (see the next-steps from 'themis init').
# Set it to the tag you build, e.g. themis-myproject:latest. Required to run.
image: ""

# runtime optionally pins the container runtime: podman or docker. Leave unset
# to autodetect (podman, then docker).
# runtime: podman
`

// containerfile is the scaffolded Containerfile written by `themis init`. It is
// the user's to customise per project: the toolchain section is a TODO they fill.
// themis is built from source in a throwaway builder stage so the binary matches
// the image's architecture (amd64/arm64/riscv/…) with no prebuilt binary or
// registry image — the repo URL is a build-arg defaulting to the public GitHub.
// See docs/getting-started for per-stack toolchain examples.
const containerfile = `# Themis sandbox image for THIS project — build it locally:
#   podman build -t <the image: tag from .themis/workflow.yaml> .
#   docker build -f Containerfile -t <the image: tag> .
# themis is built from source in the builder stage below, so it matches THIS
# image's architecture — amd64, arm64, riscv, etc. — with no prebuilt binary and
# no registry image. Go stays in the builder; the final image is Go-free unless
# your toolchain adds it. See the Themis getting-started docs for per-stack examples.

# --- themis agent binary: built once at image-build time, for this arch --------
# Override THEMIS_REPO to build from your own host (e.g. a Gitea mirror) and
# THEMIS_REF to pin a branch or tag:
#   podman build --build-arg THEMIS_REPO=<git-url> --build-arg THEMIS_REF=<ref> -t <tag> .
ARG THEMIS_REPO=https://github.com/saaga0h/themis.git
ARG THEMIS_REF=main
# The builder's Go must be >= themis's go.mod version. GOTOOLCHAIN=auto lets Go
# fetch a newer toolchain automatically if go.mod moves ahead of this base, so
# this doesn't silently break on a future Go bump.
FROM golang:1.25-bookworm AS themis-build
ENV GOTOOLCHAIN=auto
ARG THEMIS_REPO
ARG THEMIS_REF
RUN git clone --depth 1 --branch "${THEMIS_REF}" "${THEMIS_REPO}" /src \
    && cd /src && CGO_ENABLED=0 go build -o /themis ./cmd/themis
# -------------------------------------------------------------------------------

# A Debian+Node base provides bash, apt, and npm. Claude Code is an npm package
# and Claude is Themis's one hard dependency, so a Node base is the least-friction
# default. Change the base if you prefer — just keep bash and git.
FROM node:22-bookworm

RUN apt-get update && apt-get install -y git ca-certificates && rm -rf /var/lib/apt/lists/*

# --- TODO: install your project's toolchain --------------------------------
# WHAT THIS IS: the factory runs the 'verify' commands from your
# .themis/workflow.yaml *inside this image*. So this image must contain every
# tool those commands call — your language's compiler/interpreter, its test
# runner, and any formatter/linter you verify with. git and Claude Code are
# already here (below); you add the language-specific tools.
#
# HOW: read your workflow.yaml 'verify:' list and add a RUN line installing each
# tool it invokes. Examples (adapt to your stack; this base is Debian + Node):
#   Go:     RUN curl -fsSL https://go.dev/dl/go1.25.0.linux-$(dpkg --print-architecture).tar.gz | tar -C /usr/local -xz \
#             && ln -s /usr/local/go/bin/go /usr/local/bin/go
#   Python: RUN apt-get update && apt-get install -y python3 python3-pip && rm -rf /var/lib/apt/lists/*
#   Rust:   RUN apt-get update && apt-get install -y cargo && rm -rf /var/lib/apt/lists/*
#   Node:   already installed (this base image is node:22-bookworm)
# Full copy-paste examples per stack are in the Themis getting-started docs
# ("Configure a project").
# ---------------------------------------------------------------------------

# --- Themis agent layer (keep this) ----------------------------------------
# The themis binary, built above for this image's architecture:
COPY --from=themis-build /themis /usr/local/bin/themis
# Claude Code — Themis's one hard dependency (npm keeps the fetch integrity-checked).
RUN npm install -g @anthropic-ai/claude-code
# GitHub projects also need the gh CLI (Gitea needs no provider CLI). Uncomment
# and see docs/getting-started for the install snippet:
# RUN <install gh — see docs/getting-started>
# ---------------------------------------------------------------------------
`

// envExample is the scaffolded .env.example (token NAMES only, never values).
// The real values go in a .env file that must not be committed.
const envExample = `# Themis credentials. Copy to .env (do NOT commit .env) and fill in real values.

# Claude Code authentication — Themis's one hard dependency.
CLAUDE_CODE_OAUTH_TOKEN=

# Provider token — set the one for your host:
#   GitHub:
GH_TOKEN=
#   Gitea:
# GITEA_TOKEN=
`

// WriteSkeleton scaffolds .themis/workflow.yaml in dir. See writeScaffold for the
// force/skip semantics.
func WriteSkeleton(dir string, force bool) (created bool, err error) {
	return writeScaffold(filepath.Join(dir, workflowFile), skeleton, force)
}

// WriteContainerfile scaffolds a Containerfile in dir (same force/skip semantics).
func WriteContainerfile(dir string, force bool) (created bool, err error) {
	return writeScaffold(filepath.Join(dir, "Containerfile"), containerfile, force)
}

// WriteEnvExample scaffolds a .env.example in dir (same force/skip semantics).
func WriteEnvExample(dir string, force bool) (created bool, err error) {
	return writeScaffold(filepath.Join(dir, ".env.example"), envExample, force)
}

// writeScaffold writes content to path. Without force, an existing file is left
// byte-for-byte unchanged and created=false is returned; with force, it always
// overwrites. Parent directories are created as needed.
func writeScaffold(path, content string, force bool) (created bool, err error) {
	if !force {
		if _, statErr := os.Stat(path); statErr == nil {
			return false, nil
		} else if !os.IsNotExist(statErr) {
			return false, statErr
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
