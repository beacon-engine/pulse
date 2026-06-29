use axum::{
    extract::{State, WebSocketUpgrade},
    response::Response,
    routing::{delete, get, post},
    Json, Router,
};
use serde::Serialize;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

mod config;
mod containers;
mod environments;
mod observability;
mod orchestration;
mod stacks;
mod state;
mod storage;
mod swarm;
mod webhooks;

use environments::manager::EnvironmentManager;
use state::AppState;

#[tokio::main]
async fn main() {
    println!("Starting Beacon v0.1.0");

    let cfg = config::Config::from_env();
    observability::metrics::init();
    observability::metrics::init_events_counter();

    let docker = containers::docker::connect().expect("Failed to connect to Docker daemon");
    let events = Arc::new(observability::websocket::new_channel());

    let db = storage::init(&cfg.database_url)
        .await
        .expect("Failed to initialize database");
    println!("Database ready at {}", cfg.database_url);

    let k8s = match orchestration::kubernetes::connect().await {
        Ok(client) => { println!("Kubernetes connected"); Some(client) }
        Err(e) => { println!("Kubernetes unavailable ({}), skipping", e); None }
    };

    if cfg.slack_webhook_url.is_some() {
        println!("Slack alerting enabled");
    }

    let state = AppState {
        docker: Arc::new(docker),
        env_manager: Arc::new(EnvironmentManager::new()),
        events,
        slack_webhook: cfg.slack_webhook_url,
        k8s,
        db,
    };

    let app = Router::new()
        .route("/health", get(health_handler))
        .route("/api/status", get(status_handler))
        // Hosts (formerly environments) — CRUD
        .route("/api/hosts", get(environments::api::list).post(environments::api::create))
        .route("/api/hosts/:id", delete(environments::api::delete).put(environments::api::rename))
        // Per-host container operations
        .route("/api/hosts/:id/containers", get(environments::api::list_containers))
        .route("/api/hosts/:id/containers/:cid/start", post(environments::api::start))
        .route("/api/hosts/:id/containers/:cid/stop", post(environments::api::stop))
        .route("/api/hosts/:id/containers/:cid/restart", post(environments::api::restart))
        .route("/api/hosts/:id/containers/:cid/inspect", get(environments::api::inspect_container))
        .route("/api/hosts/:id/containers/:cid/raw", get(environments::api::inspect_raw_container))
        .route("/api/hosts/:id/containers/:cid/recreate", post(environments::api::recreate_container))
        .route("/api/hosts/:id/containers/:cid/env", post(environments::api::update_container_env))
        .route("/api/hosts/:id/containers/:cid/exec", post(environments::api::exec_container))
        // System info
        .route("/api/hosts/:id/system", get(environments::api::get_system_info))
        .route("/api/hosts/:id/hardware", get(environments::api::get_hardware_metrics))
        // Images
        .route("/api/hosts/:id/images", get(environments::api::list_images))
        .route("/api/hosts/:id/images/prune", post(environments::api::prune_images))
        .route("/api/hosts/:id/images/:image_id", delete(environments::api::delete_image))
        // Volumes
        .route("/api/hosts/:id/volumes", get(environments::api::list_volumes))
        .route("/api/hosts/:id/volumes/prune", post(environments::api::prune_volumes))
        .route("/api/hosts/:id/volumes/:vol_name", delete(environments::api::delete_volume))
        // Networks
        .route("/api/hosts/:id/networks", get(environments::api::list_host_networks))
        .route("/api/hosts/:id/containers/:cid/networks", get(environments::api::get_container_networks))
        .route("/api/hosts/:id/containers/:cid/networks/connect", post(environments::api::network_connect))
        .route("/api/hosts/:id/containers/:cid/networks/disconnect", post(environments::api::network_disconnect))
        .route("/api/hosts/:id/containers/:cid/logs", get(environments::api::container_logs))
        // Per-host stacks
        .route("/api/hosts/:host_id/stacks", get(stacks::api::list).post(stacks::api::create))
        .route("/api/hosts/:host_id/stacks/detected", get(environments::api::list_stacks))
        .route("/api/hosts/:host_id/swarm/services", get(swarm::api::list_services))
        .route("/api/hosts/:host_id/swarm/nodes", get(swarm::api::list_nodes))
        // Webhook execution endpoints
        .route("/api/webhooks/:host_id/:container_id/recreate", post(webhooks::api::recreate_container))
        .route("/api/webhooks/:host_id/containers/:name/deploy", post(webhooks::api::deploy_container))
        .route("/api/webhooks/:host_id/stacks/:stack_id/redeploy", post(webhooks::api::redeploy_stack))
        .route("/api/webhooks/:host_id/swarm/:service_id/update", post(webhooks::api::update_swarm_service))
        .route("/api/webhooks/:host_id/k8s/:namespace/:deployment/rollout", post(webhooks::api::k8s_rollout))
        // Webhook token management (generic: entity_type = container | swarm | k8s)
        .route("/api/webhooks/:host_id/:entity_type/:entity_key/token",
            get(webhooks::api::get_token)
            .post(webhooks::api::set_token)
            .delete(webhooks::api::delete_token)
        )
        .route("/api/stacks/:stack_id", get(stacks::api::get).put(stacks::api::update).delete(stacks::api::delete))
        .route("/api/stacks/:stack_id/up", post(stacks::api::up))
        .route("/api/stacks/:stack_id/down", post(stacks::api::down))
        .route("/api/stacks/:stack_id/token", post(stacks::api::set_token).delete(stacks::api::delete_token))
        // Events (global, from local Docker)
        .route("/api/events", get(containers::api::list_events))
        // Prometheus proxy
        .route("/api/prometheus/query", get(observability::prometheus_proxy::query))
        // Kubernetes
        .route("/api/k8s/pods", get(orchestration::api::pods))
        .route("/api/k8s/namespaces", get(orchestration::api::namespaces))
        .route("/metrics", get(metrics_handler))
        .route("/ws", get(ws_handler))
        .with_state(state.clone());

    let addr = format!("0.0.0.0:{}", cfg.port);
    let listener = tokio::net::TcpListener::bind(&addr)
        .await
        .unwrap_or_else(|_| panic!("Failed to bind to port {}", cfg.port));

    println!("Beacon listening on http://localhost:{}", cfg.port);

    tokio::spawn(containers::watcher::run(
        state.docker.clone(),
        state.db.clone(),
        state.events.clone(),
        state.slack_webhook.clone(),
    ));

    axum::serve(listener, app).await.expect("Server failed to start");
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
    observability::metrics::render()
}

async fn ws_handler(ws: WebSocketUpgrade, State(state): State<AppState>) -> Response {
    let tx = state.events.clone();
    ws.on_upgrade(move |socket| observability::websocket::handle(socket, tx))
}
