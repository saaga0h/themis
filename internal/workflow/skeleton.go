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
// the user's to customise per project: the toolchain section is a TODO they fill,
// the Themis "agent layer" (the binary + Claude Code + gh) is pre-filled. The
// image is built locally — no registry is involved. See docs/getting-started for
// per-stack toolchain examples.
const containerfile = `# Themis sandbox image for THIS project — build it locally:
#   podman build -t <the image: tag from .themis/workflow.yaml> .
#   docker build -f Containerfile -t <the image: tag> .
# The image needs your project's toolchain (TODO below) plus the fixed Themis
# agent layer (already filled in). See docs/getting-started for per-stack examples.

# A Debian+Node base provides bash, apt, and npm. Claude Code is an npm package
# and Claude is Themis's one hard dependency, so a Node base is the least-friction
# default. Change the base if you prefer — just keep bash, git, and a way to
# install Claude Code and your toolchain.
FROM node:22-bookworm

RUN apt-get update && apt-get install -y git ca-certificates && rm -rf /var/lib/apt/lists/*

# --- TODO: your project's toolchain ----------------------------------------
# Install whatever your .themis/workflow.yaml 'verify' commands need — your
# compiler, test runner, linters. See docs/getting-started for copy-paste
# examples (Go, Node, Python, ...). For example, for Go:
#   ARG GO_VERSION=1.24.3
#   RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz \
#       | tar -C /usr/local -xz && ln -s /usr/local/go/bin/go /usr/local/bin/go
# ---------------------------------------------------------------------------

# --- Themis agent layer (keep this) ----------------------------------------
# The themis LINUX binary: drop a linux build next to this Containerfile (see
# docs/getting-started for where to get it) so this COPY picks it up.
COPY themis /usr/local/bin/themis
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
