use bollard::{Docker, API_DEFAULT_VERSION};
use std::{collections::HashMap, sync::Arc};
use tokio::sync::RwLock;

use crate::storage::{models::Environment, Repository};

pub struct EnvironmentManager {
    connections: RwLock<HashMap<String, Arc<Docker>>>,
}

impl EnvironmentManager {
    pub fn new() -> Self {
        Self { connections: RwLock::new(HashMap::new()) }
    }

    /// Returns a cached Docker client for the environment, connecting on first use.
    pub async fn client_for(&self, env: &Environment) -> Result<Arc<Docker>, String> {
        {
            let map = self.connections.read().await;
            if let Some(docker) = map.get(&env.id) {
                return Ok(docker.clone());
            }
        }

        let docker = if env.is_local || env.docker_host.starts_with("unix://") {
            Docker::connect_with_local_defaults()
        } else {
            let addr = env.docker_host.replace("tcp://", "http://");
            Docker::connect_with_http(&addr, 30, API_DEFAULT_VERSION)
        }
        .map_err(|e| e.to_string())?;

        let docker = Arc::new(docker);
        self.connections.write().await.insert(env.id.clone(), docker.clone());
        Ok(docker)
    }

    /// Evicts the cached connection when a host is deleted.
    pub async fn remove(&self, id: &str) {
        self.connections.write().await.remove(id);
    }

    /// Convenience: load the environment from the database and return its Docker client.
    pub async fn client_for_env(
        &self,
        db: &Arc<dyn Repository>,
        id: &str,
    ) -> Result<Arc<Docker>, String> {
        let env = db
            .get_environment(id)
            .await
            .map_err(|e| e.to_string())?
            .ok_or_else(|| format!("host {id} not found"))?;
        self.client_for(&env).await
    }
}
