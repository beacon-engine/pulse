package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/beacon-engine/pulse/internal/protocol"
)

func TestMapPodRunningReady(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				Image:        "nginx:1.27",
				Ready:        true,
				RestartCount: 3,
				State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}

	info := mapPod(pod)
	if info.Name != "web" || info.Namespace != "prod" {
		t.Errorf("identity: %+v", info)
	}
	if info.Status != "Running" || !info.Ready {
		t.Errorf("status=%q ready=%v, want Running/true", info.Status, info.Ready)
	}
	if len(info.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(info.Containers))
	}
	c := info.Containers[0]
	if c.Name != "app" || c.Image != "nginx:1.27" || !c.Ready || c.RestartCount != 3 || c.State != "running" {
		t.Errorf("container mapped wrong: %+v", c)
	}
}

func TestMapPodPendingWaitingDefaults(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Status: corev1.PodStatus{
			// Empty phase must default to "Unknown"; no Ready condition => not ready.
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "init",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			}},
		},
	}

	info := mapPod(pod)
	if info.Status != "Unknown" {
		t.Errorf("empty phase => %q, want Unknown", info.Status)
	}
	if info.Ready {
		t.Error("no Ready condition => should not be ready")
	}
	if info.Containers[0].State != "waiting" {
		t.Errorf("state = %q, want waiting", info.Containers[0].State)
	}
}

func TestMapPodOwnerAndPlacement(t *testing.T) {
	yes := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-85b9b8c477-m242l", Namespace: "demo",
			Labels:            map[string]string{"pod-template-hash": "85b9b8c477"},
			CreationTimestamp: metav1.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC),
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet", Name: "nginx-85b9b8c477", Controller: &yes,
			}},
		},
		Spec:   corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{PodIP: "10.0.0.7"},
	}
	info := mapPod(pod)
	if info.Owner == nil || info.Owner.Kind != "Deployment" || info.Owner.Name != "nginx" {
		t.Errorf("owner = %+v, want Deployment/nginx", info.Owner)
	}
	if info.Node != "node-a" || info.IP != "10.0.0.7" || info.Created != "2026-11-10T09:00:00Z" {
		t.Errorf("placement: %+v", info)
	}

	// A StatefulSet owns its pods directly.
	pod.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db", Controller: &yes}}
	if o := mapPod(pod).Owner; o == nil || o.Kind != "StatefulSet" || o.Name != "db" {
		t.Errorf("owner = %+v, want StatefulSet/db", o)
	}
	// A bare pod has no owner.
	pod.OwnerReferences = nil
	if o := mapPod(pod).Owner; o != nil {
		t.Errorf("bare pod owner = %+v, want nil", o)
	}
}

func TestListWorkloadsAllKinds(t *testing.T) {
	three := int32(3)
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod"},
			Spec:       appsv1.DeploymentSpec{Replicas: &three, Template: podTemplate("api:1", nil)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2, UpdatedReplicas: 3, AvailableReplicas: 2},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "prod"},
			Spec:       appsv1.StatefulSetSpec{Template: podTemplate("postgres:17", nil)}, // replicas unset => 1
		},
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "pulse-agent", Namespace: "pulse"},
			Spec:       appsv1.DaemonSetSpec{Template: podTemplate("pulse-agent", map[string]string{LabelRole: "agent"})},
			Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 4, NumberReady: 4},
		},
	)
	got, err := listWorkloads(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d workloads, want 3: %+v", len(got), got)
	}
	api, db, agent := got[0], got[1], got[2]
	if api.Kind != "Deployment" || api.Replicas != 3 || api.Ready != 2 || api.Images[0] != "api:1" || api.Protected {
		t.Errorf("deployment: %+v", api)
	}
	if db.Kind != "StatefulSet" || db.Replicas != 1 {
		t.Errorf("statefulset: %+v", db)
	}
	if agent.Kind != "DaemonSet" || agent.Replicas != 4 || !agent.Protected {
		t.Errorf("daemonset: %+v", agent)
	}
}

