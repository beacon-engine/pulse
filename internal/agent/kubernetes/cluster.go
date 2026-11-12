// Package kubernetes runs the agent's Kubernetes operations through client-go:
// workloads, pods and their containers, namespaces, nodes, services and events.

package kubernetes

import (
	"errors"
	"sync"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Ports agent/src/kubernetes.rs — Kubernetes access via client-go.

// Cluster lazily connects and caches a clientset, mirroring the Rust agent's
// k8s_client: it connects on first use (or after a failed attempt) so a
// kubeconfig/ServiceAccount that appears after startup is picked up without a
// restart. kubeconfigYAML == "" means auto-discover (in-pod ServiceAccount, or
// the host's ~/.kube/config).
type Cluster struct {
	kubeconfigYAML string
	mu             sync.Mutex
	client         *clientset.Clientset
	host           string
}

// NewCluster builds a lazy Kubernetes accessor for the given target.
func NewCluster(kubeconfigYAML string) *Cluster {
	return &Cluster{kubeconfigYAML: kubeconfigYAML}
}

// Client returns a connected clientset, connecting and caching on first use.
func (k *Cluster) Client() (*clientset.Clientset, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.client != nil {
		return k.client, nil
	}
	cfg, err := k.restConfig()
	if err != nil {
		return nil, err
	}
	cs, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	k.client = cs
	k.host = cfg.Host
	return cs, nil
}

// Host is the API server URL of the connected cluster ("" before connecting).
func (k *Cluster) Host() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.host
}

// restConfig resolves the REST config: explicit kubeconfig YAML, else the in-pod
// ServiceAccount, else the default kubeconfig loading rules (~/.kube/config or
// $KUBECONFIG).
func (k *Cluster) restConfig() (*rest.Config, error) {
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
