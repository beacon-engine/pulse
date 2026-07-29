//! HTTP handlers for the frontend-facing API.
//!
//! Host / stack / token / event records are served from the gateway's own
//! database. Anything that touches Docker, Swarm or Kubernetes is resolved to a
//! `docker_host` connection string and forwarded to the agent over TCP.

pub mod auth;
pub mod docker;
pub mod events;
pub mod hosts;
pub mod k8s;
pub mod stacks;
pub mod users;
pub mod webhooks;

use axum::http::StatusCode;
use serde::{Deserialize, Serialize};

use crate::state::AppState;

/// Query string for paginated list endpoints: `?page=1&per_page=50&search=foo`.
#[derive(Debug, Deserialize)]
pub struct PageQuery {
    pub page: Option<u32>,
    pub per_page: Option<u32>,
    pub search: Option<String>,
}

impl PageQuery {
    /// Normalised (page, per_page) with sane bounds.
    pub fn bounds(&self) -> (u32, u32) {
        (self.page.unwrap_or(1).max(1), self.per_page.unwrap_or(50).clamp(1, 500))
    }

    /// Trimmed, lower-cased search term (None when empty).
    pub fn search_term(&self) -> Option<String> {
        self.search
            .as_ref()
            .map(|s| s.trim().to_lowercase())
            .filter(|s| !s.is_empty())
    }
}

/// A page of results plus the total count (for building page controls).
#[derive(Debug, Serialize)]
pub struct Page<T> {
    pub items: Vec<T>,
    pub total: usize,
    pub page: u32,
    pub per_page: u32,
}

/// Slices an already-filtered in-memory list into the requested page.
pub fn paginate<T>(items: Vec<T>, page: u32, per_page: u32) -> Page<T> {
    let total = items.len();
    let start = ((page - 1) as usize).saturating_mul(per_page as usize);
    let items = items.into_iter().skip(start).take(per_page as usize).collect();
    Page { items, total, page, per_page }
}

/// Resolves a host id to the `docker_host` connection string the agent needs.
pub async fn resolve_docker_host(
    state: &AppState,
    host_id: &str,
) -> Result<String, (StatusCode, String)> {
    let env = state
        .db
        .get_environment(host_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {host_id} not found")))?;
    Ok(env.docker_host)
}
