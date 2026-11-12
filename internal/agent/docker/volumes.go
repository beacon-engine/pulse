package docker

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Volumes ──────────────────────────────────────────────────────────────────

func ListVolumes(ctx context.Context, cli *client.Client) ([]protocol.VolumeInfo, error) {
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

func DeleteVolume(ctx context.Context, cli *client.Client, name string) error {
	return cli.VolumeRemove(ctx, name, true)
}

func PruneVolumes(ctx context.Context, cli *client.Client) (int64, error) {
	report, err := cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		return 0, err
	}
	return int64(report.SpaceReclaimed), nil
}
