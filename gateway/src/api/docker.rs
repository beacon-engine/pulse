//! Per-host Docker / Swarm operations. Each resolves the host to a `docker_host`
//! and forwards a [`Request`] to the agent, translating the [`Response`] back
//! into an HTTP result.

use axum::{
    Json,
    extract::{Path, Query, State},
    http::StatusCode,
};

use crate::agent::unexpected;
use crate::api::{Page, PageQuery, paginate, resolve_docker_host};
use crate::observability::alerts;
use crate::state::AppState;

use protocol::data::orchestration::{SwarmNode, SwarmService, SwarmTask};
use protocol::data::system::{HardwareMetrics, SystemInfo};
use protocol::data::{
    container::{
        ContainerDetail, ContainerInfo, ContainerNetworkInfo, HostNetwork, ImageInfo, Stack,
        VolumeInfo,
    },
    orchestration::ScaleQuery,
};
use protocol::protocol::request::{EnvBody, ExecBody, LogsQuery, NetworkBody, RecreateBody};
use protocol::wire::{Request, Response};

type ApiError = (StatusCode, String);

// ── Containers ──────────────────────────────────────────────────────────────

pub async fn list_containers(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<ContainerInfo>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListContainers { docker_host })
        .await
    {
        Response::Containers(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn inspect_container(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<Json<ContainerDetail>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::InspectContainer {
            docker_host,
            id: cid,
        })
        .await
    {
        Response::ContainerDetail(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn inspect_raw_container(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::InspectRawContainer {
            docker_host,
            id: cid,
        })
        .await
    {
        Response::Raw(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn recreate_container(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Json(body): Json<RecreateBody>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::RecreateContainer {
            docker_host,
            id: cid,
            ports: body.ports,
        })
        .await
    {
        Response::Id(id) => Ok(Json(serde_json::json!({ "id": id }))),
        other => Err(unexpected(other)),
    }
}

pub async fn update_container_env(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Json(body): Json<EnvBody>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::UpdateContainerEnv {
            docker_host,
            id: cid,
            env: body.env,
        })
        .await
    {
        Response::Id(id) => Ok(Json(serde_json::json!({ "id": id }))),
        other => Err(unexpected(other)),
    }
}

pub async fn exec_container(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Json(body): Json<ExecBody>,
) -> Result<String, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ExecContainer {
            docker_host,
            id: cid,
            cmd: body.cmd,
        })
        .await
    {
        Response::Text(out) => Ok(out),
        other => Err(unexpected(other)),
    }
}

pub async fn start(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::StartContainer {
            docker_host,
            id: cid.clone(),
        })
        .await
    {
        Response::Ok => {
            if let Some(url) = state.slack_webhook.clone() {
                tokio::spawn(async move { alerts::container_started(&url, &cid).await });
            }
            Ok(StatusCode::NO_CONTENT)
        }
        other => Err(unexpected(other)),
    }
}

pub async fn stop(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::StopContainer {
            docker_host,
            id: cid.clone(),
        })
        .await
    {
        Response::Ok => {
            if let Some(url) = state.slack_webhook.clone() {
                tokio::spawn(async move { alerts::container_stopped(&url, &cid).await });
            }
            Ok(StatusCode::NO_CONTENT)
        }
        other => Err(unexpected(other)),
    }
}

pub async fn restart(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::RestartContainer {
            docker_host,
            id: cid.clone(),
        })
        .await
    {
        Response::Ok => {
            if let Some(url) = state.slack_webhook.clone() {
                tokio::spawn(async move { alerts::container_restarted(&url, &cid).await });
            }
            Ok(StatusCode::NO_CONTENT)
        }
        other => Err(unexpected(other)),
    }
}

pub async fn container_logs(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Query(params): Query<LogsQuery>,
) -> Result<String, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    let tail = params.tail.unwrap_or(200).min(2000);
    match state
        .agent
        .call(Request::ContainerLogs {
            docker_host,
            id: cid,
            tail,
        })
        .await
    {
        Response::Text(out) => Ok(out),
        other => Err(unexpected(other)),
    }
}

