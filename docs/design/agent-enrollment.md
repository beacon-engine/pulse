# Design: Agent enrollment (UI-driven install + per-agent credentials)

**Status:** Draft for review · **Scope:** how an agent gets connected to a gateway
and how it authenticates afterwards. Builds on the push and pull transports ([architecture](../architecture.md)); the
request/response protocol is unchanged.

## Summary

Connecting an agent should be something Pulse does for you, not something you
assemble from environment variables. Two changes:

1. **Enroll from the UI.** "Add environment" creates a short-lived **join token**
   in the gateway's database and shows a ready-to-run install command (a
   Kubernetes manifest or a `docker run`) with everything already filled in.
   The user runs one command; the environment appears.
2. **Join once, then use the agent's own credential.** On first contact the agent trades the
   join token for a **per-agent credential** that it stores locally and uses from
   then on. Each agent can be listed, identified and **revoked individually**
   from the UI, with no restart.

`PULSE_AGENT_TOKENS` keeps working unchanged for GitOps/Terraform setups, but it
stops being the way most people connect an agent.

## Problem

To connect one agent today, a user must:

| Step | What they have to know |
|---|---|
| Invent a secret | That one is needed, and how strong |
| Put `name:secret` in the gateway's `PULSE_AGENT_TOKENS` | The pair format; that the name is a label the agent never sees |
| Restart the gateway | That tokens are only read at boot |
| Put the same secret in a Kubernetes Secret / env var | That the halves must match exactly |
| Edit the manifest: image, gateway URL, token | Which `REPLACE_ME`s exist, `ws://` vs `wss://`, which URL the cluster can reach |
| Not re-apply the manifest later | That doing so resets the Secret to the placeholder |

And afterwards: agents sharing a token can't be told apart, one can't be revoked
without revoking all of them, the secret sits in plaintext in the gateway env,
and nothing shows which tokens are actually in use.

## Goals / non-goals

**Goals**
- Connecting a cluster or host is: open the UI → pick a kind → run one command.
- No secret is ever invented, copied between two places, or seen again after install.
- Every agent has its own credential: listable, revocable, rotatable.
- Credentials at rest are hashed; a database leak doesn't hand out agent access.
- Works for one-off hosts *and* autoscaled fleets (ECS, Karpenter).
- Existing `PULSE_AGENT_TOKENS` deployments keep working with no change.

**Non-goals (for now)**
- mTLS / client certificates (credential-over-TLS is enough; revisit later).
- A Helm chart (the generated manifest is the first step; Helm can wrap it).
- Multi-gateway / HA token replication.

## User flow

1. **Hosts → Add environment → "Connect with a Pulse agent"**, then pick a kind:
   *Kubernetes cluster*, *Docker host* or *Podman host*. Give it a name.
