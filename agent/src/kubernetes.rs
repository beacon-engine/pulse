use k8s_openapi::api::apps::v1::Deployment;
use k8s_openapi::api::core::v1::{Namespace, Pod};
use kube::api::{DeleteParams, ListParams, Patch, PatchParams};
use kube::{Api, Client};

pub use protocol::data::orchestration::{NamespaceInfo, PodContainer, PodInfo};

pub async fn connect() -> Result<Client, kube::Error> {
    Client::try_default().await
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
                .and_then(|c| c.iter().find(|c| c.type_ == "Ready").map(|c| c.status == "True"))
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

            PodInfo { name, namespace, status: phase, ready, containers }
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
            status: ns.status.and_then(|s| s.phase).unwrap_or_else(|| "Unknown".to_string()),
        })
        .collect();
    result.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(result)
}

/// Triggers a rollout restart of a deployment by stamping the standard
/// `kubectl.kubernetes.io/restartedAt` pod-template annotation.
pub async fn rollout(client: &Client, namespace: &str, deployment: &str) -> Result<(), kube::Error> {
    let deployments: Api<Deployment> = Api::namespaced(client.clone(), namespace);
    let now = chrono::Utc::now().to_rfc3339();
    let patch = serde_json::json!({
        "spec": { "template": { "metadata": { "annotations": {
            "kubectl.kubernetes.io/restartedAt": now
        } } } }
    });
    deployments
        .patch(deployment, &PatchParams::default(), &Patch::Merge(patch))
        .await?;
    Ok(())
}
