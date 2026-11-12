package docker

import (
	"context"
	"sort"

	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Stacks (from container labels) ───────────────────────────────────────────

func ListStacks(ctx context.Context, cli *client.Client, self selfguard.Self) ([]protocol.Stack, error) {
	all, err := List(ctx, cli, self)
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
