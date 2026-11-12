# Pulse documentation

## Run it

| Guide | For |
|---|---|
| [Running on a server](deploy-docker.md) | Pulse from the published Docker images, HTTPS, connecting more servers, upgrades, backups |
| [Managing Kubernetes clusters](deploy-kubernetes.md) | Installing the agent in a cluster (push or pull), permissions, the Kubernetes page |
| [Configuration](configuration.md) | Every environment variable and runtime setting |

## Work on it

| Guide | For |
|---|---|
| [Local development](local-development.md) | Running from source, hot reload, tests, building binaries and images |
| [Local Swarm and Kubernetes clusters](local-clusters.md) | Throwaway clusters to develop against |
| [Releasing](releasing.md) | Release branches, versions, CI workflows |

## How it works

| Page | For |
|---|---|
| [Architecture](architecture.md) | Gateway, agent and dashboard; push vs pull; identity and liveness; storage; footprint |
| [Security](security.md) | Sign-in and roles, gateway↔agent security, scanning |
| [HTTP API](api.md) | Every endpoint |
| [Roadmap](roadmap.md) | What's done, what's next, more platforms |

## Design notes

- [Agent enrollment](design/agent-enrollment.md): one-time join codes and
  per-agent credentials (planned)
- [Push and pull transport](design/push-pull-transport.md): the WebSocket
  transport design

[archive/](archive/) holds early notes from when Pulse was written in Rust.
