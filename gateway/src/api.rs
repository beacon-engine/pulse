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
        (
            self.page.unwrap_or(1).max(1),
            self.per_page.unwrap_or(50).clamp(1, 500),
        )
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
    let items = items
        .into_iter()
        .skip(start)
        .take(per_page as usize)
        .collect();
    Page {
        items,
        total,
        page,
        per_page,
    }
}

/// Resolves a host id to the `docker_host` connection string the agent needs.
/// Resolves a host id to `(agent_id, docker_host)`. `agent_id` selects the push
/// session to route the request to (`None` → the default pull agent); its
/// `docker_host` is the daemon connection string the chosen agent then uses.
pub async fn resolve_target(
    state: &AppState,
    host_id: &str,
) -> Result<(Option<String>, String), (StatusCode, String)> {
    let env = state
        .db
        .get_environment(host_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or_else(|| (StatusCode::NOT_FOUND, format!("host {host_id} not found")))?;
    Ok((env.agent_id, env.docker_host))
}

/// Lists the agents currently connected via a push session (agents that dialed
/// into the gateway). The default pull agent is always reachable and not listed.
pub async fn list_agents(
    axum::extract::State(state): axum::extract::State<AppState>,
) -> axum::Json<Vec<String>> {
    axum::Json(state.agent.connected_agent_ids().await)
}

/// WebSocket endpoint agents dial into (push mode). Authenticated by a bearer
/// token that maps to the agent's id; on success the session is registered and
/// driven by [`crate::agent::run_session`] for its lifetime.
pub async fn agent_ws(
    ws: axum::extract::ws::WebSocketUpgrade,
    axum::extract::State(state): axum::extract::State<AppState>,
    headers: axum::http::HeaderMap,
) -> axum::response::Response {
    use axum::response::IntoResponse;
    let Some(agent_id) = authenticate_agent(&state, &headers) else {
        return (StatusCode::UNAUTHORIZED, "invalid or missing agent token").into_response();
    };
    ws.on_upgrade(move |socket| crate::agent::run_session(socket, state, agent_id))
}

/// Resolves the `Authorization: Bearer <token>` header to an agent id via the
/// configured token map. Returns `None` when the header is absent or the token
/// is unknown.
fn authenticate_agent(state: &AppState, headers: &axum::http::HeaderMap) -> Option<String> {
    let header = headers
        .get(axum::http::header::AUTHORIZATION)?
        .to_str()
        .ok()?;
    let token = header.strip_prefix("Bearer ")?.trim();
    state.agent_tokens.get(token).cloned()
}
