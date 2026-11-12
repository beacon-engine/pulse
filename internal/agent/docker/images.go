package docker

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Images ───────────────────────────────────────────────────────────────────

func ListImages(ctx context.Context, cli *client.Client) ([]protocol.ImageInfo, error) {
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

func DeleteImage(ctx context.Context, cli *client.Client, id string) error {
	_, err := cli.ImageRemove(ctx, id, image.RemoveOptions{Force: true})
	return err
}

func PruneImages(ctx context.Context, cli *client.Client) (int64, error) {
	report, err := cli.ImagesPrune(ctx, filters.NewArgs())
	if err != nil {
		return 0, err
	}
	return int64(report.SpaceReclaimed), nil
}
