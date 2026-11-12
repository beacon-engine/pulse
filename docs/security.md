# Security

## Signing in

- **First run:** with no users, the dashboard asks you to create the first
  account, which becomes an **admin**. Do this as soon as Pulse is reachable.
- **Sessions:** logging in sets an `HttpOnly` cookie (`pulse_session`) holding a
  random token stored in the database, valid for 7 days. Passwords are hashed
  with Argon2.
- **Roles:** `admin` (everything, including user management) and `viewer`
  (read-only: any change is refused with `403`). Custom roles are planned.
- **Users** are managed in **Settings → Users**, which won't let you delete the
  last admin or your own account.

**Before exposing Pulse:** the session cookie isn't marked `Secure`, so that
local development works over plain HTTP. In production, serve Pulse over HTTPS
([deploy-docker.md](deploy-docker.md#3-put-it-behind-https)). Marking the cookie
`Secure` is a one-line change in `SetSessionCookie`
(`internal/gateway/auth/auth.go`): add `Secure: true`.

## Gateway↔agent connections

| Mode | How the agent is authenticated | Encryption |
|---|---|---|
| **Push** (agent dials `/agent/ws`) | A token from `PULSE_AGENT_TOKENS`, sent when connecting; unknown tokens are refused. The agent checks the gateway's identity only through its TLS certificate. | Only with `wss://`. With `ws://` the token and all traffic are plain text. |
| **Pull** (gateway dials the agent) | **None.** Anyone who can reach the agent's port can drive it, and with a Docker socket that is root on the host. | None over TCP; `wss://` only with TLS in front of the agent. |

So: use push mode with a `wss://` gateway address wherever the network isn't
fully trusted, and never expose a pull agent's port beyond a private network.
The provided compose files keep the local agent's port on the internal Docker
network.

Other limits today: agents share tokens (one can't be revoked on its own), and
the token comparison isn't constant-time.

### Planned

**Agent enrollment** ([design](design/agent-enrollment.md)): one-time join codes
and a separate, revocable credential per agent.

**Encrypted, mutually authenticated links in both modes,** built on the
[Noise protocol](https://noiseprotocol.org) (the framework behind WireGuard):

- On first start each side generates a long-lived key pair.
- The first handshake exchanges and pins the public keys, authenticated by a
  one-time pairing token (`PULSE_PAIRING_TOKEN`, printed by the agent and given
  to the gateway), so nobody can intercept that first connection.
- Every session then derives fresh keys, so recorded traffic stays private even
  if a long-lived key leaks later (forward secrecy).

## What the agent can touch

- **Docker hosts:** the Docker socket, which is full control of the host's
  Docker. It refuses to stop or replace Pulse's own containers (labelled
  `pulse/role`).
- **Kubernetes:** only what its ClusterRole allows: list most objects, read
  logs, delete pods, scale and restart workloads. No Secrets, no exec, nothing
  created. See [deploy-kubernetes.md](deploy-kubernetes.md#what-the-agent-may-do).

## Scanning the code

`.github/workflows/security.yml` runs on every PR into a `release/**` or
`hotfix/**` branch, and weekly on `master` to catch newly published
vulnerabilities in code that hasn't changed:

| Scanner | Checks | Fails on |
|---|---|---|
| **Checkov** | Dockerfile, Kubernetes manifests, GitHub workflows, hard-coded secrets | Any failed check |
| **Trivy** (`fs`) | Go and npm dependencies, committed secrets | HIGH/CRITICAL with a fix available |
| **Trivy** (`image`) | The built agent and gateway images (OS packages and binaries) | HIGH/CRITICAL with a fix available |

Findings without a fix don't block, but every finding goes to the repository's
**Security** tab (when code scanning is enabled).

The same scans run locally, using the scanners' official images pinned by
digest, so there's nothing to install:

```bash
make security          # everything
make security-iac      # Checkov
make security-deps     # Trivy: dependencies and secrets
make security-images   # Trivy: build and scan both images
```

**Accepting a finding:** don't loosen the global configuration. Record the
exception next to what it's about, with the reason:

- Checkov in a Dockerfile: `# checkov:skip=CKV_DOCKER_3: why`
- Checkov in a Kubernetes manifest: a `checkov.io/skipN: CKV_K8S_38=why`
  annotation on the resource
- Trivy: an entry in `.trivyignore.yaml`, scoped by `paths`, with a `statement`
  and an `expired_at` date so it gets looked at again
