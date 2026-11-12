# HTTP API

Everything the dashboard does goes through this API, served by the gateway.

**Authentication:** log in with `POST /api/auth/login` (`{"username", "password"}`)
and send back the `pulse_session` cookie it sets. Every `/api/*` route needs it,
except `/api/status` and the two auth routes marked public below. Requests that
change something need the `admin` role; viewers get `403`.

`{id}` and `{host_id}` are an environment's id (`GET /api/hosts` lists them).

## Account and users

| Method | Path | Description |
|---|---|---|
| GET / POST | `/api/auth/setup` | Whether first-run setup is needed / create the first admin (public) |
| POST | `/api/auth/login` · `/api/auth/logout` | Start / end a session (login is public) |
| GET | `/api/auth/me` | The signed-in user |
| GET / POST | `/api/users` | List users / create one (admin) |
| PUT / DELETE | `/api/users/{id}` | Change a user's role or password / delete them (admin) |

## Environments

| Method | Path | Description |
|---|---|---|
| GET / POST | `/api/hosts` | List environments (`?search=&page=&per_page=`) / add one by agent address or Docker endpoint |
| GET / PUT / DELETE | `/api/hosts/{id}` | One environment / rename / remove |
| GET | `/api/hosts/{id}/capabilities` | Whether its agent answers, and which runtimes it reaches (Docker, Kubernetes, …) |
| GET | `/api/agents` | Connected push agents |
| GET / PUT | `/api/settings` | Agent liveness settings (PUT: admin); see [configuration.md](configuration.md#runtime-settings) |

## Docker

| Method | Path | Description |
|---|---|---|
| GET | `/api/hosts/{id}/containers` | Containers |
| GET | `/api/hosts/{id}/containers/{cid}/inspect` · `/raw` | Details / the raw Docker inspect output |
| POST | `/api/hosts/{id}/containers/{cid}/start` · `/stop` · `/restart` | Lifecycle |
| POST | `/api/hosts/{id}/containers/{cid}/recreate` · `/env` | Recreate the container / recreate it with new environment variables |
| POST | `/api/hosts/{id}/containers/{cid}/exec` | Run a command |
| GET | `/api/hosts/{id}/containers/{cid}/logs` | Logs |
| GET / POST | `/api/hosts/{id}/containers/{cid}/networks` · `/networks/connect` · `/networks/disconnect` | Networks |
| GET | `/api/hosts/{id}/images` · `/volumes` · `/networks` | Images, volumes, networks |
| POST | `/api/hosts/{id}/images/prune` · `/volumes/prune` | Prune unused |
| DELETE | `/api/hosts/{id}/images/{image_id}` · `/volumes/{vol_name}` | Delete one |
| GET | `/api/hosts/{id}/system` · `/hardware` | Docker info / CPU, memory and disks |

## Compose stacks

| Method | Path | Description |
|---|---|---|
| GET / POST | `/api/hosts/{host_id}/stacks` | Saved stacks / save a new one |
| GET | `/api/hosts/{host_id}/stacks/detected` | Compose projects found running on the host |
| GET / PUT / DELETE | `/api/stacks/{stack_id}` | One stack / update its compose file / delete it |
| POST | `/api/stacks/{stack_id}/up` · `/down` | Deploy / take down |
| POST / DELETE | `/api/stacks/{stack_id}/token` | Enable / disable its redeploy webhook |

## Docker Swarm

| Method | Path | Description |
|---|---|---|
| GET | `/api/hosts/{host_id}/swarm/services` · `/nodes` | Services, nodes |
| GET | `/api/hosts/{host_id}/swarm/services/{service_id}/tasks` | A service's tasks |
| POST | `/api/hosts/{host_id}/swarm/services/{service_id}/update` | Force a rolling update |
| POST | `/api/hosts/{host_id}/swarm/services/{service_id}/scale` | Scale |

## Kubernetes

| Method | Path | Description |
|---|---|---|
| GET | `/api/hosts/{id}/k8s/workloads` · `/pods` · `/namespaces` · `/services` · `/nodes` · `/events` | Cluster objects |
| GET | `/api/hosts/{id}/k8s/pods/{namespace}/{name}/logs?container=&tail=` | A container's logs |
| DELETE | `/api/hosts/{id}/k8s/pods/{namespace}/{name}` | Delete a pod (its controller replaces it) |
| POST | `/api/hosts/{id}/k8s/workloads/{namespace}/{kind}/{name}/scale?replicas=N` | Scale a Deployment or StatefulSet |
| POST | `/api/hosts/{id}/k8s/workloads/{namespace}/{kind}/{name}/restart` | Rolling restart |

## Webhooks

Trigger a redeploy from CI. Enable a webhook to get its token, then call the
action with `?token=<token>`. Action calls also need a signed-in session.

| Method | Path | Description |
|---|---|---|
| GET / POST / DELETE | `/api/webhooks/{host_id}/{entity_type}/{entity_key}/token` | Show / create / revoke a webhook's token |
| POST | `/api/webhooks/{host_id}/containers/{name}/deploy` | Pull and recreate a container by name |
| POST | `/api/webhooks/{host_id}/{container_id}/recreate` | Recreate a container |
| POST | `/api/webhooks/{host_id}/stacks/{stack_id}/redeploy` | Redeploy a stack |
| POST | `/api/webhooks/{host_id}/swarm/{service_id}/update` · `/scale` | Update / scale a Swarm service |
| POST | `/api/webhooks/{host_id}/k8s/{namespace}/{deployment}/rollout` | Rolling restart of a Deployment |

## Events and monitoring

| Method | Path | Description |
|---|---|---|
| GET | `/api/events` | Recent container lifecycle events |
| GET | `/ws` | Live events over WebSocket (needs a session) |
| GET | `/api/prometheus/query?url=&q=` | Run a PromQL query against the Prometheus at `url` |
| GET | `/metrics` | The gateway's own Prometheus metrics (public) |
| GET | `/health` · `/api/status` | Liveness / status (public) |
