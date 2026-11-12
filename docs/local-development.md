# Local development

Run Pulse from source, change it, and build it.

## Prerequisites

- Go 1.26+
- Node.js 20+ (the dashboard)
- Docker (Pulse needs something to manage; the agent uses the local daemon)

## Run it

Pulse is three processes. Start each in its own terminal:

```bash
# 1. Agent: talks to Docker, Swarm and Kubernetes; the gateway dials it on :9000
go run ./cmd/agent

# 2. Gateway: API, database and auth on :9080
HTTP_PORT=9080 go run ./cmd/gateway

# 3. Dashboard: Vite dev server on :9010, proxying /api and /ws to :9080
cd frontend && npm install && npm run dev
```

Open http://localhost:9010. The first visit asks you to create the admin
account; after that you log in with it.

The gateway keeps everything (users, environments, stacks, settings) in
`pulse.db` in its working directory. Delete the file to start over.

The dashboard's dev server expects the gateway on port 9080. If yours runs
elsewhere: `PULSE_GATEWAY_TARGET=http://localhost:8080 npm run dev`.

### With hot reload

`make tools` installs [air](https://github.com/air-verse/air) (rebuild on save)
and [dlv](https://github.com/go-delve/delve) (debugger) once. Then:

| Command | What it runs |
|---|---|
| `make watch-agent` | The agent, rebuilt and restarted on every save |
| `make watch-gateway` | The gateway on :9080 with its own `pulse-dev.db`, rebuilt on save |
| `make frontend` | The dashboard dev server |
| `make dev` | Agent and gateway **in containers** with your source mounted (hot reload, nothing to install but Docker). Stop with `make dev-down`. |
| `make debug-gateway` / `make debug-agent` | dlv in DAP mode on :2345 / :2346 for your editor to attach to |
| `make dev-debug-gateway` / `make dev-debug-agent` | The same, inside the `make dev` containers |

`make help` lists every target.

## Test and check

```bash
make test        # all Go tests
make check       # gofmt + go vet + tests
make security    # Checkov + Trivy, the same scans CI runs (see security.md)
cd frontend && npm run build   # type-check and build the dashboard
```

`scripts/setup-hooks.sh` turns on the repo's git hooks: before each push they
run gofmt, go vet, the Go tests and the dashboard build. Run it once per clone.

## Build

```bash
make build                 # ./bin/agent and ./bin/gateway, stamped with VERSION
docker compose up -d --build   # the full stack from your checkout; dashboard on :9080
```

The gateway serves the dashboard itself in production: build it with
`npm run build` in `frontend/`, and the gateway serves `frontend/dist` (or
wherever `STATIC_DIR` points).

Docker images are built from the root `Dockerfile`, one target per image:

```bash
docker build --target agent   --build-arg PULSE_VERSION=0.1.0 -t pulse-agent .
docker build --target gateway --build-arg PULSE_VERSION=0.1.0 -t pulse-gateway .
```

Without `PULSE_VERSION` the binaries and dashboard report `dev`. Published
images are built by CI; see [releasing.md](releasing.md).

## Trying Swarm and Kubernetes

See [local-clusters.md](local-clusters.md) for throwaway Swarm and Kubernetes
clusters to point Pulse at.

## Where things are

| Path | What it is |
|---|---|
| `cmd/agent`, `internal/agent` | The agent: Docker, Swarm and Kubernetes access |
| `cmd/gateway`, `internal/gateway` | The gateway: HTTP API, auth, database (`internal/gateway/storage`) |
| `internal/protocol` | The gateway↔agent wire types, shared by both |
| `frontend/` | The Svelte 5 dashboard |
| `deploy/` | Compose file for servers, Kubernetes manifests |
| `scripts/` | Local cluster setup, benchmarks, version bumps |

[architecture.md](architecture.md) explains how the pieces fit together.
