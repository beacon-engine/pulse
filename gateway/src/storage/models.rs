//! Database record types.
//!
//! Docker/host records are re-exported from the shared `protocol` crate. Auth
//! records (users, sessions) are gateway-only and defined here.

use serde::Serialize;

pub use protocol::data::container::ContainerEvent;
pub use protocol::host::{Environment, SavedStack};

/// A dashboard user. `password_hash` is never serialized to API clients.
#[derive(Debug, Clone, Serialize)]
pub struct User {
    pub id: String,
    pub username: String,
    #[serde(skip_serializing)]
    pub password_hash: String,
    pub role: String,
    pub created_at: String,
}

/// A login session backing a session cookie.
#[derive(Debug, Clone)]
#[allow(dead_code)] // token/created_at are persisted but not yet read back
pub struct Session {
    pub token: String,
    pub user_id: String,
    pub created_at: String,
    pub expires_at: String,
}
