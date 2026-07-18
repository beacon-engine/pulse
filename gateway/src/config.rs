pub struct Config {
    pub slack_webhook_url: Option<String>,
    pub http_port: u16,
    pub tcp_server_port: u16,
    pub database_url: String,
    pub beacon_api_url: String,
}

impl Config {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok(); // Load env file

        Self {
            slack_webhook_url: std::env::var("SLACK_WEBHOOK_URL").ok(),
            http_port: std::env::var("HTTP_PORT").ok().and_then(|v| v.parse().ok()).unwrap_or(8080),
            tcp_server_port: std::env::var("TCP_SERVER_PORT").ok().and_then(|v| v.parse().ok()).unwrap_or(9000),
            database_url: std::env::var("DATABASE_URL").ok().unwrap(),
            beacon_api_url: std::env::var("BEACON_API_URL").ok().unwrap(),
        }
    }
}
