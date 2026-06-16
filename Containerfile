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

# tea CLI for Gitea interaction
ARG TEA_VERSION=0.9.2
RUN GOARCH=$(dpkg --print-architecture) && \
    curl -fsSL "https://dl.gitea.com/tea/${TEA_VERSION}/tea-${TEA_VERSION}-linux-${GOARCH}" \
    -o /usr/local/bin/tea && chmod +x /usr/local/bin/tea

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

# Verify Go is accessible as the agent user
RUN go version

# Install Claude Code CLI — baked into image, not at runtime
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/agent/.local/bin:$PATH"

WORKDIR /home/agent

ENTRYPOINT ["sleep", "infinity"]
