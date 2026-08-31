use bollard::query_parameters::EventsOptions;
use bollard::Docker;
use futures_util::StreamExt;
use std::collections::HashMap;

use protocol::wire::{EventFrame, Response};
use tokio::io::AsyncWriteExt;

/// Streams container lifecycle events from `docker` to `writer` as
/// [`Response::Event`] frames until the Docker stream ends or the connection is
/// closed by the gateway. The gateway decides which events to persist / alert on.
pub async fn watch<W>(docker: &Docker, writer: &mut W) -> std::io::Result<()>
where
    W: AsyncWriteExt + Unpin,
{
    let mut filters: HashMap<String, Vec<String>> = HashMap::new();
    filters.insert("type".to_string(), vec!["container".to_string()]);

    let options = EventsOptions {
        filters: Some(filters),
        ..Default::default()
    };
    let mut stream = docker.events(Some(options));

    while let Some(result) = stream.next().await {
        let event = match result {
            Ok(event) => event,
            Err(e) => {
                eprintln!("Docker event stream error: {e}");
                continue;
            }
        };

        if let Some(frame) = to_frame(event) {
            protocol::wire::write_frame(writer, &Response::Event(frame)).await?;
        }
    }

    Ok(())
}

fn to_frame(event: bollard::models::EventMessage) -> Option<EventFrame> {
    let action = event.action.as_deref()?.to_string();
    let actor = event.actor.unwrap_or_default();
    let container_id = actor.id.unwrap_or_default();
    let attrs = actor.attributes.unwrap_or_default();
    let container_name = attrs
        .get("name")
        .cloned()
        .unwrap_or_else(|| container_id.clone());
    let exit_code = attrs.get("exitCode").cloned();

    Some(EventFrame {
        container_id,
        container_name,
        action,
        exit_code,
    })
}
