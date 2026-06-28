use axum::{
    extract::{Path, State},
    http::StatusCode,
    Json,
};

use crate::{state::AppState, swarm};

pub async fn list_services(
    State(state): State<AppState>,
    Path(env_id): Path<String>,
) -> Result<Json<Vec<swarm::SwarmService>>, (StatusCode, String)> {
    let docker = state
        .env_manager
        .client_for_env(&state.db, &env_id)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))?;

    swarm::services(&docker)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn list_nodes(
    State(state): State<AppState>,
    Path(env_id): Path<String>,
) -> Result<Json<Vec<swarm::SwarmNode>>, (StatusCode, String)> {
    let docker = state
        .env_manager
        .client_for_env(&state.db, &env_id)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))?;

    swarm::nodes(&docker)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}
