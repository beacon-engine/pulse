//! Request payloads the gateway forwards to an agent.

use serde::{Deserialize, Serialize};

use crate::data::container::PortMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateHostBody {
    pub name: String,
    pub docker_host: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RenameHostBody {
    pub name: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct LogsQuery {
    pub tail: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RecreateBody {
    pub ports: Vec<PortMap>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EnvBody {
    pub env: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecBody {
    pub cmd: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkBody {
    pub network_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateStackBody {
    pub name: String,
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EnvVar {
    pub key: String,
    pub value: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UpdateStackBody {
    pub content: String,
    pub env_vars: Option<Vec<EnvVar>>,
}
