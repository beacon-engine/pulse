package kubernetes

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Operations ───────────────────────────────────────────────────────────────

func listPods(ctx context.Context, cs *clientset.Clientset, self selfguard.Self) ([]protocol.PodInfo, error) {
	list, err := cs.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.PodInfo, 0, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		info := mapPod(pod)
		info.ProtectedReason = self.PodProtection(pod.Namespace, pod.Name, pod.Labels)
		info.Protected = info.ProtectedReason != ""
		result = append(result, info)
	}
	// Stable order so periodic refreshes don't reshuffle the UI.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// mapPod projects a Kubernetes Pod onto the wire PodInfo. Factored out so the
// readiness/state logic is unit-testable without a live cluster.
func mapPod(pod *corev1.Pod) protocol.PodInfo {
	phase := string(pod.Status.Phase)
	if phase == "" {
		phase = "Unknown"
	}

	ready := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			ready = cond.Status == corev1.ConditionTrue
		}
	}

	specs := make(map[string]corev1.Container, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		specs[c.Name] = c
	}
	containers := make([]protocol.PodContainer, 0, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		c := mapContainerStatus(cs)
		if spec, ok := specs[cs.Name]; ok {
			c.Ports = containerPorts(spec)
			c.Resources = containerResources(spec)
		}
		containers = append(containers, c)
	}

	return protocol.PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Status:     phase,
		Reason:     podReason(pod),
		Ready:      ready,
		Containers: containers,
		Node:       pod.Spec.NodeName,
		IP:         pod.Status.PodIP,
		Created:    timestamp(pod.CreationTimestamp),
		Owner:      podOwner(pod),
		Labels:     pod.Labels,
		QOSClass:   string(pod.Status.QOSClass),
	}
}

func mapContainerStatus(cs corev1.ContainerStatus) protocol.PodContainer {
	c := protocol.PodContainer{
		Name:         cs.Name,
		Image:        cs.Image,
		Ready:        cs.Ready,
		RestartCount: cs.RestartCount,
		State:        "unknown",
		ContainerID:  cs.ContainerID,
	}
	switch s := cs.State; {
	case s.Running != nil:
		c.State = "running"
		c.Started = timestamp(s.Running.StartedAt)
	case s.Terminated != nil:
		c.State = "terminated"
		c.Reason, c.Message = s.Terminated.Reason, s.Terminated.Message
		code := s.Terminated.ExitCode
		c.ExitCode = &code
		c.Started = timestamp(s.Terminated.FinishedAt)
	case s.Waiting != nil:
		c.State = "waiting"
		c.Reason, c.Message = s.Waiting.Reason, s.Waiting.Message
	}
	if last := cs.LastTerminationState.Terminated; last != nil {
		c.LastReason = last.Reason
		code := last.ExitCode
		c.LastExitCode = &code
	}
	return c
}

func containerPorts(c corev1.Container) []protocol.PodPort {
	if len(c.Ports) == 0 {
		return nil
	}
	ports := make([]protocol.PodPort, 0, len(c.Ports))
	for _, p := range c.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, protocol.PodPort{Name: p.Name, ContainerPort: p.ContainerPort, Protocol: proto})
	}
	return ports
}

func containerResources(c corev1.Container) *protocol.PodResources {
	r := protocol.PodResources{
		CPURequest:    quantity(c.Resources.Requests, corev1.ResourceCPU),
		CPULimit:      quantity(c.Resources.Limits, corev1.ResourceCPU),
		MemoryRequest: quantity(c.Resources.Requests, corev1.ResourceMemory),
		MemoryLimit:   quantity(c.Resources.Limits, corev1.ResourceMemory),
	}
	if r == (protocol.PodResources{}) {
		return nil
	}
	return &r
}

func quantity(list corev1.ResourceList, name corev1.ResourceName) string {
	if q, ok := list[name]; ok {
		return q.String()
	}
	return ""
}

// podReason is the status `kubectl get pods` prints when the phase alone is
// misleading: a "Running" pod whose container is in CrashLoopBackOff, a pod
// being deleted, or a finished one. Empty when the phase says it all.
func podReason(pod *corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}
	for _, cs := range pod.Status.InitContainerStatuses {
		if w := cs.State.Waiting; w != nil && w.Reason != "" && w.Reason != "PodInitializing" {
			return "Init:" + w.Reason
		}
		if t := cs.State.Terminated; t != nil && t.ExitCode != 0 {
			return "Init:Error"
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if w := cs.State.Waiting; w != nil && w.Reason != "" {
			return w.Reason
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if t := cs.State.Terminated; t != nil && t.Reason != "" && pod.Status.Phase != corev1.PodRunning {
			return t.Reason
		}
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason // e.g. Evicted
	}
	return ""
}

// podOwner names the workload that manages a pod. A Deployment's pods are owned
// by a ReplicaSet named "<deployment>-<pod-template-hash>", so the Deployment
// is recovered from the name without an extra API call.
func podOwner(pod *corev1.Pod) *protocol.WorkloadRef {
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return nil
	}
	if ref.Kind == "ReplicaSet" {
		if hash := pod.Labels["pod-template-hash"]; hash != "" && strings.HasSuffix(ref.Name, "-"+hash) {
			return &protocol.WorkloadRef{Kind: protocol.WorkloadDeployment, Name: strings.TrimSuffix(ref.Name, "-"+hash)}
		}
	}
	return &protocol.WorkloadRef{Kind: ref.Kind, Name: ref.Name}
}

func timestamp(t metav1.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func podContainerNames(pod *corev1.Pod) []string {
	names := make([]string, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}

// podLogs returns the last tail lines of one container in a pod. container may
// be empty for a single-container pod; the API server rejects it otherwise.
func podLogs(ctx context.Context, cs clientset.Interface, namespace, name, container string, tail uint32) (string, error) {
	lines := int64(tail)
	opts := &corev1.PodLogOptions{Container: container, Timestamps: true}
	if lines > 0 {
		opts.TailLines = &lines
	}
	b, err := cs.CoreV1().Pods(namespace).GetLogs(name, opts).DoRaw(ctx)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
