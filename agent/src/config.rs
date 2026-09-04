use crate::kubernetes::K8sTarget;

pub struct Config {
    /// TCP port the agent listens on for gateway connections.
    pub port: u16,
    /// Explicit kubeconfig YAML for Kubernetes access. Sourced from
    /// `BEACON_KUBECONFIG_PATH` (a file to read) or, failing that, the inline
    /// `BEACON_KUBECONFIG` YAML. `None` means auto-discover.
    pub kubeconfig_yaml: Option<String>,
}

impl Config {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok(); // Load .env file if present

        let kubeconfig_yaml = std::env::var("BEACON_KUBECONFIG_PATH")
            .ok()
            .and_then(|path| std::fs::read_to_string(path).ok())
            .or_else(|| std::env::var("BEACON_KUBECONFIG").ok());

        Self {
            port: std::env::var("BEACON_PORT")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(9000),
            kubeconfig_yaml,
        }
    }

    /// The Kubernetes target implied by the current configuration.
    pub fn k8s_target(&self) -> K8sTarget {
        K8sTarget::from_optional_yaml(self.kubeconfig_yaml.clone())
    }
}
