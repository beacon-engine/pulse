# Pulse Implementation Plan

This document outlines the step-by-step implementation of Pulse, broken down into small, manageable modules. Each module builds on the previous one, progressing from simplest to most complex.

## Learning Approach

- **One module at a time** - Complete each fully before moving to the next
- **Heavily commented code** - Every line explained for learning
- **Test as you go** - Verify each module works before proceeding
- **Incremental complexity** - Start simple, add features gradually

---

## Module 1: Basic HTTP Server ⭐ (EASIEST)

**Goal**: Create a simple HTTP server with one endpoint

**What you'll learn**:
- Setting up a Rust project with Cargo
- Using Axum web framework
- Async/await basics with Tokio
- Running and testing a server

**Endpoints**:
- `GET /health` - Returns "OK"

**Dependencies**:
- `tokio` - Async runtime
- `axum` - Web framework

**Estimated time**: 30 minutes

**Files to create**:
- `src/main.rs` - Entry point
- `Cargo.toml` - Dependencies

**Success criteria**:
- Server starts on port 3000
- `curl http://localhost:3000/health` returns "OK"

---

## Module 2: JSON Responses ⭐

**Goal**: Add structured JSON responses

**What you'll learn**:
- Defining structs with Serde
- Serializing data to JSON
- Returning JSON from endpoints

**New endpoints**:
- `GET /api/status` - Returns JSON with server status

**New dependencies**:
- `serde` - Serialization framework
- `serde_json` - JSON support

**Estimated time**: 20 minutes

**Files to modify**:
- `src/main.rs` - Add JSON handlers
- `Cargo.toml` - Add serde

**Success criteria**:
- `curl http://localhost:3000/api/status` returns valid JSON
- JSON includes: status, version, uptime

---

## Module 3: Docker Connection (Read-Only) ⭐⭐

**Goal**: Connect to Docker and list containers

**What you'll learn**:
- Using Bollard Docker client
- Handling async operations
- Error handling with Result
- Working with external APIs

**New endpoints**:
- `GET /api/containers` - List all containers

**New dependencies**:
- `bollard` - Docker API client

**Estimated time**: 1 hour

**Files to create**:
- `src/docker.rs` - Docker client module

**Files to modify**:
- `src/main.rs` - Add container endpoint
- `Cargo.toml` - Add bollard

**Success criteria**:
- Successfully connects to Docker daemon
- Lists all containers with ID, name, status
- Returns empty array if no containers running

---

## Module 4: Prometheus Metrics ⭐⭐

**Goal**: Expose metrics for monitoring

**What you'll learn**:
- Prometheus metrics format
- Using the prometheus crate
- Tracking application metrics

**New endpoints**:
- `GET /metrics` - Prometheus metrics endpoint

**Metrics to expose**:
- `pulse_http_requests_total` - Total HTTP requests
- `pulse_containers_total` - Number of containers
- `pulse_uptime_seconds` - Server uptime

**New dependencies**:
- `prometheus` - Metrics library

**Estimated time**: 45 minutes

**Files to create**:
- `src/metrics.rs` - Metrics module

**Success criteria**:
- `/metrics` endpoint returns Prometheus format
- Metrics update in real-time
- Can be scraped by Prometheus

---

## Module 5: WebSocket Server ⭐⭐⭐

**Goal**: Real-time updates for container status

**What you'll learn**:
- WebSocket protocol
- Broadcasting messages to clients
- Maintaining connection state

**New endpoints**:
- `GET /ws` - WebSocket connection

**Features**:
- Clients connect via WebSocket
- Server broadcasts container status changes
- Heartbeat to keep connections alive

**New dependencies**:
- `tokio-tungstenite` - WebSocket support

**Estimated time**: 1.5 hours

**Files to create**:
- `src/websocket.rs` - WebSocket handler

**Success criteria**:
- Clients can connect via WebSocket
- Real-time updates when container status changes
- Multiple clients can connect simultaneously

---

## Module 6: Slack Integration ⭐⭐

**Goal**: Send alerts to Slack

**What you'll learn**:
- HTTP POST requests
- Webhook integration
- Async HTTP client

