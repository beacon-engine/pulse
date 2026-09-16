package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/kubernetes.rs — Kubernetes access via client-go.

// K8s lazily connects and caches a clientset, mirroring the Rust agent's
// k8s_client: it connects on first use (or after a failed attempt) so a
// kubeconfig/ServiceAccount that appears after startup is picked up without a
// restart. kubeconfigYAML == "" means auto-discover (in-pod ServiceAccount, or
// the host's ~/.kube/config).
type K8s struct {
	kubeconfigYAML string
	mu             sync.Mutex
	client         *kubernetes.Clientset
}

// NewK8s builds a lazy Kubernetes accessor for the given target.
func NewK8s(kubeconfigYAML string) *K8s {
	return &K8s{kubeconfigYAML: kubeconfigYAML}
}

// Client returns a connected clientset, connecting and caching on first use.
func (k *K8s) Client() (*kubernetes.Clientset, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.client != nil {
		return k.client, nil
	}
	cfg, err := k.restConfig()
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	k.client = cs
	return cs, nil
}

// restConfig resolves the REST config: explicit kubeconfig YAML, else the in-pod
// ServiceAccount, else the default kubeconfig loading rules (~/.kube/config or
// $KUBECONFIG).
func (k *K8s) restConfig() (*rest.Config, error) {
	if k.kubeconfigYAML != "" {
		return clientcmd.RESTConfigFromKubeConfig([]byte(k.kubeconfigYAML))
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// ── Dispatch handlers ────────────────────────────────────────────────────────

func handleK8sPods(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable()
	}
	pods, err := listPods(ctx, cs)
	if err != nil {
		return protocol.RespError(500, err.Error())
	}
	return protocol.RespPods(pods)
}

func handleK8sNamespaces(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable()
	}
	ns, err := listNamespaces(ctx, cs)
	if err != nil {
		return protocol.RespError(500, err.Error())
	}
	return protocol.RespNamespaces(ns)
}

func handleK8sRollout(ctx context.Context, st *State, namespace, deployment string) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable()
	}
	if err := rollout(ctx, cs, namespace, deployment); err != nil {
		return protocol.RespError(500, err.Error())
	}
	return protocol.RespOk()
}

func handleK8sDeletePod(ctx context.Context, st *State, namespace, name string) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable()
	}
	if err := cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return protocol.RespError(500, err.Error())
	}
	return protocol.RespOk()
}

func k8sUnavailable() protocol.Response {
	return protocol.RespError(503, "Kubernetes not connected")
}

// ── Operations ───────────────────────────────────────────────────────────────

func listPods(ctx context.Context, cs *kubernetes.Clientset) ([]protocol.PodInfo, error) {
	list, err := cs.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.PodInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, mapPod(&list.Items[i]))
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

	containers := make([]protocol.PodContainer, 0, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		state := "unknown"
		switch {
		case cs.State.Running != nil:
			state = "running"
		case cs.State.Terminated != nil:
			state = "terminated"
		case cs.State.Waiting != nil:
			state = "waiting"
		}
		containers = append(containers, protocol.PodContainer{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			State:        state,
		})
	}

	return protocol.PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Status:     phase,
		Ready:      ready,
		Containers: containers,
	}
}

func listNamespaces(ctx context.Context, cs *kubernetes.Clientset) ([]protocol.NamespaceInfo, error) {
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.NamespaceInfo, 0, len(list.Items))
	for i := range list.Items {
		status := string(list.Items[i].Status.Phase)
		if status == "" {
			status = "Unknown"
		}
		result = append(result, protocol.NamespaceInfo{Name: list.Items[i].Name, Status: status})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// rollout triggers a rollout restart by stamping the standard
// kubectl.kubernetes.io/restartedAt pod-template annotation.
func rollout(ctx context.Context, cs *kubernetes.Clientset, namespace, deployment string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		now,
	)
	_, err := cs.AppsV1().Deployments(namespace).Patch(
		ctx, deployment, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	return err
}
