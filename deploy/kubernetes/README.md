# Running the Pulse agent in Kubernetes (push mode)

In **push (dial) mode** the agent opens an outbound WebSocket to the gateway and
serves every request over that one session. The cluster needs only egress to the
gateway — no inbound ports, no ingress for the agent. This is the firewall-friendly
default; see `docs/PUSH_PULL_TRANSPORT.md` for the full design.

Kubernetes credentials come from the pod's **ServiceAccount** (in-pod RBAC): the
agent's `K8sTarget::Auto` path calls `Client::try_default()`, which reads the
mounted ServiceAccount token. No kubeconfig required.

## 1. Gateway side

The gateway must (a) accept the agent's token and (b) be reachable from the cluster.

- Set `PULSE_AGENT_TOKENS` to `<agent_id>:<token>` pairs (comma-separated). The
  gateway maps `token -> agent_id` and registers the pushed session under that id.
  Example: `PULSE_AGENT_TOKENS=prod-k8s:s3cret-value`.
- The `<token>` half must equal the agent's `BEACON_TOKEN` (the Secret below).
- Expose the gateway's `/agent/ws` endpoint to the cluster:
  - **Plaintext, in-cluster:** if the gateway also runs in the cluster, dial
    `ws://<gateway-service>.<ns>.svc.cluster.local:8080/agent/ws`.
  - **TLS:** put the gateway behind an ingress/LB terminating TLS (or set
    `PULSE_TLS_CERT`/`PULSE_TLS_KEY` for native rustls) and dial `wss://host/agent/ws`.

## 2. Agent side

1. Build & publish the agent image (`docker build --target agent -t <registry>/pulse-agent .`
   and push, or `kind load` / `minikube image load` for local clusters).
2. In `agent-push.yaml` replace:
   - `image:` → your image reference,
   - `REPLACE_ME_AGENT_TOKEN` → the token half of the gateway pair,
   - `PULSE_GATEWAY_WS_URL` → your `ws://…`/`wss://…` endpoint.
3. Apply:
   ```bash
   kubectl apply -f deploy/kubernetes/agent-push.yaml
   ```

## 3. Verify

```bash
kubectl -n pulse logs deploy/pulse-agent -f
# expect: "Pulse agent dialing gateway at <url> (push mode)"
#         "Uplink connected to gateway <url>"
#         "Kubernetes connected"   (or "…will retry on demand")
```
On the gateway you should see `Agent '<agent_id>' connected (push session)`, and
`GET /api/agents` should list `<agent_id>`. Pods/namespaces for that host will then
resolve through the pushed session.

If you see `Uplink connect … failed` looping: the URL/DNS is wrong or egress is
blocked. If the gateway logs `invalid or missing agent token`: the `BEACON_TOKEN`
Secret doesn't match a `PULSE_AGENT_TOKENS` entry. For a self-signed gateway cert,
uncomment `PULSE_INSECURE_SKIP_TLS_VERIFY=1` in the Deployment.

## RBAC granted

Cluster-wide, matching exactly what `agent/src/kubernetes.rs` calls:

| Resource (apiGroup)      | Verbs                | Used by            |
|--------------------------|----------------------|--------------------|
| `pods` (core)            | get, list, delete    | list/delete pods   |
| `namespaces` (core)      | get, list            | list namespaces    |
| `deployments` (apps)     | get, list, patch     | rollout restart    |

`pods`/`namespaces` use cluster-wide `list`, so this must be a **ClusterRole**. To
scope the agent to one namespace, swap the ClusterRole/Binding for a namespaced
Role/RoleBinding and use `Api::default_namespaced` in the agent (larger change).

## Alternative: explicit kubeconfig instead of in-pod RBAC

To point the agent at a *different* cluster (or a specific context) rather than the
one it runs in, skip the ServiceAccount RBAC and mount a kubeconfig:

```yaml
# Secret with the kubeconfig YAML:
#   kubectl -n pulse create secret generic pulse-kubeconfig \
#     --from-file=kubeconfig=$HOME/.kube/config
# Then in the Deployment container:
env:
  - name: BEACON_KUBECONFIG_PATH
    value: /etc/pulse/kubeconfig
volumeMounts:
  - name: kubeconfig
    mountPath: /etc/pulse
    readOnly: true
volumes:
  - name: kubeconfig
    secret:
      secretName: pulse-kubeconfig
```
`BEACON_KUBECONFIG_PATH` (a file) takes precedence; the inline `BEACON_KUBECONFIG`
YAML is the fallback. A non-blank kubeconfig selects `K8sTarget::CustomYaml`;
otherwise the agent falls back to in-pod auto-discovery.
