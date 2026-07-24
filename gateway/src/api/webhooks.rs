//! Webhook endpoints. Tokens are stored in the gateway DB; the triggered action
//! (recreate / redeploy / swarm update / k8s rollout) runs on the agent.

use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::agent::unexpected;
use crate::api::resolve_docker_host;
use crate::state::AppState;

use protocol::data::container::ContainerInfo;
use protocol::wire::{ComposeStack, Request, Response};

pub use protocol::protocol::response::TokenResponse;

type ApiError = (StatusCode, String);

#[derive(Deserialize, Default)]
pub struct PullQuery {
    pub pull: Option<bool>,
    pub token: Option<String>,
}

#[derive(Serialize)]
pub struct TokenStatusResponse {
    pub enabled: bool,
    pub token: Option<String>,
}

// ── Token management (generic: entity_type = container | swarm | k8s) ─────────

pub async fn get_token(
    State(state): State<AppState>,
    Path((host_id, entity_type, entity_key)): Path<(String, String, String)>,
) -> Result<Json<TokenStatusResponse>, ApiError> {
    let token = state
        .db
        .get_webhook_token(&entity_type, &entity_key, &host_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(TokenStatusResponse { enabled: token.is_some(), token }))
}

pub async fn set_token(
    State(state): State<AppState>,
    Path((host_id, entity_type, entity_key)): Path<(String, String, String)>,
) -> Result<Json<TokenResponse>, ApiError> {
    let token = Uuid::new_v4().to_string();
    state
        .db
        .set_webhook_token(&entity_type, &entity_key, &host_id, &token)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(TokenResponse { token }))
}

pub async fn delete_token(
    State(state): State<AppState>,
    Path((host_id, entity_type, entity_key)): Path<(String, String, String)>,
) -> Result<StatusCode, ApiError> {
    state
        .db
        .delete_webhook_token(&entity_type, &entity_key, &host_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(StatusCode::NO_CONTENT)
}

async fn verify_webhook_token(
    state: &AppState,
    entity_type: &str,
    entity_key: &str,
    host_id: &str,
    provided: Option<&str>,
) -> Result<(), ApiError> {
    match state.db.get_webhook_token(entity_type, entity_key, host_id).await {
        Err(e) => Err((StatusCode::INTERNAL_SERVER_ERROR, e.to_string())),
        Ok(None) => Err((StatusCode::FORBIDDEN, "webhook not enabled".into())),
        Ok(Some(required)) => {
            if provided == Some(required.as_str()) {
                Ok(())
            } else {
                Err((StatusCode::UNAUTHORIZED, "invalid or missing token".into()))
            }
        }
    }
}

// ── Container: pull latest and recreate (name-based, token-protected) ─────────

#[derive(Serialize)]
pub struct ContainerWebhookResponse {
    pub id: String,
    pub pulled: bool,
}

pub async fn deploy_container(
    State(state): State<AppState>,
    Path((host_id, container_name)): Path<(String, String)>,
    Query(params): Query<PullQuery>,
) -> Result<Json<ContainerWebhookResponse>, ApiError> {
    verify_webhook_token(&state, "container", &container_name, &host_id, params.token.as_deref())
        .await?;

    let docker_host = resolve_docker_host(&state, &host_id).await?;

    // Find the container id by name via the agent.
    let containers: Vec<ContainerInfo> =
        match state.agent.call(Request::ListContainers { docker_host: docker_host.clone() }).await {
            Response::Containers(v) => v,
            other => return Err(unexpected(other)),
        };
    let target = containers
        .iter()
        .find(|c| c.name == container_name || c.name == format!("/{container_name}"))
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("no container named '{container_name}'")))?;

    let pull = params.pull.unwrap_or(true);
    match state
        .agent
        .call(Request::RecreateLatest { docker_host, id: target.id.clone(), pull })
        .await
    {
        Response::Id(id) => Ok(Json(ContainerWebhookResponse { id, pulled: pull })),
        other => Err(unexpected(other)),
    }
}

// ── Container: legacy endpoint (no token, kept for backward compatibility) ────

#[derive(Deserialize, Default)]
pub struct RecreateQuery {
    pub pull: Option<bool>,
}

