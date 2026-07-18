use axum::{extract::State, http::StatusCode, Json};

use crate::{orchestration::kubernetes, state::AppState};

pub async fn pods(
    State(state): State<AppState>,
) -> Result<Json<Vec<kubernetes::PodInfo>>, (StatusCode, String)> {
    let client = state.k8s.as_ref().ok_or((
        StatusCode::SERVICE_UNAVAILABLE,
        "Kubernetes not connected".to_string(),
    ))?;
    kubernetes::list_pods(client)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn namespaces(
    State(state): State<AppState>,
) -> Result<Json<Vec<kubernetes::NamespaceInfo>>, (StatusCode, String)> {
    let client = state.k8s.as_ref().ok_or((
        StatusCode::SERVICE_UNAVAILABLE,
        "Kubernetes not connected".to_string(),
    ))?;
    kubernetes::list_namespaces(client)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}
