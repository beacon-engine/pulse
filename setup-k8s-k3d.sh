#!/usr/bin/env bash

set -e

CLUSTER_NAME="pulse-k8s-cluster"
AGENT_NAME="pulse-k8s-agent"
AGENT_IMAGE="alpine:latest" # Replace with your Pulse K8s agent image

echo "🚀 Setting up 3-node Kubernetes cluster using k3d..."

# 1. Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    echo "❌ k3d is not installed. Install it via 'brew install k3d' or 'curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash'"
    exit 1
fi

# 2. Delete old cluster if it exists
if k3d cluster list | grep -q "$CLUSTER_NAME"; then
    echo "⚠️ Deleting existing k3d cluster $CLUSTER_NAME..."
    k3d cluster delete "$CLUSTER_NAME"
fi

# 3. Create cluster with 1 Server (Control Plane) and 2 Agents (Workers)
echo "🌐 Creating multi-node cluster (1 Server + 2 Worker nodes)..."
k3d cluster create "$CLUSTER_NAME" --servers 1 --agents 2 --wait

# 4. Verify Nodes
echo ""
echo "================ Kubernetes Nodes ================"
kubectl get nodes -o wide
echo "=================================================="
echo ""

# 5. Deploy Pulse DaemonSet (Global Agent across all K8s nodes)
echo "📡 Deploying Pulse DaemonSet agent across all nodes..."

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ${AGENT_NAME}
  namespace: default
  labels:
    app: ${AGENT_NAME}
spec:
  selector:
    matchLabels:
      app: ${AGENT_NAME}
  template:
    metadata:
      labels:
        app: ${AGENT_NAME}
    spec:
      containers:
      - name: agent
        image: ${AGENT_IMAGE}
        command: ["sh", "-c", "echo 'Pulse Agent running on K8s Node: \$NODE_NAME'; sleep infinity"]
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: host-proc
          mountPath: /host/proc
          readOnly: true
      volumes:
      - name: host-proc
        hostPath:
          path: /proc
EOF

echo "⏳ Waiting for DaemonSet pods to roll out..."
kubectl rollout status daemonset/${AGENT_NAME} --timeout=60s

echo ""
echo "================ Pulse Pod Status ================"
kubectl get pods -o wide -l app=${AGENT_NAME}
echo "=================================================="
echo ""
echo "✅ K8s cluster ready! Use 'kubectl' to interact with it."
