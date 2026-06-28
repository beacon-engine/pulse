use bollard::container::LogOutput;
use bollard::models::ContainerCreateBody;
use bollard::query_parameters::{
    CreateContainerOptions, CreateImageOptions, InspectContainerOptions, ListContainersOptions,
    LogsOptions, RemoveContainerOptions, RestartContainerOptions, StartContainerOptions,
    StopContainerOptions,
};
use bollard::Docker;
use futures_util::StreamExt;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ── Basic container info (list view) ─────────────────────────────────────────

#[derive(Serialize, Clone)]
pub struct ContainerInfo {
    pub id: String,
    pub name: String,
    pub image: String,
    pub status: String,
    pub state: String,
    pub stack: Option<String>,
}

// ── Detailed inspect types ────────────────────────────────────────────────────

#[derive(Serialize)]
pub struct ContainerPort {
    pub container_port: String,
    pub protocol: String,
    pub host_ip: String,
    pub host_port: String,
}

#[derive(Serialize)]
pub struct ContainerMount {
    pub source: String,
    pub destination: String,
    pub mode: String,
    pub rw: bool,
}

#[derive(Serialize)]
pub struct ContainerDetail {
    pub id: String,
    pub name: String,
    pub image: String,
    pub created: String,
    pub state: String,
    pub started_at: String,
    pub finished_at: String,
    pub restart_count: i64,
    pub ports: Vec<ContainerPort>,
    pub mounts: Vec<ContainerMount>,
    pub env: Vec<String>,
    pub cmd: Vec<String>,
    pub network_mode: String,
    pub restart_policy: String,
}

// ── Stacks ────────────────────────────────────────────────────────────────────

#[derive(Serialize)]
pub struct Stack {
    pub name: String,
    pub services: Vec<ContainerInfo>,
}

// ── Port-remapping input (for recreate) ───────────────────────────────────────

#[derive(Deserialize)]
pub struct PortMap {
    pub host_port: String,
    pub container_port: String,
    pub protocol: String,
}

// ── Connect ───────────────────────────────────────────────────────────────────

pub fn connect() -> Result<Docker, bollard::errors::Error> {
    Docker::connect_with_local_defaults()
}

// ── List ──────────────────────────────────────────────────────────────────────

pub async fn list(docker: &Docker) -> Result<Vec<ContainerInfo>, bollard::errors::Error> {
    let mut filters: HashMap<String, Vec<String>> = HashMap::new();
    filters.insert(
        "status".to_string(),
        vec!["running".to_string(), "exited".to_string(), "paused".to_string(), "created".to_string()],
    );

    let containers = docker
        .list_containers(Some(ListContainersOptions { all: true, filters: Some(filters), ..Default::default() }))
        .await?;

    Ok(containers
        .into_iter()
        .map(|c| {
            let stack = c.labels.as_ref().and_then(|l| {
                l.get("com.docker.compose.project")
                    .or_else(|| l.get("com.docker.stack.namespace"))
                    .cloned()
            });
            ContainerInfo {
                id: c.id.unwrap_or_default(),
                name: c
                    .names
                    .unwrap_or_default()
                    .first()
                    .cloned()
                    .unwrap_or_default()
                    .trim_start_matches('/')
                    .to_string(),
                image: c.image.unwrap_or_default(),
                status: c.status.unwrap_or_default(),
                state: c.state.as_ref().map(|s| s.to_string()).unwrap_or_default(),
                stack,
            }
        })
        .collect())
}

// ── Inspect ───────────────────────────────────────────────────────────────────

pub async fn inspect(docker: &Docker, id: &str) -> Result<ContainerDetail, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;

    let config = info.config.unwrap_or_default();
    let state = info.state.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();

    let ports = info
        .network_settings
        .and_then(|ns| ns.ports)
        .map(|port_map| {
            let mut result: Vec<ContainerPort> = port_map
                .into_iter()
                .flat_map(|(port_proto, bindings)| {
                    let mut parts = port_proto.splitn(2, '/');
                    let cport = parts.next().unwrap_or("").to_string();
                    let proto = parts.next().unwrap_or("tcp").to_string();
                    bindings
                        .unwrap_or_default()
                        .into_iter()
                        .map(move |b| ContainerPort {
                            container_port: cport.clone(),
                            protocol: proto.clone(),
                            host_ip: b.host_ip.unwrap_or_else(|| "0.0.0.0".to_string()),
                            host_port: b.host_port.unwrap_or_default(),
                        })
                })
                .collect();
            result.sort_by(|a, b| a.container_port.cmp(&b.container_port));
            result
        })
        .unwrap_or_default();

    let mounts = info
        .mounts
        .unwrap_or_default()
        .into_iter()
        .map(|m| ContainerMount {
            source: m.source.unwrap_or_default(),
            destination: m.destination.unwrap_or_default(),
            mode: m.mode.unwrap_or_default(),
            rw: m.rw.unwrap_or(false),
        })
        .collect();

    let restart_policy = host_config
        .restart_policy
        .and_then(|rp| rp.name)
        .map(|n| n.to_string())
        .unwrap_or_default();

    Ok(ContainerDetail {
        id: info.id.unwrap_or_default(),
        name: info.name.unwrap_or_default().trim_start_matches('/').to_string(),
        image: config.image.unwrap_or_default(),
        created: info.created.unwrap_or_default(),
        state: state.status.as_ref().map(|s| s.to_string()).unwrap_or_default(),
        started_at: state.started_at.unwrap_or_default(),
        finished_at: state.finished_at.unwrap_or_default(),
        restart_count: info.restart_count.unwrap_or(0),
        ports,
        mounts,
        env: config.env.unwrap_or_default(),
        cmd: config.cmd.unwrap_or_default(),
        network_mode: host_config.network_mode.unwrap_or_default(),
        restart_policy,
    })
}

