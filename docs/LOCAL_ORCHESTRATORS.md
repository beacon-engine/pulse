# Testing Beacon against Swarm & Kubernetes locally

Beacon talks to three orchestrators through the **agent**:

| System          | How the agent reaches it                          | Beacon features |
| --------------- | ------------------------------------------------- | --------------- |
| Docker          | Docker Engine API (`bollard`) via socket/TCP      | containers, images, volumes, networks, logs, exec, stacks |
| Docker Swarm    | same Docker socket, Swarm API                     | services, nodes, force rolling update |
| Kubernetes      | kubeconfig / in-cluster (`kube`)                  | pods, namespaces, deployment rollout |

This guide spins each up **in isolation** on your machine so you can exercise the
integrations without touching anything real.

> The agent connects to Kubernetes using your default kubeconfig
> (`~/.kube/config`) or in-cluster config. It connects to Docker/Swarm using the
> host's `docker_host` string (empty / `unix://…` = local daemon).

---

## 1. Docker Swarm (single-node)

Swarm mode is built into Docker — no extra tools. It runs alongside your normal
containers, so a single-node swarm is the cleanest isolated test bed.

```bash
# Turn the local Docker daemon into a one-node swarm manager
docker swarm init
#   → "Swarm initialized: current node (xxxx) is now a manager."

# Deploy something so Beacon's Swarm page has data
docker service create --name web --replicas 3 -p 8081:80 nginx:alpine
docker service create --name redis redis:7-alpine

# Verify
docker service ls
docker node ls
```

In Beacon: open the **Swarm** page for your local host — you should see the
`web` (3/3) and `redis` services and one manager node. The
`/api/webhooks/:host/swarm/:service/update` endpoint maps to
`docker service update --force`.

Tear down when done:

```bash
docker service rm web redis
docker swarm leave --force        # exits swarm mode; normal containers unaffected
```

---

## 2. Kubernetes (isolated local cluster)

Use a throwaway cluster that lives entirely in Docker so nothing leaks into a
real environment. Any **one** of these works — `kind` is the lightest.

### Option A — kind (Kubernetes in Docker) — recommended

```bash
# macOS
brew install kind kubectl
# or: go install sigs.k8s.io/kind@latest

kind create cluster --name beacon-test
kubectl config use-context kind-beacon-test

# Seed some workloads
kubectl create deployment web --image=nginx:alpine --replicas=2
kubectl create namespace demo
kubectl -n demo create deployment api --image=nginx:alpine

kubectl get pods -A
```

Beacon's **Kubernetes** page reads pods & namespaces cluster-wide. The rollout
webhook (`/api/webhooks/:host/k8s/:namespace/:deployment/rollout`) runs the
equivalent of `kubectl rollout restart deployment/<name> -n <ns>`.

Delete the cluster (removes everything):

```bash
kind delete cluster --name beacon-test
```

### Option B — minikube

```bash
brew install minikube kubectl
minikube start --profile beacon-test
kubectl config use-context beacon-test
kubectl create deployment web --image=nginx:alpine --replicas=2
# ...
minikube delete --profile beacon-test
```

### Option C — k3d (k3s in Docker, very fast)

```bash
brew install k3d kubectl
k3d cluster create beacon-test
kubectl create deployment web --image=nginx:alpine --replicas=2
# ...
k3d cluster delete beacon-test
```

> **Isolation note:** each of these creates its own kube-context. The agent uses
> whatever `~/.kube/config` points at, so run `kubectl config use-context …`
> before starting the agent, and switch back afterwards. Deleting the cluster
> removes all associated resources — nothing persists on your host.

---

## 3. Pointing the agent at each

- **Local Docker/Swarm** — the seeded `Local` host already uses
  `unix:///var/run/docker.sock`. Nothing to configure.
- **A remote Docker host** — add a host in the UI with a `tcp://host:2375`
  address (expose the daemon with TLS in real use).
- **Kubernetes** — the agent auto-detects `~/.kube/config` on startup. Watch the
  agent log: `Kubernetes connected` (good) vs `Kubernetes unavailable (… )`.
  Restart the agent after switching kube-contexts.

## 4. Quick smoke test

```bash
# assuming gateway on :8080 and you're logged in (cookie in cj.txt)
curl -b cj.txt localhost:8080/api/k8s/pods
curl -b cj.txt localhost:8080/api/hosts/<local-host-id>/swarm/services
```
