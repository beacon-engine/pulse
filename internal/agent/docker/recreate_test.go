package docker

import (
	"slices"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
)

func TestWithoutImageDefaults(t *testing.T) {
	port := network.MustParsePort("6379/tcp")
	cfg := &container.Config{
		Env:          []string{"FOO=bar", "PATH=/usr/bin", "REDIS_VERSION=7.4.1"},
		Cmd:          []string{"redis-server"},
		Entrypoint:   []string{"docker-entrypoint.sh"},
		Labels:       map[string]string{"com.docker.compose.service": "db", "maintainer": "x"},
		WorkingDir:   "/data",
		ExposedPorts: network.PortSet{port: {}},
		Volumes:      map[string]struct{}{"/data": {}},
	}
	withoutImageDefaults(cfg,
		[]string{"PATH=/usr/bin", "REDIS_VERSION=7.4.1"}, []string{"redis-server"}, []string{"docker-entrypoint.sh"},
		map[string]string{"maintainer": "x"}, "/data", "", "",
		map[string]struct{}{"6379/tcp": {}}, map[string]struct{}{"/data": {}}, nil)

	if !slices.Equal(cfg.Env, []string{"FOO=bar"}) {
		t.Errorf("env = %v, want only the container's own FOO=bar", cfg.Env)
	}
	if cfg.Cmd != nil || cfg.Entrypoint != nil || cfg.WorkingDir != "" {
		t.Errorf("image cmd/entrypoint/workdir kept: %v %v %q", cfg.Cmd, cfg.Entrypoint, cfg.WorkingDir)
	}
	if len(cfg.Labels) != 1 || cfg.Labels["com.docker.compose.service"] != "db" {
		t.Errorf("labels = %v, want only the compose label", cfg.Labels)
	}
	if len(cfg.ExposedPorts) != 0 || len(cfg.Volumes) != 0 {
		t.Errorf("image ports/volumes kept: %v %v", cfg.ExposedPorts, cfg.Volumes)
	}
}

func TestKeepAnonymousVolumes(t *testing.T) {
	hc := &container.HostConfig{Binds: []string{"play_db_data:/var/lib/postgresql/data:rw"}}
	keepAnonymousVolumes(hc, []container.MountPoint{
		{Type: mount.TypeVolume, Name: "play_db_data", Destination: "/var/lib/postgresql/data", RW: true},
		{Type: mount.TypeVolume, Name: "fe6b625f", Destination: "/cache", RW: true},
		{Type: mount.TypeBind, Source: "/host", Destination: "/host"},
	})
	if len(hc.Mounts) != 1 || hc.Mounts[0].Source != "fe6b625f" || hc.Mounts[0].Target != "/cache" {
		t.Errorf("mounts = %+v, want only the anonymous /cache volume added", hc.Mounts)
	}
}

func TestEndpoints(t *testing.T) {
	id := "24d264c28b65aaaaaaaa"
	ns := &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{
		"play_default": {Aliases: []string{"play.db", "db", "24d264c28b65"}},
		"play_back":    {Aliases: []string{"db"}},
		"bridge":       {Aliases: []string{"x"}},
	}}
	primary, others := endpoints("play_default", ns, id)
	if ep := primary["play_default"]; ep == nil || !slices.Equal(ep.Aliases, []string{"play.db", "db"}) {
		t.Errorf("primary = %+v, want play_default with aliases play.db, db", primary)
	}
	if len(primary) != 1 || len(others) != 2 {
		t.Errorf("primary %d, others %d networks; want 1 and 2", len(primary), len(others))
	}
	if ep := others["bridge"]; ep == nil || ep.Aliases != nil {
		t.Errorf("bridge = %+v, want joined without aliases", ep)
	}
	if p, o := endpoints("host", ns, id); len(p)+len(o) != 0 {
		t.Errorf("host network mode got endpoints %v %v", p, o)
	}
}
