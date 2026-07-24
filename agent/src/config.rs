pub struct Config {
    /// TCP port the agent listens on for gateway connections.
    pub port: u16,
}

impl Config {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok(); // Load .env file if present

        Self {
            port: std::env::var("BEACON_PORT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(9000),
        }
    }
}
