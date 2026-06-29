use bollard::query_parameters::EventsOptions;
use bollard::Docker;
use futures_util::StreamExt;
use std::collections::HashMap;
use std::sync::Arc;

use crate::{
    observability::{alerts, websocket::EventSender},
    storage::Repository,
};

pub async fn run(
    docker: Arc<Docker>,
    db: Arc<dyn Repository>,
    events: Arc<EventSender>,
    slack_webhook: Option<String>,
) {
    let mut filters: HashMap<String, Vec<String>> = HashMap::new();
    filters.insert("type".to_string(), vec!["container".to_string()]);

    let options = EventsOptions {
        filters: Some(filters),
        ..Default::default()
    };

    let mut stream = docker.events(Some(options));

    println!("Docker event watcher started");

    while let Some(result) = stream.next().await {
        match result {
            Ok(event) => handle_event(event, &db, &events, slack_webhook.as_deref()).await,
            Err(e) => eprintln!("Docker event stream error: {e}"),
        }
    }

    eprintln!("Docker event stream ended — watcher stopped");
}

async fn handle_event(
    event: bollard::models::EventMessage,
    db: &Arc<dyn Repository>,
    events: &Arc<EventSender>,
    slack_webhook: Option<&str>,
) {
    let action = match event.action.as_deref() {
        Some(a) => a.to_string(),
        None => return,
    };

    let actor = event.actor.unwrap_or_default();
    let container_id = actor.id.unwrap_or_default();
    let attrs = actor.attributes.unwrap_or_default();
    let container_name = attrs.get("name").cloned().unwrap_or_else(|| container_id.clone());

    let (event_type, alert_msg) = match action.as_str() {
        "die" => {
            let exit_code = attrs.get("exitCode").map(|s| s.as_str()).unwrap_or("?");
            let msg = if exit_code != "0" {
                Some(format!(
                    ":red_circle: *Beacon* — container `{}` crashed (exit code {})",
                    container_name, exit_code
                ))
            } else {
                None
            };
            ("crash", msg)
        }
        "oom" => (
            "oom",
            Some(format!(":rotating_light: *Beacon* — container `{}` was OOM-killed", container_name)),
        ),
        "start" => ("start", None),
        "stop" => ("stop", None),
        "restart" => ("restart", None),
        _ => return,
    };

    let _ = db.record_event(&container_id, &container_name, event_type).await;
    crate::observability::metrics::record_event(event_type);

    let ws_msg = format!(
        "{{\"event\":\"{event_type}\",\"container\":\"{container_name}\"}}",
    );
    crate::observability::websocket::broadcast_event(events, ws_msg);

    if let (Some(url), Some(msg)) = (slack_webhook, alert_msg) {
        alerts::send_slack(url, &msg).await.ok();
    }
}