**Features**:
- Send alert when container crashes
- Send notification when container starts
- Configurable webhook URL

**New dependencies**:
- `reqwest` - HTTP client

**Estimated time**: 1 hour

**Files to create**:
- `src/alerts.rs` - Alert module
- `src/config.rs` - Configuration

**Success criteria**:
- Successfully sends messages to Slack
- Alerts triggered on container events
- Configurable via environment variables

---

## Module 7: Docker Container Control ⭐⭐⭐

**Goal**: Start, stop, restart containers

**What you'll learn**:
- Docker API write operations
- Error handling for operations
- State management

**New endpoints**:
- `POST /api/containers/{id}/start` - Start container
- `POST /api/containers/{id}/stop` - Stop container
- `POST /api/containers/{id}/restart` - Restart container

**Estimated time**: 1 hour

**Files to modify**:
- `src/docker.rs` - Add control functions

**Success criteria**:
- Can start stopped containers
- Can stop running containers
- Can restart containers
- Proper error messages for invalid operations

---

## Module 8: Kubernetes Basic Connectivity ⭐⭐⭐⭐

**Goal**: Connect to Kubernetes and list pods

**What you'll learn**:
- Kubernetes API
- Using kube-rs client
- Working with namespaces

**New endpoints**:
- `GET /api/k8s/pods` - List all pods
- `GET /api/k8s/namespaces` - List namespaces

**New dependencies**:
- `kube` - Kubernetes client
- `k8s-openapi` - Kubernetes API types

**Estimated time**: 2 hours

**Files to create**:
- `src/kubernetes.rs` - K8s client module

**Success criteria**:
- Connects to local or remote K8s cluster
- Lists pods across all namespaces
- Returns pod status and metadata

---

## Module 9: Database Layer ⭐⭐⭐

**Goal**: Persist configuration and state

**What you'll learn**:
- SQLite with sqlx
- Database migrations
- Async database queries

**Features**:
- Store container history
- Save alert configurations
- Track metrics over time

**New dependencies**:
- `sqlx` - Async SQL client
- `sqlx-cli` - Migration tool

**Estimated time**: 2 hours

**Files to create**:
- `src/db.rs` - Database module
- `migrations/` - SQL migrations

**Success criteria**:
- Database initializes on startup
- Can query historical data
- Migrations run automatically

---

## Module 10: Frontend UI (Svelte) ⭐⭐⭐⭐

**Goal**: Build simple web interface

**What you'll learn**:
- Svelte basics
- API integration
- Real-time updates via WebSocket

**Features**:
- Dashboard showing container status
- Start/stop buttons for containers
- Real-time status updates

**New dependencies**:
- Node.js and npm (for Svelte)
- SvelteKit

**Estimated time**: 4 hours

**Files to create**:
- `frontend/` - Svelte application

**Success criteria**:
- UI displays container list
- Can control containers from UI
- Real-time updates work

---

## Progress Tracking

| Module | Status | Completion Date |
|--------|--------|-----------------|
| 1. Basic HTTP Server | ⏳ Not Started | - |
| 2. JSON Responses | ⏳ Not Started | - |
| 3. Docker Read-Only | ⏳ Not Started | - |
| 4. Prometheus Metrics | ⏳ Not Started | - |
| 5. WebSocket Server | ⏳ Not Started | - |
| 6. Slack Integration | ⏳ Not Started | - |
| 7. Docker Control | ⏳ Not Started | - |
| 8. Kubernetes Basic | ⏳ Not Started | - |
| 9. Database Layer | ⏳ Not Started | - |
| 10. Frontend UI | ⏳ Not Started | - |

**Legend**: ⏳ Not Started | 🚧 In Progress | ✅ Complete

---

## Current Focus

**Next up**: Module 1 - Basic HTTP Server

This is the foundation. Once you complete this, everything else builds on top of it.

---

## Notes

- Each module has its own guide in `docs/modules/`
- Code is heavily commented for learning
- Test each module thoroughly before moving on
- Don't rush - understanding is more important than speed
- Ask questions if anything is unclear!
