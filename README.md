# Pulse

**A lightweight, high-performance container management platform with native observability**

[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## What is Pulse?

Pulse is a modern alternative to Portainer, built in Go to manage Docker,
Docker Swarm, and Kubernetes from one dashboard with first-class observability
and alerting.

- **Lightweight** — small, static Go binaries (one per process; no external runtime)
- **Split control/executor design** — a public HTTP gateway backed by an agent that owns cluster access
- **First-class observability** — Prometheus metrics, live WebSocket events, Slack alerting
- **API-first** — every dashboard action is a documented HTTP endpoint

---

## Architecture

Pulse is two Go binaries (agent + gateway) over a shared protocol package, plus a Svelte frontend:

```
┌──────────┐   HTTP/JSON    ┌───────────┐   TCP (framed)    ┌─────────┐   Docker API   ┌────────┐
│ frontend │ ─────────────> │  gateway  │ ────────────────> │  agent  │ ─────────────> │ Docker │
│ (Svelte) │ <───────────── │ HTTP + DB │ <──────────────── │executor │ <───────────── │ Swarm  │
└──────────┘   cookies/WS   └───────────┘   Request/Resp    └─────────┘   kube API     │  k8s   │
                                  │                                                    └────────┘
                              SQLite DB
                     (users, hosts, stacks, tokens, events)
```

- **`cmd/gateway` + `internal/gateway`** — the frontend-facing **HTTP API**. Owns
  the database (users, hosts, stacks, webhook tokens, events), authentication,
  Slack alerting, the Prometheus proxy, WebSocket fan-out, and metrics. It holds
  **no** Docker access itself — it resolves each host to a `docker_host`
  connection string and forwards the operation to the agent. Agents may also dial
  *in* over WebSocket (push mode); see [docs/PUSH_PULL_TRANSPORT.md](docs/PUSH_PULL_TRANSPORT.md).
- **`cmd/agent` + `internal/agent`** — a **stateless executor** that owns Docker,
  Docker Swarm, and Kubernetes access. It answers `Request`s from the gateway
  with `Response`s over a framed TCP protocol (pull) or an outbound WebSocket
  (push). It keeps no database.
- **`internal/protocol`** — shared wire types (the `Request`/`Response` union,
  domain models, and length-prefixed / WS framing) imported by both sides, so the
  vocabulary is defined exactly once.
- **`frontend/`** — Svelte 5 + Vite dashboard; talks only to the gateway.

> Nothing should send arbitrary HTTP to the agent's TCP port — it only speaks the
> binary gateway protocol. (A stray HTTP client there is logged and dropped.)

---

## Supported container systems

| System | Support | Notes |
| :--- | :--- | :--- |
| **Docker** | ✅ Full | containers, images, volumes, networks, logs, exec, compose stacks |
| **Docker Swarm** | ✅ Full | services, nodes, force rolling update |
| **Kubernetes** | 🟡 Partial | workloads (Deployments, StatefulSets, DaemonSets): scale, rolling restart · pods and containers: logs, inspect, delete · services, nodes, events. In-cluster agent or kubeconfig — see [Kubernetes](#kubernetes) |
| **Podman** | 🟡 Partial | works via Podman's Docker-compatible socket for container/image/volume ops; no Swarm; compose needs `podman compose` |
| **AWS ECS** | 🚧 Planned | Task Metadata Endpoint v4 & AWS SDK |
| **HashiCorp Nomad**| 🚧 Planned | Nomad HTTP REST API (`/v1/allocations`) |
| **Fargate / Cloud Run** | 🚧 Planned | Agentless API / Polling Sidecar |
| containerd / nerdctl / LXC | ❌ | not supported |

See **[docs/LOCAL_ORCHESTRATORS.md](docs/LOCAL_ORCHESTRATORS.md)** for setting up
Swarm and Kubernetes locally in isolation to test these integrations.

---

## Module status

### ✅ Completed

- [x] **Gateway/agent split** — HTTP control plane + TCP executor over a shared protocol package
- [x] **Docker** — list/inspect, start/stop/restart, recreate, env update, exec, logs
- [x] **Images / volumes / networks** — list, delete, prune, connect/disconnect
- [x] **Compose stacks** — saved stacks, up/down, detected stacks
- [x] **Docker Swarm** — services, nodes, force update
- [x] **Kubernetes** — workloads (scale, rolling restart), pods and containers (logs, inspect, delete), services, nodes, events
- [x] **Multi-host** — manage several Docker endpoints from one gateway
- [x] **Push agents** — agents dial out to the gateway, auto-register as environments (many per gateway), and go offline / recover on heartbeat
- [x] **Webhooks** — token-protected recreate / redeploy / swarm update / k8s rollout
- [x] **Observability** — Prometheus `/metrics`, Prometheus proxy, live WebSocket events
- [x] **Slack alerting** — container crash / OOM / lifecycle events
- [x] **Event history** — SQLite behind a storage interface (swappable backend)
- [x] **Authentication** — first-run admin setup, login/logout, DB-backed session cookies (Argon2), **admin/viewer roles** (viewers read-only), user management
- [x] **Svelte dashboard** — containers, stacks, swarm, k8s, events, metrics, resources, settings (users today; the other sections are "coming soon" screens)

### 🚧 To implement

- [ ] **Expanded Engine Integrations**
  - [ ] **Podman native support** (SSH socket forwarding + Quadlet deployment parsing)
  - [ ] **AWS ECS agentless driver** (Task Metadata v4 API parsing & CloudWatch integration)
  - [ ] **HashiCorp Nomad driver** (Nomad `/v1` API integration for job & allocation status)
  - [ ] **Serverless/Agentless sidecar mode** (Pushing metrics directly to Gateway for Fargate/Cloud Run)
- [ ] **Access control** (Settings → Access)
  - [ ] **Roles & permissions** — custom roles beyond admin / viewer, built from a JSON permission catalog the gateway loads and serves to the dashboard, so the API checks and what the UI shows come from one source
  - [ ] Audit log of user actions
  - [ ] API tokens / SSO
- [ ] **Alerts and notifications** (Settings → Monitoring)
  - [ ] Metric alert rules (CPU / memory thresholds, restarting pods, unready workloads, NotReady nodes)
  - [ ] Notification channels configured in the UI (Slack is env-only today; email, Teams, webhooks)
- [ ] **Security scanning in the product** (Settings → Security)
  - [ ] Scheduled / on-deploy image scans (Trivy) for running containers and workloads
  - [ ] Vulnerability findings with notices when new ones appear
- [ ] **Security modules**
  - [ ] **Agent enrollment** — one-time join tokens and per-agent, revocable credentials ([design](docs/AGENT_ENROLLMENT.md))
  - [ ] **Authenticated, encrypted pull-mode agents** (pull agents accept any caller today — see [Security](#authentication--security))
  - [ ] `Secure` session cookie + TLS by default in production
- [ ] GitOps reconciliation with drift detection
- [ ] Advanced Kubernetes (exec, Ingress, ConfigMaps, YAML view/edit, CPU/memory usage via metrics-server, namespace-scoped agents)
- [ ] Fleet management across agents (tags / groups, bulk actions, agent versions and upgrades)
- [ ] Collapsible sidebar for small screens
- [ ] OpenTelemetry tracing + Grafana dashboard templates
- [ ] High availability, backup/restore

---

## Engine Integration Guides: How New Systems Will Be Supported

To maintain Pulse's low memory footprint (~20 MB RSS) and clean gateway↔agent abstraction, extending support to new container runtimes follows specific architectural patterns:

### 1. Podman (Native & Remote)

**The Challenge:** Podman is daemonless and rootless. It lacks a persistent background daemon like `dockerd` by default.

**Implementation Strategy:**
* **Local Socket Mode:** Pulse agents talk to the Podman system service socket (`podman system service --time=0 unix:///run/podman/podman.sock`). Because Podman exposes a Docker-compatible REST API, standard container operations map directly to existing `protocol` request types.
* **Remote Host Mode (Agentless over SSH):** For remote Podman hosts (such as RHEL servers using Podman Quadlets), the Gateway will establish an SSH tunnel directly to the user's `/run/podman/podman.sock` using an SSH key pair configured in the host settings.

---

### 2. AWS ECS (Elastic Container Service) & Fargate

**The Challenge:** You cannot run traditional host-level agents (like Swarm global services or K8s DaemonSets) inside AWS Fargate or managed ECS clusters where underlying EC2 instances are hidden or restricted.

**Implementation Strategy:**
* **Task-Level Agent (Sidecar / In-Task):** The agent runs inside an ECS Task and queries the **ECS Task Metadata Endpoint v4** (`${ECS_CONTAINER_METADATA_URI_V4}/task/stats`). This endpoint provides container-level CPU, memory, network stats, and lifecycle events without requiring root or Docker socket access.
* **Gateway Cloud Poller (Agentless):** For cluster-wide views, the Gateway uses the AWS SDK for Go to poll `DescribeTasks`, `ListServices`, and CloudWatch metrics.

```
┌──────────────────────────────────────────────┐
│ AWS ECS Task                                 │
│  ┌──────────────────┐  ┌──────────────────┐  │
│  │ App Container    │  │ Pulse Agent      │  │
│  └──────────────────┘  │ (Sidecar)        │  │
│            │           └────────┬─────────┘  │
└────────────┼────────────────────┼────────────┘
             │                    │
             ▼                    ▼
   [Task Metadata API v4] ◄───────┘
```

---

### 3. HashiCorp Nomad

**The Challenge:** Nomad manages non-Docker workloads (executables, Java JARs, QEMU VMs) alongside raw containers, using a job/allocation model rather than pods/deployments.

**Implementation Strategy:**
* **Nomad API client in the agent:** Add a native Nomad HTTP client to `internal/agent` that interfaces with Nomad's `/v1/jobs`, `/v1/allocations`, and `/v1/nodes` REST endpoints.
* **Protocol extension:** Extend the shared `internal/protocol` package with Nomad domain models:
  ```go
  // Existing Docker/K8s request kinds, plus:
  // ReqNomadListAllocations, ReqNomadPromoteJob
  ```

---

## Quick start (local dev)

### Prerequisites

- Go 1.26+
- Docker
- Node.js 20+ (frontend)

### Run the three processes

```bash
# 1. Agent — owns Docker/Swarm/k8s, listens on TCP
go run ./cmd/agent            # tcp://localhost:9000  (PULSE_PORT)

# 2. Gateway — HTTP API + DB, forwards to the agent
go run ./cmd/gateway          # http://localhost:8080 (HTTP_PORT, AGENT_ADDR)

# 3. Frontend
cd frontend && npm install && npm run dev   # http://localhost:9010
```

Open the frontend dev URL. On first run you'll be asked to **create an admin
account** (this is the setup step). After that, log in with those credentials.

> Starting fresh: the gateway stores everything in `pulse.db` (its working dir).
> Delete that file to reset all users/hosts/stacks.

### Build the binaries

```bash
go build ./cmd/agent      # ./agent
go build ./cmd/gateway    # ./gateway  (serves the built frontend from STATIC_DIR)
```

---

## Testing Multi-Node Clusters Locally

Testing Pulse against multi-node environments locally requires distinct network namespaces or Docker daemons.

* **Docker Swarm (Multi-Node):** Use **Docker-in-Docker (DinD)** to spin up a 3-node Swarm cluster inside isolated container daemons without managing full VMs:
  ```bash
  # See scripts/setup-swarm-dind.sh for an automated 3-node Swarm setup
  ./scripts/setup-swarm-dind.sh
  ```
* **Kubernetes (Multi-Node):** Use **`k3d`** to run a multi-node K3s cluster in Docker:
  ```bash
  # See scripts/setup-k8s-k3d.sh for an automated 3-node K3s setup
  ./scripts/setup-k8s-k3d.sh
  ```
* **Multi-Machine VMs:** For true kernel-level isolation, use **Canonical Multipass** or **Vagrant** running lightweight Linux nodes.

---

## Environment variables

Pulse aims to run with almost no configuration. Pulse's own settings use the
`PULSE_` prefix.

The agent-liveness *tunables* (offline threshold, alert grace, retention) are
**not** env vars — they live in the gateway database and are editable at runtime
via `PUT /api/settings`. See [Auto-registration & liveness](#auto-registration--liveness).

### Gateway

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | Port the dashboard + API + `/agent/ws` are served on. |
| `AGENT_ADDR` | `127.0.0.1:9000` | Default **pull** agent address (`host:port`), used when an environment has no push session or per-host `agent_addr`. |
| `DATABASE_URL` | `sqlite://pulse.db` | Database URL: `sqlite://<path>` or `sqlite::memory:`. |
| `STATIC_DIR` | `./frontend/dist` | Built SPA directory to serve. |
| `SLACK_WEBHOOK_URL` | — | Slack incoming webhook for lifecycle + agent-down alerts. |
| `PULSE_AGENT_TOKENS` | — | Comma-separated `agent_id:token` pairs authenticating **push** agents on `/agent/ws`, e.g. `prod:s3cret,staging:h0nk`. Empty disables push. A token may be shared by many duplicated agents — each still registers as its **own** environment (see below). |
| `PULSE_TLS_CERT` / `PULSE_TLS_KEY` | — | PEM cert/key. When **both** are set the gateway serves HTTPS/WSS natively; otherwise plain HTTP behind a TLS-terminating proxy. |
| `PULSE_DB_RESET_ON_MISMATCH` | `false` | Dev only: recreate the DB on a migration mismatch instead of aborting. **Destroys all data.** |

### Agent

Point it at the gateway and give it the token — it self-registers on connect.
There is **no identity env var to set**: the agent derives its identity from the
cluster or container daemon it manages, so restarts keep the same environment.

| Variable | Default | Description |
|----------|---------|-------------|
| `PULSE_MODE` | `listen` | `dial`/`push` (agent dials out over WSS — required for auto-register) or `listen`/`pull` (agent binds a socket; the gateway dials in). |
| `PULSE_GATEWAY_WS_URL` | — | Gateway WS URL to dial in `dial` mode, e.g. `wss://gateway.example.com/agent/ws`. Required for `dial`. |
| `PULSE_TOKEN` | — | Bearer token presented on the WS upgrade (must match a `PULSE_AGENT_TOKENS` entry). |
| `PULSE_PORT` | `9000` | TCP port to listen on in `listen` mode. |
| `PULSE_TRANSPORT` | `tcp` | `listen`-mode transport: `tcp` (framed TCP) or `ws` (WebSocket over HTTP, rides 80/443). |
| `PULSE_INSECURE_SKIP_TLS_VERIFY` | `false` | Skip server-cert verification on the `wss://` uplink. Trusted networks / dev only. |
| `HOST_ROOT` | — | Host filesystem root as seen from the container (e.g. `/host` for `-v /:/host:ro`) so host disk/mem metrics reflect the real host. Empty = bare-metal. |
| `PULSE_KUBECONFIG_PATH` | — | Path to a kubeconfig file to read. |
| `PULSE_KUBECONFIG` | — | Inline kubeconfig YAML (ignored if `…_PATH` is set). |

**Agent — auto-registration (all optional):**

| Variable | Default | Description |
|----------|---------|-------------|
| `PULSE_AUTO_REGISTER` | `on` | Self-register as an environment on connect (`dial` mode). Set `off` for the legacy static path where an admin pre-creates the environment and links it by `agent_id`. |
| `PULSE_ENV_NAME` | hostname | Display name to register the environment under. |
| `PULSE_INSTANCE_ID` | derived | Override the agent's identity. Normally unset — see *Identity* below. |
| `PULSE_DOCKER_HOST` | `unix:///var/run/docker.sock` | Local Docker endpoint the registered environment points at. |
| `PULSE_HEARTBEAT_INTERVAL` | `30s` | How often the agent calls home. Go duration (`45s`, `2m`) or bare seconds (`30`); floored at `5s`. |

### Auto-registration & liveness

When a `dial`-mode agent connects with `PULSE_AUTO_REGISTER` on (the default), it
**self-registers** as an environment — no manual `POST /api/hosts`, no per-agent
config. This is what makes autoscaled fleets (ECS, Karpenter, Terraform-managed
duplicates) appear in the dashboard on their own.

How it stays correct:

- **Identity.** The environment is keyed by `<token_id>:<instance_id>`, where the
  instance id is derived from what the agent manages: in a pod, the cluster's id
  (the `kube-system` namespace UID); on a host, the container daemon's id
  (`docker info`). A restart, redeploy or reinstall therefore *upserts* the same
  environment instead of leaving an offline duplicate, while agents on different
  hosts (an autoscaled fleet) still each appear as their **own** environment. Run
  **one agent per cluster or daemon**: two agents with the same identity are the
  same environment, and the newest connection takes it over. If neither a cluster
  nor a daemon is reachable at boot, the agent uses a temporary random id and
  switches to the derived one once a runtime answers.
- **Heartbeat.** The agent calls home every `PULSE_HEARTBEAT_INTERVAL`; a
  missed-heartbeat backstop marks an environment offline even if the socket
  half-opens without a clean close.
- **Keepalive and reconnect.** Both ends ping each other every 20 s and drop
  the session when a pong doesn't arrive within 10 s, so a link that dies
  silently (network drop, crashed host) is noticed in about 30 s instead of
  whenever TCP gives up. The agent then reconnects with a jittered backoff
  (1 s, doubling to 30 s while the gateway stays unreachable).
- **Scale-down vs. outage.** On a clean shutdown (SIGTERM — how ECS/K8s/Karpenter
  stop a task) the agent sends a *deregister* first, so it goes offline
  **silently**. An abrupt drop with no deregister alerts (Slack, if configured)
  after a short grace window — so real outages notify but rolling replacements
  stay quiet.
- **Retention.** Offline auto-registered environments are kept as history, then
  hard-deleted once the retention window elapses.
- **Pull agents.** An environment added by agent address never calls home, so
  the gateway pings its agent every 30 s and marks it offline / online from the
  result. It alerts after the same grace window, but is never deleted.

The three windows are **gateway settings** (DB-backed, editable at runtime via
`GET`/`PUT /api/settings`; a settings page is planned) — not env vars:

| Setting key | Default | Meaning |
|-------------|---------|---------|
| `agent_offline_threshold_seconds` | `90` | No heartbeat for this long → mark offline. |
| `agent_alert_grace_seconds` | `60` | Wait this long after an abrupt disconnect before alerting. |
| `agent_retention_hours` | `6` | Delete an environment after it has been offline this long. |

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
keeps its SQLite DB in a named volume. To manage a Kubernetes cluster, run an
agent inside it (see [Kubernetes](#kubernetes)); for a quick local look you can
instead uncomment the kubeconfig mount in `docker-compose.yml`.

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

Add `--build-arg PULSE_VERSION=<version>` to stamp the binaries and dashboard;
without it they report `dev`.

### Releases

The version lives in [`VERSION`](VERSION) (`X.Y.Z`, no `v`). CI publishes
images from `master`, and `master` only takes release branches:

1. Cut `release/vX.Y.Z` (or `hotfix/vX.Y.Z`) and merge the finished feature
   branches into it.
2. Set `VERSION` and `frontend/package.json`'s `version` to `X.Y.Z`.
3. Open a PR into `master`. CI rejects any other branch, a name that doesn't
   match `VERSION`, or a version that is already tagged.
4. Merge. The release workflow pushes `pulse-gateway` and `pulse-agent` as
   `X.Y.Z`, `X.Y`, `latest` and `sha-<commit>`, then tags the commit `vX.Y.Z`
   and creates a GitHub release.

A push to `master` whose version is already tagged only refreshes `latest` and
`sha-<commit>`, and those builds report `X.Y.Z-dev+<commit>`.

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

## Kubernetes

Run one agent **inside** each cluster. It uses its own ServiceAccount, dials out
to the gateway (no ingress needed), and shows up as its own environment.
[`deploy/kubernetes/README.md`](deploy/kubernetes/README.md) has the full
walkthrough. The typical setup, with the gateway in Docker Compose and a local
cluster (OrbStack / Docker Desktop):

```bash
# 1. Gateway: accept an agent token (read at startup)
TOKEN=$(openssl rand -hex 32)
echo "PULSE_AGENT_TOKENS=k8s:$TOKEN" >> .env
docker compose up -d

# 2. Cluster: the same token, as a Secret
kubectl create namespace pulse
kubectl -n pulse create secret generic pulse-agent-token --from-literal=token="$TOKEN"

# 3. In deploy/kubernetes/agent-push.yaml set:
#      image: pulse-agent:local (imagePullPolicy: IfNotPresent) or a published tag
#      PULSE_GATEWAY_WS_URL: ws://host.docker.internal:9080/agent/ws
#      PULSE_ENV_NAME: my-cluster
docker compose build agent && docker tag pulse-agent:latest pulse-agent:local
kubectl apply -f deploy/kubernetes/agent-push.yaml

# 4. Optional: something to look at
kubectl apply -f deploy/kubernetes/examples/demo-workloads.yaml
```

Pick the new environment in the selector and open **Kubernetes**. The header
shows the cluster's Kubernetes version, API server and how Pulse reaches it:
**Push agent** (the agent dials the gateway) or **Pull agent** (the gateway
dials the agent). Below it are health counts, then one tab per kind of object.
The filter box and namespace selector apply to every tab.

| Tab | What you can do |
|---|---|
| **Workloads** | Deployments, StatefulSets and DaemonSets with ready/desired counts. Scale with − / + (Deployments, StatefulSets), start a rolling restart (all three kinds), or jump to the workload's pods. |
| **Pods** | Status as `kubectl get pods` shows it (CrashLoopBackOff, Terminating, …), ready containers, restarts, controller, node, age. Expand a pod to see its containers; inspect it for placement, labels, resources and its events. Delete a pod and its controller replaces it. |
| **Containers** | Every pod container, laid out like the Docker containers page: status, pod, ports, restarts, **logs**, inspect, and restart (which replaces the pod). Filter to the ones restarting or not ready. |
| **Services** | Type, cluster IP, external address, ports (`port:nodePort→target`) and selector. |
| **Nodes** | Ready state, roles, kubelet version, OS / arch / runtime, CPU, memory, and pods running against capacity. Pressure conditions and cordoned nodes are flagged. |
| **Events** | The cluster's newest 300 events (Kubernetes keeps about an hour), with a Warnings filter. Click a pod to inspect it. |

You manage containers in Kubernetes **through their workload**, not one by one.
There is no "stop container": the kubelet would just start it again. To stop
something, scale it to 0. To restart it, do a rolling restart, or delete the
pod. To change it, change its workload. Pulse refuses to scale, restart or
delete its own agent (or anything labelled `pulse/protected=true`), since that
would cut off its connection to the cluster.

**Why pod containers also show up under Docker.** OrbStack, Docker Desktop and
other clusters that use the Docker runtime run every pod container on the host's
Docker engine. They appear in that host's Docker environment as `k8s_<container>_<pod>_…`,
alongside a `k8s_POD_…` "pause" container per pod that holds the pod's network.
Pulse tags them **k8s**. Logs, exec and restart work there, but recreating them
or editing their env through Docker is refused (409). The copy would run outside
the cluster while the kubelet started another one. Clusters that use containerd
(most cloud clusters) don't expose their containers to Docker at all, so there
they're only visible on the Kubernetes page.

---

## Authentication & Security

### Authentication (implemented)

- **First-run setup**: with no users, the dashboard forces creation of the first
  account, which becomes an **admin**.
- **Login** issues an opaque, DB-backed session token in an `HttpOnly` cookie
  (`pulse_session`, 7-day expiry). Passwords are hashed with **Argon2**.
- **Roles**: `admin` (full access + user management) and `viewer` (read-only —
  any state-changing request is rejected with `403`). Custom roles are planned
  (Settings → Roles & permissions).
- **User management** in **Settings → Users** (API: `/api/users`, admin only),
  with guards against deleting the last admin or your own account.

> **Production TLS note:** the session cookie is **not** marked `Secure` so it
> works over plain HTTP in local/dev. Put the gateway behind TLS (or a
> TLS-terminating proxy) in production and flip it on — it's a one-line change in
> **`internal/gateway/auth.go`**, in `setSessionCookie` (add `Secure: true`).

### Gateway↔agent connection (today)

| Mode | Authentication | Encryption |
|---|---|---|
| **Push** (agent dials `/agent/ws`) | The agent sends a bearer token from `PULSE_AGENT_TOKENS` on the WebSocket upgrade; unknown tokens are rejected. The agent does **not** verify which gateway it reached beyond TLS. | Only with `wss://`: serve the gateway over TLS (`PULSE_TLS_CERT` / `PULSE_TLS_KEY`, or a TLS proxy). With `ws://`, the token and all traffic travel in plaintext. |
| **Pull** (gateway dials the agent, TCP or `ws://`) | **None.** Anyone who can reach the agent's port can drive it, and with a Docker socket that means root on the host. | None over TCP; `wss://` only if you put TLS in front of the agent. |

So: use push mode with a `wss://` gateway URL anywhere the network isn't fully
trusted, and never expose a pull agent's port beyond a private network (the
Docker Compose setup keeps `agent:9000` on the internal compose network only).
Agent enrollment ([design](docs/AGENT_ENROLLMENT.md)) replaces shared tokens
with per-agent revocable credentials.

### Encrypted gateway↔agent transport (planned)

The planned design for authenticating and encrypting every link, including pull mode:

- On first boot each side generates a long-lived keypair (persisted).
- The **first handshake** exchanges public keys and pins them (Trust On First
  Use), authenticated by a **one-time pairing token** shared via config —
  `PULSE_PAIRING_TOKEN` — so no man-in-the-middle can slip in during that first
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
| `PULSE_PAIRING_TOKEN` | agent + gateway | One-time shared secret that authenticates the first handshake. The agent prints it on first boot; paste it into the gateway config. |

### Security scanning

Every PR to `master`, every push to `master`, and a weekly scheduled run go
through `.github/workflows/security.yml`:

| Scanner | What it checks | Fails the build on |
|---|---|---|
| **Checkov** | Dockerfile, Kubernetes manifests, GitHub workflows, hard-coded secrets | Any failed check |
| **Trivy** (`fs`) | Go and npm dependencies, committed secrets | HIGH/CRITICAL with a fix available |
| **Trivy** (`image`) | The built agent and gateway images (OS packages + binaries) | HIGH/CRITICAL with a fix available |

Issues with no upstream fix yet don't block, but every finding is uploaded to
the repository's **Security** tab (when code scanning is enabled).

Run the same scans locally (they use the scanners' official images, pinned by
digest, so there's nothing to install):

```bash
make security          # everything
make security-iac      # Checkov only
make security-deps     # Trivy: dependencies + secrets
make security-images   # Trivy: build and scan both images
```

**Accepting a finding.** Don't loosen the global config. Record the exception
next to the thing it's about, with the reason:

- Checkov in a Dockerfile: `# checkov:skip=CKV_DOCKER_3: why`
- Checkov in a Kubernetes manifest: a `checkov.io/skipN: CKV_K8S_38=why`
  annotation on the resource.
- Trivy: an entry in `.trivyignore.yaml`, scoped by `paths`, with a
  `statement` and an `expired_at` date so it gets re-checked.

**Upgrading from a root gateway.** The gateway image now runs as uid 10001. An
existing data volume created by an older (root) gateway must be handed over
once, or the gateway can't write its database:

```bash
docker run --rm -v pulse_pulse-data:/data debian:bookworm-slim chown -R 10001:10001 /data
```

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
| GET | `/api/hosts/:id/k8s/{pods,namespaces,workloads,nodes,services,events}` | Kubernetes |
| GET | `/api/hosts/:id/k8s/pods/:ns/:pod/logs?container=&tail=` | Pod container logs |
| POST | `/api/hosts/:id/k8s/workloads/:ns/:kind/:name/{scale?replicas=N,restart}` | Scale / rolling restart |
| DELETE | `/api/hosts/:id/k8s/pods/:ns/:pod` | Delete a pod |
| POST | `/api/webhooks/:host/...` | Token-protected deploy/redeploy hooks |
| GET | `/api/events` | Recent container lifecycle events |
| GET | `/metrics` · `/ws` | Prometheus metrics · live event WebSocket |

---

## Switching databases

The gateway's storage layer is behind a `Repository` interface. To move from
SQLite to PostgreSQL:

1. Add a `postgres.go` in `internal/gateway/storage` implementing `Repository`
   (e.g. with `database/sql` + `pgx`)
2. Update the one place in `internal/gateway/storage/storage.go` that constructs
   the repository

Nothing else changes.

---

## Technology stack

- **Backend**: Go — net/http, the official Docker SDK, client-go (Kubernetes),
  coder/websocket, pure-Go SQLite (modernc), gopsutil, Argon2id
- **Frontend**: Svelte 5, Vite, TypeScript, axios
- **Database**: SQLite (default) / PostgreSQL
- **Transport**: length-prefixed JSON over TCP, or JSON over WebSocket (gateway↔agent)

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

**Built with Go**
