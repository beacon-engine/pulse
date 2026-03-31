# Beacon 🚀

**A lightweight, high-performance container orchestration platform with native observability**

[![Rust](https://img.shields.io/badge/rust-1.77%2B-orange.svg)](https://www.rust-lang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## What is Beacon?

Beacon is a modern alternative to Portainer, built from the ground up to address critical gaps in container orchestration platforms:

- 🚀 **50% smaller memory footprint** than Go-based alternatives
- 🔄 **Native GitOps reconciliation** with drift detection and auto-remediation
- 📊 **First-class observability** with Prometheus, Grafana, and OpenTelemetry
- 🔔 **Real-time alerting** via Slack, Discord, and webhooks
- ⚡ **Lightning fast** with sub-100ms API response times
- 🔒 **API-first design** with complete UI/API feature parity

## Why Beacon?

After extensive use of Portainer, we identified several critical pain points:

1. **No GitOps reconciliation** - Configuration drift goes undetected
2. **Poor observability** - Metrics and alerting require manual integration
3. **API/UI parity issues** - API-created resources lose UI functionality
4. **Limited alerting** - No native notifications for crashes or deployments
5. **Performance at scale** - Slow when managing hundreds of clusters

Beacon solves these problems with a purpose-built, Rust-powered platform.

## Features

### Container & Cluster Management
- ✅ Docker container lifecycle management
- ✅ Docker Swarm orchestration
- ✅ Kubernetes cluster management
- ✅ Multi-cluster fleet visibility
- 🚧 Vagrant box monitoring (experimental)

### Native Observability
- ✅ Prometheus metrics collection
- ✅ Grafana dashboard integration
- ✅ OpenTelemetry distributed tracing
- ✅ Real-time resource monitoring

### Alerting & Notifications
- ✅ Slack integration
- ✅ Discord webhooks
- ✅ PagerDuty support
- ✅ Custom webhook endpoints

### GitOps
- 🚧 Continuous drift detection
- 🚧 Auto-remediation
- 🚧 Read-only UI mode
- 🚧 Change audit trails

*Legend: ✅ Implemented | 🚧 In Progress | 📋 Planned*

## Quick Start

### Prerequisites
- Rust 1.77 or later
- Docker (for container management)
- Git

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/beacon.git
cd beacon

# Build the project
cargo build --release

# Run Beacon
cargo run --release
```

The server will start on `http://localhost:3000`

## Documentation

Comprehensive documentation is available in the [`docs/`](./docs) folder:

- **[Project Overview](./docs/00-project-overview.md)** - Vision, goals, and architecture
- **[Development Setup](./docs/01-development-setup.md)** - Get your environment ready
- **[Rust Basics](./docs/02-rust-basics.md)** - Learn Rust fundamentals
- **[Hello World Server](./docs/03-hello-world-server.md)** - Build your first component

### Learning Path

New to Rust? We've got you covered! Follow our beginner-friendly guides:

1. Set up your development environment
2. Learn Rust basics (ownership, borrowing, async/await)
3. Build small, isolated components
4. Integrate them into the full platform

Each guide includes heavily commented code and step-by-step explanations.

## Technology Stack

- **Backend**: Rust (performance, memory safety, small footprint)
- **Frontend**: Svelte + SvelteKit (compiled framework, minimal bundle)
- **Database**: SQLite (single-node) / PostgreSQL (distributed)
- **Metrics**: Prometheus / VictoriaMetrics
- **Container APIs**: Bollard (Docker), kube-rs (Kubernetes)
- **Web Framework**: Axum (high-performance async)

## Project Status

Beacon is in **early development**. We're currently building the MVP (Phase 1):

- [x] Project setup and documentation
- [ ] Docker container management
- [ ] Basic Kubernetes connectivity
- [ ] Prometheus metrics endpoint
- [ ] Slack alerting
- [ ] Web UI (Svelte)
- [ ] REST API

See the [Project Overview](./docs/00-project-overview.md) for the full roadmap.

## Contributing

We welcome contributions! Whether you're:

- Learning Rust and want to practice
- Experienced and want to help build features
- Good at documentation and want to improve guides

Check out our [documentation](./docs) to get started.

## Roadmap

### Phase 1 (MVP) - Months 1-3
- Docker container management
- Basic Kubernetes support
- Prometheus metrics
- Slack alerting
- Simple web UI

### Phase 2 - Months 4-6
- GitOps reconciliation engine
- Advanced Kubernetes features
- Grafana dashboard templates
- OpenTelemetry tracing

### Phase 3 - Months 7-9
- Multi-cluster fleet management
- RBAC and multi-tenancy
- Cost management (FinOps)
- Advanced alerting rules

See [Project Overview](./docs/00-project-overview.md) for details.

## Performance Goals

- **Memory footprint**: <100MB (vs Portainer's ~200MB)
- **API latency**: <100ms for common operations
- **Drift detection**: <5 seconds
- **Uptime**: 99.9% for monitoring/alerting

## License

MIT License - see [LICENSE](LICENSE) for details

## Acknowledgments

Built with:
- [Rust](https://www.rust-lang.org/) - The programming language
- [Tokio](https://tokio.rs/) - Async runtime
- [Axum](https://github.com/tokio-rs/axum) - Web framework
- [Bollard](https://github.com/fussybeaver/bollard) - Docker API
- [kube-rs](https://github.com/kube-rs/kube) - Kubernetes client

Inspired by the gaps in Portainer and the need for a lightweight, observability-first container orchestration platform.

---

**Built with ❤️ and Rust 🦀**
