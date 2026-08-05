#!/usr/bin/env bash

set -e

# Configuration
NETWORK_NAME="pulse-dind-net"
MANAGER_NODE="pulse-manager"
WORKER_NODES=("pulse-worker-1" "pulse-worker-2")
AGENT_SERVICE_NAME="pulse-agent"
AGENT_IMAGE="alpine:latest" # Replace with your Pulse agent image (e.g., your-org/pulse-agent:latest)

echo "🚀 Setting up local 3-Node Docker Swarm cluster via DinD..."

# 1. Create a dedicated bridge network for inter-node communication
if ! docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
    echo "🌐 Creating network $NETWORK_NAME..."
    docker network create --driver bridge "$NETWORK_NAME"
fi

# Function to run a DinD container
run_dind_node() {
    local node_name=$1
    if [ "$(docker ps -aq -f name=^${node_name}$)" ]; then
        echo "⚠️ Removing existing container $node_name..."
        docker rm -f "$node_name" >/dev/null 2>&1
    fi

    echo "🐳 Starting DinD node: $node_name..."
    docker run -d \
        --name "$node_name" \
        --hostname "$node_name" \
        --network "$NETWORK_NAME" \
        --privileged \
        -v /var/run/docker.sock:/var/run/docker.sock \
        docker:dind >/dev/null
}

# 2. Launch Manager and Worker DinD nodes
run_dind_node "$MANAGER_NODE"
for worker in "${WORKER_NODES[@]}"; do
    run_dind_node "$worker"
done

echo "⏳ Waiting 5 seconds for Docker daemons to initialize inside DinD..."
sleep 5

# 3. Initialize Docker Swarm on Manager Node
echo "🔧 Initializing Docker Swarm on $MANAGER_NODE..."
MANAGER_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$MANAGER_NODE")
docker exec "$MANAGER_NODE" docker swarm init --advertise-addr "$MANAGER_IP" >/dev/null

# Get worker join token
JOIN_TOKEN=$(docker exec "$MANAGER_NODE" docker swarm join-token worker -q | tr -d '\r')

# 4. Join Worker Nodes to the Swarm
for worker in "${WORKER_NODES[@]}"; do
    echo "🔗 Joining $worker to the cluster..."
    docker exec "$worker" docker swarm join --token "$JOIN_TOKEN" "$MANAGER_IP:2377" >/dev/null
done

# 5. Verify Swarm Status
echo ""
echo "================ Swarm Cluster Status ================"
docker exec "$MANAGER_NODE" docker node ls
echo "======================================================"
echo ""

# 6. Deploy Pulse Global Agent Service
echo "📡 Deploying global agent service ($AGENT_SERVICE_NAME) across all swarm nodes..."

docker exec "$MANAGER_NODE" docker service create \
    --name "$AGENT_SERVICE_NAME" \
    --mode global \
    --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
    --env NODE_HOSTNAME='{{.Node.Hostname}}' \
    --env NODE_ID='{{.Node.ID}}' \
    "$AGENT_IMAGE" \
    sh -c "echo 'Pulse Agent active on node: \$NODE_HOSTNAME (\$NODE_ID)'; sleep infinity" >/dev/null

echo "⏳ Waiting for global agent replicas to schedule..."
sleep 3

# 7. Print Deployment Results
echo ""
echo "================ Global Agent Status ================"
docker exec "$MANAGER_NODE" docker service ps "$AGENT_SERVICE_NAME"
echo "====================================================="
echo ""
echo "✅ Setup complete! Your 3-node Swarm environment is ready for testing."