2. The UI shows the install command for that kind, plus the gateway URL agents
   will dial (pre-filled from the browser's origin, editable, saved as a
   setting — see [Gateway public URL](#gateway-public-url)):

   ```sh
   # Kubernetes
   curl -fsSL -H "Authorization: Bearer pjt_9f3a…" \
     https://pulse.example.com/install/kubernetes.yaml | kubectl apply -f -

   # Docker
   docker run -d --name pulse-agent --restart unless-stopped \
     -l pulse/role=agent -v pulse-agent:/var/lib/pulse \
     -v /var/run/docker.sock:/var/run/docker.sock -v /:/host:ro -e HOST_ROOT=/host \
     -e PULSE_GATEWAY_URL=https://pulse.example.com -e PULSE_JOIN_TOKEN=pjt_9f3a… \
     beaconengine/pulse-agent:0.2.0
   ```
3. The modal shows **"Waiting for the agent to connect…"** and updates live once
   it does: *"Connected: k8s-orbstack · Kubernetes v1.36.5"*. Done.

Pull-mode environments ("Pulse agent address", "Container API endpoint") stay as
they are in the same modal, for setups where the gateway must dial in.

## Join tokens

Created by `POST /api/enrollments` (admin only):

```json
{ "name": "prod-cluster", "kind": "kubernetes", "ttl_hours": 24, "max_uses": 1 }
```

returns the token **once** (`pjt_` + 32 random bytes, base64url), plus the
rendered install commands. The gateway stores only `sha256(token)`.

| Field | Default | Why |
|---|---|---|
| `ttl_hours` | 24 | An unused install command stops working on its own. |
| `max_uses` | 1 | One token, one agent: a leaked command can't enroll a second agent. |

**Fleet tokens** (`max_uses: null`, `ttl_hours: null`) are the opt-in for
autoscaling: every task/node enrolls with the same token and each becomes its
own agent (its derived identity differs per host). They're shown with a warning
and are revocable like any other.

## Joining (join token → agent credential)

```
agent                                   gateway
  │  POST /agent/enroll                    │
  │  Authorization: Bearer pjt_…           │
  │  { identity, kind, name, version }  ─► │  verify hash, unexpired, uses left
  │                                        │  upsert agents(enrollment, identity)
  │  ◄─ { agent_id, credential: pak_… }    │  store sha256(credential) only
  │                                        │
  │  persist credential locally            │
  │                                        │
  │  GET /agent/ws  (upgrade)              │
  │  Authorization: Bearer pak_…        ─► │  look up by hash → agent row
  │  ◄═══════ session (as today) ═══════►  │
```

- **Identity** is the derived instance id from proposal 3 (`k8s-<kube-system
  uid>` / `docker-<daemon id>`), so `(enrollment, identity)` is naturally unique
  and re-enrollment of the same cluster updates the same agent row.
- **Re-enrolling** (credential lost, e.g. the volume was wiped) is allowed while
  the join token is still valid; it rotates that agent's credential rather than
  creating a new agent. After the token expires, the user re-runs "Add
  environment", which is the right prompt, since the agent's state was lost.
- **Where the agent keeps its credential:**
  - *Kubernetes:* a Secret `pulse-agent-credential` in its own namespace. The
    generated RBAC grants `get/create/update` on **that one Secret by
    `resourceNames`**, nothing broader. The join token Secret is deleted once used.
  - *Docker/Podman:* `/var/lib/pulse/credential` (mode 0600) on the
    `pulse-agent` volume in the generated command.
  - *Nowhere writable:* the agent logs a clear warning and re-enrolls on each
    boot, which only works with a fleet token.
- On startup the agent uses its credential if it has one, and the join token only if
  it doesn't. A `401` on the credential triggers one re-enroll attempt, then
  backs off and logs *"credential revoked or unknown — re-run Add environment"*.

## Revocation and rotation

- **Revoke** (`DELETE /api/agents/{id}`): marks the row revoked, **closes its live
  session immediately** (the router already holds it), and marks its environment
  offline *gracefully* (no down alert, since it was deliberate). The agent's next
  reconnect gets `401`.
- **Revoke a join token** stops further enrollments; agents it already enrolled
  keep their own credentials.
- **Rotate** (later milestone): the gateway sends `RotateCredential` over the
  live session; the agent persists the new credential and acks; only then does
  the gateway drop the old hash. No reconnect, no downtime.

## Data model (migration `0013_agent_enrollment.sql`)

```sql
CREATE TABLE enrollments (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,              -- kubernetes | docker | podman
    token_hash  TEXT NOT NULL UNIQUE,       -- sha256, hex
    max_uses    INTEGER,                    -- NULL = fleet token
    uses        INTEGER NOT NULL DEFAULT 0,
    expires_at  TEXT,                       -- NULL = no expiry
    created_by  TEXT NOT NULL,              -- users.id
    created_at  TEXT NOT NULL,
    revoked_at  TEXT
);

CREATE TABLE agents (
    id                TEXT PRIMARY KEY,
    enrollment_id     TEXT REFERENCES enrollments(id),
    identity          TEXT NOT NULL,        -- derived instance id
    credential_hash   TEXT NOT NULL UNIQUE,
    credential_prefix TEXT NOT NULL,        -- first 8 chars, for display only
    kind              TEXT NOT NULL,
    version           TEXT,
    created_at        TEXT NOT NULL,
    last_seen         TEXT,
    revoked_at        TEXT,
    UNIQUE (enrollment_id, identity)
);
```

`environments.agent_id` then holds `agents.id` for enrolled agents. Static-token
agents keep today's `"<token_name>:<identity>"` key, so both kinds coexist in
the same router and table without a data migration.

Tokens are random 256-bit values, so a fast hash (SHA-256) is appropriate; a slow
password hash (argon2, used for user passwords) buys nothing here.

## Gateway public URL

The gateway can't know how agents reach it (`host.docker.internal`, an ingress,
a LAN IP). Add a `public_url` setting (in the `settings` table, editable in the UI),
pre-filled from the browser's origin on first use. The install endpoint derives
`ws(s)://…/agent/ws` from it. The UI warns when it's plain `http://` and not
`localhost`, since credentials would then travel unencrypted.

## API summary

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/enrollments` | admin session | Create a join token + install commands |
| GET | `/api/enrollments` | admin session | List tokens (never the token itself) |
| DELETE | `/api/enrollments/{id}` | admin session | Revoke a join token |
| GET | `/install/{kind}.yaml` | join token (header) | Rendered manifest for that enrollment |
| POST | `/agent/enroll` | join token | Join-token → credential exchange |
| GET | `/agent/ws` | agent credential **or** static token | Session (unchanged) |
| GET | `/api/agents` | session | List agents: name, kind, identity, version, last seen, status |
| DELETE | `/api/agents/{id}` | admin session | Revoke an agent (drops its session) |

`/agent/enroll` and `/install/*` are rate-limited per source IP and log each
attempt (success and failure) to the events table for auditing.

## Compatibility

- `PULSE_AGENT_TOKENS` + `PULSE_TOKEN` keep working exactly as today. The README
  moves them to an *Advanced / headless* section.
- An agent with neither `PULSE_JOIN_TOKEN` nor a stored credential behaves as
  today.
- The existing `deploy/kubernetes/*.yaml` stay as the "bring your own manifest"
  path; the install endpoint is the default.

## Related gaps this should close (or schedule)

- **Pull-mode agents are unauthenticated.** Anything that can reach the agent's
  port can drive its Docker socket. Pull agents should also enroll; the gateway
  then presents the credential when dialing in. This matters more than the
  push-side work, since an exposed pull agent is effectively root on the host.
- **Webhook tokens are stored in plaintext** (`webhook_tokens.token`). Same fix:
  store `sha256`, show the token once.

## Milestones

1. **Credentials (backend):** migration, `/api/enrollments`, `/agent/enroll`,
   credential auth on `/agent/ws` alongside static tokens, agent join + local
   persistence (k8s Secret, Docker volume). *Testable end to end with curl.*
2. **Install UX:** `public_url` setting, `/install/{kind}.yaml`, the Add-environment
   wizard with live "waiting for agent" status.
3. **Lifecycle:** Agents page, revoke (with session teardown), join-token list,
   rotation over the live session.
4. **Pull-mode enrollment** and hashed webhook tokens.
5. **Docs:** quick start becomes "Add environment"; env vars move to Advanced.

## Test plan

- Join token: expired / exhausted / revoked → `401`; hash-only at rest; fleet
  token enrolls N distinct agents.
- Re-enroll same identity → same `agents` row, new credential, old one rejected.
- Revoke → live session closed within one round-trip, env offline with
  `graceful=1`, reconnect `401`, no alert fired.
- Credential persistence: restart the pod / container → no re-enroll, same
  environment.
- Static `PULSE_AGENT_TOKENS` agents unaffected (existing tests stay green).
- Generated manifest passes `kubectl apply --dry-run=server`; generated RBAC
  can't read any Secret but its own.

## Open questions

1. Join-token default TTL: 24 h, or shorter (1 h) since the UI waits live?
2. Should non-admin users be able to enroll agents into environments they own,
   or is enrollment admin-only?
3. For Kubernetes, also offer a Helm values snippet in the wizard, or wait for a chart?
4. Fleet tokens: require an explicit "I understand" toggle, or just a warning?
