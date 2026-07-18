// state.rs
use tokio::net::TcpStream;
use tokio::sync::Mutex;
use std::sync::Arc;

#[derive(Clone)]
pub struct AppState {
    pub worker: Arc<Mutex<TcpStream>>,     // TCP connection to background worker
    pub events: Arc<observability::websocket::EventChannel>, // keep if you want WS from frontend
    pub slack_webhook: Option<String>,
    pub db: sqlx::SqlitePool,
}
