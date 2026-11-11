package agent

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Agent identity: what the gateway keys this agent's environment by.
//
// The agent represents something that outlives its process — a Kubernetes
// cluster or a container daemon — so its identity comes from that thing, not
// from configuration. Restarts, redeploys and reinstalls then land on the same
// environment, and nobody has to invent an id:
//
//  1. PULSE_INSTANCE_ID, if set (escape hatch for unusual setups).
//  2. In a pod: the cluster's id (the kube-system namespace UID).
//  3. The container daemon's id (`docker info` ID), for a Docker/Podman host.
//  4. A kubeconfig-reached cluster's id, for an agent with no local daemon.
//  5. Otherwise a random per-process id, retried on every (re)connect so the
//     agent moves to a stable id as soon as a runtime becomes reachable.
//
// One agent per cluster/daemon: two agents that resolve to the same id are the
// same environment, and the newest connection takes it over.

const identityProbeTimeout = 3 * time.Second

// identity caches the resolved instance id. Once stable it never changes for
// the life of the process.
type identity struct {
	mu       sync.Mutex
	stable   string
	fallback string
}

// identitySources are the probes resolveIdentity consults, injectable for tests.
type identitySources struct {
	explicit  string
	inCluster bool
	k8sID     func(context.Context) (string, error)
	dockerID  func(context.Context) (string, error)
}

// resolveIdentity applies the precedence above. ok=false means no stable
// source answered.
func resolveIdentity(ctx context.Context, src identitySources) (id string, ok bool) {
	if src.explicit != "" {
		return src.explicit, true
	}
	try := func(prefix string, probe func(context.Context) (string, error)) (string, bool) {
		pctx, cancel := context.WithTimeout(ctx, identityProbeTimeout)
		defer cancel()
		v, err := probe(pctx)
		v = sanitizeID(v)
		if err != nil || v == "" {
			return "", false
		}
		return prefix + v, true
	}
	if src.inCluster {
		if id, ok := try("k8s-", src.k8sID); ok {
			return id, true
		}
	}
	if id, ok := try("docker-", src.dockerID); ok {
		return id, true
	}
	if !src.inCluster {
		if id, ok := try("k8s-", src.k8sID); ok {
			return id, true
		}
	}
	return "", false
}

// sanitizeID lowercases and drops the colons older Docker daemon ids contain
// ("ABCD:EFGH:…"), since the gateway joins token and instance with ':'.
func sanitizeID(v string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(v), ":", ""))
}

// InstanceID returns this agent's identity, resolving it on first use and
// retrying while only the random fallback is available.
func (st *State) InstanceID(ctx context.Context) string {
	st.id.mu.Lock()
	defer st.id.mu.Unlock()
	if st.id.stable != "" {
		return st.id.stable
	}
	if id, ok := resolveIdentity(ctx, st.identitySources()); ok {
		if st.id.fallback != "" {
			log.Printf("Agent identity is now %s (was temporary %s until a runtime became reachable)", id, st.id.fallback)
		}
		st.id.stable = id
		return id
	}
	if st.id.fallback == "" {
		st.id.fallback = uuid.NewString()
	}
	return st.id.fallback
}

func (st *State) identitySources() identitySources {
	return identitySources{
		explicit:  st.Cfg.InstanceID,
		inCluster: st.Self.PodNamespace != "",
		k8sID: func(ctx context.Context) (string, error) {
			cs, err := st.K8s.Client()
			if err != nil {
				return "", err
			}
			ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			return string(ns.UID), nil
		},
		dockerID: func(ctx context.Context) (string, error) {
			cli, err := st.Docker.ClientFor(st.Cfg.DockerHost)
			if err != nil {
				return "", err
			}
			info, err := cli.Info(ctx)
			if err != nil {
				return "", err
			}
			return info.ID, nil
		},
	}
}
