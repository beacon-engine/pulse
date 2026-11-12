# Managing Kubernetes clusters

Pulse manages a cluster through an **agent running inside it**. The gateway
stays where it is (on a server, see [deploy-docker.md](deploy-docker.md)), and
each cluster gets one agent, which appears in Pulse as its own environment.

```
 Gateway (your server)  ◄──── wss ──── agent pod ──► Kubernetes API
                                       (cluster A)
                        ◄──── wss ──── agent pod ──► Kubernetes API
                                       (cluster B)
```

The agent talks to the Kubernetes API with its own ServiceAccount and a
read-mostly ClusterRole, so no kubeconfig is involved. Its identity comes from
the cluster itself (`k8s-<kube-system namespace UID>`), so restarting or
reinstalling it keeps the same environment in Pulse.

There are two ways to connect it:

| Manifest | Who connects | Use it when |
|---|---|---|
| [`agent-push.yaml`](../deploy/kubernetes/agent-push.yaml) | The agent dials the gateway | **Default.** The cluster only needs outbound access to the gateway: no ingress, no open ports. |
| [`agent-pull.yaml`](../deploy/kubernetes/agent-pull.yaml) | The gateway dials the agent | The gateway can reach into the cluster, but the cluster can't reach the gateway. |

## Push mode (recommended)

### 1. Give the gateway a token for the cluster

On the gateway's server, next to its compose file:

```bash
TOKEN=$(openssl rand -hex 32)
echo "PULSE_AGENT_TOKENS=k8s:$TOKEN" >> .env   # add to the list if you already have one
docker compose up -d                              # the gateway reads tokens at startup
```

`PULSE_AGENT_TOKENS` is a comma-separated list of `name:token` pairs. The name is
only a label; several clusters can share one token and still appear separately.

### 2. Store the same token in the cluster

```bash
kubectl create namespace pulse
kubectl -n pulse create secret generic pulse-agent-token --from-literal=token="$TOKEN"
```

The manifest deliberately doesn't contain this Secret, so re-applying it never
resets the token.

### 3. Point the manifest at your gateway and apply it

Download [`agent-push.yaml`](../deploy/kubernetes/agent-push.yaml) and edit
three values:

| Field | Set it to |
|---|---|
| `image` | A pinned version, e.g. `beaconengine/pulse-agent:0.1.0` (the default `latest` changes under you) |
| `PULSE_GATEWAY_WS_URL` | The address the **cluster** uses to reach the gateway, ending in `/agent/ws` (below) |
| `PULSE_ENV_NAME` | The environment's name in Pulse, e.g. `prod-eks` |

| Where the gateway is | `PULSE_GATEWAY_WS_URL` |
|---|---|
| Behind HTTPS (the normal case) | `wss://pulse.example.com/agent/ws` |
| On your machine in Docker, cluster in OrbStack / Docker Desktop | `ws://host.docker.internal:9080/agent/ws` |
| In the same cluster | `ws://pulse-gateway.<namespace>.svc.cluster.local:8080/agent/ws` |

Use `wss://` whenever the traffic crosses a network you don't fully trust: with
`ws://` the token and everything the agent reports travel in plain text.

```bash
kubectl apply -f agent-push.yaml
```

### 4. Check it connected

```bash
kubectl -n pulse logs deploy/pulse-agent
#   Kubernetes connected
#   Pulse agent dialing gateway at wss://… (push mode; auto-register as "prod-eks", identity k8s-…)
#   Uplink connected to gateway wss://…
```

The cluster appears in the environment selector. Select it and open
**Kubernetes**. The Docker pages are hidden for it, because the agent reports
Kubernetes but no container runtime.

| Symptom | Cause |
|---|---|
| `Uplink to … failed/closed`, repeating | Wrong URL or DNS, or the cluster can't reach the gateway |
| Gateway logs `invalid or missing agent token` | The Secret doesn't match a `PULSE_AGENT_TOKENS` entry, or the gateway wasn't restarted after adding it |
| A tab says Pulse "isn't allowed" or couldn't load nodes / services / events | The ClusterRole is older than the agent: re-apply the manifest (below) |
| `x509: certificate signed by unknown authority` | A self-signed gateway certificate. Uncomment `PULSE_INSECURE_SKIP_TLS_VERIFY` in the manifest (trusted networks only) |

If the connection drops, the agent notices within about 30 seconds and
reconnects on its own, waiting 1 s at first and up to 30 s between attempts.

## Pull mode

