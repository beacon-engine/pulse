use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::Deserialize;
use std::sync::Arc;

use bollard::Docker;

use crate::{
    containers::docker::{self, PortMap},
    observability::{alerts, metrics},
    state::AppState,
    storage::models::Environment,
};

#[derive(Deserialize)]
pub struct CreateHostBody {
    pub name: String,
    pub docker_host: String,
}

#[derive(Deserialize)]
pub struct RenameHostBody {
    pub name: String,
}

#[derive(Deserialize, Default)]
pub struct LogsQuery {
    pub tail: Option<u32>,
}

#[derive(Deserialize)]
pub struct RecreateBody {
    pub ports: Vec<PortMap>,
}

#[derive(Deserialize)]
pub struct EnvBody {
    pub env: Vec<String>,
}

// ── Host CRUD ─────────────────────────────────────────────────────────────────

pub async fn list(
    State(state): State<AppState>,
) -> Result<Json<Vec<Environment>>, (StatusCode, String)> {
    state
        .db
        .list_environments()
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn create(
    State(state): State<AppState>,
    Json(body): Json<CreateHostBody>,
) -> Result<Json<Environment>, (StatusCode, String)> {
    let id = state
        .db
        .create_environment(&body.name, &body.docker_host)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let env = state
        .db
        .get_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::INTERNAL_SERVER_ERROR, "not found after insert".into()))?;

    Ok(Json(env))
}

pub async fn rename(
    State(state): State<AppState>,
    Path(id): Path<String>,
    Json(body): Json<RenameHostBody>,
) -> Result<Json<Environment>, (StatusCode, String)> {
    state
        .db
        .rename_environment(&id, &body.name)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let env = state
        .db
        .get_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {id} not found")))?;

    Ok(Json(env))
}

pub async fn delete(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, (StatusCode, String)> {
    let env = state
        .db
        .get_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {id} not found")))?;

    if env.is_local {
        return Err((StatusCode::BAD_REQUEST, "cannot delete the local host".into()));
    }

    state.env_manager.remove(&id).await;
    state
        .db
        .delete_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}

// ── Container operations ──────────────────────────────────────────────────────

pub async fn list_containers(
    State(state): State<AppState>,
    Path(env_id): Path<String>,
) -> Result<Json<Vec<docker::ContainerInfo>>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    let containers = docker::list(&d)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    metrics::set_container_count(containers.len() as i64);
    metrics::record_request("GET", "/api/hosts/:id/containers", "200");
    Ok(Json(containers))
}

pub async fn inspect_container(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
) -> Result<Json<docker::ContainerDetail>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::inspect(&d, &container_id)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn inspect_raw_container(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::inspect_raw(&d, &container_id)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn recreate_container(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
    Json(body): Json<RecreateBody>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    let new_id = docker::recreate(&d, &container_id, &body.ports)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(serde_json::json!({ "id": new_id })))
}

pub async fn update_container_env(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
    Json(body): Json<EnvBody>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    let new_id = docker::update_env(&d, &container_id, &body.env)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(serde_json::json!({ "id": new_id })))
}

pub async fn container_logs(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
    Query(params): Query<LogsQuery>,
) -> Result<String, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::logs(&d, &container_id, params.tail.unwrap_or(200).min(2000))
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn list_stacks(
    State(state): State<AppState>,
    Path(env_id): Path<String>,
) -> Result<Json<Vec<docker::Stack>>, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::list_stacks(&d)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn start(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
) -> Result<StatusCode, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::start(&d, &container_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    if let Some(ref url) = state.slack_webhook {
        let (url, id) = (url.clone(), container_id.clone());
        tokio::spawn(async move { alerts::container_started(&url, &id).await });
    }
    Ok(StatusCode::NO_CONTENT)
}

pub async fn stop(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
) -> Result<StatusCode, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::stop(&d, &container_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    if let Some(ref url) = state.slack_webhook {
        let (url, id) = (url.clone(), container_id.clone());
        tokio::spawn(async move { alerts::container_stopped(&url, &id).await });
    }
    Ok(StatusCode::NO_CONTENT)
}

pub async fn restart(
    State(state): State<AppState>,
    Path((env_id, container_id)): Path<(String, String)>,
) -> Result<StatusCode, (StatusCode, String)> {
    let d = get_docker(&state, &env_id).await?;
    docker::restart(&d, &container_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    if let Some(ref url) = state.slack_webhook {
        let (url, id) = (url.clone(), container_id.clone());
        tokio::spawn(async move { alerts::container_restarted(&url, &id).await });
    }
    Ok(StatusCode::NO_CONTENT)
}

// ── Helper ────────────────────────────────────────────────────────────────────

async fn get_docker(state: &AppState, env_id: &str) -> Result<Arc<Docker>, (StatusCode, String)> {
    let env = state
        .db
        .get_environment(env_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {env_id} not found")))?;

    state
        .env_manager
        .client_for(&env)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))
}
