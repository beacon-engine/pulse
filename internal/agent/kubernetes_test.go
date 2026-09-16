package agent

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
