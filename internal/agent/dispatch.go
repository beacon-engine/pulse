package agent

import (
	"context"

	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Dispatch executes a single request and produces its response. It is
// transport-agnostic: the TCP listener (pull) and the WS uplink (push) both call
// it. Handlers are filled in file by file (docker.go, swarm.go, kubernetes.go,
// …); until then they return a 501 so the gateway sees a clean "not implemented"
// rather than a dropped connection.
func Dispatch(ctx context.Context, st *State, req protocol.Request) protocol.Response {
	switch req.Kind {
	// ── Liveness ───────────────────────────────────────────────────────────────
	case protocol.ReqPing:
		return protocol.RespPong()
	case protocol.ReqCapabilities:
		return protocol.RespCapabilities(capabilities(ctx, st, req.DockerHost))

	// ── Containers ──────────────────────────────────────────────────────────────
	case protocol.ReqListContainers:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerList(ctx, cli, st.Self)
			if err != nil {
				return execError(err)
			}
			return protocol.RespContainers(v)
		})
	case protocol.ReqInspectContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerInspect(ctx, cli, st.Self, req.ID)
			if err != nil {
				return execError(err)
			}
			return protocol.RespContainerDetail(v)
		})
	case protocol.ReqInspectRawContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerInspectRaw(ctx, cli, req.ID)
			if err != nil {
				return execError(err)
			}
			return protocol.RespRaw(v)
		})
	case protocol.ReqRecreateContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(dockerRecreate(ctx, cli, req.ID, req.Ports))
		})
	case protocol.ReqRecreateLatest:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(dockerRecreateLatest(ctx, cli, req.ID, req.Pull))
		})
	case protocol.ReqUpdateContainerEnv:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(dockerUpdateEnv(ctx, cli, req.ID, req.Env))
		})
	case protocol.ReqExecContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			out, err := dockerExec(ctx, cli, req.ID, req.Cmd)
			if err != nil {
				return execError(err)
			}
			return protocol.RespText(out)
		})
	case protocol.ReqStartContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(dockerStart(ctx, cli, req.ID))
		})
	case protocol.ReqStopContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return okOrErr(dockerStop(ctx, cli, req.ID))
		})
	case protocol.ReqRestartContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return okOrErr(dockerRestart(ctx, cli, req.ID))
		})
	case protocol.ReqContainerLogs:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			out, err := dockerLogs(ctx, cli, req.ID, req.Tail)
			if err != nil {
				return execError(err)
			}
			return protocol.RespText(out)
		})
	case protocol.ReqContainerNetworks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerContainerNetworks(ctx, cli, req.ID)
			if err != nil {
				return execError(err)
			}
			return protocol.RespContainerNetworks(v)
		})

	// ── System / hardware ────────────────────────────────────────────────────────
	case protocol.ReqSystemInfo:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerSystemInfo(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespSystemInfo(v)
		})
	case protocol.ReqHardwareMetrics:
		return protocol.RespHardwareMetrics(hardwareMetrics(st.Cfg.HostRoot))

	// ── Images ────────────────────────────────────────────────────────────────────
	case protocol.ReqListImages:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerListImages(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespImages(v)
		})
	case protocol.ReqDeleteImage:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(dockerDeleteImage(ctx, cli, req.ID))
		})
	case protocol.ReqPruneImages:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return countOrErr(dockerPruneImages(ctx, cli))
		})

	// ── Volumes ─────────────────────────────────────────────────────────────────
	case protocol.ReqListVolumes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerListVolumes(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespVolumes(v)
		})
	case protocol.ReqDeleteVolume:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(dockerDeleteVolume(ctx, cli, req.Name))
		})
	case protocol.ReqPruneVolumes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return countOrErr(dockerPruneVolumes(ctx, cli))
		})

	// ── Networks ──────────────────────────────────────────────────────────────────
	case protocol.ReqListHostNetworks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerListHostNetworks(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespHostNetworks(v)
		})
	case protocol.ReqNetworkConnect:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(dockerConnectNetwork(ctx, cli, req.NetworkID, req.ContainerID))
		})
	case protocol.ReqNetworkDisconnect:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := guardProtected(ctx, cli, st.Self, req.ContainerID); !ok {
				return resp
			}
			return okOrErr(dockerDisconnectNetwork(ctx, cli, req.NetworkID, req.ContainerID))
		})

	// ── Detected stacks ───────────────────────────────────────────────────────────
	case protocol.ReqListDetectedStacks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := dockerListStacks(ctx, cli, st.Self)
			if err != nil {
				return execError(err)
			}
			return protocol.RespDetectedStacks(v)
		})

	// ── Compose ───────────────────────────────────────────────────────────────────
	case protocol.ReqComposeUp:
		return composeText(composeUp(ctx, req.DockerHost, req.Stack))
	case protocol.ReqComposeDown:
		return composeText(composeDown(ctx, req.DockerHost, req.Stack))
	case protocol.ReqComposePull:
		return composeText(composePull(ctx, req.DockerHost, req.Stack))

	// ── Swarm ─────────────────────────────────────────────────────────────────────
	case protocol.ReqListSwarmServices:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarmServices(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespSwarmServices(v)
		})
	case protocol.ReqListSwarmNodes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarmNodes(ctx, cli)
			if err != nil {
				return execError(err)
			}
			return protocol.RespSwarmNodes(v)
		})
	case protocol.ReqListSwarmTasks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarmTasks(ctx, cli, req.ServiceID)
			if err != nil {
				return execError(err)
			}
			return protocol.RespSwarmTasks(v)
		})
	case protocol.ReqUpdateSwarmService:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(swarmForceUpdate(ctx, cli, req.ServiceID))
		})
	case protocol.ReqScaleSwarmService:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(swarmScale(ctx, cli, req.ServiceID, req.TargetReplicas))
		})

	// ── Kubernetes ──────────────────────────────────────────────────────────────
	case protocol.ReqK8sPods:
		return handleK8sPods(ctx, st)
	case protocol.ReqK8sNamespaces:
		return handleK8sNamespaces(ctx, st)
	case protocol.ReqK8sRollout:
		return handleK8sRollout(ctx, st, req.Namespace, req.Deployment)
	case protocol.ReqK8sDeletePod:
		return handleK8sDeletePod(ctx, st, req.Namespace, req.Name)
	case protocol.ReqK8sWorkloads:
		return handleK8sWorkloads(ctx, st)
	case protocol.ReqK8sPodLogs:
		return handleK8sPodLogs(ctx, st, req.Namespace, req.Name, req.Container, req.Tail)
	case protocol.ReqK8sScale:
		return handleK8sScale(ctx, st, req.Namespace, req.WorkloadKind, req.Name, req.TargetReplicas)
	case protocol.ReqK8sRestart:
		return handleK8sRestart(ctx, st, req.Namespace, req.WorkloadKind, req.Name)
	case protocol.ReqK8sNodes:
		return handleK8sNodes(ctx, st)
	case protocol.ReqK8sServices:
		return handleK8sServices(ctx, st)
	case protocol.ReqK8sEvents:
		return handleK8sEvents(ctx, st)

	// ── Event stream ────────────────────────────────────────────────────────────
	// WatchEvents takes over a dedicated connection (handled before Dispatch in
	// the TCP server); reaching here means it was multiplexed, which we reject.
	case protocol.ReqWatchEvents:
		return protocol.RespError(400, "WatchEvents must be sent on a dedicated connection")

	default:
		return protocol.RespError(400, "unknown request: "+req.Kind)
	}
}
