//! Persisted host and stack records.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Environment {
    pub id: String,
    pub name: String,
    pub docker_host: String,
    pub is_local: bool,
    pub created_at: String,
    /// The push agent that represents this environment. `None` routes to the
    /// default pull agent. See the gateway's `AgentRouter`.
    #[serde(default)]
    pub agent_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SavedStack {
    pub id: String,
    pub host_id: String,
    pub name: String,
    pub content: String,
    pub env_vars: Option<String>,
    pub webhook_token: Option<String>,
    pub status: String,
    pub created_at: String,
    pub updated_at: String,
}