func TestScaleAndRolloutPatchTheWorkload(t *testing.T) {
	one := int32(1)
	cs := fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "prod"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &one, Template: podTemplate("postgres:17", nil)},
	})
	ctx := context.Background()
	if err := scale(ctx, cs, "prod", protocol.WorkloadStatefulSet, "db", 3); err != nil {
		t.Fatal(err)
	}
	if err := rollout(ctx, cs, "prod", protocol.WorkloadStatefulSet, "db"); err != nil {
		t.Fatal(err)
	}
	ss, err := cs.AppsV1().StatefulSets("prod").Get(ctx, "db", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if *ss.Spec.Replicas != 3 {
		t.Errorf("replicas = %d, want 3", *ss.Spec.Replicas)
	}
	if ss.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] == "" {
		t.Error("rollout did not stamp restartedAt")
	}
}

func TestScaleRejectsDaemonSetsAndHugeCounts(t *testing.T) {
	st := &State{} // validation runs before the cluster is touched
	ctx := context.Background()
	if resp := handleK8sScale(ctx, st, "ns", protocol.WorkloadDaemonSet, "x", 2); resp.Kind != protocol.RespKindError {
		t.Errorf("DaemonSet scale = %s, want error", resp.Kind)
	}
	if status, _, _ := handleK8sScale(ctx, st, "ns", protocol.WorkloadDeployment, "x", maxScaleReplicas+1).AsError(); status != 400 {
		t.Errorf("huge scale status = %d, want 400", status)
	}
}

func podTemplate(image string, labels map[string]string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: image}}},
	}
}

func TestMapPodCrashLoopDetail(t *testing.T) {
	started := metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "flaky-1", Namespace: "demo", Labels: map[string]string{"app": "flaky"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:  "app",
			Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
			},
		}, {Name: "sidecar"}}},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				RestartCount: 4,
				ContainerID:  "docker://abc",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff", Message: "back-off 40s",
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					Reason: "Error", ExitCode: 1,
				}},
			}, {
				Name:  "sidecar",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: started}},
			}},
		},
	}

	info := mapPod(pod)
	if info.Reason != "CrashLoopBackOff" || info.QOSClass != "Burstable" || info.Labels["app"] != "flaky" {
		t.Errorf("pod: reason=%q qos=%q labels=%v", info.Reason, info.QOSClass, info.Labels)
	}
	app, side := info.Containers[0], info.Containers[1]
	if app.State != "waiting" || app.Reason != "CrashLoopBackOff" || app.Message != "back-off 40s" {
		t.Errorf("app state: %+v", app)
	}
	if app.LastReason != "Error" || app.LastExitCode == nil || *app.LastExitCode != 1 {
		t.Errorf("app last termination: %+v", app)
	}
	if len(app.Ports) != 1 || app.Ports[0].ContainerPort != 8080 || app.Ports[0].Protocol != "TCP" {
		t.Errorf("app ports: %+v", app.Ports)
	}
	if app.Resources == nil || app.Resources.MemoryLimit != "64Mi" || app.Resources.CPULimit != "" {
		t.Errorf("app resources: %+v", app.Resources)
	}
	if app.ContainerID != "docker://abc" {
		t.Errorf("container id = %q", app.ContainerID)
	}
	if side.Started != "2026-01-02T03:04:05Z" || side.Resources != nil || side.Ports != nil {
		t.Errorf("sidecar: %+v", side)
	}
}

