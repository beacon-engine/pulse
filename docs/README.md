# Pulse Documentation

Welcome to the Pulse documentation! This folder contains comprehensive guides to help you understand, build, and contribute to Pulse.

## What is Pulse?

Pulse is a lightweight, high-performance container orchestration platform built in Rust with native observability and alerting. It addresses the gaps found in existing solutions like Portainer by providing:

- **Native GitOps reconciliation** - Continuous drift detection and auto-remediation
- **First-class observability** - Built-in Prometheus, Grafana, and OpenTelemetry support
- **Real-time alerting** - Slack, Discord, and webhook integrations for instant notifications
- **Lightweight & fast** - Smaller memory footprint and faster execution than Go-based alternatives
- **API-first design** - Complete feature parity between API and UI

## Documentation Structure

### Getting Started
- [**00-project-overview.md**](./00-project-overview.md) - High-level vision, goals, and architecture
- [**01-development-setup.md**](./01-development-setup.md) - Setting up your development environment
- [**02-rust-basics.md**](./02-rust-basics.md) - Rust fundamentals for beginners

### Learning Path
- [**03-hello-world-server.md**](./03-hello-world-server.md) - Your first Rust HTTP server with Axum
- [**04-docker-integration.md**](./04-docker-integration.md) - Connecting to Docker API with Bollard
- [**05-prometheus-metrics.md**](./05-prometheus-metrics.md) - Exposing metrics endpoints
- [**06-websocket-realtime.md**](./06-websocket-realtime.md) - Real-time updates with WebSockets
- [**07-slack-integration.md**](./07-slack-integration.md) - Sending alerts to Slack

### Architecture & Design
- [**architecture/system-overview.md**](./architecture/system-overview.md) - Complete system architecture
- [**architecture/api-design.md**](./architecture/api-design.md) - API endpoints and contracts
- [**architecture/database-schema.md**](./architecture/database-schema.md) - Data models and relationships

### Reference
- [**reference/tech-stack.md**](./reference/tech-stack.md) - Complete technology stack breakdown
- [**reference/dependencies.md**](./reference/dependencies.md) - All Rust crates and their purposes
- [**reference/glossary.md**](./reference/glossary.md) - Terms and concepts

## Learning Approach

Pulse is designed to be beginner-friendly for those new to Rust. The documentation follows a progressive learning path:

1. **Understand the why** - Learn what problems Pulse solves
2. **Set up your environment** - Get Rust and tools installed
3. **Learn Rust basics** - Core concepts needed for the project
4. **Build small components** - Create isolated, well-documented modules
5. **Integrate gradually** - Combine components into the full platform

Each guide includes:
- Clear explanations of concepts
- Heavily commented code examples
- Step-by-step instructions
- Common pitfalls and how to avoid them
- Further reading resources

## Contributing

As you learn and build, feel free to improve these docs! If something is unclear or could be explained better, update the documentation to help future contributors.

## Quick Start

1. Read [Project Overview](./00-project-overview.md) to understand Pulse's vision
2. Follow [Development Setup](./01-development-setup.md) to install Rust and tools
3. Work through [Rust Basics](./02-rust-basics.md) to learn fundamental concepts
4. Build your [first HTTP server](./03-hello-world-server.md) to get hands-on experience

Let's build something great! 🚀
