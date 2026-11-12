package agent

import (
	"context"

	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/agent/docker"
	"github.com/beacon-engine/pulse/internal/agent/hostmetrics"
	"github.com/beacon-engine/pulse/internal/agent/kubernetes"
	"github.com/beacon-engine/pulse/internal/agent/swarm"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// Dispatch executes a single request and produces its response. It is
// transport-agnostic: the TCP listener (pull) and the WS uplink (push) both call
// it. The work itself lives in the runtime packages (docker, swarm, kubernetes,
// hostmetrics); this switch only routes each request and shapes the response.
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
			v, err := docker.List(ctx, cli, st.Self)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespContainers(v)
		})
	case protocol.ReqInspectContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.Inspect(ctx, cli, st.Self, req.ID)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespContainerDetail(v)
		})
	case protocol.ReqInspectRawContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.InspectRaw(ctx, cli, req.ID)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespRaw(v)
		})
	case protocol.ReqRecreateContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(docker.Recreate(ctx, cli, req.ID, req.Ports))
		})
	case protocol.ReqRecreateLatest:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(docker.RecreateLatest(ctx, cli, req.ID, req.Pull))
		})
	case protocol.ReqUpdateContainerEnv:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardReplace(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return idOrErr(docker.UpdateEnv(ctx, cli, req.ID, req.Env))
		})
	case protocol.ReqExecContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			out, err := docker.Exec(ctx, cli, req.ID, req.Cmd)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespText(out)
		})
	case protocol.ReqStartContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(docker.Start(ctx, cli, req.ID))
		})
	case protocol.ReqStopContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return okOrErr(docker.Stop(ctx, cli, req.ID))
		})
	case protocol.ReqRestartContainer:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardProtected(ctx, cli, st.Self, req.ID); !ok {
				return resp
			}
			return okOrErr(docker.Restart(ctx, cli, req.ID))
		})
	case protocol.ReqContainerLogs:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			out, err := docker.Logs(ctx, cli, req.ID, req.Tail)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespText(out)
		})
	case protocol.ReqContainerNetworks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.ContainerNetworks(ctx, cli, req.ID)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespContainerNetworks(v)
		})

	// ── System / hardware ────────────────────────────────────────────────────────
	case protocol.ReqSystemInfo:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.SystemInfo(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespSystemInfo(v)
		})
	case protocol.ReqHardwareMetrics:
		return protocol.RespHardwareMetrics(hostmetrics.Collect(st.Cfg.HostRoot))

	// ── Images ────────────────────────────────────────────────────────────────────
	case protocol.ReqListImages:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.ListImages(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespImages(v)
		})
	case protocol.ReqDeleteImage:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(docker.DeleteImage(ctx, cli, req.ID))
		})
	case protocol.ReqPruneImages:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return countOrErr(docker.PruneImages(ctx, cli))
		})

	// ── Volumes ─────────────────────────────────────────────────────────────────
	case protocol.ReqListVolumes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.ListVolumes(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespVolumes(v)
		})
	case protocol.ReqDeleteVolume:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(docker.DeleteVolume(ctx, cli, req.Name))
		})
	case protocol.ReqPruneVolumes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return countOrErr(docker.PruneVolumes(ctx, cli))
		})

	// ── Networks ──────────────────────────────────────────────────────────────────
	case protocol.ReqListHostNetworks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.ListNetworks(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespHostNetworks(v)
		})
	case protocol.ReqNetworkConnect:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(docker.ConnectNetwork(ctx, cli, req.NetworkID, req.ContainerID))
		})
	case protocol.ReqNetworkDisconnect:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			if resp, ok := docker.GuardProtected(ctx, cli, st.Self, req.ContainerID); !ok {
				return resp
			}
			return okOrErr(docker.DisconnectNetwork(ctx, cli, req.NetworkID, req.ContainerID))
		})

	// ── Detected stacks ───────────────────────────────────────────────────────────
	case protocol.ReqListDetectedStacks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := docker.ListStacks(ctx, cli, st.Self)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespDetectedStacks(v)
		})

	// ── Compose ───────────────────────────────────────────────────────────────────
	case protocol.ReqComposeUp:
		return textOrErr(docker.ComposeUp(ctx, req.DockerHost, req.Stack))
	case protocol.ReqComposeDown:
		return textOrErr(docker.ComposeDown(ctx, req.DockerHost, req.Stack))
	case protocol.ReqComposePull:
		return textOrErr(docker.ComposePull(ctx, req.DockerHost, req.Stack))

	// ── Swarm ─────────────────────────────────────────────────────────────────────
	case protocol.ReqListSwarmServices:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarm.Services(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespSwarmServices(v)
		})
	case protocol.ReqListSwarmNodes:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarm.Nodes(ctx, cli)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespSwarmNodes(v)
		})
	case protocol.ReqListSwarmTasks:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			v, err := swarm.Tasks(ctx, cli, req.ServiceID)
			if err != nil {
				return docker.ExecError(err)
			}
			return protocol.RespSwarmTasks(v)
		})
	case protocol.ReqUpdateSwarmService:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(swarm.ForceUpdate(ctx, cli, req.ServiceID))
		})
	case protocol.ReqScaleSwarmService:
		return dockerOp(st, req.DockerHost, func(cli *client.Client) protocol.Response {
			return okOrErr(swarm.Scale(ctx, cli, req.ServiceID, req.TargetReplicas))
		})

	// ── Kubernetes ──────────────────────────────────────────────────────────────
	case protocol.ReqK8sPods:
		return kubernetes.Pods(ctx, st.K8s, st.Self)
	case protocol.ReqK8sNamespaces:
		return kubernetes.Namespaces(ctx, st.K8s)
	case protocol.ReqK8sRollout:
		return kubernetes.Rollout(ctx, st.K8s, req.Namespace, req.Deployment)
	case protocol.ReqK8sDeletePod:
		return kubernetes.DeletePod(ctx, st.K8s, st.Self, req.Namespace, req.Name)
	case protocol.ReqK8sWorkloads:
		return kubernetes.Workloads(ctx, st.K8s)
	case protocol.ReqK8sPodLogs:
		return kubernetes.PodLogs(ctx, st.K8s, req.Namespace, req.Name, req.Container, req.Tail)
	case protocol.ReqK8sScale:
		return kubernetes.Scale(ctx, st.K8s, req.Namespace, req.WorkloadKind, req.Name, req.TargetReplicas)
	case protocol.ReqK8sRestart:
		return kubernetes.Restart(ctx, st.K8s, req.Namespace, req.WorkloadKind, req.Name)
	case protocol.ReqK8sNodes:
		return kubernetes.Nodes(ctx, st.K8s)
	case protocol.ReqK8sServices:
		return kubernetes.Services(ctx, st.K8s)
	case protocol.ReqK8sEvents:
		return kubernetes.Events(ctx, st.K8s)

	// ── Event stream ────────────────────────────────────────────────────────────
	// WatchEvents takes over a dedicated connection (handled before Dispatch in
	// the TCP server); reaching here means it was multiplexed, which we reject.
	case protocol.ReqWatchEvents:
		return protocol.RespError(400, "WatchEvents must be sent on a dedicated connection")

	default:
		return protocol.RespError(400, "unknown request: "+req.Kind)
	}
}
