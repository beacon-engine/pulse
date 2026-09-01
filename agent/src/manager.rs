use bollard::{Docker, API_DEFAULT_VERSION};
use std::{collections::HashMap, sync::Arc};
use tokio::sync::RwLock;

/// Caches one Docker client per `docker_host` connection string.
///
/// The agent is stateless with respect to hosts: the gateway resolves a host to
/// its `docker_host` and passes it on every request. An empty string or a
/// `unix://` address means the local daemon; `tcp://…` is a remote daemon.
pub struct DockerManager {
    connections: RwLock<HashMap<String, Arc<Docker>>>,
}

impl DockerManager {
    pub fn new() -> Self {
        Self {
            connections: RwLock::new(HashMap::new()),
        }
    }

    /// Returns a cached Docker client for the connection string, connecting on
    /// first use.
    pub async fn client_for(&self, docker_host: &str) -> Result<Arc<Docker>, String> {
        {
            let map = self.connections.read().await;
            if let Some(docker) = map.get(docker_host) {
                return Ok(docker.clone());
            }
        }

        let is_local = docker_host.is_empty() || docker_host.starts_with("unix://");
        let docker = if is_local {
            Docker::connect_with_local_defaults()
        } else {
            let addr = docker_host.replace("tcp://", "http://");
            Docker::connect_with_http(&addr, 30, API_DEFAULT_VERSION)
        }
        .map_err(|e| e.to_string())?;

        let docker = Arc::new(docker);
        self.connections
            .write()
            .await
            .insert(docker_host.to_string(), docker.clone());
        Ok(docker)
    }
}

impl Default for DockerManager {
    fn default() -> Self {
        Self::new()
    }
}
