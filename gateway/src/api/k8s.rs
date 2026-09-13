//! Per-host Kubernetes reads, routed to the agent that represents that host's
//! cluster. Each host maps to an agent (see `resolve_target`), so the cluster
//! shown depends on which host is addressed.

use axum::{
    Json,
    extract::{Path, State},
    http::StatusCode,
};

use crate::agent::unexpected;
use crate::api::resolve_target;
use crate::state::AppState;

use protocol::data::orchestration::{NamespaceInfo, PodInfo};
use protocol::wire::{Request, Response};

type ApiError = (StatusCode, String);

pub async fn pods(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<PodInfo>>, ApiError> {
    let (agent_id, _docker_host) = resolve_target(&state, &host_id).await?;
    match state
        .agent
        .call_on(agent_id.as_deref(), Request::K8sPods)
        .await
    {
        Response::Pods(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn namespaces(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<NamespaceInfo>>, ApiError> {
    let (agent_id, _docker_host) = resolve_target(&state, &host_id).await?;
    match state
        .agent
        .call_on(agent_id.as_deref(), Request::K8sNamespaces)
        .await
    {
        Response::Namespaces(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn delete_pod(
    State(state): State<AppState>,
    Path((host_id, namespace, name)): Path<(String, String, String)>,
) -> Result<StatusCode, ApiError> {
    let (agent_id, _docker_host) = resolve_target(&state, &host_id).await?;
    match state
        .agent
        .call_on(
            agent_id.as_deref(),
            Request::K8sDeletePod { namespace, name },
        )
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}
