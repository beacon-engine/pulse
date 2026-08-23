pub struct Config {
    pub slack_webhook_url: Option<String>,
    /// HTTP port the gateway serves the frontend API on.
    pub http_port: u16,
    /// TCP address of the agent (host:port), e.g. `127.0.0.1:9000`.
    pub agent_addr: String,
    pub database_url: String,
    pub static_dir: String,
}

impl Config {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok(); // Load .env file if present

        Self {
            slack_webhook_url: std::env::var("SLACK_WEBHOOK_URL")
                .ok()
                .filter(|s| !s.is_empty()),
            http_port: std::env::var("HTTP_PORT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(8080),
            agent_addr: std::env::var("AGENT_ADDR")
                .unwrap_or_else(|_| "127.0.0.1:9000".to_string()),
            database_url: std::env::var("DATABASE_URL")
                .unwrap_or_else(|_| "sqlite://pulse.db".to_string()),
            // Defaults to frontend/dist for local cargo run, or /app/dist in Docker
            static_dir: std::env::var("STATIC_DIR")
                .unwrap_or_else(|_| "./frontend/dist".to_string()),
        }
    }
}
