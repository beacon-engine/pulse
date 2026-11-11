package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/docker.rs — Docker operations via the moby client.

// ── List ─────────────────────────────────────────────────────────────────────

func dockerList(ctx context.Context, cli *client.Client, self Self) ([]protocol.ContainerInfo, error) {
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
		reason := self.containerProtection(c.ID, c.Labels)
		result = append(result, protocol.ContainerInfo{
			ID:              c.ID,
			Name:            name,
			Image:           c.Image,
			Status:          c.Status,
			State:           c.State,
			Stack:           containerStack(c.Labels),
			Protected:       reason != "",
			ProtectedReason: reason,
			Kubernetes:      kubeletRef(c.Labels),
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

func dockerInspect(ctx context.Context, cli *client.Client, self Self, id string) (protocol.ContainerDetail, error) {
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
	detail.ProtectedReason = self.containerProtection(info.ID, labels)
	detail.Protected = detail.ProtectedReason != ""
	detail.Kubernetes = kubeletRef(labels)
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

func dockerInspectRaw(ctx context.Context, cli *client.Client, id string) (json.RawMessage, error) {
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

func dockerRecreate(ctx context.Context, cli *client.Client, id string, newPorts []protocol.PortMap) (string, error) {
	return recreateWith(ctx, cli, id, func(_ *container.Config, hc *container.HostConfig) {
		bindings := nat.PortMap{}
		for _, pm := range newPorts {
			key := nat.Port(fmt.Sprintf("%s/%s", pm.ContainerPort, pm.Protocol))
			bindings[key] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: pm.HostPort}}
		}
		hc.PortBindings = bindings
	}, false)
}

func dockerRecreateLatest(ctx context.Context, cli *client.Client, id string, pull bool) (string, error) {
	return recreateWith(ctx, cli, id, nil, pull)
}

func dockerUpdateEnv(ctx context.Context, cli *client.Client, id string, env []string) (string, error) {
	return recreateWith(ctx, cli, id, func(cfg *container.Config, _ *container.HostConfig) {
		cfg.Env = env
	}, false)
}

// ── Logs / exec ──────────────────────────────────────────────────────────────

func dockerLogs(ctx context.Context, cli *client.Client, id string, tail uint32) (string, error) {
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

func dockerExec(ctx context.Context, cli *client.Client, id string, cmd []string) (string, error) {
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

func dockerStart(ctx context.Context, cli *client.Client, id string) error {
	return cli.ContainerStart(ctx, id, container.StartOptions{})
}

func dockerStop(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	return cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

func dockerRestart(ctx context.Context, cli *client.Client, id string) error {
	t := 10
	return cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &t})
}

// ── Stacks (from container labels) ───────────────────────────────────────────

func dockerListStacks(ctx context.Context, cli *client.Client, self Self) ([]protocol.Stack, error) {
	all, err := dockerList(ctx, cli, self)
	if err != nil {
		return nil, err
	}
	groups := map[string][]protocol.ContainerInfo{}
	for _, c := range all {
		if c.Stack != nil {
			groups[*c.Stack] = append(groups[*c.Stack], c)
		}
	}
	stacks := make([]protocol.Stack, 0, len(groups))
	for name, services := range groups {
		stacks = append(stacks, protocol.Stack{Name: name, Services: services})
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	return stacks, nil
}

// ── System info ──────────────────────────────────────────────────────────────

func dockerSystemInfo(ctx context.Context, cli *client.Client) (protocol.SystemInfo, error) {
	info, err := cli.Info(ctx)
	if err != nil {
		return protocol.SystemInfo{}, err
	}
	return protocol.SystemInfo{
		DockerVersion:     info.ServerVersion,
		APIVersion:        "1.43",
		OS:                strings.TrimSpace(info.OperatingSystem + " " + info.OSVersion),
		Arch:              info.Architecture,
		Kernel:            info.KernelVersion,
		TotalMemory:       info.MemTotal,
		CPUs:              int64(info.NCPU),
		ContainersRunning: int64(info.ContainersRunning),
		ContainersStopped: int64(info.ContainersPaused + info.ContainersStopped),
		Images:            int64(info.Images),
	}, nil
}

// ── Images ───────────────────────────────────────────────────────────────────

func dockerListImages(ctx context.Context, cli *client.Client) ([]protocol.ImageInfo, error) {
	images, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ImageInfo, 0, len(images))
	for _, img := range images {
		tags := make([]string, 0, len(img.RepoTags))
		dangling := len(img.RepoTags) == 0
		allNone := true
		for _, t := range img.RepoTags {
			if t != "<none>:<none>" {
				tags = append(tags, t)
				allNone = false
			}
		}
		result = append(result, protocol.ImageInfo{
			ID:       img.ID,
			Tags:     tags,
			Size:     img.Size,
			Created:  img.Created,
			Dangling: dangling || allNone,
		})
	}
	return result, nil
}

func dockerDeleteImage(ctx context.Context, cli *client.Client, id string) error {
	_, err := cli.ImageRemove(ctx, id, image.RemoveOptions{Force: true})
	return err
}

func dockerPruneImages(ctx context.Context, cli *client.Client) (int64, error) {
	report, err := cli.ImagesPrune(ctx, filters.NewArgs())
	if err != nil {
		return 0, err
	}
	return int64(report.SpaceReclaimed), nil
}

// ── Volumes ──────────────────────────────────────────────────────────────────

func dockerListVolumes(ctx context.Context, cli *client.Client) ([]protocol.VolumeInfo, error) {
	list, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.VolumeInfo, 0, len(list.Volumes))
	for _, v := range list.Volumes {
		if v == nil {
			continue
		}
		result = append(result, protocol.VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Created:    v.CreatedAt,
		})
	}
	return result, nil
}

func dockerDeleteVolume(ctx context.Context, cli *client.Client, name string) error {
	return cli.VolumeRemove(ctx, name, true)
}

func dockerPruneVolumes(ctx context.Context, cli *client.Client) (int64, error) {
	report, err := cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		return 0, err
	}
	return int64(report.SpaceReclaimed), nil
}

// ── Networks ─────────────────────────────────────────────────────────────────

func dockerListHostNetworks(ctx context.Context, cli *client.Client) ([]protocol.HostNetwork, error) {
	nets, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.HostNetwork, 0, len(nets))
	for _, n := range nets {
		result = append(result, protocol.HostNetwork{
			ID:       n.ID,
			Name:     n.Name,
			Driver:   n.Driver,
			Scope:    n.Scope,
			Internal: n.Internal,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func dockerContainerNetworks(ctx context.Context, cli *client.Client, containerID string) ([]protocol.ContainerNetworkInfo, error) {
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}
	var result []protocol.ContainerNetworkInfo
	if info.NetworkSettings != nil {
		for name, ep := range info.NetworkSettings.Networks {
			if ep == nil {
				continue
			}
			result = append(result, protocol.ContainerNetworkInfo{
				ID:         ep.NetworkID,
				Name:       name,
				IPAddress:  ep.IPAddress,
				Gateway:    ep.Gateway,
				MacAddress: ep.MacAddress,
			})
		}
	}
	return result, nil
}

func dockerConnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	return cli.NetworkConnect(ctx, networkID, containerID, &network.EndpointSettings{})
}

func dockerDisconnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	return cli.NetworkDisconnect(ctx, networkID, containerID, false)
}

// ── Error helpers (match the Rust conn_error / exec_error status codes) ───────

func connError(msg string) protocol.Response {
	return protocol.RespError(502, "cannot connect to Docker: "+msg)
}

func execError(err error) protocol.Response {
	if client.IsErrConnectionFailed(err) {
		return protocol.RespError(503, "no container runtime (Docker/Podman API) reachable from this agent: "+err.Error())
	}
	return protocol.RespError(500, err.Error())
}

// guardProtected refuses a disruptive action on one of Pulse's own containers.
// It resolves id (a name or short id) through inspect so it can't be sidestepped
// by addressing the container differently. ok=false → return resp as-is.
func guardProtected(ctx context.Context, cli *client.Client, self Self, id string) (resp protocol.Response, ok bool) {
	return guardContainer(ctx, cli, self, id, false)
}

// guardReplace is guardProtected for actions that replace the container
// (recreate, env update). Those are also refused for kubelet-managed
// containers: the copy would run outside Kubernetes while the kubelet starts
// yet another one for the pod.
func guardReplace(ctx context.Context, cli *client.Client, self Self, id string) (resp protocol.Response, ok bool) {
	return guardContainer(ctx, cli, self, id, true)
}

func guardContainer(ctx context.Context, cli *client.Client, self Self, id string, replace bool) (protocol.Response, bool) {
	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return execError(err), false
	}
	var labels map[string]string
	if info.Config != nil {
		labels = info.Config.Labels
	}
	name := strings.TrimPrefix(info.Name, "/")
	if reason := self.containerProtection(info.ID, labels); reason != "" {
		return protectedError(name, reason), false
	}
	if ref := kubeletRef(labels); replace && ref != nil {
		return protocol.RespError(409, fmt.Sprintf(
			"%s is managed by Kubernetes (pod %s/%s): change its pod spec or restart its workload instead; "+
				"recreating it with Docker would leave a copy running outside the cluster",
			name, ref.Namespace, ref.Pod)), false
	}
	return protocol.Response{}, true
}

func protectedError(name, reason string) protocol.Response {
	return protocol.RespError(403, fmt.Sprintf("%s is protected (%s): Pulse blocks actions that could cut off its own connection", name, reason))
}

// dockerOp resolves the Docker client for host (502 on failure) then runs fn.
func dockerOp(st *State, host string, fn func(*client.Client) protocol.Response) protocol.Response {
	cli, err := st.Docker.ClientFor(host)
	if err != nil {
		return connError(err.Error())
	}
	return fn(cli)
}

// Result → Response helpers matching the Rust dispatch's mapping.
func okOrErr(err error) protocol.Response {
	if err != nil {
		return execError(err)
	}
	return protocol.RespOk()
}

func idOrErr(id string, err error) protocol.Response {
	if err != nil {
		return execError(err)
	}
	return protocol.RespID(id)
}

func countOrErr(n int64, err error) protocol.Response {
	if err != nil {
		return execError(err)
	}
	return protocol.RespCount(n)
}

func composeText(out string, err error) protocol.Response {
	if err != nil {
		return execError(err)
	}
	return protocol.RespText(out)
}

// inContainer is a best-effort check for whether we're running inside a container.
func inContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}
