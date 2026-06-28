use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::{Deserialize, Serialize};

use crate::containers::docker;
use crate::state::AppState;

#[derive(Deserialize, Default)]
pub struct RecreateQuery {
    pub pull: Option<bool>,
}

#[derive(Serialize)]
pub struct RecreateWebhookResponse {
    pub id: String,
    pub pulled: bool,
}

pub async fn recreate_container(
    State(state): State<AppState>,
    Path((host_id, container_id)): Path<(String, String)>,
    Query(params): Query<RecreateQuery>,
) -> Result<Json<RecreateWebhookResponse>, (StatusCode, String)> {
    let docker = state
        .env_manager
        .client_for_env(&state.db, &host_id)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))?;

    let pull = params.pull.unwrap_or(false);
    let new_id = docker::recreate_latest(&docker, &container_id, pull)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(RecreateWebhookResponse {
        id: new_id,
        pulled: pull,
    }))
}
