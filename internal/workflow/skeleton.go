package workflow

import (
	"os"
	"path/filepath"
	"strings"
)

// workflowTmpl renders .themis/workflow.yaml. Everything language-specific comes
// from the InitConfig / preset (see compose.go); the factory binary stays
// stack-agnostic. Language names appear only in comments, never hardcoded as the
// live stack: value.
const workflowTmpl = `# .themis/workflow.yaml — this project's pipeline descriptor.
# The factory binary is stack-agnostic: everything stack-specific (the Green
# Gate commands, the authoritative docs) is declared here, not hardcoded.

# stack is an informational label only — it selects no behavior. Examples:
# go, node, python, rust, java, ruby, javascript, typescript, c++.
stack: "{{.Stack}}"

# verify is the Green Gate: shell commands run in order via ` + "`bash -c`" + `;
# each must exit 0 for a run to ship. Replace these with your project's real
# build/lint/test commands. See docs/configuration-reference.md.
{{.VerifyBlock}}

# docs names the authoritative documents agents read during review. Paths are
# relative to the project root; unset fields are skipped.
docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md
  architecture: ""

# provider is the host this repo lives on — where the factory reads issues and
# opens PRs. Auto-detected from your git remote (github.com -> github, any other
# host -> gitea), so you usually don't set it. The --provider flag also overrides.
{{.ProviderLine}}

# image is the sandbox container image the factory runs in — built locally from
# the Containerfile in this project. Required to run.
image: "{{.Image}}"

# runtime optionally pins the container runtime: podman or docker. Leave unset
# to autodetect (podman, then docker).
{{.RuntimeLine}}
`

// containerfileTmpl renders the scaffolded Containerfile. themis is built from
// source in a throwaway builder stage so the binary matches the image's
// architecture (amd64/arm64/riscv/…). The toolchain block is substituted per
// language (compose.go); no per-language examples live here — those are in the
// preset table (internal/preset) and the reference docs.
const containerfileTmpl = `# Themis sandbox image for THIS project — build it locally:
#   podman build -t <the image: tag from .themis/workflow.yaml> .
#   docker build -f Containerfile -t <the image: tag> .
# themis is built from source in the builder stage below, so it matches THIS
# image's architecture — amd64, arm64, riscv, etc. — with no prebuilt binary and
# no registry image. Go stays in the builder; the final image is Go-free unless
# your toolchain adds it. See docs/configuration-reference.md for details.

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

{{.ToolchainBlock}}

# --- Themis agent layer (keep this) ----------------------------------------
# The themis binary, built above for this image's architecture:
COPY --from=themis-build /themis /usr/local/bin/themis
# Claude Code — Themis's one hard dependency (npm keeps the fetch integrity-checked).
RUN npm install -g @anthropic-ai/claude-code
# GitHub projects also need the gh CLI (Gitea needs no provider CLI). Uncomment
# and see docs/configuration-reference.md for the install snippet:
# RUN <install gh — see docs/configuration-reference.md>
# ---------------------------------------------------------------------------
`

// toolchainTODO is the Containerfile toolchain block for the "other"/unconfigured
// path — a TODO that points at the reference docs, with no per-language examples
// (those are generated for known languages and documented in the reference).
const toolchainTODO = `# --- TODO: install your project's toolchain --------------------------------
# The factory runs the 'verify' commands from your .themis/workflow.yaml *inside
# this image*, so it must contain every tool those commands call — your
# compiler/interpreter, test runner, and any linter/formatter you verify with.
# git and Claude Code are already here (below); you add the language tools.
#
# Add a RUN line per tool your verify: list invokes. See
# docs/configuration-reference.md for how to write this for your language.
# ---------------------------------------------------------------------------`

// toolchainGenerated wraps a preset's toolchain stanza with a header naming the
// language it was generated for. Args: language id, preset toolchain stanza.
const toolchainGenerated = `# --- Project toolchain (generated for %s) ----------------------------------
# The factory runs your workflow.yaml verify commands inside this image, so it
# needs your language's tools. Generated from the themis preset — edit freely.
%s
# ---------------------------------------------------------------------------`

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

// WriteSkeleton scaffolds the neutral (unconfigured) .themis/workflow.yaml in dir
// — the "other"/no-preset path. Known languages use WriteWorkflow with a
// language InitConfig. See writeScaffold for the force/skip semantics.
func WriteSkeleton(dir string, force bool) (created bool, err error) {
	return WriteWorkflow(dir, neutralConfig(), force)
}

// WriteContainerfile scaffolds the neutral Containerfile (toolchain TODO) in dir.
// Known languages use WriteContainerfileFor. Same force/skip semantics.
func WriteContainerfile(dir string, force bool) (created bool, err error) {
	return WriteContainerfileFor(dir, neutralConfig(), force)
}

// WriteEnvExample scaffolds a .env.example in dir (same force/skip semantics).
func WriteEnvExample(dir string, force bool) (created bool, err error) {
	return writeScaffold(filepath.Join(dir, ".env.example"), envExample, force)
}

// gitignoreEntries are the paths themis needs git-ignored: the secrets file
// (never commit .env) and the factory's per-run artifacts. .themis/workflow.yaml
// and the scaffolded Containerfile/.env.example stay committed.
var gitignoreEntries = []string{
	".env",
	".themis/state.json",
	".themis/review-results.json",
	".themis/factory-cc/",
}

// EnsureGitignore makes sure dir/.gitignore contains the themis entries, appending
// any that are missing (creating the file if absent). Existing content is
// preserved verbatim — it never removes or reorders. Returns changed=true when it
// wrote. This is how `themis init` keeps .env (secrets) out of the repository.
func EnsureGitignore(dir string) (changed bool, err error) {
	path := filepath.Join(dir, ".gitignore")
	existing := ""
	if data, rerr := os.ReadFile(path); rerr == nil {
		existing = string(data)
	} else if !os.IsNotExist(rerr) {
		return false, rerr
	}

	present := map[string]bool{}
	for _, line := range strings.Split(existing, "\n") {
		present[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, e := range gitignoreEntries {
		if !present[e] {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}

	var b strings.Builder
	b.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	if existing != "" {
		b.WriteString("\n")
	}
	b.WriteString("# Themis — secrets and per-run artifacts (keep out of the repo)\n")
	for _, e := range missing {
		b.WriteString(e + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
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
