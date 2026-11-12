# Running the Pulse agent in Kubernetes

One agent per cluster gives Pulse a **Kubernetes** environment. In it you can see
workloads, pods and their containers, services, nodes and events; read each
container's logs; scale and restart workloads; and delete pods. The page's header
says whether the cluster is connected through a push or a pull agent.

| Manifest | Mode | When to use |
|---|---|---|
| `agent-push.yaml` | **Push** (agent dials the gateway) | Default. The cluster needs only outbound access to the gateway; no ingress, no open ports. |
| `agent-pull.yaml` | Pull (gateway dials the agent) | The gateway can reach into the cluster but the cluster can't reach the gateway. |

The agent authenticates to the Kubernetes API with its own **ServiceAccount**
(the RBAC in the manifest). No kubeconfig is needed. Its environment keeps the
same identity across restarts: it is derived from the cluster
(`k8s-<kube-system namespace UID>`), so a redeploy reuses the existing
environment in Pulse.

## Push mode, step by step

### 1. Give the gateway a token for the agent

The gateway accepts agents whose token appears in `PULSE_AGENT_TOKENS`
(`<name>:<token>` pairs, comma-separated). The name is only a label, and the
environment shows up as `<name>:<cluster identity>`.

```bash
TOKEN=$(openssl rand -hex 32)
echo "PULSE_AGENT_TOKENS=k8s:$TOKEN" >> .env   # next to docker-compose.yml
docker compose up -d                              # the gateway reads tokens at startup
```

### 2. Store the same token in the cluster

```bash
kubectl create namespace pulse
kubectl -n pulse create secret generic pulse-agent-token --from-literal=token="$TOKEN"
```

The manifest deliberately doesn't contain this Secret, so re-applying it never
resets your token.

### 3. Point the manifest at your gateway and apply it

Edit three values in `agent-push.yaml`:

| Field | Set it to |
|---|---|
| `image` | `beaconengine/pulse-agent:sha-<commit>` (pinned), or a local image (below) |
| `PULSE_GATEWAY_WS_URL` | The URL the **cluster** uses to reach the gateway's `/agent/ws` (examples below) |
| `PULSE_ENV_NAME` | The environment's name in Pulse, e.g. `prod-eks` |

| Where the gateway runs | `PULSE_GATEWAY_WS_URL` |
|---|---|
| Docker Compose on your machine, cluster in OrbStack / Docker Desktop | `ws://host.docker.internal:9080/agent/ws` |
| In the same cluster | `ws://pulse-gateway.<ns>.svc.cluster.local:8080/agent/ws` |
| Behind TLS (ingress / load balancer) | `wss://pulse.example.com/agent/ws` |

```bash
kubectl apply -f deploy/kubernetes/agent-push.yaml
```

### 4. Check it connected

```bash
kubectl -n pulse logs deploy/pulse-agent
#   Kubernetes connected
#   Pulse agent dialing gateway at ws://… (push mode; auto-register as "prod-eks", identity k8s-…)
#   Uplink connected to gateway ws://…
```

The environment appears under **Hosts** and in the environment selector. The
**Kubernetes** page lights up when you select it. The Docker-only pages are
hidden, because the agent reports that it has Kubernetes but no container
runtime.

| Symptom | Cause |
|---|---|
| `Uplink connect … failed`, repeating | Wrong URL/DNS, or the cluster can't reach the gateway |
| Gateway logs `invalid or missing agent token` | The Secret doesn't match a `PULSE_AGENT_TOKENS` entry, or the gateway wasn't restarted after adding it |
| Kubernetes page: *"isn't allowed to do this"*, or a tab saying it couldn't load nodes / services / events | The ClusterRole is older than the agent; re-apply it (below) |
| Self-signed gateway certificate | Uncomment `PULSE_INSECURE_SKIP_TLS_VERIFY` (trusted networks only) |

### Updating an existing install

Pulse releases sometimes need more RBAC (the Nodes, Services and Events tabs
need read access to those objects). Re-applying the whole manifest is safe: it
doesn't contain your token Secret, and it keeps the image and settings you
edited into it.

```bash
kubectl apply -f deploy/kubernetes/agent-push.yaml     # RBAC + deployment
kubectl -n pulse rollout restart deploy/pulse-agent    # pick up a rebuilt image with the same tag
```

### Local clusters: using an image you built

OrbStack and Docker Desktop run Kubernetes on the same Docker engine, so a
locally built image is already available to the cluster:

```bash
docker compose build agent
docker tag pulse-agent:latest pulse-agent:local
# in agent-push.yaml: image: pulse-agent:local, imagePullPolicy: IfNotPresent
```

For **kind**, run `kind load docker-image pulse-agent:local`. For **minikube**,
run `minikube image load pulse-agent:local`.

## Pull mode

`agent-pull.yaml` runs the agent as a TCP server on port 9000 behind a
`ClusterIP` Service, with a NetworkPolicy that admits only that port. Expose it
to the gateway (a `LoadBalancer`/`NodePort`, or `kubectl port-forward` for a
quick test). Then add it in Pulse: **Hosts → Add environment → Pulse agent**,
with the address `host:port`.

Pull agents don't authenticate the gateway yet (see
`docs/AGENT_ENROLLMENT.md`), so never expose port 9000 beyond networks you
trust. Prefer push mode.

## What the agent may do (RBAC)

Cluster-wide, exactly what the Kubernetes page uses:

| Resource (apiGroup) | Verbs | Used for |
|---|---|---|
| `pods` (core) | get, list, delete | Pods table; delete (the controller recreates it) |
| `pods/log` (core) | get | Container logs |
| `namespaces` (core) | get, list | Namespace list and filter |
| `nodes`, `services`, `events` (core) | get, list | Nodes, Services and Events tabs (read-only) |
| `deployments`, `statefulsets`, `daemonsets` (apps) | get, list, patch | Workloads table; scale (`spec.replicas`); rolling restart |

It **cannot** read Secrets or ConfigMaps, exec into pods, or create anything.
The agent also refuses to scale, restart or delete its own pods, or any
workload labelled `pulse/role` or `pulse/protected=true`, because that would
cut off the connection Pulse manages the cluster through.

Pods and namespaces are listed cluster-wide, so this must be a ClusterRole.
Limiting the agent to one namespace needs a Role/RoleBinding plus agent
changes; that isn't supported yet.

## Alternative: a kubeconfig instead of the ServiceAccount

To manage a *different* cluster from the one the agent runs in, mount a
kubeconfig:

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

`examples/demo-workloads.yaml` creates a `pulse-demo` namespace with something
to look at:

- a 3-replica Deployment
- a two-container Deployment, for per-container logs
- a StatefulSet
- a DaemonSet
- a Deployment that crash-loops on purpose

```bash
kubectl apply -f deploy/kubernetes/examples/demo-workloads.yaml
kubectl delete namespace pulse-demo   # clean up
```
