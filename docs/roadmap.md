# Roadmap

## Done

- **Gateway/agent split:** an HTTP control plane and an executor next to what it
  manages, over a shared protocol
- **Docker:** list and inspect, start / stop / restart, recreate, edit
  environment variables, exec, logs
- **Images, volumes, networks:** list, delete, prune, connect / disconnect
- **Compose stacks:** saved stacks, up / down, detection of running projects
- **Docker Swarm:** services, nodes, tasks, scale, force update
- **Kubernetes:** workloads (scale, rolling restart), pods and containers (logs,
  inspect, delete), services, nodes, events
- **Many environments:** several Docker hosts and clusters from one gateway
- **Push agents:** agents dial out, register themselves, and go offline / come
  back with heartbeats and keepalives; pull agents are probed
- **Webhooks:** token-protected recreate, redeploy, Swarm update, Kubernetes
  rollout
- **Observability:** Prometheus `/metrics`, a Prometheus query proxy, live
  WebSocket events
- **Slack alerts:** container crashes, OOM kills, lifecycle events, agents down
- **Event history** in SQLite, behind a storage interface
- **Authentication:** first-run setup, sessions, Argon2, admin and viewer roles,
  user management
- **Dashboard:** containers, stacks, Swarm, Kubernetes, events, metrics,
  resources, settings
- **Releases:** versioned images, release branches, scans on every PR into a
  release

## Next

- **Access control** (Settings → Access)
  - Custom roles built from a JSON permission catalog that the gateway loads and
    serves to the dashboard, so the API checks and what the UI shows come from
    one source
  - Audit log of user actions
  - API tokens, SSO
- **Alerts and notifications** (Settings → Monitoring)
  - Metric alerts: CPU / memory thresholds, restarting pods, unready workloads,
    NotReady nodes
  - Notification channels set up in the UI (Slack is env-only today; email,
    Teams, webhooks)
- **Security scanning in the product** (Settings → Security)
  - Scheduled and on-deploy image scans (Trivy) of running containers and
    workloads
  - Vulnerability findings, with notices when new ones appear
- **Security**
  - Agent enrollment: one-time join codes and per-agent, revocable credentials
    ([design](design/agent-enrollment.md))
  - Authenticated, encrypted pull agents (they accept any caller today)
  - `Secure` session cookie and TLS by default
- **More Kubernetes:** exec, Ingress, ConfigMaps, YAML view / edit, CPU / memory
  usage (metrics-server), namespace-scoped agents
- **Fleet management:** tags and groups, bulk actions, agent versions and
  upgrades
- **ARM images** (`linux/arm64`)
- GitOps reconciliation with drift detection
- Collapsible sidebar for small screens
- OpenTelemetry tracing, Grafana dashboard templates
- High availability, backup and restore

## More container platforms

| Platform | Status | Approach |
|---|---|---|
| Podman | Partial | Works today through Podman's Docker-compatible socket (no Swarm; Compose needs `podman compose`). Planned: remote hosts over SSH, Quadlet support. |
| AWS ECS / Fargate | Planned | See below |
| HashiCorp Nomad | Planned | See below |
| Cloud Run | Planned | Agentless polling, or a sidecar that pushes metrics |
| containerd / nerdctl / LXC | Not planned | |

New platforms follow the same pattern: the agent learns to talk to the
platform's API, and the shared protocol gains the request types for it, so the
gateway and dashboard stay platform-neutral and the agent stays small (~20 MB).

**Podman** is daemonless and often rootless. Locally, the agent talks to the
Podman service socket (`podman system service --time=0 unix:///run/podman/podman.sock`),
whose Docker-compatible API maps onto the existing requests. For remote Podman
hosts (such as RHEL servers using Quadlets), the gateway would reach the socket
over SSH with a key configured on the environment.

**AWS ECS and Fargate** hide or restrict the hosts, so there's nowhere for a
host-level agent. Two options:

- An agent inside a task, as a sidecar, reading the **Task Metadata Endpoint
  v4** (`${ECS_CONTAINER_METADATA_URI_V4}/task/stats`) for CPU, memory, network
  and lifecycle, with no Docker socket or root.
- Agentless: the gateway polls `DescribeTasks`, `ListServices` and CloudWatch
  with the AWS SDK for a cluster-wide view.

**Nomad** schedules containers alongside executables, JARs and VMs, with jobs
and allocations instead of pods and deployments. The agent would gain a Nomad
client for `/v1/jobs`, `/v1/allocations` and `/v1/nodes`, and the protocol would
gain matching requests (listing allocations, promoting a job, …).
