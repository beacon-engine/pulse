use axum::{extract::Query, http::StatusCode, Json};
use serde::{Deserialize, Serialize};

#[derive(Deserialize)]
pub struct ProxyQuery {
    pub url: String,
    pub q: String,
}

#[derive(Serialize)]
pub struct ProxyError {
    pub error: String,
}

/// Proxies a PromQL query to a Prometheus server. This is a pure HTTP passthrough
/// and needs no agent involvement, so it lives entirely in the gateway.
pub async fn query(
    Query(params): Query<ProxyQuery>,
) -> Result<Json<serde_json::Value>, (StatusCode, Json<ProxyError>)> {
    let base = format!("{}/api/v1/query", params.url.trim_end_matches('/'));

    let resp = reqwest::Client::new()
        .get(&base)
        .query(&[("query", &params.q)])
        .send()
        .await
        .map_err(|e| {
            (
                StatusCode::BAD_GATEWAY,
                Json(ProxyError { error: format!("Cannot reach Prometheus: {e}") }),
            )
        })?;

    let body: serde_json::Value = resp.json().await.map_err(|e| {
        (
            StatusCode::BAD_GATEWAY,
            Json(ProxyError { error: format!("Invalid Prometheus response: {e}") }),
        )
    })?;

    Ok(Json(body))
}
