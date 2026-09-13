//! Host (formerly "environment") CRUD — served entirely from the gateway DB.

use axum::{
    Json,
    extract::{Path, Query, State},
    http::StatusCode,
};

use crate::api::{Page, PageQuery};
use crate::state::AppState;
use crate::storage::models::Environment;

pub use protocol::protocol::request::{CreateHostBody, RenameHostBody};

pub async fn list(
    State(state): State<AppState>,
    Query(q): Query<PageQuery>,
) -> Result<Json<Page<Environment>>, (StatusCode, String)> {
    let (page, per_page) = q.bounds();
    let search = q.search_term();
    let total = state
        .db
        .count_environments(search.as_deref())
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))? as usize;
    let offset = ((page - 1) * per_page) as i64;
    let items = state
        .db
        .list_environments_page(search.as_deref(), per_page as i64, offset)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(Page {
        items,
        total,
        page,
        per_page,
    }))
}

pub async fn create(
    State(state): State<AppState>,
    Json(body): Json<CreateHostBody>,
) -> Result<Json<Environment>, (StatusCode, String)> {
    let id = state
        .db
        .create_environment(&body.name, &body.docker_host, body.agent_id.as_deref())
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let env = state
        .db
        .get_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                "not found after insert".into(),
            )
        })?;

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
        return Err((
            StatusCode::BAD_REQUEST,
            "cannot delete the local host".into(),
        ));
    }

    state
        .db
        .delete_environment(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}
