//! HTTP handlers for the frontend-facing API.
//!
//! Host / stack / token / event records are served from the gateway's own
//! database. Anything that touches Docker, Swarm or Kubernetes is resolved to a
//! `docker_host` connection string and forwarded to the agent over TCP.

pub mod auth;
pub mod docker;
pub mod events;
pub mod hosts;
pub mod k8s;
pub mod stacks;
pub mod users;
pub mod webhooks;

use axum::http::StatusCode;

use crate::state::AppState;

/// Resolves a host id to the `docker_host` connection string the agent needs.
pub async fn resolve_docker_host(
    state: &AppState,
    host_id: &str,
) -> Result<String, (StatusCode, String)> {
    let env = state
        .db
        .get_environment(host_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {host_id} not found")))?;
    Ok(env.docker_host)
}
