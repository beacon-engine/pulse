# syntax=docker/dockerfile:1.4

# ─────────────────────────────── 1. Frontend Builder ─────────────────────────
FROM node:20-bookworm-slim AS frontend-builder
WORKDIR /frontend

# Cache package dependencies
COPY frontend/package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY frontend/ ./
RUN npm run build

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
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agent ./cmd/agent && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

# ─────────────────────────────── Target 1: Agent ─────────────────────────────
# The agent shells out to `docker compose`, so the runtime needs the Docker CLI.
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

ENV BEACON_PORT=9000
EXPOSE 9000
ENTRYPOINT ["agent"]

# ─────────────────────────────── Target 2: Gateway (With Frontend) ───────────
FROM debian:bookworm-slim AS gateway
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

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
ENTRYPOINT ["gateway"]
