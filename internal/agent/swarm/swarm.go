// Package swarm runs the agent's Docker Swarm operations: services, nodes and
// tasks.
package swarm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/swarm.rs — Docker Swarm services, nodes, tasks and scaling.

func Services(ctx context.Context, cli *client.Client) ([]protocol.SwarmService, error) {
	list, err := cli.ServiceList(ctx, client.ServiceListOptions{Status: true})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.SwarmService, 0, len(list.Items))
	for _, s := range list.Items {
		var replicas uint64
		if rep := s.Spec.Mode.Replicated; rep != nil && rep.Replicas != nil {
			replicas = *rep.Replicas
		}
		var running uint64
		if s.ServiceStatus != nil {
			running = s.ServiceStatus.RunningTasks
		}
		image := ""
		if cs := s.Spec.TaskTemplate.ContainerSpec; cs != nil {
			image = cs.Image
		}
		result = append(result, protocol.SwarmService{
			ID:           s.ID,
			Name:         s.Spec.Annotations.Name,
			Image:        image,
			Replicas:     replicas,
			Running:      running,
			DesiredState: "unknown",
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func Nodes(ctx context.Context, cli *client.Client) ([]protocol.SwarmNode, error) {
	list, err := cli.NodeList(ctx, client.NodeListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.SwarmNode, 0, len(list.Items))
	for _, n := range list.Items {
		result = append(result, protocol.SwarmNode{
			ID:           n.ID,
			Hostname:     n.Description.Hostname,
			Role:         string(n.Spec.Role),
			State:        string(n.Status.State),
			Availability: string(n.Spec.Availability),
			Addr:         n.Status.Addr,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Hostname < result[j].Hostname })
	return result, nil
}

func Tasks(ctx context.Context, cli *client.Client, serviceID string) ([]protocol.SwarmTask, error) {
	list, err := cli.TaskList(ctx, client.TaskListOptions{Filters: client.Filters{}.Add("service", serviceID)})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.SwarmTask, 0, len(list.Items))
	for _, t := range list.Items {
		containerID := ""
		if t.Status.ContainerStatus != nil {
			containerID = t.Status.ContainerStatus.ContainerID
		}
		image := ""
		if cs := t.Spec.ContainerSpec; cs != nil {
			image = cs.Image
		}
		result = append(result, protocol.SwarmTask{
			ID:          t.ID,
			ServiceID:   t.ServiceID,
			NodeID:      t.NodeID,
			Slot:        int64(t.Slot),
			ContainerID: containerID,
			Image:       image,
			State:       strings.ToLower(string(t.Status.State)),
		})
	}
	return result, nil
}

// ForceUpdate forces a rolling update by bumping the service's force_update
// counter — the swarm equivalent of "redeploy with the current spec".
func ForceUpdate(ctx context.Context, cli *client.Client, serviceID string) error {
	res, err := cli.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
	if err != nil {
		return err
	}
	spec := res.Service.Spec
	spec.TaskTemplate.ForceUpdate++
	_, err = cli.ServiceUpdate(ctx, serviceID, client.ServiceUpdateOptions{Version: res.Service.Version, Spec: spec})
	return err
}

func Scale(ctx context.Context, cli *client.Client, serviceID string, targetReplicas uint64) error {
	res, err := cli.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
	if err != nil {
		return err
	}
	spec := res.Service.Spec

	switch {
	case spec.Mode.Global != nil:
		return fmt.Errorf("cannot scale a global swarm service (runs 1 replica per node)")
	case spec.Mode.Replicated != nil:
		spec.Mode.Replicated.Replicas = &targetReplicas
	default:
		spec.Mode.Replicated = &swarm.ReplicatedService{Replicas: &targetReplicas}
	}

	_, err = cli.ServiceUpdate(ctx, serviceID, client.ServiceUpdateOptions{Version: res.Service.Version, Spec: spec})
	return err
}
