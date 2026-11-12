package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	host           string
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
	k.host = cfg.Host
	return cs, nil
}

// Host is the API server URL of the connected cluster ("" before connecting).
func (k *K8s) Host() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.host
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
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if clientcmd.IsEmptyConfig(err) {
		return nil, errNoKubeconfig
	}
	return cfg, err
}

var errNoKubeconfig = errors.New("no cluster configured: not running in a pod with a ServiceAccount, " +
	"and no kubeconfig (set PULSE_KUBECONFIG_PATH or mount ~/.kube/config)")

// ── Dispatch handlers ────────────────────────────────────────────────────────

func handleK8sPods(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	pods, err := listPods(ctx, cs, st.Self)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespPods(pods)
}

func handleK8sNamespaces(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	ns, err := listNamespaces(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespNamespaces(ns)
}

func handleK8sRollout(ctx context.Context, st *State, namespace, deployment string) protocol.Response {
	return handleK8sRestart(ctx, st, namespace, protocol.WorkloadDeployment, deployment)
}

// handleK8sRestart does a rolling restart (`kubectl rollout restart`) of a
// Deployment, StatefulSet or DaemonSet.
func handleK8sRestart(ctx context.Context, st *State, namespace, kind, name string) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	w, err := getWorkload(ctx, cs, namespace, kind, name)
	if err != nil {
		return k8sError(err)
	}
	// A restart replaces every pod, so judge by the pod template's labels.
	if reason := labelProtection(w.templateLabels); reason != "" {
		return protectedError(namespace+"/"+name, reason)
	}
	if err := rollout(ctx, cs, namespace, kind, name); err != nil {
		return k8sError(err)
	}
	return protocol.RespOk()
}

func handleK8sScale(ctx context.Context, st *State, namespace, kind, name string, replicas uint64) protocol.Response {
	if kind != protocol.WorkloadDeployment && kind != protocol.WorkloadStatefulSet {
		return protocol.RespError(400, kind+" can't be scaled: only Deployments and StatefulSets have a replica count")
	}
	if replicas > maxScaleReplicas {
		return protocol.RespError(400, fmt.Sprintf("replicas must be between 0 and %d", maxScaleReplicas))
	}
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	w, err := getWorkload(ctx, cs, namespace, kind, name)
	if err != nil {
		return k8sError(err)
	}
	// Scaling the agent to 0 cuts the connection; scaling it up starts agents
	// with the same cluster identity that would keep taking over each other's session.
	if reason := labelProtection(w.templateLabels); reason != "" {
		return protectedError(namespace+"/"+name, reason)
	}
	if err := scale(ctx, cs, namespace, kind, name, int32(replicas)); err != nil {
		return k8sError(err)
	}
	return protocol.RespOk()
}

func handleK8sWorkloads(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	ws, err := listWorkloads(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespWorkloads(ws)
}

func handleK8sNodes(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	nodes, err := listNodes(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespNodes(nodes)
}

func handleK8sServices(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	svcs, err := listServices(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespServices(svcs)
}

func handleK8sEvents(ctx context.Context, st *State) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	events, err := listEvents(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespK8sEvents(events)
}

func handleK8sPodLogs(ctx context.Context, st *State, namespace, name, container string, tail uint32) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	if container == "" {
		// The API server's error for this is an opaque 400; name the choices.
		pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return k8sError(err)
		}
		if names := podContainerNames(pod); len(names) > 1 {
			return protocol.RespError(400, fmt.Sprintf("pod %s/%s has %d containers; pick one of: %s",
				namespace, name, len(names), strings.Join(names, ", ")))
		}
	}
	out, err := podLogs(ctx, cs, namespace, name, container, tail)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespText(out)
}

func handleK8sDeletePod(ctx context.Context, st *State, namespace, name string) protocol.Response {
	cs, err := st.K8s.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return k8sError(err)
	}
	if reason := st.Self.podProtection(namespace, name, pod.Labels); reason != "" {
		return protectedError(namespace+"/"+name, reason)
	}
	if err := cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return k8sError(err)
	}
	return protocol.RespOk()
}

func k8sUnavailable(err error) protocol.Response {
	return protocol.RespError(503, "Kubernetes is not available on this agent: "+err.Error())
}

