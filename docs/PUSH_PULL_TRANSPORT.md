# Design: WebSocket (WSS) agent transport, pull **and** push

**Status:** Draft for review · **Scope:** gateway ↔ agent transport layer only

## Summary

Two changes to how the gateway and agent talk:

1. **Move the transport from raw TCP (L4) to WebSocket Secure (WSS, L7).**
   WSS rides HTTPS/443 with an HTTP `Upgrade` handshake, so it passes through the
   corporate firewalls, HTTP proxies, WAFs and ingresses that block arbitrary TCP
   ports. WebSocket is also natively full-duplex, which is exactly what the
   bidirectional session below needs.
2. **Support both connection directions**, operator's choice per deployment:
   - **Pull (listen):** the agent serves; the gateway connects to it. *(today's
     direction, now over WSS)*
   - **Push (dial):** the agent dials out to the gateway on 443 and holds a
     persistent, **bidirectional** session the gateway drives.

Capability is a **full bidirectional session** in both modes — pod restart,
rollout, delete, logs, and the live event feed all work regardless of who opened
the socket. The application-level `Request`/`Response` types are **unchanged**;
only the transport/session layer changes.

## Why WSS

| Property | Benefit |
|----------|---------|
| Runs on 443 via HTTP `Upgrade` | Traverses firewalls/proxies that allow only HTTPS — the motivating requirement |
| Layer 7 | Works through HTTP `CONNECT` proxies, k8s ingresses, LBs, path routing |
| Full-duplex by design | One socket carries gateway→agent requests **and** agent→gateway replies/events |
| TLS built in ("S") | Encryption + server identity without a bespoke scheme |
| Standard infra | LB health checks, TLS termination, auth headers all off-the-shelf |
| Built-in ping/pong | Liveness/keepalive without a custom heartbeat |

## Core principle (why push keeps full actions)

TCP/WebSocket sessions are full-duplex: once a socket is open, either side may
send, **independent of who initiated it.** "Push vs pull" only decides who dials —
not the direction of application requests. So an outbound-only agent loses no
functionality as long as the push channel is a *persistent bidirectional session*
(which WebSocket is). This is how Teleport, Rancher's cluster agent, and
self-hosted CI runners operate.

## Current architecture (baseline)

- **Agent** (`agent/src/main.rs`): binds a `TcpListener` on `BEACON_PORT`, accepts
  raw TCP, reads framed `Request`s, runs `dispatch`, writes a `Response`.
  `WatchEvents` takes over its connection and streams events.
- **Gateway** (`gateway/src/agent.rs`): `AgentClient::call` opens a **short-lived
  TCP connection per request** to `AGENT_ADDR`; `watch_events` opens a separate
  long-lived stream consumed by `watcher::run`.
- **Framing** (`protocol/src/wire.rs`): bare length-prefixed JSON, **one value per
  frame, no correlation id.** Requests correlate to responses only because each
  gets its own dedicated socket.

Two things change: raw TCP → WebSocket messages, and per-request sockets → one
multiplexed session (which is why we need a correlation id).

## Goals / non-goals

**Goals**
- Transport is WSS in both modes.
- Operator picks pull or push per deployment via config.
- Both modes support the full command set (actions included).
- No change to `Request`/`Response` semantics or the HTTP API handlers.
- Authenticated sessions; groundwork for multiple agents.

**Non-goals (this iteration)**
- Telemetry-only push (rejected — we want full actions).
- A new binary serialization (keep JSON; binary is a later optimization).
- UI redesign beyond an "agent offline" indicator.

## Design

### 1. Transport: WebSocket sessions

