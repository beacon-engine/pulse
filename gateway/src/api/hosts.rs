//! Host (formerly "environment") CRUD — served entirely from the gateway DB.

use axum::{
    extract::{Path, State},
    http::StatusCode,
    Json,
};

use crate::state::AppState;
use crate::storage::models::Environment;

pub use protocol::protocol::request::{CreateHostBody, RenameHostBody};

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

    state
        .db
        .get_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {id} not found")))
        .map(Json)
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

    state
        .db
        .delete_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}
