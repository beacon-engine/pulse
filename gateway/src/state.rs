use std::sync::Arc;

use crate::agent::AgentClient;
use crate::observability::websocket::EventSender;
use crate::storage::Repository;

#[derive(Clone)]
pub struct AppState {
    /// TCP client to the agent (Docker / Swarm / Kubernetes executor).
    pub agent: AgentClient,
    /// Gateway-owned database: hosts, stacks, webhook tokens, events.
    pub db: Arc<dyn Repository>,
    /// Live-event fan-out to connected WebSocket clients.
    pub events: Arc<EventSender>,
    /// Optional Slack incoming-webhook URL for lifecycle alerts.
    pub slack_webhook: Option<String>,
}
