use k8s_openapi::api::apps::v1::Deployment;
use k8s_openapi::api::core::v1::{Namespace, Pod};
use kube::api::{DeleteParams, ListParams, Patch, PatchParams};
use kube::config::{KubeConfigOptions, Kubeconfig, KubeconfigError};
use kube::{Api, Client, Config};

pub use protocol::data::orchestration::{NamespaceInfo, PodContainer, PodInfo};

/// Where the agent should source its Kubernetes credentials from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum K8sTarget {
    /// Use auto-discovery: in-cluster pod ServiceAccount or agent host $KUBECONFIG / ~/.kube/config
    Auto,
    /// Explicit kubeconfig YAML, e.g. provided out-of-band to the agent host.
    CustomYaml(String),
}

impl K8sTarget {
    /// Picks a target from an optional kubeconfig YAML: a non-blank YAML selects
    /// [`K8sTarget::CustomYaml`]; `None` or a blank string falls back to
    /// [`K8sTarget::Auto`] so misconfiguration degrades to auto-discovery rather
    /// than a hard failure.
    pub fn from_optional_yaml(yaml: Option<String>) -> Self {
        match yaml {
            Some(y) if !y.trim().is_empty() => K8sTarget::CustomYaml(y),
            _ => K8sTarget::Auto,
        }
    }
}

/// Everything that can go wrong while building a [`Client`] in [`connect`].
///
/// A dedicated type is needed because `kube::Error::InferConfig` wraps a
/// `InferConfigError` with private fields, so a raw [`KubeconfigError`] can't be
/// folded into `kube::Error`.
#[derive(Debug)]
pub enum ConnectError {
    /// The supplied kubeconfig YAML could not be parsed or loaded.
    Kubeconfig(KubeconfigError),
    /// The client could not be built from the resolved configuration.
    Client(kube::Error),
}

impl std::fmt::Display for ConnectError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConnectError::Kubeconfig(e) => write!(f, "invalid kubeconfig: {e}"),
            ConnectError::Client(e) => write!(f, "{e}"),
        }
    }
}

impl std::error::Error for ConnectError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            ConnectError::Kubeconfig(e) => Some(e),
            ConnectError::Client(e) => Some(e),
        }
    }
}

impl From<KubeconfigError> for ConnectError {
    fn from(e: KubeconfigError) -> Self {
        ConnectError::Kubeconfig(e)
    }
}

impl From<kube::Error> for ConnectError {
    fn from(e: kube::Error) -> Self {
        ConnectError::Client(e)
    }
}

/// Builds a Kubernetes [`Client`] from the requested [`K8sTarget`].
pub async fn connect(target: K8sTarget) -> Result<Client, ConnectError> {
    match target {
        K8sTarget::CustomYaml(yaml_str) if !yaml_str.trim().is_empty() => {
            // Parse the raw kubeconfig YAML, then build a client from it directly
            // instead of going through the ambient inference path.
            let kubeconfig = Kubeconfig::from_yaml(&yaml_str)?;
            let config =
                Config::from_custom_kubeconfig(kubeconfig, &KubeConfigOptions::default()).await?;
            Ok(Client::try_from(config)?)
        }
        _ => {
            // Auto (or an empty CustomYaml): ambient in-cluster ServiceAccount or
            // the local ~/.kube/config on the agent host.
            Ok(Client::try_default().await?)
        }
    }
}

pub async fn list_pods(client: &Client) -> Result<Vec<PodInfo>, kube::Error> {
    let pods: Api<Pod> = Api::all(client.clone());
    let list = pods.list(&ListParams::default()).await?;

    let mut result: Vec<PodInfo> = list
        .items
        .into_iter()
        .map(|pod| {
            let name = pod.metadata.name.unwrap_or_default();
            let namespace = pod.metadata.namespace.unwrap_or_default();
            let status = pod.status.as_ref();
            let phase = status
                .and_then(|s| s.phase.clone())
                .unwrap_or_else(|| "Unknown".to_string());
            let ready = status
                .and_then(|s| s.conditions.as_ref())
                .and_then(|c| {
                    c.iter()
                        .find(|c| c.type_ == "Ready")
                        .map(|c| c.status == "True")
                })
                .unwrap_or(false);

            let containers = status
                .and_then(|s| s.container_statuses.clone())
                .unwrap_or_default()
                .into_iter()
                .map(|cs| {
                    let state = cs
                        .state
                        .as_ref()
                        .map(|st| {
                            if st.running.is_some() {
                                "running"
                            } else if st.terminated.is_some() {
                                "terminated"
                            } else if st.waiting.is_some() {
                                "waiting"
                            } else {
                                "unknown"
                            }
                        })
                        .unwrap_or("unknown")
                        .to_string();
                    PodContainer {
                        name: cs.name,
                        image: cs.image,
                        ready: cs.ready,
                        restart_count: cs.restart_count,
                        state,
                    }
                })
                .collect();

            PodInfo {
                name,
                namespace,
                status: phase,
                ready,
                containers,
            }
        })
        .collect();

    // Stable order so periodic refreshes don't reshuffle the UI.
    result.sort_by(|a, b| (&a.namespace, &a.name).cmp(&(&b.namespace, &b.name)));
    Ok(result)
}

/// Deletes a pod (its controller, if any, will recreate it).
pub async fn delete_pod(client: &Client, namespace: &str, name: &str) -> Result<(), kube::Error> {
    let pods: Api<Pod> = Api::namespaced(client.clone(), namespace);
    let _ = pods.delete(name, &DeleteParams::default()).await?;
    Ok(())
}

pub async fn list_namespaces(client: &Client) -> Result<Vec<NamespaceInfo>, kube::Error> {
    let namespaces: Api<Namespace> = Api::all(client.clone());
    let list = namespaces.list(&ListParams::default()).await?;

    let mut result: Vec<NamespaceInfo> = list
        .items
        .into_iter()
        .map(|ns| NamespaceInfo {
            name: ns.metadata.name.unwrap_or_default(),
            status: ns
                .status
                .and_then(|s| s.phase)
                .unwrap_or_else(|| "Unknown".to_string()),
        })
        .collect();
    result.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(result)
}

/// Triggers a rollout restart of a deployment by stamping the standard
/// `kubectl.kubernetes.io/restartedAt` pod-template annotation.
pub async fn rollout(
    client: &Client,
    namespace: &str,
    deployment: &str,
) -> Result<(), kube::Error> {
    let deployments: Api<Deployment> = Api::namespaced(client.clone(), namespace);
    let now = chrono::Utc::now().to_rfc3339();
    let patch = serde_json::json!({
        "spec": { "template": { "metadata": { "annotations": {
            "kubectl.kubernetes.io/restartedAt": now
        } } } }
    });
    deployments
        .patch(
            deployment,
            &PatchParams::default(),
            &Patch::Strategic(patch),
        )
        .await?;
    Ok(())
}
