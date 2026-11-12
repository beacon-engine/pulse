package docker

import (
	"context"
	"sort"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Networks ─────────────────────────────────────────────────────────────────

func ListNetworks(ctx context.Context, cli *client.Client) ([]protocol.HostNetwork, error) {
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

func ContainerNetworks(ctx context.Context, cli *client.Client, containerID string) ([]protocol.ContainerNetworkInfo, error) {
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

func ConnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	return cli.NetworkConnect(ctx, networkID, containerID, &network.EndpointSettings{})
}

func DisconnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	return cli.NetworkDisconnect(ctx, networkID, containerID, false)
}
