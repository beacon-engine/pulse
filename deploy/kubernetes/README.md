# Kubernetes manifests

| File | What it is |
|---|---|
| `agent-push.yaml` | The agent in **push** mode: it dials the gateway, so the cluster needs no open ports. The default. |
| `agent-pull.yaml` | The agent in **pull** mode: the gateway dials it on NodePort 30900. |
| `examples/demo-workloads.yaml` | Demo workloads in a `pulse-demo` namespace, including one that crash-loops on purpose. |

Each agent manifest has its namespace, ServiceAccount, ClusterRole and
Deployment. The token Secret is created separately, so re-applying a manifest
never resets it.

Setup, configuration, permissions and troubleshooting:
**[docs/deploy-kubernetes.md](../../docs/deploy-kubernetes.md)**.
