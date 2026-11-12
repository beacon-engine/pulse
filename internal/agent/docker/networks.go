package docker

import (
	"context"
	"net/netip"
	"sort"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Networks ─────────────────────────────────────────────────────────────────

func ListNetworks(ctx context.Context, cli *client.Client) ([]protocol.HostNetwork, error) {
	nets, err := cli.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.HostNetwork, 0, len(nets.Items))
	for _, n := range nets.Items {
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
	res, err := cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	info := res.Container
	var result []protocol.ContainerNetworkInfo
	if info.NetworkSettings != nil {
		for name, ep := range info.NetworkSettings.Networks {
			if ep == nil {
				continue
			}
			result = append(result, protocol.ContainerNetworkInfo{
				ID:         ep.NetworkID,
				Name:       name,
				IPAddress:  addrString(ep.IPAddress),
				Gateway:    addrString(ep.Gateway),
				MacAddress: ep.MacAddress.String(),
			})
		}
	}
	return result, nil
}

func ConnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	_, err := cli.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container:      containerID,
		EndpointConfig: &network.EndpointSettings{},
	})
	return err
}

func DisconnectNetwork(ctx context.Context, cli *client.Client, networkID, containerID string) error {
	_, err := cli.NetworkDisconnect(ctx, networkID, client.NetworkDisconnectOptions{Container: containerID})
	return err
}

// addrString renders an unset address as "", as the Engine API did before it
// was typed.
func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
