package docker

import (
	"context"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// recreateWith stops and removes the container, then creates and starts a
// replacement under the same name, optionally pulling its image first.
//
// The replacement keeps everything the container was created with: its
// settings on top of the image, host config, networks and their aliases, and
// anonymous volumes. That's what lets a Compose service such as `db` keep
// answering at its service name. What the container only inherited from its
// old image (env, command, labels…) is left to the new image instead.
func recreateWith(ctx context.Context, cli *client.Client, id string, mutate func(cfg *container.Config, hc *container.HostConfig), pull bool) (string, error) {
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	info := res.Container
	name := strings.TrimPrefix(info.Name, "/")

	cfg := &container.Config{}
	if info.Config != nil {
		c := *info.Config
		cfg = &c
	}
	hc := &container.HostConfig{}
	if info.HostConfig != nil {
		h := *info.HostConfig
		hc = &h
	}
	if img, err := cli.ImageInspect(ctx, info.Image); err == nil && img.Config != nil {
		withoutImageDefaults(cfg, img.Config.Env, img.Config.Cmd, img.Config.Entrypoint, img.Config.Labels,
			img.Config.WorkingDir, img.Config.User, img.Config.StopSignal,
			img.Config.ExposedPorts, img.Config.Volumes, img.Config.Healthcheck)
	}
	// Docker names a container's host after its short id unless told otherwise.
	if cfg.Hostname == shortID(info.ID) {
		cfg.Hostname = ""
	}
	keepAnonymousVolumes(hc, info.Mounts)
	primary, others := endpoints(hc.NetworkMode, info.NetworkSettings, info.ID)
	if mutate != nil {
		mutate(cfg, hc)
	}

	// Best-effort stop, then force remove.
	timeout := 5
	_, _ = cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout})
	if _, err := cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true}); err != nil {
		return "", err
	}

	if pull && cfg.Image != "" {
		rc, err := cli.ImagePull(ctx, cfg.Image, client.ImagePullOptions{})
		if err != nil {
			return "", err
		}
		_, _ = io.Copy(io.Discard, rc) // drain so the pull completes
		_ = rc.Close()
	}

	created, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           cfg,
		HostConfig:       hc,
		NetworkingConfig: &network.NetworkingConfig{EndpointsConfig: primary},
		Name:             name,
	})
	if err != nil {
		return "", err
	}
	// Older Engine APIs take one network at create; join the rest before start.
	for net, ep := range others {
		if _, err := cli.NetworkConnect(ctx, net, client.NetworkConnectOptions{Container: created.ID, EndpointConfig: ep}); err != nil {
			return "", err
		}
	}
	if _, err := cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		return "", err
	}
	return created.ID, nil
}

// withoutImageDefaults removes from cfg what equals the image's own config,
// so a newly pulled image's defaults apply rather than the old image's.
func withoutImageDefaults(cfg *container.Config, env, cmd, entrypoint []string, labels map[string]string,
	workdir, user, stopSignal string, exposed, volumes map[string]struct{}, health *container.HealthConfig) {
	cfg.Env = slices.DeleteFunc(slices.Clone(cfg.Env), func(e string) bool { return slices.Contains(env, e) })
	if slices.Equal(cfg.Cmd, cmd) {
		cfg.Cmd = nil
	}
	if slices.Equal(cfg.Entrypoint, entrypoint) {
		cfg.Entrypoint = nil
	}
	if cfg.Labels != nil {
		cfg.Labels = maps.Clone(cfg.Labels)
		maps.DeleteFunc(cfg.Labels, func(k, v string) bool { iv, ok := labels[k]; return ok && iv == v })
	}
	if cfg.WorkingDir == workdir {
		cfg.WorkingDir = ""
	}
	if cfg.User == user {
		cfg.User = ""
	}
	if cfg.StopSignal == stopSignal {
		cfg.StopSignal = ""
	}
	if cfg.ExposedPorts != nil {
		cfg.ExposedPorts = maps.Clone(cfg.ExposedPorts)
		maps.DeleteFunc(cfg.ExposedPorts, func(p network.Port, _ struct{}) bool { _, ok := exposed[p.String()]; return ok })
	}
	if cfg.Volumes != nil {
		cfg.Volumes = maps.Clone(cfg.Volumes)
		maps.DeleteFunc(cfg.Volumes, func(v string, _ struct{}) bool { _, ok := volumes[v]; return ok })
	}
	if health != nil && reflect.DeepEqual(cfg.Healthcheck, health) {
		cfg.Healthcheck = nil
	}
}

// keepAnonymousVolumes reattaches volumes that have no name in the host
// config (from the image's VOLUME, or `-v /path`), which would otherwise be
// replaced by new, empty ones.
func keepAnonymousVolumes(hc *container.HostConfig, mounts []container.MountPoint) {
	covered := map[string]bool{}
	for _, b := range hc.Binds {
		if parts := strings.Split(b, ":"); len(parts) >= 2 {
			covered[parts[1]] = true
		}
	}
	for _, m := range hc.Mounts {
		covered[m.Target] = true
	}
	for _, m := range mounts {
		if m.Type == mount.TypeVolume && m.Name != "" && !covered[m.Destination] {
			hc.Mounts = append(hc.Mounts, mount.Mount{Type: mount.TypeVolume, Source: m.Name, Target: m.Destination, ReadOnly: !m.RW})
		}
	}
}

// endpoints splits the container's networks into the one it's created on
// (its network mode) and the ones it joins after, keeping what was set by
// hand: aliases, static IPs, links, driver options.
func endpoints(mode container.NetworkMode, ns *container.NetworkSettings, id string) (primary, others map[string]*network.EndpointSettings) {
	primary, others = map[string]*network.EndpointSettings{}, map[string]*network.EndpointSettings{}
	if ns == nil || mode.IsHost() || mode.IsNone() || mode.IsContainer() {
		return primary, others
	}
	for name, ep := range ns.Networks {
		if ep == nil {
			continue
		}
		kept := &network.EndpointSettings{IPAMConfig: ep.IPAMConfig, Links: ep.Links, DriverOpts: ep.DriverOpts}
		// The default bridge refuses aliases; elsewhere drop the old short id,
		// which Docker added itself.
		if name != "bridge" {
			kept.Aliases = slices.DeleteFunc(slices.Clone(ep.Aliases), func(a string) bool { return a == shortID(id) })
		}
		if name == string(mode) || (mode.IsDefault() && name == "bridge") {
			primary[name] = kept
		} else {
			others[name] = kept
		}
	}
	return primary, others
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
