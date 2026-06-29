use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::containers::docker;
use crate::state::AppState;

// ── Shared types ──────────────────────────────────────────────────────────────

#[derive(Deserialize, Default)]
pub struct PullQuery {
    pub pull: Option<bool>,
    pub token: Option<String>,
}

#[derive(Serialize)]
pub struct TokenResponse {
    pub token: String,
}

#[derive(Serialize)]
pub struct TokenStatusResponse {
    pub enabled: bool,
    pub token: Option<String>,
}

// ── Token management (generic) ────────────────────────────────────────────────

pub async fn get_token(
    State(state): State<AppState>,
    Path((host_id, entity_type, entity_key)): Path<(String, String, String)>,
) -> Result<Json<TokenStatusResponse>, (StatusCode, String)> {
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
) -> Result<Json<TokenResponse>, (StatusCode, String)> {
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
) -> Result<StatusCode, (StatusCode, String)> {
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
) -> Result<(), (StatusCode, String)> {
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

// ── Container: pull latest and recreate (name-based, token-protected) ────────

#[derive(Serialize)]
pub struct ContainerWebhookResponse {
    pub id: String,
    pub pulled: bool,
}

pub async fn deploy_container(
    State(state): State<AppState>,
    Path((host_id, container_name)): Path<(String, String)>,
    Query(params): Query<PullQuery>,
) -> Result<Json<ContainerWebhookResponse>, (StatusCode, String)> {
    verify_webhook_token(&state, "container", &container_name, &host_id, params.token.as_deref()).await?;

    let docker = state
        .env_manager
        .client_for_env(&state.db, &host_id)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))?;

    // Find container ID by name
    let containers: Vec<docker::ContainerInfo> = docker::list(&docker).await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    let target = containers
        .iter()
        .find(|c| c.name == container_name || c.name == format!("/{container_name}"))
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("no container named '{container_name}'")))?;

    let pull = params.pull.unwrap_or(true);
    let new_id = docker::recreate_latest(&docker, &target.id, pull)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(ContainerWebhookResponse { id: new_id, pulled: pull }))
}

// ── Container: legacy endpoint (no token, kept for backward compatibility) ───

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

    Ok(Json(RecreateWebhookResponse { id: new_id, pulled: pull }))
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
) -> Result<Json<StackWebhookResponse>, (StatusCode, String)> {
    let stack = state
        .db
        .get_stack(&stack_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("stack {stack_id} not found")))?;

    if stack.host_id != host_id {
        return Err((StatusCode::FORBIDDEN, "stack does not belong to this host".into()));
    }

    // Token is required — if not set the webhook is disabled
    match &stack.webhook_token {
        None => return Err((StatusCode::FORBIDDEN, "webhook not enabled for this stack".into())),
        Some(required) => {
            if params.token.as_deref() != Some(required.as_str()) {
                return Err((StatusCode::UNAUTHORIZED, "invalid or missing token".into()));
            }
        }
    }

    let mut output = String::new();

    if params.pull.unwrap_or(false) {
        match crate::stacks::compose::pull(&state, &stack).await {
            Ok(out) => { output.push_str("=== pull ===\n"); output.push_str(&out); output.push('\n'); }
            Err(e)  => { output.push_str("=== pull (warn) ===\n"); output.push_str(&e); output.push('\n'); }
        }
    }

    let up_out = crate::stacks::compose::up(&state, &stack)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e))?;
    output.push_str("=== up ===\n");
    output.push_str(&up_out);

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
) -> Result<Json<SwarmWebhookResponse>, (StatusCode, String)> {
    verify_webhook_token(&state, "swarm", &service_id, &host_id, params.token.as_deref()).await?;

    use bollard::service::{InspectServiceOptions, UpdateServiceOptions};

    let docker = state
        .env_manager
        .client_for_env(&state.db, &host_id)
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, format!("cannot connect to Docker: {e}")))?;

    let service = docker
        .inspect_service(&service_id, None::<InspectServiceOptions>)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let version = service
        .version
        .and_then(|v| v.index)
        .ok_or_else(|| (StatusCode::INTERNAL_SERVER_ERROR, "missing service version".into()))?;

    let mut spec = service
        .spec
        .ok_or_else(|| (StatusCode::INTERNAL_SERVER_ERROR, "missing service spec".into()))?;

    if let Some(ref mut tt) = spec.task_template {
        tt.force_update = Some(tt.force_update.unwrap_or(0) + 1);
    }

    docker
        .update_service(&service_id, spec, UpdateServiceOptions { version, ..Default::default() }, None)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(SwarmWebhookResponse { service: service_id, updated: true }))
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
    Path((_host_id, namespace, deployment)): Path<(String, String, String)>,
    Query(params): Query<PullQuery>,
) -> Result<Json<K8sRolloutResponse>, (StatusCode, String)> {
    let entity_key = format!("{namespace}/{deployment}");
    verify_webhook_token(&state, "k8s", &entity_key, &_host_id, params.token.as_deref()).await?;

    use k8s_openapi::api::apps::v1::Deployment;
    use kube::api::{Api, Patch, PatchParams};

    let client = state.k8s.as_ref().ok_or((
        StatusCode::SERVICE_UNAVAILABLE,
        "Kubernetes not connected".into(),
    ))?;

    let deployments: Api<Deployment> = Api::namespaced(client.clone(), &namespace);
    let now = chrono::Utc::now().to_rfc3339();
    let patch = serde_json::json!({
        "spec": { "template": { "metadata": { "annotations": { "kubectl.kubernetes.io/restartedAt": now } } } }
    });

    deployments
        .patch(&deployment, &PatchParams::default(), &Patch::Merge(patch))
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(K8sRolloutResponse { namespace, deployment, restarted: true }))
}
