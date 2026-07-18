use k8s_openapi::api::core::v1::{Namespace, Pod};
use kube::{api::ListParams, Api, Client};

pub use protocol::data::orchestration::{NamespaceInfo, PodInfo};

pub async fn connect() -> Result<Client, kube::Error> {
    Client::try_default().await
}

pub async fn list_pods(client: &Client) -> Result<Vec<PodInfo>, kube::Error> {
    let pods: Api<Pod> = Api::all(client.clone());
    let list = pods.list(&ListParams::default()).await?;

    Ok(list
        .items
        .into_iter()
        .map(|pod| {
            let name = pod.metadata.name.unwrap_or_default();
            let namespace = pod.metadata.namespace.unwrap_or_default();
            let phase = pod
                .status
                .as_ref()
                .and_then(|s| s.phase.clone())
                .unwrap_or_else(|| "Unknown".to_string());
            let ready = pod
                .status
                .as_ref()
                .and_then(|s| s.conditions.as_ref())
                .and_then(|c| c.iter().find(|c| c.type_ == "Ready").map(|c| c.status == "True"))
                .unwrap_or(false);
            PodInfo { name, namespace, status: phase, ready }
        })
        .collect())
}

pub async fn list_namespaces(client: &Client) -> Result<Vec<NamespaceInfo>, kube::Error> {
    let namespaces: Api<Namespace> = Api::all(client.clone());
    let list = namespaces.list(&ListParams::default()).await?;

    Ok(list
        .items
        .into_iter()
        .map(|ns| NamespaceInfo {
            name: ns.metadata.name.unwrap_or_default(),
            status: ns.status.and_then(|s| s.phase).unwrap_or_else(|| "Unknown".to_string()),
        })
        .collect())
}
