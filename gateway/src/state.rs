use std::collections::HashMap;
use std::sync::Arc;

use crate::agent::AgentRouter;
use crate::observability::websocket::EventSender;
use crate::storage::Repository;

#[derive(Clone)]
pub struct AppState {
    /// Routes each request to the right agent transport (pull client or a live
    /// push session). Exposes `call`/`watch_events` like the old single client.
    pub agent: AgentRouter,
    /// Bearer `token -> agent_id` for authenticating push agents on `/agent/ws`.
    pub agent_tokens: Arc<HashMap<String, String>>,
    /// Gateway-owned database: hosts, stacks, webhook tokens, events.
    pub db: Arc<dyn Repository>,
    /// Live-event fan-out to connected WebSocket clients.
    pub events: Arc<EventSender>,
    /// Optional Slack incoming-webhook URL for lifecycle alerts.
    pub slack_webhook: Option<String>,
}
