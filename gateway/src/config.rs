use std::collections::HashMap;

pub struct Config {
    pub slack_webhook_url: Option<String>,
    /// HTTP port the gateway serves the frontend API on.
    pub http_port: u16,
    /// TCP address of the agent (host:port), e.g. `127.0.0.1:9000`.
    pub agent_addr: String,
    pub database_url: String,
    pub static_dir: String,
    /// Bearer tokens that authenticate agents dialing in over `/agent/ws`
    /// (push mode). Maps `token -> agent_id`; the token both authenticates the
    /// agent and identifies which agent (environment) it represents. Empty
    /// disables push — only the pull agent (`agent_addr`) is used.
    pub agent_tokens: HashMap<String, String>,
    /// PEM certificate + private key for serving HTTPS/WSS directly (native
    /// rustls). When both are set the gateway serves TLS; otherwise it serves
    /// plain HTTP (terminate TLS at an upstream ingress/LB instead).
    pub tls_cert_path: Option<String>,
    pub tls_key_path: Option<String>,
    /// Dev-only: when true, a migration-integrity mismatch recreates the database
    /// from scratch instead of aborting (`PULSE_DB_RESET_ON_MISMATCH`). Destroys
    /// data — leave unset in production.
    pub db_reset_on_mismatch: bool,
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
            agent_tokens: parse_agent_tokens(std::env::var("PULSE_AGENT_TOKENS").ok().as_deref()),
            tls_cert_path: std::env::var("PULSE_TLS_CERT")
                .ok()
                .filter(|s| !s.is_empty()),
            tls_key_path: std::env::var("PULSE_TLS_KEY")
                .ok()
                .filter(|s| !s.is_empty()),
            db_reset_on_mismatch: env_flag("PULSE_DB_RESET_ON_MISMATCH"),
        }
    }
}

/// Reads a boolean env flag: truthy = `1`/`true`/`yes`/`on` (case-insensitive);
/// anything else (including unset) is false.
fn env_flag(key: &str) -> bool {
    matches!(
        std::env::var(key)
            .map(|v| v.trim().to_ascii_lowercase())
            .as_deref(),
        Ok("1" | "true" | "yes" | "on")
    )
}

/// Parses `PULSE_AGENT_TOKENS` — a comma-separated list of `agent_id:token`
/// pairs — into a `token -> agent_id` lookup. Blank entries and those missing a
/// `:` are skipped. Example: `prod:s3cret,staging:h0nk`.
pub fn parse_agent_tokens(raw: Option<&str>) -> HashMap<String, String> {
    let mut map = HashMap::new();
    let Some(raw) = raw else { return map };
    for entry in raw.split(',') {
        let entry = entry.trim();
        if let Some((agent_id, token)) = entry.split_once(':') {
            let (agent_id, token) = (agent_id.trim(), token.trim());
            if !agent_id.is_empty() && !token.is_empty() {
                map.insert(token.to_string(), agent_id.to_string());
            }
        }
    }
    map
}

#[cfg(test)]
mod tests {
    use super::parse_agent_tokens;

    #[test]
    fn none_and_blank_yield_no_tokens() {
        assert!(parse_agent_tokens(None).is_empty());
        assert!(parse_agent_tokens(Some("")).is_empty());
        assert!(parse_agent_tokens(Some("   ")).is_empty());
    }

    #[test]
    fn maps_token_to_agent_id() {
        // Configured as agent_id:token; looked up by token, so the map inverts.
        let map = parse_agent_tokens(Some(" prod:s3cret , staging:h0nk "));
        assert_eq!(map.get("s3cret"), Some(&"prod".to_string()));
        assert_eq!(map.get("h0nk"), Some(&"staging".to_string()));
        assert_eq!(map.len(), 2);
    }

    #[test]
    fn skips_malformed_entries() {
        let map = parse_agent_tokens(Some("noColon,:emptyId,emptyToken:,ok:tok"));
        assert_eq!(map.len(), 1);
        assert_eq!(map.get("tok"), Some(&"ok".to_string()));
    }
}
