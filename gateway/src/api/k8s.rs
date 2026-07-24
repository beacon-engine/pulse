//! Cluster-wide Kubernetes reads, forwarded to the agent's kube client.

use axum::{extract::State, http::StatusCode, Json};

use crate::agent::unexpected;
use crate::state::AppState;

use protocol::data::orchestration::{NamespaceInfo, PodInfo};
use protocol::wire::{Request, Response};

type ApiError = (StatusCode, String);

pub async fn pods(State(state): State<AppState>) -> Result<Json<Vec<PodInfo>>, ApiError> {
    match state.agent.call(Request::K8sPods).await {
        Response::Pods(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn namespaces(
    State(state): State<AppState>,
) -> Result<Json<Vec<NamespaceInfo>>, ApiError> {
    match state.agent.call(Request::K8sNamespaces).await {
        Response::Namespaces(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}
