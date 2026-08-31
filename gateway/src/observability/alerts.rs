use serde_json::json;

pub async fn send_slack(webhook_url: &str, message: &str) -> Result<(), reqwest::Error> {
    reqwest::Client::new()
        .post(webhook_url)
        .json(&json!({ "text": message }))
        .send()
        .await?;
    Ok(())
}

pub async fn container_started(webhook_url: &str, container_id: &str) {
    let msg = format!(
        ":white_check_mark: *Pulse* — container `{}` started",
        container_id
    );
    if let Err(e) = send_slack(webhook_url, &msg).await {
        eprintln!("Slack alert failed: {e}");
    }
}

pub async fn container_stopped(webhook_url: &str, container_id: &str) {
    let msg = format!(":stop_sign: *Pulse* — container `{}` stopped", container_id);
    if let Err(e) = send_slack(webhook_url, &msg).await {
        eprintln!("Slack alert failed: {e}");
    }
}

pub async fn container_restarted(webhook_url: &str, container_id: &str) {
    let msg = format!(
        ":arrows_counterclockwise: *Pulse* — container `{}` restarted",
        container_id
    );
    if let Err(e) = send_slack(webhook_url, &msg).await {
        eprintln!("Slack alert failed: {e}");
    }
}
