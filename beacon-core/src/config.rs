pub struct Config {
    pub slack_webhook_url: Option<String>,
    pub port: u16,
    pub database_url: String,
}

impl Config {
    pub fn from_env() -> Self {
        Self {
            slack_webhook_url: std::env::var("SLACK_WEBHOOK_URL").ok(),
            port: std::env::var("BEACON_PORT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(9000),
            database_url: std::env::var("DATABASE_URL")
                .unwrap_or_else(|_| "sqlite://beacon.db".to_string()),
        }
    }
}
