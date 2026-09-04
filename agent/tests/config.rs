//! Integration tests for agent configuration and the config → Kubernetes target
//! mapping. Exercises `pulse_core::config` through its public API.

use pulse_core::config::Config;
use pulse_core::kubernetes::K8sTarget;

#[test]
fn k8s_target_none_is_auto() {
    let cfg = Config {
        port: 9000,
        kubeconfig_yaml: None,
    };
    assert_eq!(cfg.k8s_target(), K8sTarget::Auto);
}

#[test]
fn k8s_target_blank_is_auto() {
    let cfg = Config {
        port: 9000,
        kubeconfig_yaml: Some("  \n ".to_string()),
    };
    assert_eq!(cfg.k8s_target(), K8sTarget::Auto);
}

#[test]
fn k8s_target_yaml_is_custom() {
    let cfg = Config {
        port: 9000,
        kubeconfig_yaml: Some("apiVersion: v1".to_string()),
    };
    assert_eq!(
        cfg.k8s_target(),
        K8sTarget::CustomYaml("apiVersion: v1".to_string())
    );
}

/// All env-var behaviour is asserted in a single test: env vars are process-wide
/// global state, so splitting these across parallel tests would race.
#[test]
fn from_env_sources_kubeconfig_by_precedence() {
    // A file for the `*_PATH` variant to read.
    let mut path = std::env::temp_dir();
    path.push(format!(
        "pulse-agent-kubeconfig-{}.yaml",
        std::process::id()
    ));
    std::fs::write(&path, "from: file").expect("write temp kubeconfig");

    // Guard the vars we touch so a failure doesn't leak into other tests/CI.
    let _guard = EnvGuard::new(&["BEACON_KUBECONFIG_PATH", "BEACON_KUBECONFIG"]);

    // 1. Path takes precedence over the inline value.
    std::env::set_var("BEACON_KUBECONFIG_PATH", &path);
    std::env::set_var("BEACON_KUBECONFIG", "from: inline");
    assert_eq!(
        Config::from_env().kubeconfig_yaml.as_deref(),
        Some("from: file"),
        "BEACON_KUBECONFIG_PATH should win over inline BEACON_KUBECONFIG"
    );

    // 2. With no path (or an unreadable one), fall back to the inline value.
    std::env::remove_var("BEACON_KUBECONFIG_PATH");
    assert_eq!(
        Config::from_env().kubeconfig_yaml.as_deref(),
        Some("from: inline")
    );

    // 3. Neither set → auto-discovery.
    std::env::remove_var("BEACON_KUBECONFIG");
    assert_eq!(Config::from_env().kubeconfig_yaml, None);

    std::fs::remove_file(&path).ok();
}

/// Restores (or clears) the named env vars when dropped, so a panicking test
/// can't poison sibling tests running in the same process.
struct EnvGuard {
    saved: Vec<(String, Option<String>)>,
}

impl EnvGuard {
    fn new(keys: &[&str]) -> Self {
        let saved = keys
            .iter()
            .map(|k| (k.to_string(), std::env::var(k).ok()))
            .collect();
        Self { saved }
    }
}

impl Drop for EnvGuard {
    fn drop(&mut self) {
        for (key, value) in &self.saved {
            match value {
                Some(v) => std::env::set_var(key, v),
                None => std::env::remove_var(key),
            }
        }
    }
}