Each **WebSocket message carries exactly one envelope** (below). WS message
boundaries replace the manual `u32` length prefix, so `write_frame`/`read_frame`
are retired in favour of `ws.send(Message::Text(json))` / `ws.next()`. Payloads
are JSON text (parity with today's `serde_json`, debuggable); `Message::Binary`
+ bincode is a later optimization. `MAX_FRAME` becomes the WS max-message-size
limit.

### 2. Correlation envelope (protocol crate)

One session multiplexes many concurrent requests **plus** the event stream, so
each message carries an id:

```rust
// protocol/src/wire.rs (sketch)
pub struct RequestFrame  { pub id: u64, pub req:  Request }
pub struct ResponseFrame { pub id: u64, pub resp: Response }
```

- The **gateway** assigns a monotonic `id` per request; the **agent** echoes it on
  the reply.
- **Events** are `ResponseFrame`s carrying `Response::Event` on a reserved stream
  id (e.g. `0`), so the watcher demultiplexes them from RPC replies.
- The special-cased `WatchEvents` connection goes away — events are just messages
  on the one session.

### 3. Directions mapped onto WebSocket

| Mode | Agent role | Gateway role | Agent connects on |
|------|------------|--------------|-------------------|
| **Push** (dial) | WS **client** → `wss://gateway/agent/ws` | WS **server** (axum, already has `ws`) | **outbound 443** |
| **Pull** (listen) | WS **server** on `BEACON_PORT` | WS **client** → `wss://agent/...` | inbound (agent exposes 443) |

Push is the firewall-friendly default (agent only needs outbound 443). Pull still
benefits from WSS because inbound 443 is far more often permitted than an
arbitrary port — but the reachability requirement is inverted, as today.

### 4. Gateway transport abstraction

Both modes sit behind one interface so API handlers never change:

```rust
// gateway (sketch)
trait AgentTransport {
    async fn call(&self, req: Request) -> Response;
    async fn subscribe_events(&self) -> Stream<EventFrame>;
}
```

- **`DialingTransport`** — pull. A WS **client** (tokio-tungstenite) to the agent,
  speaking envelopes.
- **`SessionTransport`** — push. Writes a `RequestFrame` into a **registered agent
  session** and awaits the correlated reply.

### 5. Gateway session registry (push)

- A new WS route on the **existing axum server**, e.g. `GET /agent/ws`, upgraded
  via axum's `WebSocketUpgrade` (same primitive as `observability/websocket.rs`).
- On upgrade the agent authenticates (below). On success the gateway registers a
  `Session`:
  - a writer task owning the WS sink,
  - a `HashMap<u64, oneshot::Sender<Response>>` of in-flight requests,
  - a read loop routing each `ResponseFrame` to its `oneshot`, and
    `Response::Event` frames to the watcher.
- Registry: `HashMap<AgentId, Session>`. v1 can run single-agent (matching today's
  single `AGENT_ADDR`), but keying by `AgentId` now makes **multiple agents** a
  free extension.
- `SessionTransport::call`: allocate `id` → register `oneshot` → send
  `RequestFrame` → await reply with timeout.

### 6. Agent

- New config `BEACON_MODE = listen | dial` (default `listen`).
- **`dial` (push):** open a WSS client to `PULSE_GATEWAY_WS_URL`, authenticate,
  then **serve `RequestFrame`s over that one socket by reusing the existing
  `dispatch`** — identical logic, different transport — and push events on the
  reserved stream id. Reconnect with backoff on drop.
- **`listen` (pull):** accept connections, perform the WS server handshake
  (validating the caller's token), then serve envelopes via the same `dispatch`.

### 7. Authentication — on the WS upgrade

WSS makes auth clean: it's an HTTP handshake, so credentials ride in **headers**,
validated *before* the socket is accepted — no bespoke in-band handshake.

- Agent sends `Authorization: Bearer <BEACON_TOKEN>` in the upgrade request.
- Gateway validates against `PULSE_AGENT_TOKENS` in an axum extractor/middleware
  before completing the upgrade; on mismatch it returns `401` and no session is
  created.
- Pull mode mirrors this: the gateway presents the token to the agent's WS server.
- Upgrade path: **mTLS** (client cert instead of / in addition to the token) where
  the environment supports it.

### 8. TLS termination — **decided: support both, operator's choice** *(implemented)*

The agent always speaks `wss://`; where TLS terminates is a per-deployment choice.
We ship **both** paths rather than mandating a fronting proxy, so proxy-less
deploys are first-class. The gateway picks its mode from config at startup:

- **Terminate upstream (recommended where a proxy exists):** an ingress/LB
  terminates TLS on 443 and forwards `ws://` to the gateway's `HTTP_PORT`. Least
  code on our side — the gateway serves plain HTTP; the "S" is standard infra.
  Selected when `PULSE_TLS_CERT` / `PULSE_TLS_KEY` are **unset**.
- **Native TLS (proxy-less):** the gateway serves rustls directly
  (`axum-server` + the **ring** provider — no OpenSSL, so slim/musl images link
  without a system `libssl`). Selected when **both** `PULSE_TLS_CERT` and
  `PULSE_TLS_KEY` point at a PEM cert/key pair.

Rationale for shipping both: upstream termination covers managed k8s/LB
environments with zero extra gateway code, while native rustls is required for
the on-prem / proxy-less deploys we target — and it's cheap because the crypto
provider (`ring`) is already in the tree for the agent's outbound `wss://`
client. See `gateway/src/main.rs` (the `tls_cert_path`/`tls_key_path` match) and
`gateway/src/config.rs`.

On the client side the agent verifies the gateway certificate against the
bundled **webpki** roots (no OS cert store dependency); `PULSE_INSECURE_SKIP_TLS_VERIFY`
disables verification for self-signed gateways on trusted networks only.

## Config summary

| Side | Pull (listen) | Push (dial) |
|------|---------------|-------------|
| Agent | `BEACON_MODE=listen`, `BEACON_PORT`, `BEACON_TOKEN` | `BEACON_MODE=dial`, `PULSE_GATEWAY_WS_URL=wss://host/agent/ws`, `BEACON_TOKEN` |
| Gateway | `AGENT_WS_URL=wss://agent-host/...`, token | exposes `/agent/ws`, `PULSE_AGENT_TOKENS` |

The gateway can run both simultaneously; each agent's transport is chosen by how
it's configured/registered.

## Failure & offline semantics

- **Liveness:** WebSocket ping/pong (control frames) detects dead sessions without
  a custom heartbeat; the client reconnects with backoff (like today's 5s watcher
  reconnect).
- **Push:** actions work only while the session is live. With no session,
  `SessionTransport::call` returns the existing `Response::Error { status: 503 }`
  ("agent offline") — same path a pull failure already uses.
- **Pull:** unreachable agent yields the existing `502`.
- The UI should show an **agent connected / offline** indicator, since in push
  mode a missing session is an operational state, not an exception.

## Dependencies

- **Agent:** add `tokio-tungstenite` (rustls TLS — the agent already pulls rustls
  in via `kube`) for both the WS client (push) and WS server (pull) roles.
- **Gateway:** push server reuses the existing `axum` `ws` feature; the pull client
  adds `tokio-tungstenite`.

## Testing plan

- **Unit:** envelope round-trip + id correlation; registry routes replies to the
  right `oneshot`; unknown/duplicate id handling; auth extractor accept/reject.
- **Integration:** in-process agent **dials** an in-process gateway WS endpoint
  over loopback (`ws://`); assert a gateway-issued action (e.g. `K8sRollout`) is
  executed and its reply correlated, and that events flow on the reserved id.
- **Both modes** over loopback in one test binary (extends `agent/tests/`).

## Phased implementation

1. **Envelope** in the protocol crate (+ unit tests). No behaviour change.
2. **WS transport module** shared shape; gateway `AgentTransport` trait; port the
   pull path to a WS client (`DialingTransport`).
3. **Gateway `/agent/ws` endpoint + session registry + token auth** and
   `SessionTransport`.
4. **Agent `dial` mode** (WS client, reconnect) and **`listen` mode over WS**,
   both reusing `dispatch`; event push on the reserved id.
5. **Config, TLS docs, UI offline state**, and the integration tests above.

Each phase is independently reviewable; the existing path keeps working until
phase 4 swaps it.

## Open questions

1. ~~**TLS termination:** standardize on upstream ingress/LB termination (simplest),
   or also ship native rustls in the gateway for proxy-less deploys?~~
   **Resolved — ship both** (§8). Gateway serves native rustls when
   `PULSE_TLS_CERT`/`PULSE_TLS_KEY` are set, else plain HTTP behind an upstream
   terminator. Implemented in `gateway/src/{main,config}.rs`.
2. **Multi-agent now or later?** The registry keyed by `AgentId` is cheap; wiring
   the gateway's host model to route to a specific agent is the larger part. Ship
   v1 single-agent, or design host→agent routing up front?
3. **JSON vs binary messages:** start with JSON text (debuggable, parity), or go
   straight to bincode `Binary` for throughput?
4. **Path/routing:** single `/agent/ws` endpoint with the agent id from the token,
   or per-agent paths?
