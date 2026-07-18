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

// ── Kubernetes ────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PodInfo {
    pub name: String,
    pub namespace: String,
    pub status: String,
    pub ready: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NamespaceInfo {
    pub name: String,
    pub status: String,
}
