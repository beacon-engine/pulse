# syntax=docker/dockerfile:1.4

# ─────────────────────────────── 1. Frontend Builder ─────────────────────────
FROM node:24-bookworm-slim AS frontend-builder
WORKDIR /frontend

# Cache package dependencies
COPY frontend/package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY frontend/ ./
# Shown in the dashboard. CI passes the release version (see VERSION).
ARG PULSE_VERSION=dev
RUN PULSE_VERSION=${PULSE_VERSION} npm run build

# ─────────────────────────────── 2. Go Builder ───────────────────────────────
FROM golang:1.26-bookworm AS go-builder
WORKDIR /build

# Cache modules first, then build. CGO is off so the binaries are fully static
# (pure-Go SQLite, plus the Docker SDK and client-go) — they run on a slim base
# with no libc surprises.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd ./cmd
COPY internal ./internal
# Linked into both binaries. CI passes the release version (see VERSION).
ARG PULSE_VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    LDFLAGS="-s -w -X github.com/beacon-engine/pulse/internal/version.Version=${PULSE_VERSION}" && \
    CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" -o /out/agent ./cmd/agent && \
    CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" -o /out/gateway ./cmd/gateway

# ─────────────────────────────── Target 1: Agent ─────────────────────────────
# The agent shells out to `docker compose`, so the runtime needs the Docker CLI.
#
# It runs as root on purpose: it drives the host's Docker socket, whose group id
# differs per host, so a fixed non-root user can't reach it. In Kubernetes,
# where no socket is mounted, the manifests run it as a non-root user instead.
# Its liveness is the gateway session (push) or the gateway's dial (pull), both
# of which reconnect on their own, so there is no HEALTHCHECK.
# checkov:skip=CKV_DOCKER_3: agent needs the host Docker socket (see above)
FROM debian:bookworm-slim AS agent
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
 && install -m 0755 -d /etc/apt/keyrings \
 && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
 && chmod a+r /etc/apt/keyrings/docker.asc \
 && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian bookworm stable" \
      > /etc/apt/sources.list.d/docker.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin \
 && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/agent /usr/local/bin/agent

ENV PULSE_PORT=9000
EXPOSE 9000
ENTRYPOINT ["agent"]

# ─────────────────────────────── Target 2: Gateway (With Frontend) ───────────
FROM debian:bookworm-slim AS gateway
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --system --gid 10001 pulse \
 && useradd --system --uid 10001 --gid pulse --home-dir /data --shell /usr/sbin/nologin pulse \
 && mkdir -p /data && chown pulse:pulse /data

# Copy backend binary and frontend static assets
COPY --from=go-builder /out/gateway /usr/local/bin/gateway
COPY --from=frontend-builder /frontend/dist /app/dist

WORKDIR /data
ENV HTTP_PORT=8080 \
    AGENT_ADDR=agent:9000 \
    STATIC_DIR=/app/dist \
    DATABASE_URL=sqlite:///data/pulse.db

EXPOSE 8080
VOLUME ["/data"]
USER pulse
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["gateway", "healthcheck"]
ENTRYPOINT ["gateway"]

# ─────────────────────────────── Dev (hot-reload + debug) ────────────────────
# Used by docker-compose.dev.yml: source is bind-mounted, `air` rebuilds on save,
# and `dlv` is available for in-container breakpoint debugging (DAP on 2345/2346).
FROM golang:1.26-bookworm AS dev
# Docker CLI so the agent can run `docker compose` during dev.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
 && install -m 0755 -d /etc/apt/keyrings \
 && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
 && chmod a+r /etc/apt/keyrings/docker.asc \
 && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian bookworm stable" \
      > /etc/apt/sources.list.d/docker.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin \
 && rm -rf /var/lib/apt/lists/*
RUN go install github.com/air-verse/air@latest \
 && go install github.com/go-delve/delve/cmd/dlv@latest
WORKDIR /app
# Source is bind-mounted at runtime; air + dlv are on PATH (/go/bin).
CMD ["air", "-c", ".air.gateway.toml"]
