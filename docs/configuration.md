# Configuration

Pulse runs with almost no configuration. Its own settings use the `PULSE_`
prefix; a few gateway settings use plain names.

## Gateway

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | Port for the dashboard, the API and `/agent/ws` |
| `AGENT_ADDR` | `127.0.0.1:9000` (`agent:9000` in the image) | The default **pull** agent, `host:port`: the one that manages the gateway's own server |
| `DATABASE_URL` | `sqlite://pulse.db` (`sqlite:///data/pulse.db` in the image) | `sqlite://<path>` or `sqlite::memory:` |
| `STATIC_DIR` | `./frontend/dist` (`/app/dist` in the image) | The built dashboard to serve |
| `SLACK_WEBHOOK_URL` | (none) | Slack incoming webhook for container lifecycle alerts and agents going down |
| `PULSE_AGENT_TOKENS` | (none) | Push agents allowed to connect: comma-separated `name:token` pairs, e.g. `prod:s3cret,staging:h0nk`. Empty disables push. Many agents may share a token; each still registers as its own environment. |
| `PULSE_TLS_CERT` / `PULSE_TLS_KEY` | (none) | PEM certificate and key. With both set the gateway serves HTTPS/WSS itself; otherwise put it behind a TLS proxy. |
| `PULSE_DB_RESET_ON_MISMATCH` | `false` | Development only: recreate the database when its migrations don't match the binary, instead of refusing to start. **Deletes all data.** |

## Agent

| Variable | Default | Description |
|---|---|---|
| `PULSE_MODE` | `listen` | `dial` (or `push`): the agent connects out to the gateway. `listen` (or `pull`): the agent waits for the gateway to connect. |
| `PULSE_GATEWAY_WS_URL` | (none) | Where to dial in `dial` mode, e.g. `wss://pulse.example.com/agent/ws`. Required for `dial`. |
| `PULSE_TOKEN` | (none) | Token sent when dialing; must match a `PULSE_AGENT_TOKENS` entry |
| `PULSE_PORT` | `9000` | Port to listen on in `listen` mode |
| `PULSE_TRANSPORT` | `tcp` | `listen`-mode protocol: `tcp` (framed TCP) or `ws` (WebSocket over HTTP, for paths that only pass HTTP) |
| `PULSE_INSECURE_SKIP_TLS_VERIFY` | `false` | Don't verify the gateway's certificate when dialing `wss://`. Trusted networks and testing only. |
| `HOST_ROOT` | (none) | Where the host's filesystem is mounted inside the container (`/host` with `-v /:/host:ro`), so disk metrics show the host's disks |
| `PULSE_KUBECONFIG_PATH` | (none) | A kubeconfig file to use instead of the in-cluster ServiceAccount |
| `PULSE_KUBECONFIG` | (none) | Inline kubeconfig YAML (ignored when `PULSE_KUBECONFIG_PATH` is set) |

### Self-registration (dial mode)

| Variable | Default | Description |
|---|---|---|
| `PULSE_AUTO_REGISTER` | `on` | Register as an environment on connect. `off` is the older setup, where an admin creates the environment first and links it to the token's name. |
| `PULSE_ENV_NAME` | the hostname | The environment's name in Pulse. Set it in Kubernetes, where the pod's hostname changes on every restart. |
| `PULSE_INSTANCE_ID` | derived | Overrides the agent's identity. Normally left unset: see [architecture.md](architecture.md#identity). |
| `PULSE_DOCKER_HOST` | `unix:///var/run/docker.sock` | The Docker endpoint the registered environment uses |
| `PULSE_HEARTBEAT_INTERVAL` | `30s` | How often the agent reports in. A Go duration (`45s`, `2m`) or seconds (`30`); at least `5s`. |

## Runtime settings

These live in the gateway's database rather than the environment, and can be
read with `GET /api/settings` and changed while it runs with `PUT /api/settings` (admin only):

| Key | Default | Meaning |
|---|---|---|
| `agent_offline_threshold_seconds` | `90` | A self-registered agent with no heartbeat for this long is marked offline |
| `agent_alert_grace_seconds` | `60` | How long an agent must stay down, after an abrupt disconnect, before the Slack alert |
| `agent_retention_hours` | `6` | Self-registered environments offline this long are deleted. Environments you added by hand never are. |
