use bollard::Docker;
use bollard::query_parameters::{ListNodesOptions, ListServicesOptions};
use bollard::query_parameters::{InspectServiceOptions, UpdateServiceOptionsBuilder};

pub use protocol::data::orchestration::{SwarmNode, SwarmService};

pub async fn services(docker: &Docker) -> Result<Vec<SwarmService>, bollard::errors::Error> {
    let opts: Option<ListServicesOptions> = None;
    let list = docker.list_services(opts).await?;
    Ok(list
        .into_iter()
        .map(|s| {
            let spec = s.spec.as_ref();
            let mut replicas = 0;
            let mut running = 0;
            if let Some(mode) = spec.and_then(|sp| sp.mode.as_ref()) {
                if let Some(rep) = mode.replicated.as_ref() {
                    replicas = rep.replicas.unwrap_or(0);
                }
            }
            if let Some(service_status) = s.service_status.as_ref() {
                running = service_status.running_tasks.unwrap_or(0);
            }
            SwarmService {
                id: s.id.unwrap_or_default(),
                name: spec.and_then(|sp| sp.name.clone()).unwrap_or_default(),
                image: spec
                    .and_then(|sp| sp.task_template.as_ref())
                    .and_then(|t| t.container_spec.as_ref())
                    .and_then(|c| c.image.clone())
                    .unwrap_or_default(),
                replicas: replicas as u64,
                running,
                desired_state: "unknown".into(),
            }
        })
        .collect())
}

pub async fn nodes(docker: &Docker) -> Result<Vec<SwarmNode>, bollard::errors::Error> {
    let opts: Option<ListNodesOptions> = None;
    let list = docker.list_nodes(opts).await?;
    Ok(list
        .into_iter()
        .map(|n| {
            let desc = n.description.clone().unwrap_or_default();
            let status = n.status.clone().unwrap_or_default();
            let spec = n.spec.as_ref();
            SwarmNode {
                id: n.id.unwrap_or_default(),
                hostname: desc.hostname.unwrap_or_default(),
                role: spec
                    .and_then(|s| s.role.as_ref())
                    .map(|r| r.to_string())
                    .unwrap_or_default(),
                state: status
                    .state
                    .as_ref()
                    .map(|s| s.to_string())
                    .unwrap_or_default(),
                availability: spec
                    .and_then(|s| s.availability.as_ref())
                    .map(|a| a.to_string())
                    .unwrap_or_default(),
                addr: status.addr.unwrap_or_default(),
            }
        })
        .collect())
}

/// Forces a rolling update of a swarm service by bumping its `force_update`
/// counter — the swarm equivalent of "redeploy with the current spec".
pub async fn force_update(docker: &Docker, service_id: &str) -> Result<(), bollard::errors::Error> {
    let service = docker
        .inspect_service(service_id, None::<InspectServiceOptions>)
        .await?;

    let version = service
        .version
        .and_then(|v| v.index)
        .ok_or_else(|| bollard::errors::Error::DockerResponseServerError {
            status_code: 500,
            message: "missing service version".into(),
        })?;

    let mut spec = service
        .spec
        .ok_or_else(|| bollard::errors::Error::DockerResponseServerError {
            status_code: 500,
            message: "missing service spec".into(),
        })?;

    if let Some(ref mut tt) = spec.task_template {
        tt.force_update = Some(tt.force_update.unwrap_or(0) + 1);
    }

    docker
        .update_service(
            service_id,
            spec,
            UpdateServiceOptionsBuilder::new().version(version as i32).build(),
            None,
        )
        .await?;
    Ok(())
}
