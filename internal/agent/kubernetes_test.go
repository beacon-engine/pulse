package agent

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
