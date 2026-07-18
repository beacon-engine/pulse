use axum::{
    extract::{Path, State},
    http::StatusCode,
    Json,
};
use uuid::Uuid;

use crate::{stacks::compose, state::AppState};
use crate::storage::models::SavedStack;

// Stack request/response payloads are shared with the gateway via `protocol`.
pub use protocol::protocol::request::{CreateStackBody, UpdateStackBody};
pub use protocol::protocol::response::{StackOpResponse, TokenResponse};

pub async fn list(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<SavedStack>>, (StatusCode, String)> {
    state
        .db
        .list_stacks(&host_id)
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

pub async fn create(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
    Json(body): Json<CreateStackBody>,
) -> Result<Json<SavedStack>, (StatusCode, String)> {
    let id = state
        .db
        .create_stack(&host_id, &body.name, &body.content)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let stack = state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::INTERNAL_SERVER_ERROR, "not found after insert".into()))?;

    Ok(Json(stack))
}

pub async fn get(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<SavedStack>, (StatusCode, String)> {
    state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {id} not found")))
        .map(Json)
}

pub async fn update(
    State(state): State<AppState>,
    Path(id): Path<String>,
    Json(body): Json<UpdateStackBody>,
) -> Result<Json<SavedStack>, (StatusCode, String)> {
    let env_json = body
        .env_vars
        .as_ref()
        .map(|vars| serde_json::to_string(vars).unwrap_or_default());

    state
        .db
        .update_stack(&id, &body.content, env_json.as_deref())
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {id} not found")))
        .map(Json)
}

pub async fn delete(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, (StatusCode, String)> {
    if let Some(stack) = state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
    {
        let _ = compose::down(&state, &stack).await;
    }

    state
        .db
        .delete_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}

pub async fn up(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<StackOpResponse>, (StatusCode, String)> {
    let stack = state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {id} not found")))?;

    let output = compose::up(&state, &stack).await.map_err(|e| {
        let _ = state.db.update_stack_status(&id, "error");
        (StatusCode::INTERNAL_SERVER_ERROR, e)
    })?;

    state
        .db
        .update_stack_status(&id, "running")
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(StackOpResponse {
        id,
        status: "running".into(),
        output,
    }))
}

pub async fn down(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<StackOpResponse>, (StatusCode, String)> {
    let stack = state
        .db
        .get_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {id} not found")))?;

    let output = compose::down(&state, &stack)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e))?;

    state
        .db
        .update_stack_status(&id, "created")
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(StackOpResponse {
        id,
        status: "created".into(),
        output,
    }))
}

// ── Webhook token management ──────────────────────────────────────────────────

pub async fn set_token(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<TokenResponse>, (StatusCode, String)> {
    let token = Uuid::new_v4().to_string();
    state
        .db
        .set_stack_webhook_token(&id, Some(&token))
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(TokenResponse { token }))
}

pub async fn delete_token(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, (StatusCode, String)> {
    state
        .db
        .set_stack_webhook_token(&id, None)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(StatusCode::NO_CONTENT)
}
