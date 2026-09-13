use crate::kubernetes::K8sTarget;
use protocol::transport::TlsMode;

/// How the agent connects to the gateway.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentMode {
    /// Pull: bind a TCP socket and wait for the gateway to connect. *(default)*
    Listen,
    /// Push: dial out to the gateway over WSS and serve requests over that
    /// session. For firewalled agents that can only make outbound connections.
    Dial,
}

impl AgentMode {
    fn from_env(value: Option<&str>) -> Self {
        match value.map(|s| s.trim().to_ascii_lowercase()).as_deref() {
            Some("dial" | "push") => AgentMode::Dial,
            _ => AgentMode::Listen,
        }
    }
}

pub struct Config {
    /// How the agent reaches the gateway (pull `Listen` or push `Dial`).
    pub mode: AgentMode,
    /// TCP port the agent listens on for gateway connections (`Listen` mode).
    pub port: u16,
    /// Gateway WebSocket URL to dial in `Dial` mode, e.g.
    /// `wss://gateway.example.com/agent/ws`.
    pub gateway_url: Option<String>,
    /// Bearer token presented to the gateway on the WS upgrade (`Dial` mode).
    pub gateway_token: Option<String>,
    /// Explicit kubeconfig YAML for Kubernetes access. Sourced from
    /// `BEACON_KUBECONFIG_PATH` (a file to read) or, failing that, the inline
    /// `BEACON_KUBECONFIG` YAML. `None` means auto-discover.
    pub kubeconfig_yaml: Option<String>,
    /// Host filesystem root as seen from inside the container (`HOST_ROOT`, e.g.
    /// `/host` for `-v /:/host:ro`). Empty means bare-metal / no bind-mount.
    pub host_root: String,
    /// Server-certificate trust policy for the outbound `wss://` uplink
    /// (`PULSE_INSECURE_SKIP_TLS_VERIFY`). Defaults to verifying.
    pub tls: TlsMode,
}

impl Config {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok(); // Load .env file if present

        let kubeconfig_yaml = std::env::var("BEACON_KUBECONFIG_PATH")
            .ok()
            .and_then(|path| std::fs::read_to_string(path).ok())
            .or_else(|| std::env::var("BEACON_KUBECONFIG").ok());

        Self {
            mode: AgentMode::from_env(std::env::var("BEACON_MODE").ok().as_deref()),
            port: std::env::var("BEACON_PORT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(9000),
            gateway_url: std::env::var("PULSE_GATEWAY_WS_URL")
                .ok()
                .filter(|s| !s.is_empty()),
            gateway_token: std::env::var("BEACON_TOKEN").ok().filter(|s| !s.is_empty()),
            kubeconfig_yaml,
            host_root: std::env::var("HOST_ROOT").unwrap_or_default(),
            tls: TlsMode::from_flag(
                std::env::var("PULSE_INSECURE_SKIP_TLS_VERIFY")
                    .ok()
                    .as_deref(),
            ),
        }
    }

    /// The Kubernetes target implied by the current configuration.
    pub fn k8s_target(&self) -> K8sTarget {
        K8sTarget::from_optional_yaml(self.kubeconfig_yaml.clone())
    }
}
