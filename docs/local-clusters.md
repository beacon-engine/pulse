# Local Swarm and Kubernetes clusters

Throwaway clusters to point Pulse at while developing, so you can try the Swarm
and Kubernetes pages without touching anything real.

## Docker Swarm

**One node.** Swarm is built into Docker, so your local daemon can become a
one-node swarm:

```bash
docker swarm init
docker service create --name web --replicas 3 -p 8081:80 nginx:alpine
docker service create --name redis redis:7-alpine
```

Open the **Swarm** page for your local environment: the two services and one
manager node appear. Clean up with
`docker service rm web redis && docker swarm leave --force` (your other
containers are unaffected).

**Three nodes.** `scripts/setup-swarm-dind.sh` starts a manager and two workers
as Docker-in-Docker containers on their own network; `scripts/cleanup-swarm.sh`
removes them.

## Kubernetes

The recommended setup is the same as production: an agent **inside** the
cluster that dials the gateway. Any local cluster works:

| Cluster | Create | Notes |
|---|---|---|
| OrbStack / Docker Desktop | Enable Kubernetes in the app's settings | Uses your Docker engine, so a locally built agent image needs no loading, and pod containers also show up in your Docker environment |
| k3d | `scripts/setup-k8s-k3d.sh` (1 server, 2 workers) | Multi-node; load images with `k3d image import pulse-agent:local -c pulse-k8s-cluster` |
| kind | `kind create cluster --name pulse-test` | Load images with `kind load docker-image pulse-agent:local --name pulse-test` |
| k3s in one container | `docker run -d --privileged --name pulse-k3s rancher/k3s:v1.31.4-k3s1 server` | Fully isolated; its pods don't appear in your Docker engine |

Then install the agent and some demo workloads, following
[deploy-kubernetes.md](deploy-kubernetes.md):

```bash
docker compose build agent && docker tag pulse-agent:latest pulse-agent:local
# load the image if your cluster needs it (table above), create the token
# Secret (steps 1-2 of that guide), set image: pulse-agent:local, then:
kubectl apply -f deploy/kubernetes/agent-push.yaml
kubectl apply -f deploy/kubernetes/examples/demo-workloads.yaml
```

The demo creates a `pulse-demo` namespace with a 3-replica Deployment, a
two-container Deployment, a StatefulSet, a DaemonSet and a Deployment that
crash-loops on purpose, so every tab has something to show.
`kubectl delete namespace pulse-demo` removes it.

### Quick look without installing anything in the cluster

An agent running on your machine (`go run ./cmd/agent`, or the `make watch-agent`
loop) reads `~/.kube/config` and manages its current context. Switch contexts
with `kubectl config use-context …` and restart the agent. This tests the
dashboard, but not the in-cluster permissions, so check RBAC changes with an
in-cluster agent.