pub async fn recreate_container(
    State(state): State<AppState>,
    Path((host_id, container_id)): Path<(String, String)>,
    Query(params): Query<RecreateQuery>,
) -> Result<Json<ContainerWebhookResponse>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    let pull = params.pull.unwrap_or(false);
    match state
        .agent
        .call(Request::RecreateLatest { docker_host, id: container_id, pull })
        .await
    {
        Response::Id(id) => Ok(Json(ContainerWebhookResponse { id, pulled: pull })),
        other => Err(unexpected(other)),
    }
}

// ── Stack: pull and redeploy ──────────────────────────────────────────────────

#[derive(Deserialize, Default)]
pub struct RedeployQuery {
    pub pull: Option<bool>,
    pub token: Option<String>,
}

#[derive(Serialize)]
pub struct StackWebhookResponse {
    pub id: String,
    pub status: String,
    pub output: String,
}

pub async fn redeploy_stack(
    State(state): State<AppState>,
    Path((host_id, stack_id)): Path<(String, String)>,
    Query(params): Query<RedeployQuery>,
) -> Result<Json<StackWebhookResponse>, ApiError> {
    let stack = state
        .db
        .get_stack(&stack_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {stack_id} not found")))?;

    if stack.host_id != host_id {
        return Err((StatusCode::FORBIDDEN, "stack does not belong to this host".into()));
    }

    // Token is required — if not set the webhook is disabled.
    match &stack.webhook_token {
        None => return Err((StatusCode::FORBIDDEN, "webhook not enabled for this stack".into())),
        Some(required) => {
            if params.token.as_deref() != Some(required.as_str()) {
                return Err((StatusCode::UNAUTHORIZED, "invalid or missing token".into()));
            }
        }
    }

    let docker_host = resolve_docker_host(&state, &host_id).await?;
    let compose = ComposeStack {
        id: stack.id.clone(),
        name: stack.name.clone(),
        content: stack.content.clone(),
        env_vars: stack.env_vars.clone(),
    };

    let mut output = String::new();

    if params.pull.unwrap_or(false) {
        match state
            .agent
            .call(Request::ComposePull { docker_host: docker_host.clone(), stack: compose.clone() })
            .await
        {
            Response::Text(out) => {
                output.push_str("=== pull ===\n");
                output.push_str(&out);
                output.push('\n');
            }
            other => {
                let (_, msg) = unexpected(other);
                output.push_str("=== pull (warn) ===\n");
                output.push_str(&msg);
                output.push('\n');
            }
        }
    }

    match state.agent.call(Request::ComposeUp { docker_host, stack: compose }).await {
        Response::Text(up_out) => {
            output.push_str("=== up ===\n");
            output.push_str(&up_out);
        }
        other => return Err(unexpected(other)),
    }

    Ok(Json(StackWebhookResponse { id: stack_id, status: "ok".into(), output }))
}

// ── Swarm service: force rolling update ───────────────────────────────────────

#[derive(Serialize)]
pub struct SwarmWebhookResponse {
    pub service: String,
    pub updated: bool,
}

pub async fn update_swarm_service(
    State(state): State<AppState>,
    Path((host_id, service_id)): Path<(String, String)>,
    Query(params): Query<PullQuery>,
) -> Result<Json<SwarmWebhookResponse>, ApiError> {
    verify_webhook_token(&state, "swarm", &service_id, &host_id, params.token.as_deref()).await?;

    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::UpdateSwarmService { docker_host, service_id: service_id.clone() })
        .await
    {
        Response::Ok => Ok(Json(SwarmWebhookResponse { service: service_id, updated: true })),
        other => Err(unexpected(other)),
    }
}

// ── Kubernetes deployment: rollout restart ────────────────────────────────────

#[derive(Serialize)]
pub struct K8sRolloutResponse {
    pub namespace: String,
    pub deployment: String,
    pub restarted: bool,
}

pub async fn k8s_rollout(
    State(state): State<AppState>,
    Path((host_id, namespace, deployment)): Path<(String, String, String)>,
    Query(params): Query<PullQuery>,
) -> Result<Json<K8sRolloutResponse>, ApiError> {
    let entity_key = format!("{namespace}/{deployment}");
    verify_webhook_token(&state, "k8s", &entity_key, &host_id, params.token.as_deref()).await?;

    match state
        .agent
        .call(Request::K8sRollout { namespace: namespace.clone(), deployment: deployment.clone() })
        .await
    {
        Response::Ok => Ok(Json(K8sRolloutResponse { namespace, deployment, restarted: true })),
        other => Err(unexpected(other)),
    }
}
