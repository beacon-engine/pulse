//! Container, image, volume and network models.

use serde::{Deserialize, Serialize};

// ── Basic container info (list view) ─────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerInfo {
    pub id: String,
    pub name: String,
    pub image: String,
    pub status: String,
    pub state: String,
    pub stack: Option<String>,
}

// ── Detailed inspect types ────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerPort {
    pub container_port: String,
    pub protocol: String,
    pub host_ip: String,
    pub host_port: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerMount {
    pub source: String,
    pub destination: String,
    pub mode: String,
    pub rw: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Stack {
    pub name: String,
    pub services: Vec<ContainerInfo>,
}

// ── Port-remapping input (for recreate) ───────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PortMap {
    pub host_port: String,
    pub container_port: String,
    pub protocol: String,
}

// ── Images ────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImageInfo {
    pub id: String,
    pub tags: Vec<String>,
    pub size: i64,
    pub created: i64,
    pub dangling: bool,
}

// ── Volumes ───────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VolumeInfo {
    pub name: String,
    pub driver: String,
    pub mountpoint: String,
    pub created: String,
}

// ── Networks ──────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HostNetwork {
    pub id: String,
    pub name: String,
    pub driver: String,
    pub scope: String,
    pub internal: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerNetworkInfo {
    pub id: String,
    pub name: String,
    pub ip_address: String,
    pub gateway: String,
    pub mac_address: String,
}

// ── Events ────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerEvent {
    pub id: String,
    pub container_id: String,
    pub container_name: String,
    pub event: String,
    pub occurred_at: String,
}