[`agent-pull.yaml`](../deploy/kubernetes/agent-pull.yaml) runs the agent as a
server on port 9000, exposed on every node as NodePort **30900**, with a
NetworkPolicy that admits only that port. (Switch the Service to
`LoadBalancer` on a cloud cluster.) Set its `image` as above, apply it, then in
Pulse choose **Add host → Pulse agent** with the address `<node or LB address>:30900`.
Leave the optional Docker/Podman socket field empty.

The gateway pings pull agents every 30 seconds and marks them offline when they
stop answering.

**Pull agents accept any caller today**: whoever can reach port 30900 can drive
the agent, with the agent's permissions in the cluster. Only expose it on a
private network, and prefer push mode. Agent enrollment
([design](design/agent-enrollment.md)) will fix this.

## Updating

```bash
kubectl apply -f agent-push.yaml                     # new image tag and/or RBAC
kubectl -n pulse rollout restart deploy/pulse-agent  # only needed to re-pull the same tag
```

Re-applying is safe: the manifest doesn't contain the token Secret. Some
releases need extra permissions (the Nodes, Services and Events tabs did), which
re-applying brings in.

## What the agent may do

Cluster-wide, exactly what the Kubernetes page uses:

| Resource (API group) | Verbs | Used for |
|---|---|---|
| `pods` (core) | get, list, delete | Pods list; delete (its controller recreates it) |
| `pods/log` (core) | get | Container logs |
| `namespaces` (core) | get, list | Namespace filter |
| `nodes`, `services`, `events` (core) | get, list | Nodes, Services and Events tabs |
| `deployments`, `statefulsets`, `daemonsets` (apps) | get, list, patch | Workloads list; scale; rolling restart |

It **cannot** read Secrets or ConfigMaps, exec into pods, or create anything.
It also refuses to scale, restart or delete its own pods, or anything labelled
`pulse/protected=true`, since that would cut off its own connection.

Limiting the agent to some namespaces isn't supported yet: it lists pods and
namespaces cluster-wide, so it needs a ClusterRole.

## Using the Kubernetes page

The header shows the cluster's Kubernetes version, API server, and whether it's
connected through a **Push agent** or a **Pull agent**. Below it are health
counts, then a tab per kind of object. The filter box and namespace selector
apply to every tab.

| Tab | What you can do |
|---|---|
| **Workloads** | Deployments, StatefulSets and DaemonSets with ready/desired counts. Scale with − / + (Deployments, StatefulSets), start a rolling restart, or jump to the workload's pods. |
| **Pods** | Status as `kubectl get pods` shows it (CrashLoopBackOff, Terminating, …), ready containers, restarts, controller, node, age. Inspect a pod for placement, labels, resources and its events. Delete a pod and its controller replaces it. |
| **Containers** | Every pod's containers, laid out like the Docker containers page: status, ports, restarts, **logs**, inspect, and restart (which replaces the pod). Filter to the ones restarting or not ready. |
| **Services** | Type, cluster IP, external address, ports (`port:nodePort→target`) and selector. |
| **Nodes** | Ready state, roles, kubelet version, OS / architecture / runtime, CPU, memory, and pods against capacity. Pressure conditions and cordoned nodes are flagged. |
| **Events** | The newest 300 events (Kubernetes keeps about an hour), with a Warnings filter. |

You manage containers **through their workload**, not one by one. There is no
"stop container", because Kubernetes would just start it again. To stop
something, scale it to 0; to restart it, do a rolling restart or delete the pod;
to change it, change the workload.

**Pod containers in the Docker view.** Clusters that run on the host's Docker
engine (OrbStack, Docker Desktop) show every pod container in that host's Docker
environment too, as `k8s_<container>_<pod>_…`, tagged **k8s**. Logs, exec and
restart work there; recreating them or editing their environment is refused,
since the copy would run outside the cluster. Clusters that use containerd (most
cloud clusters) don't expose their containers to Docker at all.

## Managing a different cluster with a kubeconfig

An agent normally manages the cluster it runs in. To point it at another one,
mount a kubeconfig:

```yaml
# kubectl -n pulse create secret generic pulse-kubeconfig --from-file=kubeconfig=$HOME/.kube/config
env:
  - name: PULSE_KUBECONFIG_PATH
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

`PULSE_KUBECONFIG_PATH` (a file) wins over `PULSE_KUBECONFIG` (inline YAML).
With neither, the agent uses its ServiceAccount, and outside a pod it falls back
to `~/.kube/config`.

## Try it with demo workloads

```bash
kubectl apply -f deploy/kubernetes/examples/demo-workloads.yaml
kubectl delete namespace pulse-demo   # clean up
```

This creates a `pulse-demo` namespace with a 3-replica Deployment, a
two-container Deployment, a StatefulSet, a DaemonSet, and a Deployment that
crash-loops on purpose.