// k8sError keeps the API server's meaning: not found stays 404, and a missing
// RBAC permission is a 403 that names what the agent's ClusterRole lacks.
func k8sError(err error) protocol.Response {
	switch {
	case apierrors.IsNotFound(err):
		return protocol.RespError(404, err.Error())
	case apierrors.IsForbidden(err):
		return protocol.RespError(403, "the agent's ServiceAccount isn't allowed to do this "+
			"(update its ClusterRole from deploy/kubernetes): "+err.Error())
	case apierrors.IsBadRequest(err), apierrors.IsInvalid(err):
		return protocol.RespError(400, err.Error())
	}
	return protocol.RespError(500, err.Error())
}

// ── Operations ───────────────────────────────────────────────────────────────

func listPods(ctx context.Context, cs *kubernetes.Clientset, self Self) ([]protocol.PodInfo, error) {
	list, err := cs.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.PodInfo, 0, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		info := mapPod(pod)
		info.ProtectedReason = self.podProtection(pod.Namespace, pod.Name, pod.Labels)
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

func listNodes(ctx context.Context, cs kubernetes.Interface) ([]protocol.NodeInfo, error) {
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

func listServices(ctx context.Context, cs kubernetes.Interface) ([]protocol.ServiceInfo, error) {
	list, err := cs.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ServiceInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, mapService(&list.Items[i]))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func mapService(svc *corev1.Service) protocol.ServiceInfo {
	info := protocol.ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      string(svc.Spec.Type),
		ClusterIP: svc.Spec.ClusterIP,
		Ports:     make([]protocol.ServicePort, 0, len(svc.Spec.Ports)),
		Selector:  svc.Spec.Selector,
		Created:   timestamp(svc.CreationTimestamp),
	}
	if info.Type == "" {
		info.Type = string(corev1.ServiceTypeClusterIP)
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.Hostname != "" {
			info.External = append(info.External, ing.Hostname)
		} else if ing.IP != "" {
			info.External = append(info.External, ing.IP)
		}
	}
	info.External = append(info.External, svc.Spec.ExternalIPs...)
	if svc.Spec.ExternalName != "" {
		info.External = append(info.External, svc.Spec.ExternalName)
	}
	for _, p := range svc.Spec.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		info.Ports = append(info.Ports, protocol.ServicePort{
			Name: p.Name, Protocol: proto, Port: p.Port,
			TargetPort: p.TargetPort.String(), NodePort: p.NodePort,
		})
	}
	return info
}

// maxEvents caps the events returned: the newest, which are the ones worth reading.
const maxEvents = 300

func listEvents(ctx context.Context, cs kubernetes.Interface) ([]protocol.EventInfo, error) {
	list, err := cs.CoreV1().Events(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	type seen struct {
		info protocol.EventInfo
		at   time.Time
	}
	all := make([]seen, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		at := eventTime(e)
		count := e.Count
		if e.Series != nil && e.Series.Count > count {
			count = e.Series.Count
		}
		if count == 0 {
			count = 1
		}
		source := e.Source.Component
		if source == "" {
			source = e.ReportingController
		}
		all = append(all, seen{at: at, info: protocol.EventInfo{
			Namespace: e.Namespace, Type: e.Type, Reason: e.Reason, Message: strings.TrimSpace(e.Message),
			ObjectKind: e.InvolvedObject.Kind, ObjectName: e.InvolvedObject.Name,
			Count: count, LastSeen: timestamp(metav1.NewTime(at)), Source: source,
		}})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].at.After(all[j].at) })
	if len(all) > maxEvents {
		all = all[:maxEvents]
	}
	result := make([]protocol.EventInfo, len(all))
	for i, e := range all {
		result[i] = e.info
	}
	return result, nil
}

// eventTime is when an event last happened; which field holds it depends on
// whether it was written through the old core/v1 or the newer events.k8s.io API.
func eventTime(e *corev1.Event) time.Time {
	switch {
	case e.Series != nil && !e.Series.LastObservedTime.IsZero():
		return e.Series.LastObservedTime.Time
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	}
	return e.CreationTimestamp.Time
}

// maxScaleReplicas caps a scale request so a typo can't ask for 10000 pods.
const maxScaleReplicas = 100

// workload is the part of a Deployment/StatefulSet/DaemonSet the handlers need.
type workload struct {
	info           protocol.WorkloadInfo
	templateLabels map[string]string
}