pub async fn get_container_networks(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
) -> Result<Json<Vec<ContainerNetworkInfo>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ContainerNetworks {
            docker_host,
            id: cid,
        })
        .await
    {
        Response::ContainerNetworks(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn network_connect(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Json(body): Json<NetworkBody>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::NetworkConnect {
            docker_host,
            network_id: body.network_id,
            container_id: cid,
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}

pub async fn network_disconnect(
    State(state): State<AppState>,
    Path((host_id, cid)): Path<(String, String)>,
    Json(body): Json<NetworkBody>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::NetworkDisconnect {
            docker_host,
            network_id: body.network_id,
            container_id: cid,
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}

// ── System / hardware ───────────────────────────────────────────────────────

pub async fn get_system_info(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<SystemInfo>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state.agent.call(Request::SystemInfo { docker_host }).await {
        Response::SystemInfo(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn get_hardware_metrics(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<HardwareMetrics>, ApiError> {
    // Verify the host exists; hardware metrics are for the agent's machine.
    resolve_docker_host(&state, &host_id).await?;
    match state.agent.call(Request::HardwareMetrics).await {
        Response::HardwareMetrics(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

// ── Images ──────────────────────────────────────────────────────────────────

pub async fn list_images(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
    Query(q): Query<PageQuery>,
) -> Result<Json<Page<ImageInfo>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    let all = match state.agent.call(Request::ListImages { docker_host }).await {
        Response::Images(v) => v,
        other => return Err(unexpected(other)),
    };
    let (page, per_page) = q.bounds();
    let mut filtered: Vec<ImageInfo> = match q.search_term() {
        None => all,
        Some(s) => all
            .into_iter()
            .filter(|i| {
                i.id.to_lowercase().contains(&s)
                    || i.tags.iter().any(|t| t.to_lowercase().contains(&s))
            })
            .collect(),
    };
    // Newest first, so paging is stable across requests.
    filtered.sort_by(|a, b| b.created.cmp(&a.created));
    Ok(Json(paginate(filtered, page, per_page)))
}

pub async fn delete_image(
    State(state): State<AppState>,
    Path((host_id, image_id)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::DeleteImage {
            docker_host,
            id: image_id,
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}

pub async fn prune_images(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state.agent.call(Request::PruneImages { docker_host }).await {
        Response::Count(n) => Ok(Json(serde_json::json!({ "reclaimed": n }))),
        other => Err(unexpected(other)),
    }
}

// ── Volumes ─────────────────────────────────────────────────────────────────

pub async fn list_volumes(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
    Query(q): Query<PageQuery>,
) -> Result<Json<Page<VolumeInfo>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    let all = match state.agent.call(Request::ListVolumes { docker_host }).await {
        Response::Volumes(v) => v,
        other => return Err(unexpected(other)),
    };
    let (page, per_page) = q.bounds();
    let mut filtered: Vec<VolumeInfo> = match q.search_term() {
        None => all,
        Some(s) => all
            .into_iter()
            .filter(|v| v.name.to_lowercase().contains(&s) || v.driver.to_lowercase().contains(&s))
            .collect(),
    };
    filtered.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(Json(paginate(filtered, page, per_page)))
}

pub async fn delete_volume(
    State(state): State<AppState>,
    Path((host_id, vol_name)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::DeleteVolume {
            docker_host,
            name: vol_name,
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}

pub async fn prune_volumes(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::PruneVolumes { docker_host })
        .await
    {
        Response::Count(n) => Ok(Json(serde_json::json!({ "reclaimed": n }))),
        other => Err(unexpected(other)),
    }
}

// ── Networks ────────────────────────────────────────────────────────────────

pub async fn list_host_networks(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<HostNetwork>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListHostNetworks { docker_host })
        .await
    {
        Response::HostNetworks(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

// ── Detected stacks ───────────────────────────────────────────────────────────

pub async fn list_stacks(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<Stack>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListDetectedStacks { docker_host })
        .await
    {
        Response::DetectedStacks(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

// ── Swarm ───────────────────────────────────────────────────────────────────

pub async fn list_services(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<SwarmService>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListSwarmServices { docker_host })
        .await
    {
        Response::SwarmServices(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn list_nodes(
    State(state): State<AppState>,
    Path(host_id): Path<String>,
) -> Result<Json<Vec<SwarmNode>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListSwarmNodes { docker_host })
        .await
    {
        Response::SwarmNodes(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn list_service_tasks(
    State(state): State<AppState>,
    Path((host_id, service_id)): Path<(String, String)>,
) -> Result<Json<Vec<SwarmTask>>, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ListSwarmTasks {
            docker_host,
            service_id,
        })
        .await
    {
        Response::SwarmTasks(v) => Ok(Json(v)),
        other => Err(unexpected(other)),
    }
}

pub async fn update_swarm_service(
    State(state): State<AppState>,
    Path((host_id, service_id)): Path<(String, String)>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::UpdateSwarmService {
            docker_host,
            service_id: service_id.clone(),
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}

pub async fn scale_swarm_service(
    State(state): State<AppState>,
    Path((host_id, service_id)): Path<(String, String)>,
    Query(params): Query<ScaleQuery>,
) -> Result<StatusCode, ApiError> {
    let docker_host = resolve_docker_host(&state, &host_id).await?;
    match state
        .agent
        .call(Request::ScaleSwarmService {
            docker_host,
            service_id: service_id.clone(),
            target_replicas: params.target_replicas.unwrap_or(1),
        })
        .await
    {
        Response::Ok => Ok(StatusCode::NO_CONTENT),
        other => Err(unexpected(other)),
    }
}
