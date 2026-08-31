#!/usr/bin/env bash
echo "🧹 Cleaning up DinD Swarm cluster..."
docker rm -f pulse-manager pulse-worker-1 pulse-worker-2 >/dev/null 2>&1 || true
docker network rm pulse-dind-net >/dev/null 2>&1 || true
echo "✅ Cleaned up successfully."
