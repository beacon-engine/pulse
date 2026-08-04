# syntax=docker/dockerfile:1
#
# Multi-target build for the Pulse backend. Produces two images from one
# builder stage:
#   docker build --target agent   -t <user>/pulse-agent   .
#   docker build --target gateway -t <user>/pulse-gateway .
#
# The frontend has its own image — see frontend/Dockerfile.

# ─────────────────────────────── Builder ────────────────────────────────────
FROM rust:1-bookworm AS builder
WORKDIR /build

# The `protocol` crate is a path dependency of both binaries.
COPY protocol ./protocol
COPY agent ./agent
COPY gateway ./gateway

# Build both release binaries. sqlx migrations are embedded at compile time via
# `sqlx::migrate!`, so the gateway binary is self-contained.
RUN cargo build --release --manifest-path agent/Cargo.toml \
 && cargo build --release --manifest-path gateway/Cargo.toml

# ─────────────────────────────── Agent ──────────────────────────────────────
# The agent shells out to `docker compose` for stack up/down, so it needs the
# Docker CLI + compose plugin. Mount the host's Docker socket at runtime.
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
COPY --from=builder /build/agent/target/release/agent /usr/local/bin/agent
ENV BEACON_PORT=9000
EXPOSE 9000
ENTRYPOINT ["agent"]

# ─────────────────────────────── Gateway ────────────────────────────────────
FROM debian:bookworm-slim AS gateway
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/gateway/target/release/gateway /usr/local/bin/gateway
WORKDIR /data
ENV HTTP_PORT=8080 \
    AGENT_ADDR=agent:9000 \
    DATABASE_URL=sqlite:///data/pulse.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["gateway"]
