//! Consumes the agent's Docker event stream and turns it into persisted events,
//! WebSocket broadcasts and Slack alerts — the work the agent's in-process
//! watcher used to do, now that the DB and alerting live in the gateway.

use std::time::Duration;

use crate::observability::{alerts, metrics, websocket};
use crate::state::AppState;

use protocol::wire::{read_frame, EventFrame, Response};

/// Watches events for the agent's local Docker daemon, reconnecting on failure.
pub async fn run(state: AppState) {
    // Empty connection string → the agent's local daemon (matches the previous
    // single-host watcher behaviour).
    let docker_host = String::new();

    loop {
        match state.agent.watch_events(docker_host.clone()).await {
            Ok(mut stream) => {
                println!("Event watcher connected to agent");
                loop {
                    match read_frame::<_, Response>(&mut stream).await {
                        Ok(Response::Event(frame)) => handle_event(&state, frame).await,
                        Ok(Response::Error { message, .. }) => {
                            eprintln!("Event stream error from agent: {message}");
                            break;
                        }
                        Ok(_) => { /* ignore unexpected frames */ }
                        Err(e) => {
                            if e.kind() != std::io::ErrorKind::UnexpectedEof {
                                eprintln!("Event stream read error: {e}");
                            }
                            break;
                        }
                    }
                }
            }
            Err(e) => eprintln!("Cannot reach agent for events: {e}"),
        }

        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}

async fn handle_event(state: &AppState, frame: EventFrame) {
    let (event_type, alert_msg) = match frame.action.as_str() {
        "die" => {
            let exit_code = frame.exit_code.as_deref().unwrap_or("?");
            let msg = if exit_code != "0" {
                Some(format!(
                    ":red_circle: *Beacon* — container `{}` crashed (exit code {})",
                    frame.container_name, exit_code
                ))
            } else {
                None
            };
            ("crash", msg)
        }
        "oom" => (
            "oom",
            Some(format!(
                ":rotating_light: *Beacon* — container `{}` was OOM-killed",
                frame.container_name
            )),
        ),
        "start" => ("start", None),
        "stop" => ("stop", None),
        "restart" => ("restart", None),
        _ => return,
    };

    let _ = state
        .db
        .record_event(&frame.container_id, &frame.container_name, event_type)
        .await;
    metrics::record_event(event_type);

    let ws_msg = format!(
        "{{\"event\":\"{event_type}\",\"container\":\"{}\"}}",
        frame.container_name
    );
    websocket::broadcast_event(&state.events, ws_msg);

    if let (Some(url), Some(msg)) = (state.slack_webhook.as_deref(), alert_msg) {
        alerts::send_slack(url, &msg).await.ok();
    }
}
