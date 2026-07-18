//! Host system and hardware metrics.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SystemInfo {
    pub docker_version: String,
    pub api_version: String,
    pub os: String,
    pub arch: String,
    pub kernel: String,
    pub total_memory: i64,
    pub cpus: i64,
    pub containers_running: i64,
    pub containers_stopped: i64,
    pub images: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiskInfo {
    pub name: String,
    pub mount: String,
    pub total: u64,
    pub used: u64,
    pub pct: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HardwareMetrics {
    pub mem_used: u64,
    pub mem_total: u64,
    pub mem_pct: f32,
    pub disks: Vec<DiskInfo>,
}
