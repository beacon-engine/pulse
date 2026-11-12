# Architecture

Pulse is two Go programs, the **gateway** and the **agent**, plus a Svelte
dashboard that the gateway serves.

```
             HTTP / WebSocket            agent protocol
 browser  ◄─────────────────►  gateway  ◄──────────────►  agent  ──►  Docker / Swarm
(dashboard)                    API, auth,                 (one per      Kubernetes API
                               SQLite                   host/cluster)
```

- **Gateway** (`cmd/gateway`, `internal/gateway`): everything the browser talks
  to. It owns the database (users, environments, stacks, webhook tokens,
  events), authentication, Slack alerts, Prometheus metrics and the live event
  WebSocket. It has **no** access to Docker or Kubernetes itself: it works out
  which agent serves an environment and forwards the request.
- **Agent** (`cmd/agent`, `internal/agent`): a stateless executor next to what it
  manages. It answers the gateway's requests using the Docker API (containers,
  images, volumes, networks, Swarm, Compose) and the Kubernetes API. It keeps no
  database.
- **Protocol** (`internal/protocol`): the request and response types both sides
  share, so the vocabulary is defined once.
- **Dashboard** (`frontend/`): Svelte 5 + Vite. It talks only to the gateway.

## Push and pull

An environment is reached through an agent in one of two ways:

| | Push | Pull |
|---|---|---|
| Who connects | The agent dials `wss://<gateway>/agent/ws` once and keeps the connection open | The gateway dials the agent's address for each request |
| Open ports needed | None on the agent's side | The agent's port must be reachable from the gateway |
| Joining | The agent registers itself as an environment | An admin adds the environment with the agent's address |
| Typical use | Remote servers, Kubernetes clusters, autoscaled fleets | The agent next to the gateway (Docker Compose), private networks |

In both modes the gateway sends requests and the agent answers; push only
changes who opened the connection. The transport is framed JSON over TCP, or
JSON over WebSocket. The agent's port speaks only this protocol: an HTTP client
that connects to it is logged and dropped.

How the gateway picks a route for each request (`AgentRouter.CallOn`): the
environment's live push session if it has one, otherwise its own agent address,
otherwise the default agent (`AGENT_ADDR`). A push environment whose agent is
disconnected fails with "not connected" rather than falling through to another
agent, which would show the wrong machine's data.

## Self-registration and liveness

### Identity

A push agent registers as an environment keyed by `<token name>:<instance id>`.
The instance id is derived from what the agent manages: in a pod, the cluster's
id (the `kube-system` namespace UID); on a host, the Docker daemon's id. So a
restart, redeploy or reinstall updates the same environment instead of leaving
an offline duplicate, while agents on different hosts each appear on their own.

Run **one agent per cluster or daemon**: two agents with the same identity are
the same environment, and the newest connection takes over. If no runtime is
reachable at startup, the agent uses a temporary id and switches once one
answers.

### Staying accurate

- **Heartbeats.** A push agent reports in every `PULSE_HEARTBEAT_INTERVAL`
  (30 s). One silent for `agent_offline_threshold_seconds` (90 s) is marked
  offline, even if its connection never closed cleanly.
- **Keepalive and reconnect.** Both ends ping each other every 20 s and drop the
  session when a ping goes unanswered for 10 s, so a link that dies silently
  (network drop, crashed host) is noticed in about 30 s. The agent then
  reconnects, waiting 1 s at first and doubling to 30 s, with some randomness so
  a fleet doesn't reconnect all at once. Requests run concurrently, so a slow one
  doesn't block the others or the pings.
- **Shutdown vs outage.** On a clean shutdown (SIGTERM, which is how Kubernetes,
  ECS and autoscalers stop things) the agent says goodbye first and goes offline
  quietly. An abrupt disconnect sends a Slack alert if it stays down for
  `agent_alert_grace_seconds` (60 s), so rolling replacements stay quiet but
  real outages don't.
- **Retention.** Self-registered environments offline for
  `agent_retention_hours` (6 h) are deleted.
- **Pull agents** never report in, so the gateway pings each one every 30 s and
  marks it online or offline from the answer. They alert like push agents but
  are never deleted, since an admin added them.

The three windows are settings in the gateway's database
([configuration.md](configuration.md#runtime-settings)).

## Storage

The gateway's storage sits behind the `Repository` interface in
`internal/gateway/storage`, with SQLite (pure Go, no cgo) as the implementation.
Migrations are embedded in the binary and run at startup.

To add PostgreSQL: implement `Repository` in a `postgres.go` in that package
(with `database/sql` and `pgx`, for example) and choose it in the one place in
`storage.go` that constructs the repository. Nothing else changes.

## Footprint

Measured with [`scripts/benchmark.sh`](../scripts/benchmark.sh) on v0.1.1
(November 2026): release builds, Apple Silicon, one agent connected to Docker
and Kubernetes, one gateway, SQLite, 30 s per run.

| Process | State | Peak memory (RSS) | Average CPU |
|---|---|---|---|
| agent | idle | ~19.0 MB | ~0.0% |
| gateway | idle | ~19.1 MB | ~0.0% |
| agent | load | ~19.4 MB | ~0.0% |
| gateway | load | ~19.1 MB | ~2.4% |

"Load" is continuous `/health`, `/metrics` and `/api/hosts` requests. Each
sign-in briefly adds about 64 MB to the gateway: Argon2id password hashing uses
that much memory by design, and Go hands it back to the OS shortly after. Reproduce
it with the stack running:

```bash
GATEWAY_PORT=8080 AGENT_PORT=9000 LOAD=1 ./scripts/benchmark.sh 30
# add PULSE_COOKIE="pulse_session=<token>" to include an authenticated endpoint
```

## Technology

- **Backend:** Go: `net/http`, the Docker SDK, client-go, coder/websocket,
  modernc SQLite, gopsutil, Argon2id
- **Dashboard:** Svelte 5, Vite, TypeScript, axios
- **Gateway↔agent:** length-prefixed JSON over TCP, or JSON over WebSocket
