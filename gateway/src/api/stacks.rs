//! Persisted stacks: records live in the gateway DB; `up`/`down`/`pull` run
//! `docker compose` on the agent.

use axum::{
    extract::{Path, State},
    http::StatusCode,
    Json,
};
use uuid::Uuid;

use crate::agent::unexpected;
use crate::api::resolve_docker_host;
use crate::state::AppState;
use crate::storage::models::SavedStack;

use protocol::wire::{ComposeStack, Request, Response};

pub use protocol::protocol::request::{CreateStackBody, UpdateStackBody};
pub use protocol::protocol::response::{StackOpResponse, TokenResponse};

type ApiError = (StatusCode, String);

fn compose_stack(stack: &SavedStack) -> ComposeStack {
    ComposeStack {
        id: stack.id.clone(),
        name: stack.name.clone(),
        content: stack.content.clone(),
        env_vars: stack.env_vars.clone(),
    }
}

async fn load_stack(state: &AppState, id: &str) -> Result<SavedStack, ApiError> {
    state
        .db
        .get_stack(id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {id} not found")))
}

// ── CRUD ────────────────────────────────────────────────────────────────────

pub async fn list(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<SavedStack>>, ApiError> {
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
) -> Result<Json<SavedStack>, ApiError> {
    let id = state
        .db
        .create_stack(&host_id, &body.name, &body.content)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    load_stack(&state, &id).await.map(Json)
}

pub async fn get(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<SavedStack>, ApiError> {
    load_stack(&state, &id).await.map(Json)
}

pub async fn update(
    State(state): State<AppState>,
    Path(id): Path<String>,
    Json(body): Json<UpdateStackBody>,
) -> Result<Json<SavedStack>, ApiError> {
    let env_json = body
        .env_vars
        .as_ref()
        .map(|vars| serde_json::to_string(vars).unwrap_or_default());

    state
        .db
        .update_stack(&id, &body.content, env_json.as_deref())
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    load_stack(&state, &id).await.map(Json)
}

pub async fn delete(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    // Best-effort teardown before removing the record.
    if let Ok(stack) = load_stack(&state, &id).await {
        if let Ok(docker_host) = resolve_docker_host(&state, &stack.host_id).await {
            let _ = state
                .agent
                .call(Request::ComposeDown { docker_host, stack: compose_stack(&stack) })
                .await;
        }
    }

    state
        .db
        .delete_stack(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

pub async fn up(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<StackOpResponse>, ApiError> {
    let stack = load_stack(&state, &id).await?;
    let docker_host = resolve_docker_host(&state, &stack.host_id).await?;

    let output = match state
        .agent
        .call(Request::ComposeUp { docker_host, stack: compose_stack(&stack) })
        .await
    {
        Response::Text(out) => out,
        other => {
            let _ = state.db.update_stack_status(&id, "error").await;
            return Err(unexpected(other));
        }
    };

    state
        .db
        .update_stack_status(&id, "running")
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(StackOpResponse { id, status: "running".into(), output }))
}

pub async fn down(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<StackOpResponse>, ApiError> {
    let stack = load_stack(&state, &id).await?;
    let docker_host = resolve_docker_host(&state, &stack.host_id).await?;

    let output = match state
        .agent
        .call(Request::ComposeDown { docker_host, stack: compose_stack(&stack) })
        .await
    {
        Response::Text(out) => out,
        other => return Err(unexpected(other)),
    };

    state
        .db
        .update_stack_status(&id, "created")
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(StackOpResponse { id, status: "created".into(), output }))
}

// ── Webhook token ─────────────────────────────────────────────────────────────

pub async fn set_token(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<TokenResponse>, ApiError> {
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
) -> Result<StatusCode, ApiError> {
    state
        .db
        .set_stack_webhook_token(&id, None)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(StatusCode::NO_CONTENT)
}
