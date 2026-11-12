package kubernetes

import (
	"context"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/protocol"
)

func listNodes(ctx context.Context, cs clientset.Interface) ([]protocol.NodeInfo, error) {
	list, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.NodeInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, mapNode(&list.Items[i]))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

const nodeRolePrefix = "node-role.kubernetes.io/"

func mapNode(n *corev1.Node) protocol.NodeInfo {
	info := protocol.NodeInfo{
		Name:             n.Name,
		Status:           "Unknown",
		Unschedulable:    n.Spec.Unschedulable,
		Roles:            []string{},
		Version:          n.Status.NodeInfo.KubeletVersion,
		OSImage:          n.Status.NodeInfo.OSImage,
		KernelVersion:    n.Status.NodeInfo.KernelVersion,
		Arch:             n.Status.NodeInfo.Architecture,
		ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
		CPU:              quantity(n.Status.Capacity, corev1.ResourceCPU),
		Memory:           quantity(n.Status.Capacity, corev1.ResourceMemory),
		Pods:             quantity(n.Status.Capacity, corev1.ResourcePods),
		AllocCPU:         quantity(n.Status.Allocatable, corev1.ResourceCPU),
		AllocMemory:      quantity(n.Status.Allocatable, corev1.ResourceMemory),
		Created:          timestamp(n.CreationTimestamp),
	}
	for label := range n.Labels {
		if role, ok := strings.CutPrefix(label, nodeRolePrefix); ok && role != "" {
			info.Roles = append(info.Roles, role)
		}
	}
	sort.Strings(info.Roles)
	for _, c := range n.Status.Conditions {
		switch {
		case c.Type == corev1.NodeReady:
			switch c.Status {
			case corev1.ConditionTrue:
				info.Status = "Ready"
			case corev1.ConditionFalse:
				info.Status = "NotReady"
			}
		case c.Status == corev1.ConditionTrue:
			info.Pressure = append(info.Pressure, string(c.Type))
		}
	}
	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeInternalIP {
			info.InternalIP = a.Address
			break
		}
	}
	return info
}