pub async fn inspect_raw(
    docker: &Docker,
    id: &str,
) -> Result<serde_json::Value, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    Ok(serde_json::to_value(info).unwrap_or_default())
}

// ── Recreate with new port mappings ──────────────────────────────────────────

pub async fn recreate(
    docker: &Docker,
    id: &str,
    new_ports: &[PortMap],
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let mut host_config = info.host_config.unwrap_or_default();

    // Build new port bindings
    let mut bindings: HashMap<String, Option<Vec<bollard::models::PortBinding>>> = HashMap::new();
    for pm in new_ports {
        let key = format!("{}/{}", pm.container_port, pm.protocol);
        bindings.insert(
            key,
            Some(vec![bollard::models::PortBinding {
                host_ip: Some("0.0.0.0".to_string()),
                host_port: Some(pm.host_port.clone()),
            }]),
        );
    }
    host_config.port_bindings = Some(bindings);

    // Stop and remove old container (best effort stop)
    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    // Create with same name and updated ports
    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: config.image.clone(),
                cmd: config.cmd.clone(),
                env: config.env.clone(),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Recreate a container (optionally pulling the latest image first) ───────

pub async fn recreate_latest(
    docker: &Docker,
    id: &str,
    pull: bool,
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();
    let image = config.image.clone().unwrap_or_default();

    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    if pull && !image.is_empty() {
        let mut stream = docker.create_image(
            Some(CreateImageOptions {
                from_image: Some(image.clone()),
                ..Default::default()
            }),
            None,
            None,
        );
        while let Some(result) = stream.next().await {
            if let Err(e) = result {
                return Err(e);
            }
        }
    }

    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: Some(image),
                cmd: config.cmd.clone(),
                env: config.env.clone(),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Update environment variables and restart ───────────────────────────────

pub async fn update_env(
    docker: &Docker,
    id: &str,
    env: &[String],
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();

    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: config.image.clone(),
                cmd: config.cmd.clone(),
                env: Some(env.to_vec()),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Logs ──────────────────────────────────────────────────────────────────────

pub async fn logs(docker: &Docker, id: &str, tail: u32) -> Result<String, bollard::errors::Error> {
    let mut stream = docker.logs(
        id,
        Some(LogsOptions {
            stdout: true,
            stderr: true,
            tail: tail.to_string(),
            timestamps: true,
            ..Default::default()
        }),
    );
    let mut output = String::new();
    while let Some(chunk) = stream.next().await {
        match chunk? {
            LogOutput::StdOut { message } | LogOutput::StdErr { message } => {
                output.push_str(&String::from_utf8_lossy(&message));
            }
            _ => {}
        }
    }
    Ok(output)
}

// ── Stacks ────────────────────────────────────────────────────────────────────

pub async fn list_stacks(docker: &Docker) -> Result<Vec<Stack>, bollard::errors::Error> {
    let all = list(docker).await?;
    let mut groups: HashMap<String, Vec<ContainerInfo>> = HashMap::new();
    for c in all {
        if let Some(ref name) = c.stack {
            groups.entry(name.clone()).or_default().push(c);
        }
    }
    let mut stacks: Vec<Stack> = groups
        .into_iter()
        .map(|(name, services)| Stack { name, services })
        .collect();
    stacks.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(stacks)
}

// ── Basic actions ─────────────────────────────────────────────────────────────

pub async fn start(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.start_container(id, None::<StartContainerOptions>).await
}

pub async fn stop(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.stop_container(id, Some(StopContainerOptions { t: Some(10), ..Default::default() })).await
}

pub async fn restart(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.restart_container(id, Some(RestartContainerOptions { t: Some(10), ..Default::default() })).await
}
