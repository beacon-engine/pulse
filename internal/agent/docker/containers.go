// Package docker runs the agent's Docker operations: containers, images,
// volumes, networks, Compose stacks and the event stream, through the Docker
// Engine API (which Podman also serves).

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/docker.rs — Docker operations via the moby client.

// ── List ─────────────────────────────────────────────────────────────────────

func List(ctx context.Context, cli *client.Client, self selfguard.Self) ([]protocol.ContainerInfo, error) {
	f := client.Filters{}.Add("status", "running", "exited", "paused", "created")
	list, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ContainerInfo, 0, len(list.Items))
	for _, c := range list.Items {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		reason := self.ContainerProtection(c.ID, c.Labels)
		result = append(result, protocol.ContainerInfo{
			ID:              c.ID,
			Name:            name,
			Image:           c.Image,
			Status:          c.Status,
			State:           string(c.State),
			Stack:           containerStack(c.Labels),
			Protected:       reason != "",
			ProtectedReason: reason,
			Kubernetes:      selfguard.KubeletRef(c.Labels),
		})
	}
	// Stable ordering so 10s UI refreshes don't reshuffle rows.
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// containerStack resolves the compose/stack project label, if any.
func containerStack(labels map[string]string) *string {
	if v, ok := labels["com.docker.compose.project"]; ok {
		return &v
	}
	if v, ok := labels["com.docker.stack.namespace"]; ok {
		return &v
	}
	return nil
}

// ── Inspect ──────────────────────────────────────────────────────────────────

func Inspect(ctx context.Context, cli *client.Client, self selfguard.Self, id string) (protocol.ContainerDetail, error) {
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return protocol.ContainerDetail{}, err
	}
	info := res.Container

	ports := []protocol.ContainerPort{}
	if info.NetworkSettings != nil {
		for p, bindings := range info.NetworkSettings.Ports {
			for _, b := range bindings {
				hostIP := "0.0.0.0"
				if b.HostIP.IsValid() {
					hostIP = b.HostIP.String()
				}
				ports = append(ports, protocol.ContainerPort{
					ContainerPort: p.Port(),
					Protocol:      string(p.Proto()),
					HostIP:        hostIP,
					HostPort:      b.HostPort,
				})
			}
		}
		sort.Slice(ports, func(i, j int) bool { return ports[i].ContainerPort < ports[j].ContainerPort })
	}

	// Empty lists, not null: the dashboard reads their length.
	mounts := []protocol.ContainerMount{}
	for _, m := range info.Mounts {
		mounts = append(mounts, protocol.ContainerMount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}

	detail := protocol.ContainerDetail{
		ID:           info.ID,
		Name:         strings.TrimPrefix(info.Name, "/"),
		Created:      info.Created,
		RestartCount: int64(info.RestartCount),
		Ports:        ports,
		Mounts:       mounts,
		Env:          []string{},
		Cmd:          []string{},
	}
	var labels map[string]string
	if info.Config != nil {
		detail.Image = info.Config.Image
		detail.Env = append(detail.Env, info.Config.Env...)
		detail.Cmd = append(detail.Cmd, info.Config.Cmd...)
		labels = info.Config.Labels
	}
	detail.ProtectedReason = self.ContainerProtection(info.ID, labels)
	detail.Protected = detail.ProtectedReason != ""
	detail.Kubernetes = selfguard.KubeletRef(labels)
	if info.State != nil {
		detail.State = string(info.State.Status)
		detail.StartedAt = info.State.StartedAt
		detail.FinishedAt = info.State.FinishedAt
	}
	if info.HostConfig != nil {
		detail.NetworkMode = string(info.HostConfig.NetworkMode)
		detail.RestartPolicy = string(info.HostConfig.RestartPolicy.Name)
	}
	return detail, nil
}

func InspectRaw(ctx context.Context, cli *client.Client, id string) (json.RawMessage, error) {
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(res.Container)
}

// ── Recreate variants ────────────────────────────────────────────────────────

func Recreate(ctx context.Context, cli *client.Client, id string, newPorts []protocol.PortMap) (string, error) {
	bindings := network.PortMap{}
	for _, pm := range newPorts {
		key, err := network.ParsePort(fmt.Sprintf("%s/%s", pm.ContainerPort, pm.Protocol))
		if err != nil {
			return "", err
		}
		bindings[key] = []network.PortBinding{{HostIP: netip.IPv4Unspecified(), HostPort: pm.HostPort}}
	}
	return recreateWith(ctx, cli, id, func(_ *container.Config, hc *container.HostConfig) {
		hc.PortBindings = bindings
	}, false)
}

func RecreateLatest(ctx context.Context, cli *client.Client, id string, pull bool) (string, error) {
	return recreateWith(ctx, cli, id, nil, pull)
}

func UpdateEnv(ctx context.Context, cli *client.Client, id string, env []string) (string, error) {
	return recreateWith(ctx, cli, id, func(cfg *container.Config, _ *container.HostConfig) {
		cfg.Env = env
	}, false)
}

// ── Logs / exec ──────────────────────────────────────────────────────────────

func Logs(ctx context.Context, cli *client.Client, id string, tail uint32) (string, error) {
	rc, err := cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.FormatUint(uint64(tail), 10),
		Timestamps: true,
	})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func Exec(ctx context.Context, cli *client.Client, id string, cmd []string) (string, error) {
	exec, err := cli.ExecCreate(ctx, id, client.ExecCreateOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}
	attach, err := cli.ExecAttach(ctx, exec.ID, client.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attach.Close()
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, attach.Reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ── Basic actions ────────────────────────────────────────────────────────────

func Start(ctx context.Context, cli *client.Client, id string) error {
	_, err := cli.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

func Stop(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	_, err := cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &t})
	return err
}

func Restart(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	_, err := cli.ContainerRestart(ctx, id, client.ContainerRestartOptions{Timeout: &t})
	return err
}
