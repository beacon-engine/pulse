//! Docker Swarm and Kubernetes models.

use serde::{Deserialize, Serialize};

// ── Swarm ───────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwarmService {
    pub id: String,
    pub name: String,
    pub image: String,
    pub replicas: u64,
    pub running: u64,
    pub desired_state: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwarmNode {
    pub id: String,
    pub hostname: String,
    pub role: String,
    pub state: String,
    pub availability: String,
    pub addr: String,
}

/// A single task (running container) belonging to a swarm service.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwarmTask {
    pub id: String,
    pub service_id: String,
    pub node_id: String,
    pub slot: i64,
    /// The container backing this task (may be empty before it's scheduled).
    pub container_id: String,
    pub image: String,
    pub state: String,
}

// ── Kubernetes ────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PodContainer {
    pub name: String,
    pub image: String,
    pub ready: bool,
    pub restart_count: i32,
    /// running | waiting | terminated | unknown
    pub state: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PodInfo {
    pub name: String,
    pub namespace: String,
    pub status: String,
    pub ready: bool,
    pub containers: Vec<PodContainer>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NamespaceInfo {
    pub name: String,
    pub status: String,
}
