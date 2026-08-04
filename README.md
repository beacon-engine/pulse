# Pulse

**A lightweight, high-performance container management platform with native observability**

[![Rust](https://img.shields.io/badge/rust-1.85%2B-orange.svg)](https://www.rust-lang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## What is Pulse?

Pulse is a modern alternative to Portainer, built in Rust to manage Docker,
Docker Swarm, and Kubernetes from one dashboard with first-class observability
and alerting.

- **Lightweight** — Rust core, no garbage collector or heavy runtime (~20 MB RSS per process; see [Benchmarks](#benchmarks))
- **Split control/executor design** — a public HTTP gateway backed by an agent that owns cluster access
- **First-class observability** — Prometheus metrics, live WebSocket events, Slack alerting
- **API-first** — every dashboard action is a documented HTTP endpoint

---

## Architecture

Pulse is three Rust crates plus a Svelte frontend:

```
┌──────────┐   HTTP/JSON    ┌───────────┐   TCP (framed)   ┌─────────┐   Docker API   ┌────────┐
│ frontend │ ─────────────▶ │  gateway  │ ───────────────▶ │  agent  │ ─────────────▶ │ Docker │
│ (Svelte) │ ◀───────────── │ HTTP + DB │ ◀─────────────── │executor │ ◀───────────── │ Swarm  │
└──────────┘   cookies/WS   └───────────┘   Request/Resp   └─────────┘   kube API      │  k8s   │
                                  │                                                     └────────┘
                              SQLite DB
                     (users, hosts, stacks, tokens, events)
```

- **`gateway/`** — the frontend-facing **HTTP API**. Owns the database
  (users, hosts, stacks, webhook tokens, events), authentication, Slack
  alerting, the Prometheus proxy, WebSocket fan-out, and metrics. It holds **no**
  Docker access itself — it resolves each host to a `docker_host` connection
  string and forwards the operation to the agent.
- **`agent/`** — a **stateless executor** (crate `pulse-core`) that owns Docker,
  Docker Swarm, and Kubernetes access. It listens on a **binary TCP** protocol
  (not HTTP) and answers `Request`s from the gateway with `Response`s. It keeps
  no database.
- **`protocol/`** — shared wire types (the `Request`/`Response` enums, domain
  models, and length-prefixed framing) linked by both sides, so the vocabulary
  is defined exactly once.
- **`frontend/`** — Svelte 5 + Vite dashboard; talks only to the gateway.

> Nothing should send HTTP to the agent's TCP port — it only speaks the binary
> gateway protocol. (A stray HTTP client there is logged and dropped.)

---

## Supported container systems

| System           | Support | Notes |
| ---------------- | ------- | ----- |
| **Docker**       | ✅ Full | containers, images, volumes, networks, logs, exec, compose stacks |
| **Docker Swarm** | ✅ Full | services, nodes, force rolling update |
| **Kubernetes**   | 🟡 Partial | pods, namespaces, deployment rollout (via kubeconfig / in-cluster) |
| **Podman**       | 🟡 Partial | works via Podman's Docker-compatible socket for container/image/volume ops; no Swarm; compose needs `podman compose` |
| containerd / nerdctl / LXC | ❌ | not supported |

See **[docs/LOCAL_ORCHESTRATORS.md](docs/LOCAL_ORCHESTRATORS.md)** for setting up
Swarm and Kubernetes locally in isolation to test these integrations.

---

## Module status

### ✅ Completed

- [x] **Gateway/agent split** — HTTP control plane + TCP executor over a shared `protocol` crate
- [x] **Docker** — list/inspect, start/stop/restart, recreate, env update, exec, logs
- [x] **Images / volumes / networks** — list, delete, prune, connect/disconnect
- [x] **Compose stacks** — saved stacks, up/down, detected stacks
- [x] **Docker Swarm** — services, nodes, force update
- [x] **Kubernetes** — pods, namespaces, rollout restart
- [x] **Multi-host** — manage several Docker endpoints from one gateway
- [x] **Webhooks** — token-protected recreate / redeploy / swarm update / k8s rollout
- [x] **Observability** — Prometheus `/metrics`, Prometheus proxy, live WebSocket events
- [x] **Slack alerting** — container crash / OOM / lifecycle events
- [x] **Event history** — SQLite via a `Repository` trait (swappable backend)
- [x] **Authentication** — first-run admin setup, login/logout, DB-backed session cookies (Argon2), **admin/viewer roles** (viewers read-only), user management
- [x] **Svelte dashboard** — containers, stacks, swarm, k8s, events, metrics, resources, users

### 🚧 To implement

- [ ] **Security modules**
  - [ ] **Encrypted gateway↔agent transport** (Noise protocol + one-time pairing token — see [Security](#authentication--security))
  - [ ] `Secure` + TLS for session cookies in production
  - [ ] Audit log of user actions
  - [ ] API tokens / SSO
- [ ] GitOps reconciliation with drift detection
- [ ] Advanced Kubernetes (deployments, scaling, more resource types)
- [ ] Multi-agent fleet management (many agents behind one gateway)
- [ ] OpenTelemetry tracing + Grafana dashboard templates
- [ ] High availability, backup/restore

---

## Quick start (local dev)

### Prerequisites

- Rust 1.85+
- Docker
- Node.js 20+ (frontend)

### Run the three processes

```bash
# 1. Agent — owns Docker/Swarm/k8s, listens on TCP
cd agent && cargo run            # tcp://localhost:9000  (BEACON_PORT)

# 2. Gateway — HTTP API + DB, forwards to the agent
cd gateway && cargo run          # http://localhost:8080 (HTTP_PORT, AGENT_ADDR)

# 3. Frontend
cd frontend && npm install && npm run dev   # http://localhost:9003
```

Open **http://localhost:9003**. On first run you'll be asked to **create an admin
account** (this is the setup step). After that, log in with those credentials.

> Starting fresh: the gateway stores everything in `gateway/pulse.db`. Delete
> that file to reset all users/hosts/stacks.

### Environment variables

**Gateway**

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | HTTP port the dashboard/API is served on |
| `AGENT_ADDR` | `127.0.0.1:9000` | TCP address of the agent |
| `DATABASE_URL` | `sqlite://pulse.db` | SQLite path (or Postgres URL) |
| `SLACK_WEBHOOK_URL` | — | Slack incoming webhook for alerts |

**Agent**

| Variable | Default | Description |
|----------|---------|-------------|
| `BEACON_PORT` | `9000` | TCP port the agent listens on |

---

## Running with Docker

The whole stack ships as containers so you don't have to install the toolchain
on a server.

### One command (compose)

```bash
docker compose up -d --build
# dashboard → http://localhost:8088
```

This builds three images — `pulse-agent`, `pulse-gateway`, `pulse-frontend` —
and wires them together. The agent mounts the host Docker socket; the gateway
keeps its SQLite DB in a named volume. To manage Kubernetes, uncomment the
kubeconfig mount in `docker-compose.yml`.

### Build & push to Docker Hub

```bash
DOCKER_USER=youruser

# Backend (two targets from the root Dockerfile)
docker build --target agent   -t $DOCKER_USER/pulse-agent:latest   .
docker build --target gateway -t $DOCKER_USER/pulse-gateway:latest .
# Frontend
docker build -t $DOCKER_USER/pulse-frontend:latest ./frontend

docker push $DOCKER_USER/pulse-agent:latest
docker push $DOCKER_USER/pulse-gateway:latest
docker push $DOCKER_USER/pulse-frontend:latest
```

Consumers then only need `docker-compose.yml` (pointed at your pushed images)
and a Docker socket.

> The **agent** image bundles the Docker CLI + compose plugin (needed for
> `stack up/down`). The **gateway** image embeds its SQL migrations at compile
> time, so it's a single self-contained binary.

### Host metrics from inside a container

The agent's hardware metrics come from the machine it runs on:

- **CPU** — reported by the Docker daemon (`docker info`), so it's always the host.
- **Memory** — `/proc/meminfo` is not namespaced, so it already reflects the host.
- **Disk** — by default `sysinfo` would only see the container's overlay
  filesystem. To report the **host's** real disks, bind-mount the host
  read-only and set `HOST_ROOT` (the `docker-compose.yml` does this):

  ```yaml
  volumes:
    - /:/host:ro
  environment:
    HOST_ROOT: /host
  ```

  With `HOST_ROOT` set, the agent reads the host mount table and `statvfs`'s each
  real filesystem through the bind-mount. Running bare-metal (not in a
  container), it uses `sysinfo` on the real machine. Running in a container
  *without* `HOST_ROOT`, it reports **no** disks rather than the container's
  (misleading) overlay filesystem.

#### OS / platform support for host metrics

A container can only ever see itself or, with a bind-mount, the host it shares a
kernel with. That has real limits:

| Where the agent runs | Host metrics |
| --- | --- |
| Bare-metal (Linux/macOS/Windows), run directly | ✅ Real machine (`sysinfo`) |
| Linux container on a **Linux** host + `HOST_ROOT` | ✅ Real host |
| **Docker Desktop** (Windows/macOS) | ⚠️ Only the Docker **Linux VM** — never the physical machine, even with `-v /:/host` |
| **Native Windows containers** (Windows Server) | ❌ The Linux agent binary can't run there |

**To monitor a Windows or macOS host, run the agent natively** (not in a
container). Docker/Swarm/Kubernetes management still works fine from a
containerized agent everywhere — this limitation only affects **host hardware
metrics** (disk/memory), because those come from the OS the agent process sees.

---

## Authentication & Security

### Authentication (implemented)

- **First-run setup**: with no users, the dashboard forces creation of the first
  account, which becomes an **admin**.
- **Login** issues an opaque, DB-backed session token in an `HttpOnly` cookie
  (`pulse_session`, 7-day expiry). Passwords are hashed with **Argon2**.
- **Roles**: `admin` (full access + user management) and `viewer` (read-only —
  any state-changing request is rejected with `403`).
- **User management** at `/api/users` (admin only), with guards against deleting
  the last admin or your own account.

> **Production TLS note:** the session cookie is **not** marked `Secure` so it
> works over plain HTTP in local/dev. Put the gateway behind TLS (or a
> TLS-terminating proxy) in production and flip it on — it's a one-line change in
> **`gateway/src/auth.rs`**, in the `session_cookie()` function (add
> `cookie.set_secure(true);`).

### Encrypted gateway↔agent transport (planned)

Today the gateway↔agent TCP link is **unencrypted**. The planned design:

- On first boot each side generates a long-lived keypair (persisted).
- The **first handshake** exchanges public keys and pins them (Trust On First
  Use), authenticated by a **one-time pairing token** shared via config —
  `BEACON_PAIRING_TOKEN` — so no man-in-the-middle can slip in during that first
  connection.
- Every session then derives a fresh symmetric key (ephemeral ECDH) for forward
  secrecy.

This is built on the **Noise protocol** (the same handshake framework behind
WireGuard and Signal): you pick a small pattern that specifies how keys are
exchanged and mixed, and get modern crypto (X25519 + ChaCha20-Poly1305), forward
secrecy, and mutual authentication without the weight of full TLS/PKI.

**Planned config**

| Variable | Where | Description |
|----------|-------|-------------|
| `BEACON_PAIRING_TOKEN` | agent + gateway | One-time shared secret that authenticates the first handshake. The agent prints it on first boot; paste it into the gateway config. |

---

## Benchmarks

Measured with [`scripts/benchmark.sh`](scripts/benchmark.sh) (release builds,
Apple Silicon, one agent + one gateway, SQLite):

| Process | State | Peak RSS | Avg CPU |
| ------- | ----- | -------- | ------- |
| agent   | idle  | ~19.8 MB | ~0.0%   |
| gateway | idle  | ~17.7 MB | ~0.4%   |
| agent   | load  | ~19.9 MB | ~0.1%   |
| gateway | load  | ~18.0 MB | ~4.0%   |

*"load" = continuous `/health` + `/metrics` + `/api/hosts` requests. Numbers vary
by host and workload — reproduce your own:*

```bash
# start the stack, then:
GATEWAY_PORT=8080 AGENT_PORT=9000 LOAD=1 ./scripts/benchmark.sh 30
# add PULSE_COOKIE="pulse_session=<token>" to include an authed endpoint under load
```

The whole backend sits comfortably under **~40 MB RSS** combined at idle.

---

## API reference (selected)

All `/api/*` routes except `setup`/`login` require a session cookie. Writes
require the `admin` role.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/api/auth/setup` | First-run status / create first admin |
| POST | `/api/auth/login` · `/api/auth/logout` | Session login / logout |
| GET | `/api/auth/me` | Current user |
| GET/POST | `/api/users` · PUT/DELETE `/api/users/:id` | User management (admin) |
| GET/POST | `/api/hosts` · PUT/DELETE `/api/hosts/:id` | Manage Docker hosts |
| GET | `/api/hosts/:id/containers` | List containers on a host |
| POST | `/api/hosts/:id/containers/:cid/{start,stop,restart}` | Container control |
| GET | `/api/hosts/:id/{images,volumes,networks,system,hardware}` | Host resources |
| GET/POST | `/api/hosts/:id/stacks` · `/api/stacks/:id/{up,down}` | Compose stacks |
| GET | `/api/hosts/:id/swarm/{services,nodes}` | Docker Swarm |
| GET | `/api/k8s/{pods,namespaces}` | Kubernetes |
| POST | `/api/webhooks/:host/...` | Token-protected deploy/redeploy hooks |
| GET | `/api/events` | Recent container lifecycle events |
| GET | `/metrics` · `/ws` | Prometheus metrics · live event WebSocket |

---

## Switching databases

The gateway's storage layer is behind a `Repository` trait. To move from SQLite
to PostgreSQL:

1. Change `sqlx` features in `gateway/Cargo.toml`: `"sqlite"` → `"postgres"`
2. Add `gateway/src/storage/postgres.rs` implementing `Repository`
3. Update the one line in `gateway/src/storage.rs` that constructs the repository

Nothing else changes.

---

## Technology stack

- **Backend**: Rust — Axum, Tokio, Bollard (Docker), kube-rs (Kubernetes), sqlx, prometheus, Argon2
- **Frontend**: Svelte 5, Vite, TypeScript, axios
- **Database**: SQLite (default) / PostgreSQL
- **Transport**: length-prefixed JSON over TCP (gateway↔agent)

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

**Built with Rust**
