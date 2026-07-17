# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-bookworm AS cli-builder
WORKDIR /src/packages/cli
COPY packages/cli/go.mod packages/cli/go.sum ./
RUN go mod download
COPY packages/cli/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/openknowledge ./cmd/openknowledge
RUN mkdir -p /out/artifacts

# Public image: no shell, Git, agent harness, repository, or credentials.
FROM gcr.io/distroless/static-debian12:nonroot AS serve
COPY --from=cli-builder /out/openknowledge /openknowledge
COPY --from=cli-builder --chown=10001:10001 /out/artifacts /artifacts
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/openknowledge", "runtime", "serve"]
CMD ["--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]

# Credentialed private publisher: Git and GitHub App access, but no agent
# harness or model credential.
FROM debian:bookworm-slim AS publisher
RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates git tini \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd --system --gid 10001 openknowledge \
    && useradd --system --uid 10001 --gid openknowledge --home-dir /var/lib/openknowledge --create-home openknowledge
COPY --from=cli-builder /out/openknowledge /usr/local/bin/openknowledge
RUN mkdir -p /var/lib/openknowledge /artifacts /exchange \
	&& chown -R openknowledge:openknowledge /var/lib/openknowledge /artifacts /exchange
USER openknowledge:openknowledge
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge", "runtime", "worker", "--role", "publisher"]
CMD ["--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]

# Private jobs worker base. Each production target adds exactly one pinned
# harness so model credentials and toolchains remain separated by service.
FROM node:22-bookworm-slim AS worker-base
RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates git tini \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd --system --gid 10001 openknowledge \
    && useradd --system --uid 10001 --gid openknowledge --home-dir /var/lib/openknowledge --create-home openknowledge
COPY --from=cli-builder /out/openknowledge /usr/local/bin/openknowledge
COPY docker/runtime-worker-entrypoint.sh /usr/local/bin/openknowledge-worker-entrypoint
RUN chmod 0755 /usr/local/bin/openknowledge-worker-entrypoint \
	&& mkdir -p /var/lib/openknowledge /exchange \
	&& chown -R openknowledge:openknowledge /var/lib/openknowledge /exchange

FROM worker-base AS worker-codex
ARG CODEX_VERSION=0.128.0
RUN npm install --global "@openai/codex@${CODEX_VERSION}" \
    && npm cache clean --force
USER openknowledge:openknowledge
ENV OPENKNOWLEDGE_AGENT_RUNTIME=codex
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-worker-entrypoint"]
CMD ["--role", "jobs", "--runtime", "codex", "--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]

FROM worker-base AS worker-claude
ARG CLAUDE_CODE_VERSION=2.1.212
RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
    && npm cache clean --force
USER openknowledge:openknowledge
ENV OPENKNOWLEDGE_AGENT_RUNTIME=claude
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-worker-entrypoint"]
CMD ["--role", "jobs", "--runtime", "claude", "--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]

FROM worker-base AS worker-opencode
ARG OPENCODE_VERSION=1.18.3
RUN npm install --global "opencode-ai@${OPENCODE_VERSION}" \
    && npm cache clean --force
USER openknowledge:openknowledge
ENV OPENKNOWLEDGE_AGENT_RUNTIME=opencode
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-worker-entrypoint"]
CMD ["--role", "jobs", "--runtime", "opencode", "--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]

FROM worker-base AS worker-grok
ARG GROK_VERSION=0.2.103
RUN npm install --global "@xai-official/grok@${GROK_VERSION}" \
    && npm cache clean --force
USER openknowledge:openknowledge
ENV OPENKNOWLEDGE_AGENT_RUNTIME=grok
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-worker-entrypoint"]
CMD ["--role", "jobs", "--runtime", "grok", "--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]
