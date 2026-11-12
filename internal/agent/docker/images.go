package docker

import (
	"context"

	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Images ───────────────────────────────────────────────────────────────────

func ListImages(ctx context.Context, cli *client.Client) ([]protocol.ImageInfo, error) {
	images, err := cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ImageInfo, 0, len(images.Items))
	for _, img := range images.Items {
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
	_, err := cli.ImageRemove(ctx, id, client.ImageRemoveOptions{Force: true})
	return err
}

func PruneImages(ctx context.Context, cli *client.Client) (int64, error) {
	res, err := cli.ImagePrune(ctx, client.ImagePruneOptions{})
	if err != nil {
		return 0, err
	}
	return int64(res.Report.SpaceReclaimed), nil
}
