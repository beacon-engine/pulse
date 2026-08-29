//! Pulse gateway — the frontend-facing HTTP API.
//!
//! Owns the database (hosts, stacks, webhook tokens, events) and serves the
//! frontend. Every Docker / Swarm / Kubernetes operation is forwarded to the
//! agent over TCP (see [`agent::AgentClient`]).

use axum::{
    Json, Router,
    extract::{State, WebSocketUpgrade},
    middleware,
    response::{Html, Response},
    routing::{delete, get, post},
};
use serde::Serialize;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use std::path::Path;
use tower_http::services::{ServeDir, ServeFile};

mod agent;
mod api;
mod auth;
mod config;
mod observability;
mod state;
mod storage;
mod watcher;

use agent::AgentClient;
use observability::{metrics, prometheus_proxy, websocket};
use state::AppState;

#[tokio::main]
async fn main() {
    println!("Starting Pulse gateway v0.1.0");

    let cfg = config::Config::from_env();
    metrics::init();
    metrics::init_events_counter();

    let db = storage::init(&cfg.database_url)
        .await
        .expect("Failed to initialize database");
    println!("Database ready at {}", cfg.database_url);

    if cfg.slack_webhook_url.is_some() {
        println!("Slack alerting enabled");
    }

    let state = AppState {
        agent: AgentClient::new(cfg.agent_addr.clone()),
        db,
        events: Arc::new(websocket::new_channel()),
        slack_webhook: cfg.slack_webhook_url,
    };

    // Routes that require an authenticated session. A viewer may issue read
    // (GET) requests; state-changing requests require an admin (see enforce_role).
    let protected = Router::new()
        // Auth (session-scoped)
        .route("/api/auth/logout", post(api::auth::logout))
        .route("/api/auth/me", get(api::auth::me))
        // User management (admin only, enforced in-handler)
        .route("/api/users", get(api::users::list).post(api::users::create))
        .route(
            "/api/users/:id",
            axum::routing::put(api::users::update).delete(api::users::delete),
        )
        // Hosts (formerly environments) — CRUD
        .route("/api/hosts", get(api::hosts::list).post(api::hosts::create))
        .route(
            "/api/hosts/:id",
            delete(api::hosts::delete).put(api::hosts::rename),
        )
        // Per-host container operations
        .route(
            "/api/hosts/:id/containers",
            get(api::docker::list_containers),
        )
        .route(
            "/api/hosts/:id/containers/:cid/start",
            post(api::docker::start),
        )
        .route(
            "/api/hosts/:id/containers/:cid/stop",
            post(api::docker::stop),
        )
        .route(
            "/api/hosts/:id/containers/:cid/restart",
            post(api::docker::restart),
        )
        .route(
            "/api/hosts/:id/containers/:cid/inspect",
            get(api::docker::inspect_container),
        )
        .route(
            "/api/hosts/:id/containers/:cid/raw",
            get(api::docker::inspect_raw_container),
        )
        .route(
            "/api/hosts/:id/containers/:cid/recreate",
            post(api::docker::recreate_container),
        )
        .route(
            "/api/hosts/:id/containers/:cid/env",
            post(api::docker::update_container_env),
        )
        .route(
            "/api/hosts/:id/containers/:cid/exec",
            post(api::docker::exec_container),
        )
        // System info
        .route("/api/hosts/:id/system", get(api::docker::get_system_info))
        .route(
            "/api/hosts/:id/hardware",
            get(api::docker::get_hardware_metrics),
        )
        // Images
        .route("/api/hosts/:id/images", get(api::docker::list_images))
        .route(
            "/api/hosts/:id/images/prune",
            post(api::docker::prune_images),
        )
        .route(
            "/api/hosts/:id/images/:image_id",
            delete(api::docker::delete_image),
        )
        // Volumes
        .route("/api/hosts/:id/volumes", get(api::docker::list_volumes))
        .route(
            "/api/hosts/:id/volumes/prune",
            post(api::docker::prune_volumes),
        )
        .route(
            "/api/hosts/:id/volumes/:vol_name",
            delete(api::docker::delete_volume),
        )
        // Networks
        .route(
            "/api/hosts/:id/networks",
            get(api::docker::list_host_networks),
        )
        .route(
            "/api/hosts/:id/containers/:cid/networks",
            get(api::docker::get_container_networks),
        )
        .route(
            "/api/hosts/:id/containers/:cid/networks/connect",
            post(api::docker::network_connect),
        )
        .route(
            "/api/hosts/:id/containers/:cid/networks/disconnect",
            post(api::docker::network_disconnect),
        )
        .route(
            "/api/hosts/:id/containers/:cid/logs",
            get(api::docker::container_logs),
        )
        // Per-host stacks
        .route(
            "/api/hosts/:host_id/stacks",
            get(api::stacks::list).post(api::stacks::create),
        )
        .route(
            "/api/hosts/:host_id/stacks/detected",
            get(api::docker::list_stacks),
        )
        .route(
            "/api/hosts/:host_id/swarm/services",
            get(api::docker::list_services),
        )
        .route(
            "/api/hosts/:host_id/swarm/services/:service_id/tasks",
            get(api::docker::list_service_tasks),
        )
        .route(
            "/api/hosts/:host_id/swarm/nodes",
            get(api::docker::list_nodes),
        )
        .route(
            "/api/hosts/:host_id/swarm/services/:service_id/update",
            post(api::docker::update_swarm_service),
        )
        .route(
            "/api/hosts/:host_id/swarm/services/:service_id/scale",
            post(api::docker::scale_swarm_service),
        )
        // Webhook execution endpoints
        .route(
            "/api/webhooks/:host_id/:container_id/recreate",
            post(api::webhooks::recreate_container),
        )
        .route(
            "/api/webhooks/:host_id/containers/:name/deploy",
            post(api::webhooks::deploy_container),
        )
        .route(
            "/api/webhooks/:host_id/stacks/:stack_id/redeploy",
            post(api::webhooks::redeploy_stack),
        )
        .route(
            "/api/webhooks/:host_id/swarm/:service_id/update",
            post(api::webhooks::update_swarm_service),
        )
        .route(
            "/api/webhooks/:host_id/swarm/:service_id/scale",
            post(api::webhooks::scale_swarm_service),
        )
        .route(
            "/api/webhooks/:host_id/k8s/:namespace/:deployment/rollout",
            post(api::webhooks::k8s_rollout),
        )
        // Webhook token management (generic: entity_type = container | swarm | k8s)
        .route(
            "/api/webhooks/:host_id/:entity_type/:entity_key/token",
            get(api::webhooks::get_token)
                .post(api::webhooks::set_token)
                .delete(api::webhooks::delete_token),
        )
        .route(
            "/api/stacks/:stack_id",
            get(api::stacks::get)
                .put(api::stacks::update)
                .delete(api::stacks::delete),
        )
        .route("/api/stacks/:stack_id/up", post(api::stacks::up))
        .route("/api/stacks/:stack_id/down", post(api::stacks::down))
        .route(
            "/api/stacks/:stack_id/token",
            post(api::stacks::set_token).delete(api::stacks::delete_token),
        )
        // Events (persisted lifecycle events)
        .route("/api/events", get(api::events::list_events))
        // Prometheus proxy
        .route("/api/prometheus/query", get(prometheus_proxy::query))
        // Kubernetes
        .route("/api/k8s/pods", get(api::k8s::pods))
        .route(
            "/api/k8s/pods/:namespace/:name",
            delete(api::k8s::delete_pod),
        )
        .route("/api/k8s/namespaces", get(api::k8s::namespaces))
        .route("/ws", get(ws_handler))
        // Role gate runs after auth (reads AuthUser); auth runs outermost.
        .route_layer(middleware::from_fn(auth::enforce_role))
        .route_layer(middleware::from_fn_with_state(
            state.clone(),
            auth::require_auth,
        ));

    // Public routes — no session required.
    let public = Router::new()
        .route("/health", get(health_handler))
        .route("/api/status", get(status_handler))
        .route(
            "/api/auth/setup",
            get(api::auth::setup_status).post(api::auth::setup),
        )
        .route("/api/auth/login", post(api::auth::login))
        .route("/metrics", get(metrics_handler));

    let app = public.merge(protected).with_state(state.clone());

    // ── 4. Attach Frontend Static Files / SPA Fallback ─────────────────────────
    let dist_path = Path::new(&cfg.static_dir);
    let app = if dist_path.exists() && dist_path.is_dir() {
        println!("Serving static frontend assets from: {}", cfg.static_dir);
        let index_file = dist_path.join("index.html");
        let serve_dir = ServeDir::new(dist_path).fallback(ServeFile::new(index_file));
        app.fallback_service(serve_dir)
    } else {
        println!(
            "Static directory '{}' not found. Running in API-only dev mode.",
            cfg.static_dir
        );
        app.fallback(get(|| async {
                Html(
                    "<h1>Pulse Gateway API</h1>\
                     <p>Running in API-only / Development mode.</p>\
                     <p>Use the Vite dev server at <code>http://localhost:5173</code> to access the UI.</p>"
                )
            }))
    };

    let http_addr = format!("0.0.0.0:{}", cfg.http_port);
    let listener = tokio::net::TcpListener::bind(&http_addr)
        .await
        .unwrap_or_else(|_| panic!("Failed to bind to port {}", cfg.http_port));

    println!(
        "Pulse gateway listening on http://localhost:{}",
        cfg.http_port
    );
    println!(
        "Forwarding Docker operations to agent at {}",
        cfg.agent_addr
    );

    // Consume the agent's Docker event stream in the background.
    tokio::spawn(watcher::run(state.clone()));

    axum::serve(listener, app)
        .await
        .expect("Server failed to start");
}

async fn health_handler() -> &'static str {
    "OK"
}

#[derive(Serialize)]
struct StatusResponse {
    status: String,
    version: String,
    uptime_seconds: u64,
}

async fn status_handler() -> Json<StatusResponse> {
    let uptime = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
        % 1000;
    Json(StatusResponse {
        status: "running".to_string(),
        version: "0.1.0".to_string(),
        uptime_seconds: uptime,
    })
}

async fn metrics_handler() -> String {
    metrics::render()
}

async fn ws_handler(ws: WebSocketUpgrade, State(state): State<AppState>) -> Response {
    let tx = state.events.clone();
    ws.on_upgrade(move |socket| websocket::handle(socket, tx))
}