func getWorkload(ctx context.Context, cs kubernetes.Interface, namespace, kind, name string) (workload, error) {
	apps := cs.AppsV1()
	switch kind {
	case protocol.WorkloadDeployment:
		d, err := apps.Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return workload{}, err
		}
		return deploymentWorkload(d), nil
	case protocol.WorkloadStatefulSet:
		ss, err := apps.StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return workload{}, err
		}
		return statefulSetWorkload(ss), nil
	case protocol.WorkloadDaemonSet:
		ds, err := apps.DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return workload{}, err
		}
		return daemonSetWorkload(ds), nil
	}
	return workload{}, apierrors.NewBadRequest(fmt.Sprintf("unknown workload kind %q (want Deployment, StatefulSet or DaemonSet)", kind))
}

func listWorkloads(ctx context.Context, cs kubernetes.Interface) ([]protocol.WorkloadInfo, error) {
	apps := cs.AppsV1()
	var out []workload
	deps, err := apps.Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range deps.Items {
		out = append(out, deploymentWorkload(&deps.Items[i]))
	}
	sets, err := apps.StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range sets.Items {
		out = append(out, statefulSetWorkload(&sets.Items[i]))
	}
	daemons, err := apps.DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range daemons.Items {
		out = append(out, daemonSetWorkload(&daemons.Items[i]))
	}

	result := make([]protocol.WorkloadInfo, 0, len(out))
	for _, w := range out {
		w.info.ProtectedReason = labelProtection(w.templateLabels)
		w.info.Protected = w.info.ProtectedReason != ""
		result = append(result, w.info)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	return result, nil
}

func deploymentWorkload(d *appsv1.Deployment) workload {
	return workload{
		info: protocol.WorkloadInfo{
			Kind: protocol.WorkloadDeployment, Name: d.Name, Namespace: d.Namespace,
			Replicas: replicasOrOne(d.Spec.Replicas), Ready: d.Status.ReadyReplicas,
			Updated: d.Status.UpdatedReplicas, Available: d.Status.AvailableReplicas,
			Images: templateImages(d.Spec.Template), Created: timestamp(d.CreationTimestamp),
		},
		templateLabels: d.Spec.Template.Labels,
	}
}

func statefulSetWorkload(ss *appsv1.StatefulSet) workload {
	return workload{
		info: protocol.WorkloadInfo{
			Kind: protocol.WorkloadStatefulSet, Name: ss.Name, Namespace: ss.Namespace,
			Replicas: replicasOrOne(ss.Spec.Replicas), Ready: ss.Status.ReadyReplicas,
			Updated: ss.Status.UpdatedReplicas, Available: ss.Status.AvailableReplicas,
			Images: templateImages(ss.Spec.Template), Created: timestamp(ss.CreationTimestamp),
		},
		templateLabels: ss.Spec.Template.Labels,
	}
}

func daemonSetWorkload(ds *appsv1.DaemonSet) workload {
	return workload{
		info: protocol.WorkloadInfo{
			Kind: protocol.WorkloadDaemonSet, Name: ds.Name, Namespace: ds.Namespace,
			Replicas: ds.Status.DesiredNumberScheduled, Ready: ds.Status.NumberReady,
			Updated: ds.Status.UpdatedNumberScheduled, Available: ds.Status.NumberAvailable,
			Images: templateImages(ds.Spec.Template), Created: timestamp(ds.CreationTimestamp),
		},
		templateLabels: ds.Spec.Template.Labels,
	}
}

// replicasOrOne applies the API default: an unset replica count means 1.
func replicasOrOne(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}

func templateImages(t corev1.PodTemplateSpec) []string {
	images := make([]string, 0, len(t.Spec.Containers))
	for _, c := range t.Spec.Containers {
		images = append(images, c.Image)
	}
	return images
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
func podLogs(ctx context.Context, cs kubernetes.Interface, namespace, name, container string, tail uint32) (string, error) {
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

// scale sets a Deployment's or StatefulSet's replica count.
func scale(ctx context.Context, cs kubernetes.Interface, namespace, kind, name string, replicas int32) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	var err error
	switch kind {
	case protocol.WorkloadDeployment:
		_, err = cs.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	case protocol.WorkloadStatefulSet:
		_, err = cs.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	}
	return err
}

// rollout triggers a rollout restart by stamping the standard
// kubectl.kubernetes.io/restartedAt pod-template annotation, as
// `kubectl rollout restart` does.
func rollout(ctx context.Context, cs kubernetes.Interface, namespace, kind, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		now,
	))
	apps := cs.AppsV1()
	var err error
	switch kind {
	case protocol.WorkloadDeployment:
		_, err = apps.Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	case protocol.WorkloadStatefulSet:
		_, err = apps.StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	case protocol.WorkloadDaemonSet:
		_, err = apps.DaemonSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	}
	return err
}
