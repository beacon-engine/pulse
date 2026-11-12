package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

func ExecError(err error) protocol.Response {
	if client.IsErrConnectionFailed(err) {
		return protocol.RespError(503, "no container runtime (Docker/Podman API) reachable from this agent: "+err.Error())
	}
	return protocol.RespError(500, err.Error())
}

// GuardProtected refuses a disruptive action on one of Pulse's own containers.
// It resolves id (a name or short id) through inspect so it can't be sidestepped
// by addressing the container differently. ok=false → return resp as-is.
func GuardProtected(ctx context.Context, cli *client.Client, self selfguard.Self, id string) (resp protocol.Response, ok bool) {
	return guardContainer(ctx, cli, self, id, false)
}

// GuardReplace is GuardProtected for actions that replace the container
// (recreate, env update). Those are also refused for kubelet-managed
// containers: the copy would run outside Kubernetes while the kubelet starts
// yet another one for the pod.
func GuardReplace(ctx context.Context, cli *client.Client, self selfguard.Self, id string) (resp protocol.Response, ok bool) {
	return guardContainer(ctx, cli, self, id, true)
}

func guardContainer(ctx context.Context, cli *client.Client, self selfguard.Self, id string, replace bool) (protocol.Response, bool) {
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return ExecError(err), false
	}
	info := res.Container
	var labels map[string]string
	if info.Config != nil {
		labels = info.Config.Labels
	}
	name := strings.TrimPrefix(info.Name, "/")
	if reason := self.ContainerProtection(info.ID, labels); reason != "" {
		return selfguard.ProtectedError(name, reason), false
	}
	if ref := selfguard.KubeletRef(labels); replace && ref != nil {
		return protocol.RespError(409, fmt.Sprintf(
			"%s is managed by Kubernetes (pod %s/%s): change its pod spec or restart its workload instead; "+
				"recreating it with Docker would leave a copy running outside the cluster",
			name, ref.Namespace, ref.Pod)), false
	}
	return protocol.Response{}, true
}
