# Running Pulse on a server

Run the published images with Docker Compose: no source checkout, no build.
Pulse then manages the server's own containers, and any other servers or
clusters you connect to it.

**Images:** [`beaconengine/pulse-gateway`](https://hub.docker.com/r/beaconengine/pulse-gateway)
and [`beaconengine/pulse-agent`](https://hub.docker.com/r/beaconengine/pulse-agent) on
Docker Hub, tagged by version (`0.1.0`, `0.1`), `latest`, and `sha-<commit>`.
They are built for **linux/amd64 only**, so an ARM server (AWS Graviton,
Ampere, Raspberry Pi) can't run them yet.

## 1. Start it

On a server with Docker and the Compose plugin:

```bash
mkdir pulse && cd pulse
curl -fsSLO https://raw.githubusercontent.com/beacon-engine/pulse/master/deploy/docker/docker-compose.yml
echo "PULSE_VERSION=0.1.0" > .env      # pin a version; without it you get latest
docker compose up -d
```

Open `http://<server>:9080` and create the admin account. Do this straight
away: until an admin exists, whoever reaches the page first can create one.

What's running (see [`deploy/docker/docker-compose.yml`](../deploy/docker/docker-compose.yml)):

- **agent**: manages this server's Docker through `/var/run/docker.sock`, and
  reads the host's disks through a read-only `/:/host` mount. It has no
  published port; only the gateway reaches it.
- **gateway**: the dashboard and API on port 9080, with its database in the
  `pulse-data` volume.

## 2. Settings

Put them in `.env` next to the compose file, then `docker compose up -d` again.

| Variable | Default | What it does |
|---|---|---|
| `PULSE_VERSION` | `latest` | Image tag to run |
| `PULSE_HTTP_PORT` | `9080` | Port the dashboard and API are published on |
| `PULSE_AGENT_TOKENS` | (none) | Push agents allowed to connect, `name:secret` pairs (step 4) |
| `SLACK_WEBHOOK_URL` | (none) | Slack alerts for container crashes and agents going down |

Every variable the gateway and agent understand is in
[configuration.md](configuration.md).

## 3. Put it behind HTTPS

Do this before anyone logs in over the internet: without TLS, passwords, the
session cookie and agent tokens travel in plain text, and remote agents need
`wss://` to connect safely.

The simplest route is a reverse proxy that gets a certificate for you. With
[Caddy](https://caddyserver.com), on the same server, for a DNS name pointing at
it:

```bash
caddy reverse-proxy --from pulse.example.com --to localhost:9080
```

WebSockets (the live dashboard and push agents) pass through as is. Once the
proxy works, set `PULSE_HTTP_PORT=127.0.0.1:9080` so the plain-HTTP port is only
reachable from the server itself.

Alternatively the gateway can serve TLS itself: in the compose file, mount the
certificate and key into the gateway service and set `PULSE_TLS_CERT` and
`PULSE_TLS_KEY` to their paths inside the container.

The session cookie isn't marked `Secure` yet ([security.md](security.md)).

## 4. Connect more servers

Each additional Docker host runs only an agent, which dials out to the gateway.
It needs no open ports.

On the gateway's server, allow agents in with a token:

```bash
echo "PULSE_AGENT_TOKENS=servers:$(openssl rand -hex 32)" >> .env
docker compose up -d
grep PULSE_AGENT_TOKENS .env   # the part after "servers:" is the token
```

On each other server:

```bash
docker run -d --name pulse-agent --restart unless-stopped \
  --label pulse/role=agent \
  -v /var/run/docker.sock:/var/run/docker.sock -v /:/host:ro \
  -e HOST_ROOT=/host \
  -e PULSE_MODE=dial \
  -e PULSE_GATEWAY_WS_URL=wss://pulse.example.com/agent/ws \
  -e PULSE_TOKEN=<the token> \
  -e PULSE_ENV_NAME=web-1 \
  beaconengine/pulse-agent:0.1.0
```

It appears in the environment selector as `web-1` within a few seconds. Every
server can share one token; each still shows up as its own environment, keyed by
its Docker daemon's id, so reinstalling the agent keeps the same environment.
`docker logs pulse-agent` should say `Uplink connected to gateway`.

To manage a Kubernetes cluster, run an agent inside it instead:
[deploy-kubernetes.md](deploy-kubernetes.md).

## 5. Upgrade

```bash
sed -i 's/^PULSE_VERSION=.*/PULSE_VERSION=0.2.0/' .env
docker compose pull && docker compose up -d
```

The gateway migrates its database on start. Upgrade remote agents the same way
(re-run `docker run` with the new tag), ideally after the gateway.

**From an older gateway image that ran as root**: the gateway now runs as uid
10001. Hand the data volume over once, or it can't write its database:

```bash
docker run --rm -v pulse_pulse-data:/data debian:bookworm-slim chown -R 10001:10001 /data
```

(The volume is `<project>_pulse-data`; `docker volume ls` shows the name.)

## 6. Back up

Everything lives in one SQLite file in the `pulse-data` volume. Copy it while
the gateway is stopped:

```bash
docker compose stop gateway
docker run --rm -v pulse_pulse-data:/data -v "$PWD":/backup debian:bookworm-slim \
  cp /data/pulse.db /backup/pulse-$(date +%F).db
docker compose start gateway
```

## Host metrics

The **Metrics** page shows the machine the agent runs on:

- **CPU** comes from the Docker daemon, so it's always the host's.
- **Memory** comes from `/proc/meminfo`, which isn't namespaced, so it's the
  host's too.
- **Disks** need the read-only `/:/host` mount and `HOST_ROOT=/host` (both in the
  compose file and the `docker run` above). Without them the agent reports no
  disks, rather than the container's own overlay filesystem.

| Where the agent runs | Host metrics |
|---|---|
| A Linux server, in a container with the `/host` mount | The real host |
| Directly on the machine (a binary, not a container) | The real host, on Linux, macOS or Windows |
| Docker Desktop (macOS, Windows) | Docker's Linux VM, not your computer |
| Windows containers | Not supported: the agent is a Linux binary |

Managing containers works everywhere. Only the hardware numbers depend on where
the agent runs.
