//! Integration tests for the agent's Kubernetes connection layer.
//!
//! These live outside `src/` and exercise `pulse_core` through its public API
//! only — the same surface the binary uses.

use pulse_core::kubernetes::{connect, ConnectError, K8sTarget};

/// A minimal but structurally valid kubeconfig with a `current-context`.
/// `from_custom_kubeconfig` only builds the config/client — it does not dial the
/// server — so no cluster needs to be reachable for this to succeed.
const VALID_KUBECONFIG: &str = r#"
apiVersion: v1
kind: Config
current-context: test-ctx
clusters:
  - name: test-cluster
    cluster:
      server: https://127.0.0.1:6443
      insecure-skip-tls-verify: true
contexts:
  - name: test-ctx
    context:
      cluster: test-cluster
      user: test-user
users:
  - name: test-user
    user:
      token: test-token
"#;

#[test]
fn from_optional_yaml_none_is_auto() {
    assert_eq!(K8sTarget::from_optional_yaml(None), K8sTarget::Auto);
}

#[test]
fn from_optional_yaml_blank_is_auto() {
    assert_eq!(
        K8sTarget::from_optional_yaml(Some("   \n\t ".to_string())),
        K8sTarget::Auto
    );
}

#[test]
fn from_optional_yaml_content_is_custom() {
    let yaml = "apiVersion: v1".to_string();
    assert_eq!(
        K8sTarget::from_optional_yaml(Some(yaml.clone())),
        K8sTarget::CustomYaml(yaml)
    );
}

#[tokio::test]
async fn connect_custom_yaml_builds_client() {
    let client = connect(K8sTarget::CustomYaml(VALID_KUBECONFIG.to_string()))
        .await
        .expect("a well-formed kubeconfig should build a client without dialing the server");
    // Namespace comes from the selected context (defaults to "default").
    assert_eq!(client.default_namespace(), "default");
}

#[tokio::test]
async fn connect_rejects_malformed_yaml() {
    // `Client` isn't `Debug`, so match rather than `expect_err`.
    match connect(K8sTarget::CustomYaml(
        "::: not : valid : yaml :::".to_string(),
    ))
    .await
    {
        Err(ConnectError::Kubeconfig(_)) => {}
        Err(other) => panic!("expected a Kubeconfig error, got: {other:?}"),
        Ok(_) => panic!("malformed kubeconfig YAML must not build a client"),
    }
}
