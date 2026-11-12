package kubernetes

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/beacon-engine/pulse/internal/agent/selfguard"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Dispatch handlers ────────────────────────────────────────────────────────

func Pods(ctx context.Context, c *Cluster, self selfguard.Self) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	pods, err := listPods(ctx, cs, self)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespPods(pods)
}

func Namespaces(ctx context.Context, c *Cluster) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	ns, err := listNamespaces(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespNamespaces(ns)
}

func Rollout(ctx context.Context, c *Cluster, namespace, deployment string) protocol.Response {
	return Restart(ctx, c, namespace, protocol.WorkloadDeployment, deployment)
}

// Restart does a rolling restart (`kubectl rollout restart`) of a
// Deployment, StatefulSet or DaemonSet.
func Restart(ctx context.Context, c *Cluster, namespace, kind, name string) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	w, err := getWorkload(ctx, cs, namespace, kind, name)
	if err != nil {
		return k8sError(err)
	}
	// A restart replaces every pod, so judge by the pod template's labels.
	if reason := selfguard.LabelProtection(w.templateLabels); reason != "" {
		return selfguard.ProtectedError(namespace+"/"+name, reason)
	}
	if err := rollout(ctx, cs, namespace, kind, name); err != nil {
		return k8sError(err)
	}
	return protocol.RespOk()
}

func Scale(ctx context.Context, c *Cluster, namespace, kind, name string, replicas uint64) protocol.Response {
	if kind != protocol.WorkloadDeployment && kind != protocol.WorkloadStatefulSet {
		return protocol.RespError(400, kind+" can't be scaled: only Deployments and StatefulSets have a replica count")
	}
	if replicas > maxScaleReplicas {
		return protocol.RespError(400, fmt.Sprintf("replicas must be between 0 and %d", maxScaleReplicas))
	}
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	w, err := getWorkload(ctx, cs, namespace, kind, name)
	if err != nil {
		return k8sError(err)
	}
	// Scaling the agent to 0 cuts the connection; scaling it up starts agents
	// with the same cluster identity that would keep taking over each other's session.
	if reason := selfguard.LabelProtection(w.templateLabels); reason != "" {
		return selfguard.ProtectedError(namespace+"/"+name, reason)
	}
	if err := scale(ctx, cs, namespace, kind, name, int32(replicas)); err != nil {
		return k8sError(err)
	}
	return protocol.RespOk()
}

func Workloads(ctx context.Context, c *Cluster) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	ws, err := listWorkloads(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespWorkloads(ws)
}

func Nodes(ctx context.Context, c *Cluster) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	nodes, err := listNodes(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespNodes(nodes)
}

func Services(ctx context.Context, c *Cluster) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	svcs, err := listServices(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespServices(svcs)
}

func Events(ctx context.Context, c *Cluster) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	events, err := listEvents(ctx, cs)
	if err != nil {
		return k8sError(err)
	}
	return protocol.RespK8sEvents(events)
}

func PodLogs(ctx context.Context, c *Cluster, namespace, name, container string, tail uint32) protocol.Response {
	cs, err := c.Client()
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

func DeletePod(ctx context.Context, c *Cluster, self selfguard.Self, namespace, name string) protocol.Response {
	cs, err := c.Client()
	if err != nil {
		return k8sUnavailable(err)
	}
	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return k8sError(err)
	}
	if reason := self.PodProtection(namespace, name, pod.Labels); reason != "" {
		return selfguard.ProtectedError(namespace+"/"+name, reason)
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
