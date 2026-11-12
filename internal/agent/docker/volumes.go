package docker

import (
	"context"

	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Volumes ──────────────────────────────────────────────────────────────────

func ListVolumes(ctx context.Context, cli *client.Client) ([]protocol.VolumeInfo, error) {
	list, err := cli.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.VolumeInfo, 0, len(list.Items))
	for _, v := range list.Items {
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
	_, err := cli.VolumeRemove(ctx, name, client.VolumeRemoveOptions{Force: true})
	return err
}

func PruneVolumes(ctx context.Context, cli *client.Client) (int64, error) {
	res, err := cli.VolumePrune(ctx, client.VolumePruneOptions{})
	if err != nil {
		return 0, err
	}
	return int64(res.Report.SpaceReclaimed), nil
}
