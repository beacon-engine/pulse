# Pulse

**A lightweight, high-performance container orchestration platform with native observability**

---

## Background: Why Pulse?

Based on the latest developments in the Portainer ecosystem, including the release of version 2.39 LTS, the most significant missing enterprise feature has just been introduced: **Fleet Governance Policies**. However, looking at user feedback and competitor analysis, there are several other critical gaps that enterprise users still face when scaling.

Here are the key features that are missing or in need of improvement for enterprise use:

### 1. True GitOps Reconciliation (The "Source of Truth" Gap)
While Portainer supports Git as a source for stacks, it currently acts as a "pull once" mechanism rather than a continuous reconciliation engine.
- **The Gap:** If a developer or a rogue process changes a container configuration directly in the UI or via the CLI (configuration drift), Portainer does not automatically revert it to match the Git repository.
- **Enterprise Need:** Unlike ArgoCD or Flux, Portainer lacks a strict GitOps mode where the UI is read-only and the system actively fights drift. Enterprises need to enforce that the Git repo is the single source of truth, ensuring audit trails and preventing "snowflake" servers.
- **Roadmap:** Portainer has acknowledged this and plans to introduce an "intermediate review step" with optional GitOps integration in the future.

### 2. Advanced Multi-Cluster Visibility (Kubernetes Sprawl)
Even with the new policies, managing dozens or hundreds of Kubernetes clusters remains a visibility challenge.
- **The Gap:** The UI can become slow or cumbersome when managing massive fleets. There is a lack of deep, aggregated observability across *all* clusters at once (e.g., seeing all failed pods across 50 clusters on one screen).
- **Enterprise Need:** Portainer is a "hub and spoke" model, but it often lacks the "fleet-level" dashboards that competitors like Rancher or Qovery offer out of the box. Enterprises need better tools to visualize resource usage, security posture, and cost across the entire organization without clicking into each individual environment.

### 3. Policy Management Maturity (Workload Auditing)
Fleet Governance Policies are a brand-new feature (released in Feb 2026) and are still in the early stages of maturity.
- **The Gap:** Currently, you can apply policies (like security constraints), but the system lacks robust "what-if" analysis.
- **Enterprise Need:** Before applying a strict security policy to a production cluster, administrators need to audit existing workloads to see if they will be broken by the new policy. Portainer currently lacks this "dry-run" or workload auditing capability to preview the impact of policy changes.

### 4. API Consistency and Feature Parity
There is a significant disconnect between the API-driven experience and the UI experience, particularly affecting CI/CD pipelines.
- **The Gap:** A real-world enterprise complaint (as of January 2026) highlighted that stacks deployed via API (common in GitLab CI pipelines) lose critical UI functionality. Features like "Environment Variables" tabs, "Update Stack" buttons, and granular stop/start controls disappear from the UI for API-deployed stacks, forcing admins to redeploy via Git even for minor fixes.
- **Enterprise Need:** Feature parity between the API and the UI is essential. Enterprise workflows rely heavily on automation (Terraform, Pipelines). If the API creates "second-class" resources, it breaks operational workflows and forces manual intervention.

### 5. Enhanced RBAC and Multi-Tenancy
While Role-Based Access Control (RBAC) exists, managing it at scale is painful.
- **The Gap:** There are reports of issues where RBAC policies do not correctly override environment-level settings, leading to privilege leakage where users retain admin rights even after restrictive policies are applied. Additionally, managing access for hundreds of namespaces across dozens of clusters is UI-heavy.
- **Enterprise Need:** Finer-grained access control (down to the pod level) and better integration with Identity Providers (SSO) for team mapping. Competitors like Loft offer "virtual clusters" which provide stronger isolation than Portainer’s native namespace segregation.

### 6. Advanced Cost Management (FinOps)
As organizations scale, cloud costs become a critical operational concern.
- **The Gap:** Portainer focuses on operations and deployment but lacks built-in FinOps tools.
- **Enterprise Need:** Features like automated scheduling to shut down non-production environments during off-hours, resource quota recommendations, and visibility into which teams are consuming the most cloud spend are missing. Competitors like Qovery highlight this as a key differentiator.

### Summary for Enterprise Decision Makers

If you are evaluating Portainer for the Enterprise:
- **The Good:** The new **Fleet Governance Policies** (2.39 LTS) are a massive step forward for standardization. The shift towards Talos OS support (over deprecated MicroK8s) shows a commitment to modern, resilient infrastructure.
- **The Gaps:** Be prepared to supplement Portainer with external tools if you need **strict GitOps reconciliation** (like ArgoCD) or **advanced cost optimization** (like FinOps platforms).

Would you like to dive deeper into how to configure the new Fleet Governance Policies, or explore the specific alternatives mentioned for multi-cluster management?

---

## Building a Lightweight Alternative: Project Requirements

### Vision
Build a high-performance, memory-efficient container orchestration platform that addresses Portainer's gaps while maintaining a smaller footprint and faster execution.

