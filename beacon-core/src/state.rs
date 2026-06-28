use bollard::Docker;
use std::sync::Arc;

use crate::{
    environments::manager::EnvironmentManager,
    observability::websocket::EventSender,
    storage::Repository,
};

#[derive(Clone)]
pub struct AppState {
    pub docker: Arc<Docker>,
    pub env_manager: Arc<EnvironmentManager>,
    pub events: Arc<EventSender>,
    pub slack_webhook: Option<String>,
    pub k8s: Option<kube::Client>,
    pub db: Arc<dyn Repository>,
}
