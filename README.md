# Pulse

**Lightweight container management for Docker, Docker Swarm and Kubernetes,
with built-in observability.**

[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://go.dev/)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

Pulse is an alternative to Portainer, written in Go: one dashboard for all your
servers and clusters.

- **Small:** two static Go binaries, around 20 MB of memory each
- **Reaches anywhere:** a gateway you log in to, and an agent next to each
  server or cluster that dials out to it, so nothing needs open ports
- **Observable:** Prometheus metrics, live events, Slack alerts
- **API-first:** everything the dashboard does is an HTTP endpoint

## How it works

```
 browser ──► gateway ◄──── agent ──► Docker / Swarm      (server A)
             API, auth,
             database  ◄──── agent ──► Kubernetes API    (cluster B)
```

The **gateway** serves the dashboard and the API and keeps the database. Each
**agent** runs next to what it manages and does the work. More in
[docs/architecture.md](docs/architecture.md).

| Platform | Support |
|---|---|
| **Docker** | Containers, images, volumes, networks, logs, exec, Compose stacks |
| **Docker Swarm** | Services, nodes, tasks, scaling, rolling updates |
| **Kubernetes** | Workloads (scale, rolling restart), pods and containers (logs, inspect, delete), services, nodes, events |
| **Podman** | Partial, through its Docker-compatible socket |
| ECS, Nomad, Cloud Run | Planned ([roadmap](docs/roadmap.md)) |

## Get started

**On a server** (Linux, amd64), from the published images:

```bash
curl -fsSLO https://raw.githubusercontent.com/beacon-engine/pulse/master/deploy/docker/docker-compose.yml
docker compose up -d
```

Open `http://<server>:9080` and create the admin account. Then put it behind
HTTPS and connect your other servers: [docs/deploy-docker.md](docs/deploy-docker.md).

**A Kubernetes cluster:** install one agent in it, which connects back to your
gateway: [docs/deploy-kubernetes.md](docs/deploy-kubernetes.md).

**From source:**

```bash
go run ./cmd/agent                         # terminal 1
HTTP_PORT=9080 go run ./cmd/gateway        # terminal 2
cd frontend && npm install && npm run dev  # terminal 3 → http://localhost:9010
```

More in [docs/local-development.md](docs/local-development.md).

## Documentation

| | |
|---|---|
| [Running on a server](docs/deploy-docker.md) | [Local development](docs/local-development.md) |
| [Managing Kubernetes clusters](docs/deploy-kubernetes.md) | [Local Swarm and Kubernetes clusters](docs/local-clusters.md) |
| [Configuration](docs/configuration.md) | [Releasing](docs/releasing.md) |
| [Security](docs/security.md) | [Architecture](docs/architecture.md) |
| [HTTP API](docs/api.md) | [Roadmap](docs/roadmap.md) |

## License

MIT.
