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

# ─────────────────────────────── 2. Rust Builder ─────────────────────────────
FROM rust:1-bookworm AS rust-builder
WORKDIR /build

# Include workspace root files
COPY Cargo.toml Cargo.lock* ./
COPY protocol ./protocol
COPY agent ./agent
COPY gateway ./gateway

# Build release binaries for both packages
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    --mount=type=cache,target=/build/target \
    cargo build --release -p agent -p gateway && \
    cp /build/target/release/agent /build/agent-bin && \
    cp /build/target/release/gateway /build/gateway-bin

# ─────────────────────────────── Target 1: Agent ─────────────────────────────
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

COPY --from=rust-builder /build/agent-bin /usr/local/bin/agent

ENV BEACON_PORT=9000
EXPOSE 9000
ENTRYPOINT ["agent"]

# ─────────────────────────────── Target 2: Gateway (With Frontend) ───────────
FROM debian:bookworm-slim AS gateway
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Copy backend binary and frontend static assets
COPY --from=rust-builder /build/gateway-bin /usr/local/bin/gateway
COPY --from=frontend-builder /frontend/dist /app/dist

WORKDIR /data
ENV HTTP_PORT=8080 \
    AGENT_ADDR=agent:9000 \
    STATIC_DIR=/app/dist \
    DATABASE_URL=sqlite:///data/pulse.db

EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["gateway"]
