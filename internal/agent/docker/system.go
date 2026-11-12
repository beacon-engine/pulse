package docker

import (
	"context"
	"strings"

	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── System info ──────────────────────────────────────────────────────────────

func SystemInfo(ctx context.Context, cli *client.Client) (protocol.SystemInfo, error) {
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
