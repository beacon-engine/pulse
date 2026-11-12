// Package docker runs the agent's Docker operations: containers, images,
// volumes, networks, Compose stacks and the event stream, through the Docker
// Engine API (which Podman also serves).

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/docker.rs — Docker operations via the moby client.

// ── List ─────────────────────────────────────────────────────────────────────

func List(ctx context.Context, cli *client.Client, self selfguard.Self) ([]protocol.ContainerInfo, error) {
	f := filters.NewArgs()
	for _, s := range []string{"running", "exited", "paused", "created"} {
		f.Add("status", s)
	}
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ContainerInfo, 0, len(list))
	for _, c := range list {
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
			State:           c.State,
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
	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return protocol.ContainerDetail{}, err
	}

	var ports []protocol.ContainerPort
	if info.NetworkSettings != nil {
		for p, bindings := range info.NetworkSettings.Ports {
			for _, b := range bindings {
				hostIP := b.HostIP
				if hostIP == "" {
					hostIP = "0.0.0.0"
				}
				ports = append(ports, protocol.ContainerPort{
					ContainerPort: p.Port(),
					Protocol:      p.Proto(),
					HostIP:        hostIP,
					HostPort:      b.HostPort,
				})
			}
		}
		sort.Slice(ports, func(i, j int) bool { return ports[i].ContainerPort < ports[j].ContainerPort })
	}

	var mounts []protocol.ContainerMount
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
	}
	var labels map[string]string
	if info.Config != nil {
		detail.Image = info.Config.Image
		detail.Env = info.Config.Env
		detail.Cmd = []string(info.Config.Cmd)
		labels = info.Config.Labels
	}
	detail.ProtectedReason = self.ContainerProtection(info.ID, labels)
	detail.Protected = detail.ProtectedReason != ""
	detail.Kubernetes = selfguard.KubeletRef(labels)
	if info.State != nil {
		detail.State = info.State.Status
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
	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

// ── Recreate variants ────────────────────────────────────────────────────────

// recreateWith stops+removes the container and re-creates it under the same name
// from a freshly-built config, optionally pulling the image first.
func recreateWith(ctx context.Context, cli *client.Client, id string, mutate func(cfg *container.Config, hc *container.HostConfig), pull bool) (string, error) {
	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(info.Name, "/")

	oldCfg := info.Config
	hc := info.HostConfig
	if oldCfg == nil {
		oldCfg = &container.Config{}
	}
	if hc == nil {
		hc = &container.HostConfig{}
	}

	newCfg := &container.Config{
		Image:  oldCfg.Image,
		Cmd:    oldCfg.Cmd,
		Env:    oldCfg.Env,
		Labels: oldCfg.Labels,
	}
	if mutate != nil {
		mutate(newCfg, hc)
	}

	// Best-effort stop, then force remove.
	timeout := 5
	_ = cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return "", err
	}

	if pull && newCfg.Image != "" {
		rc, err := cli.ImagePull(ctx, newCfg.Image, image.PullOptions{})
		if err != nil {
			return "", err
		}
		_, _ = io.Copy(io.Discard, rc) // drain so the pull completes
		_ = rc.Close()
	}

	created, err := cli.ContainerCreate(ctx, newCfg, hc, nil, nil, name)
	if err != nil {
		return "", err
	}
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return created.ID, nil
}

func Recreate(ctx context.Context, cli *client.Client, id string, newPorts []protocol.PortMap) (string, error) {
	return recreateWith(ctx, cli, id, func(_ *container.Config, hc *container.HostConfig) {
		bindings := nat.PortMap{}
		for _, pm := range newPorts {
			key := nat.Port(fmt.Sprintf("%s/%s", pm.ContainerPort, pm.Protocol))
			bindings[key] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: pm.HostPort}}
		}
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
	rc, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
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
	exec, err := cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}
	attach, err := cli.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})
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
	return cli.ContainerStart(ctx, id, container.StartOptions{})
}

func Stop(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	return cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

func Restart(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	return cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &t})
}
