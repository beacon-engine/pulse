# Pulse

One dashboard for your Docker hosts, Swarm clusters and Kubernetes clusters.
A lightweight alternative to Portainer, written in Go.

A **gateway** serves the dashboard and API. An **agent** runs next to each
server or cluster and dials out to the gateway, so nothing needs open ports.
Each one uses about 20 MB of memory.

## Run it

Images: [beaconengine/pulse-gateway](https://hub.docker.com/r/beaconengine/pulse-gateway)
and [beaconengine/pulse-agent](https://hub.docker.com/r/beaconengine/pulse-agent)
(linux/amd64).

```bash
curl -fsSLO https://raw.githubusercontent.com/beacon-engine/pulse/master/deploy/docker/docker-compose.yml
docker compose up -d
```

Open `http://<server>:9080` and create the admin account.

## Next

- [Run it on a server](docs/deploy-docker.md): HTTPS, more servers, upgrades
- [Manage a Kubernetes cluster](docs/deploy-kubernetes.md)
- [Develop locally](docs/local-development.md)
- [All documentation](docs/README.md)

## License

MIT.