### Core Technical Requirements

**Performance & Efficiency**
- **Memory footprint**: Must be smaller than Portainer (Go-based)
- **Speed**: Prioritize fast execution and low latency
- **Language**: Consider Rust or Zig for maximum performance and minimal memory usage
- **Deployment**: Lightweight enough to run on resource-constrained environments

**Container & Cluster Management**
- Docker container management
- Docker Swarm orchestration
- Kubernetes cluster management
- Vagrant box monitoring and management (investigate feasibility)

### Key Differentiating Features

**1. Native Observability & Alerting**
- **Grafana integration**: First-class support for visualization dashboards
- **Prometheus integration**: Native metrics collection and querying
- **OpenTelemetry**: Built-in support for distributed tracing and crash detection
- **Real-time alerting**: Slack integration for deployment status, crashes, policy violations, and system events
- **Multi-channel notifications**: Support for Discord, PagerDuty, webhooks

**2. GitOps Reconciliation**
- Continuous drift detection and auto-remediation
- Git repository as single source of truth
- Read-only UI mode with strict enforcement
- Audit trails for all configuration changes

**3. API-First Architecture**
- Complete feature parity between API and UI
- No "second-class" resources from API deployments
- GraphQL or gRPC for efficient communication
- Comprehensive API documentation with OpenAPI/Swagger

**4. Fleet-Level Observability**
- Aggregated dashboards across all clusters
- Single-pane view for failures, resource usage, security posture
- Cross-cluster search and filtering
- Real-time status updates via WebSockets

### MVP Scope (Phase 1)
1. Docker container management (start/stop/restart/logs)
2. Basic Kubernetes cluster connectivity
3. Prometheus metrics collection
4. Slack alerting for container crashes
5. Simple web UI for monitoring
6. REST API with full feature parity

### Technical Stack (Confirmed)
- **Backend**: Rust (chosen for performance, memory safety, and career growth opportunities)
- **Frontend**: Svelte + SvelteKit (compiled framework, minimal attack surface, ~3KB bundle, perfect for real-time updates)
  - **Why not React/Next.js**: Frequent vulnerabilities, large bundles, complex ecosystem
  - **Why not Angular**: Too heavy (~150KB+) for a lightweight platform, though it has good security
  - **Why Svelte**: Compiles to vanilla JS, built-in XSS protection, smallest bundle, reactive by default
- **Database**: SQLite for single-node, PostgreSQL for distributed
- **Metrics Storage**: Prometheus or VictoriaMetrics
- **Real-time**: WebSockets for live updates

### Rust Ecosystem Libraries
- **Docker API**: `bollard` (async Docker client)
- **Kubernetes**: `kube-rs` (official Kubernetes client)
- **HTTP Server**: `axum` or `actix-web` (high-performance async web frameworks)
- **Metrics**: `prometheus` crate for exposing metrics
- **Async Runtime**: `tokio` (industry standard)
- **Slack Integration**: `slack-morphism` or custom webhook client
- **OpenTelemetry**: `opentelemetry` + `tracing` crates
- **Database**: `sqlx` (async SQL) or `diesel` (ORM)

### Rust Learning Path (Beginner-Friendly Approach)

**Phase 0: Rust Fundamentals (Before Building)**
1. **Week 1-2**: Basic syntax, ownership, borrowing (via Rust Book chapters 1-4)
2. **Week 3**: Error handling with `Result` and `Option` (chapter 9)
3. **Week 4**: Traits and generics basics (chapter 10)
4. **Week 5**: Async/await fundamentals with simple examples

**Phase 1: Learn by Building Small Components**
1. **Simple HTTP server**: Build "Hello World" API with `axum` (understand routing, handlers)
2. **Docker list containers**: Use `bollard` to connect to Docker and list running containers
3. **Prometheus metrics**: Add a `/metrics` endpoint that exposes basic counters
4. **WebSocket echo**: Implement simple WebSocket server for real-time updates
5. **Slack webhook**: Send a test message to Slack channel

**Phase 2: Integrate Components into MVP**
- Combine the small components into a cohesive application
- Each component will be well-commented and explained
- Incremental complexity: start simple, add features gradually

**Learning Resources**
- Official Rust Book: https://doc.rust-lang.org/book/
- Rust by Example: https://doc.rust-lang.org/rust-by-example/
- Tokio Tutorial: https://tokio.rs/tokio/tutorial
- Axum Examples: https://github.com/tokio-rs/axum/tree/main/examples

**Development Approach**
- Start with heavily commented code examples
- Build one feature at a time
- Test each component independently before integration
- Use `cargo watch` for fast feedback loop
- Leverage Rust's compiler errors as learning opportunities (they're very helpful!)

### Open Questions
- Vagrant box monitoring: Requires investigation into Vagrant API capabilities and whether it fits the orchestration model
- Self-hosted vs SaaS: Start with self-hosted, evaluate SaaS later
- Multi-tenancy: Required for MVP or Phase 2?