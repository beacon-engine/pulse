# Beacon

**A lightweight, high-performance container orchestration platform with native observability**

[![Rust](https://img.shields.io/badge/rust-1.77%2B-orange.svg)](https://www.rust-lang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## What is Beacon?

Beacon is a modern alternative to Portainer, built from the ground up to address critical gaps in container orchestration platforms:

- **Lightweight** — Rust core with no garbage collector or heavy runtime
- **Native GitOps reconciliation** with drift detection and auto-remediation
- **First-class observability** with Prometheus, Grafana, and OpenTelemetry
- **Real-time alerting** via Slack, Discord, and webhooks
- **Fast, async core** built on Tokio and Axum
- **API-first design** with complete UI/API feature parity

## Why Beacon?

After extensive use of Portainer, we identified several critical pain points:

1. **No GitOps reconciliation** — Configuration drift goes undetected
2. **Poor observability** — Metrics and alerting require manual integration
3. **API/UI parity issues** — API-created resources lose UI functionality
4. **Limited alerting** — No native notifications for crashes or deployments
5. **Performance at scale** — Slow when managing hundreds of clusters

Beacon solves these problems with a purpose-built, Rust-powered platform.

---

## Project Status

### Phase 1 — MVP ✅

- [x] HTTP server (Axum, port 9000)
- [x] Docker container listing
- [x] Docker container control (start / stop / restart)
- [x] Kubernetes connectivity (pods, namespaces)
- [x] Prometheus metrics (`/metrics`)
- [x] WebSocket live updates (`/ws`)
- [x] Slack alerting on container events
- [x] SQLite event history with repository pattern
- [x] Svelte 5 frontend dashboard
- [x] Docker event watcher (crash detection)

### Phase 2 — In Progress

- [ ] GitOps reconciliation engine
- [ ] Advanced Kubernetes features (deployments, scaling)
- [ ] Grafana dashboard templates
- [ ] OpenTelemetry tracing

### Phase 3 — Planned

- [ ] Multi-cluster fleet management
- [ ] RBAC and multi-tenancy
- [ ] Cost management (FinOps)
- [ ] Advanced alerting rules

### Phase 4 — Planned

- [ ] High availability and clustering
- [ ] Backup and disaster recovery
- [ ] Plugin system
- [ ] Enterprise features (SSO, audit logs)

---

## Quick Start

### Prerequisites

- Rust 1.77+
- Docker
- Node.js 20+ (for the frontend)

### Backend

```bash
cd agent
cargo run
```

Server starts on `http://localhost:9000` (override with `BEACON_PORT`).

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Dashboard at `http://localhost:5173`.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BEACON_PORT` | `9000` | Backend port |
| `DATABASE_URL` | `sqlite://beacon.db` | SQLite path or Postgres URL |
| `SLACK_WEBHOOK_URL` | — | Slack incoming webhook for alerts |

---

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/status` | Server version and uptime |
| GET | `/api/containers` | List all containers |
| POST | `/api/containers/{id}/start` | Start a container |
| POST | `/api/containers/{id}/stop` | Stop a container |
| POST | `/api/containers/{id}/restart` | Restart a container |
| GET | `/api/events` | Last 100 container events |
| GET | `/api/k8s/pods` | List Kubernetes pods |
| GET | `/api/k8s/namespaces` | List Kubernetes namespaces |
| GET | `/metrics` | Prometheus metrics |
| GET | `/ws` | WebSocket (live container events) |

---

## Architecture

Beacon is split into three Rust crates plus the frontend:

- **`agent/`** — runs alongside Docker/Kubernetes on each host. Talks to the
  Docker and Kubernetes APIs, executes container/stack operations, and reports
  system + hardware metrics. (crate name: `beacon-core`)
- **`gateway/`** — central control plane (in progress). Connects to agents over
  TCP and exposes the HTTP API consumed by the frontend.
- **`protocol/`** — shared wire types linked by both `agent` and `gateway`, so
  the request/response vocabulary is defined exactly once instead of duplicated
  on each side.

```
beacon/
  protocol/src/          ← shared gateway <-> agent wire types
    data/
      container.rs       ← ContainerInfo, ContainerDetail, ImageInfo, VolumeInfo,
                            HostNetwork, ContainerEvent, Stack, PortMap, …
      system.rs          ← SystemInfo, HardwareMetrics, DiskInfo
      orchestration.rs   ← SwarmService, SwarmNode, PodInfo, NamespaceInfo
    host.rs              ← Environment, SavedStack
    protocol/
      request.rs         ← request bodies the gateway sends (CreateHostBody,
                            RecreateBody, ExecBody, CreateStackBody, …)
      response.rs        ← response envelopes the agent returns (StackOpResponse,
                            TokenResponse)
  agent/src/             ← beacon-core: Docker/K8s operations + HTTP API
    main.rs              ← wiring only
    config.rs            ← env var config
    state.rs             ← shared app state
    containers/
      docker.rs          ← Docker client operations (returns protocol types)
      watcher.rs         ← Docker event stream (crash detection)
    environments/        ← per-host container/system/image/volume/network handlers
    stacks/              ← compose stack management
    swarm/               ← Docker Swarm operations
    orchestration/
      kubernetes.rs      ← Kubernetes client operations
    observability/
      alerts.rs          ← Slack webhooks
      metrics.rs         ← Prometheus metrics
      websocket.rs       ← WebSocket broadcast
    storage/
      models.rs          ← re-exports protocol record types (Environment, …)
      repository.rs      ← Repository trait (swap DB backends here)
      sqlite.rs          ← SQLite implementation
    webhooks/            ← token-protected deploy/recreate webhooks
  gateway/src/           ← control plane (WIP)
  frontend/src/
    types/containers.ts  ← Shared TypeScript types
    lib/api.ts           ← Fetch wrappers
    lib/websocket.ts     ← WebSocket client
    components/
    App.svelte
```

> **Sharing types across crates:** any struct that crosses the gateway ↔ agent
> boundary lives in `protocol` and derives both `Serialize` and `Deserialize`,
> so the gateway can build a request and decode the agent's response using the
> exact same definition. Agent modules re-export these types (e.g.
> `docker::ContainerInfo`, `storage::models::Environment`) so call sites are
> unchanged. Agent-internal types (config, app state, DB repository, webhook
> envelopes) stay in `agent`.

### Switching databases

The storage layer uses a `Repository` trait. To switch from SQLite to PostgreSQL:

1. Change `sqlx` features in `Cargo.toml`: `"sqlite"` → `"postgres"`
2. Write `src/storage/postgres.rs` implementing `Repository`
3. Update one line in `storage.rs` to use `PostgresRepository`

Nothing else changes.

---

## Technology Stack

- **Backend**: Rust — Axum, Tokio, Bollard, kube-rs, sqlx, prometheus
- **Frontend**: Svelte 5, Vite 8, TypeScript 6
- **Database**: SQLite (default) / PostgreSQL
- **Metrics**: Prometheus
- **Container APIs**: Bollard (Docker), kube-rs (Kubernetes)

---

## Performance Goals

> Targets, not yet benchmarked.

- Memory footprint: <100MB
- API latency: <100ms for common operations
- Drift detection: <5 seconds
- Uptime: 99.9% for monitoring/alerting

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

**Built with Rust**
