FROM node:22-bookworm

# System dependencies
RUN apt-get update && apt-get install -y \
  git \
  curl \
  jq \
  && rm -rf /var/lib/apt/lists/*

# Install Go
ARG GO_VERSION=1.24.3
RUN GOARCH=$(dpkg --print-architecture) && \
    curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Git environment defaults so a freshly-cloned target repo works out of the box:
# trust the mounted workspace (--userns=keep-id makes the bind-mount's owner differ
# from the in-container user, tripping git's dubious-ownership guard) and provide a
# default commit identity (the factory's commits fail with "author identity unknown"
# otherwise). --system is written while root so it applies whatever UID keep-id maps
# to; both are overridable per-repo (.git/config) or per-run (GIT_* env). The sandbox
# only ever mounts the operator's own repos.
RUN git config --system --add safe.directory /home/agent/workspace && \
    git config --system user.name "Themis Factory" && \
    git config --system user.email "[email protected]"

# Claude Code CLI — installed via npm so the fetch is integrity-verified (npm
# registry checksums), never a pipe-to-shell (the CLAUDE.md supply-chain rule).
# Runs in the root layer because `npm install -g` writes to /usr/local (already
# on PATH). The version is optional: it defaults to the latest release; pin a
# specific version for a reproducible build with
# --build-arg CLAUDE_CODE_VERSION=2.1.89.
ARG CLAUDE_CODE_VERSION=latest
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} && claude --version

# Rename the base image's "node" user to "agent" and align UID/GID.
# At runtime, --userns=keep-id maps the host user into the container.
ARG AGENT_UID=1000
ARG AGENT_GID=1000
RUN groupmod -g $AGENT_GID node && usermod -u $AGENT_UID -g $AGENT_GID -d /home/agent -m -l agent node
USER ${AGENT_UID}:${AGENT_GID}

# Go cache dirs under agent home.
# GOPROXY is not set here — it comes from the environment (.env file)
# so module downloads route through Athens if configured.
ENV GOPATH="/home/agent/go"
ENV GOCACHE="/home/agent/.cache/go-build"
ENV GOMODCACHE="/home/agent/go/pkg/mod"
# Read-only module mode for EVERY go command in the sandbox — the factory's verify
# gate AND the agent's own build/test/vet during TestRed/Implement. In writable mode
# a `go build ./...` records the full module graph's /go.mod hashes into go.sum
# (spurious drift the committed, pruned go.sum omits), which dirties the tree and
# fails a step's clean-tree checkpoint. readonly stops the rewrite and still builds;
# a genuinely missing entry fails loudly (correct — the factory should not silently
# modify go.sum; a real dependency addition is a human decision).
ENV GOFLAGS="-mod=readonly"

# Verify Go is accessible as the agent user
RUN go version

WORKDIR /home/agent

ENTRYPOINT ["sleep", "infinity"]
