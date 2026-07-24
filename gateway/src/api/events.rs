//! Recent container lifecycle events, served from the gateway DB (populated by
//! the event watcher — see `crate::watcher`).

use axum::{extract::State, http::StatusCode, Json};

use crate::state::AppState;
use crate::storage::models::ContainerEvent;

pub async fn list_events(
    State(state): State<AppState>,
) -> Result<Json<Vec<ContainerEvent>>, (StatusCode, String)> {
    state
        .db
        .get_recent_events(100)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}