func TestPodReason(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name string
		pod  corev1.Pod
		want string
	}{
		{"healthy", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}, ""},
		{"terminating", corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}}, "Terminating"},
		{"completed", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}},
		}}}}, "Completed"},
		{"init", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
		}}}}, "Init:ImagePullBackOff"},
		{"evicted", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"}}, "Evicted"},
	}
	for _, c := range cases {
		if got := podReason(&c.pod); got != c.want {
			t.Errorf("%s: podReason = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMapNode(t *testing.T) {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: map[string]string{
			"node-role.kubernetes.io/control-plane": "", "node-role.kubernetes.io/master": "", "kubernetes.io/os": "linux",
		}},
		Spec: corev1.NodeSpec{Unschedulable: true},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			Addresses: []corev1.NodeAddress{{Type: corev1.NodeHostName, Address: "n1"}, {Type: corev1.NodeInternalIP, Address: "192.168.1.5"}},
			Capacity:  corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourcePods: resource.MustParse("110")},
			NodeInfo:  corev1.NodeSystemInfo{KubeletVersion: "v1.31.1", Architecture: "arm64"},
		},
	}
	info := mapNode(n)
	if info.Status != "Ready" || !info.Unschedulable || info.InternalIP != "192.168.1.5" {
		t.Errorf("node: %+v", info)
	}
	if len(info.Roles) != 2 || info.Roles[0] != "control-plane" || info.Roles[1] != "master" {
		t.Errorf("roles = %v", info.Roles)
	}
	if len(info.Pressure) != 1 || info.Pressure[0] != "MemoryPressure" {
		t.Errorf("pressure = %v", info.Pressure)
	}
	if info.CPU != "4" || info.Pods != "110" || info.Version != "v1.31.1" || info.Arch != "arm64" {
		t.Errorf("capacity/info: %+v", info)
	}
}

func TestMapService(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeLoadBalancer,
			ClusterIP: "10.0.0.9",
			Selector:  map[string]string{"app": "web"},
			Ports: []corev1.ServicePort{{
				Name: "http", Port: 80, TargetPort: intstr.FromString("http"), NodePort: 31000,
			}},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb.example.com"}},
		}},
	}
	info := mapService(svc)
	if info.Type != "LoadBalancer" || info.ClusterIP != "10.0.0.9" || info.Selector["app"] != "web" {
		t.Errorf("service: %+v", info)
	}
	if len(info.External) != 1 || info.External[0] != "lb.example.com" {
		t.Errorf("external = %v", info.External)
	}
	p := info.Ports[0]
	if p.Port != 80 || p.TargetPort != "http" || p.NodePort != 31000 || p.Protocol != "TCP" {
		t.Errorf("port: %+v", p)
	}
}

func TestListEventsNewestFirstAndCapped(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var objs []corev1.Event
	for i := 0; i < maxEvents+5; i++ {
		objs = append(objs, corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: fmt.Sprintf("e%d", i), Namespace: "ns"},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "p"},
			Type:           "Normal",
			Reason:         "Pulled",
			LastTimestamp:  metav1.NewTime(base.Add(time.Duration(i) * time.Minute)),
		})
	}
	// A newer-API event: only EventTime and Series are set.
	objs = append(objs, corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "series", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "flaky"},
		Type:           "Warning",
		Reason:         "BackOff",
		EventTime:      metav1.NewMicroTime(base),
		Series:         &corev1.EventSeries{Count: 12, LastObservedTime: metav1.NewMicroTime(base.Add(24 * time.Hour))},
	})
	cs := fake.NewSimpleClientset()
	for i := range objs {
		if _, err := cs.CoreV1().Events("ns").Create(context.Background(), &objs[i], metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	events, err := listEvents(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != maxEvents {
		t.Fatalf("len = %d, want %d", len(events), maxEvents)
	}
	first := events[0]
	if first.Reason != "BackOff" || first.Count != 12 || first.ObjectName != "flaky" || first.LastSeen != "2026-01-02T00:00:00Z" {
		t.Errorf("newest event = %+v", first)
	}
	if events[1].LastSeen <= events[2].LastSeen {
		t.Errorf("not newest first: %s then %s", events[1].LastSeen, events[2].LastSeen)
	}
	if events[1].Count != 1 {
		t.Errorf("count without a count = %d, want 1", events[1].Count)
	}
}
