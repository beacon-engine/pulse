package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// maxScaleReplicas caps a scale request so a typo can't ask for 10000 pods.
const maxScaleReplicas = 100

// workload is the part of a Deployment/StatefulSet/DaemonSet the handlers need.
type workload struct {
	info           protocol.WorkloadInfo
	templateLabels map[string]string
}

func getWorkload(ctx context.Context, cs clientset.Interface, namespace, kind, name string) (workload, error) {
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

func listWorkloads(ctx context.Context, cs clientset.Interface) ([]protocol.WorkloadInfo, error) {
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
		w.info.ProtectedReason = selfguard.LabelProtection(w.templateLabels)
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

// scale sets a Deployment's or StatefulSet's replica count.
func scale(ctx context.Context, cs clientset.Interface, namespace, kind, name string, replicas int32) error {
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
func rollout(ctx context.Context, cs clientset.Interface, namespace, kind, name string) error {
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
