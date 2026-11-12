// Kubernetes models: pods and their containers, workloads, namespaces, nodes,
// services and events.

package protocol

type PodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	// running | waiting | terminated | unknown
	State string `json:"state"`
	// Reason explains a waiting or terminated state (CrashLoopBackOff,
	// ContainerCreating, Completed, OOMKilled, …); Message is the detail.
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	// ExitCode is set when the container has terminated.
	ExitCode *int32 `json:"exit_code,omitempty"`
	// Started is RFC 3339: when the current run began (running) or ended (terminated).
	Started string `json:"started,omitempty"`
	// LastReason/LastExitCode describe the previous run, e.g. why it restarted.
	LastReason   string `json:"last_reason,omitempty"`
	LastExitCode *int32 `json:"last_exit_code,omitempty"`
	// ContainerID is the runtime's ID, e.g. "docker://3f2a…" or "containerd://…".
	ContainerID string        `json:"container_id,omitempty"`
	Ports       []PodPort     `json:"ports,omitempty"`
	Resources   *PodResources `json:"resources,omitempty"`
}

// PodPort is a port a container declares in the pod spec.
type PodPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// PodResources are a container's requests and limits, as Kubernetes
// quantities ("100m", "64Mi"); empty when unset.
type PodResources struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Status is the pod phase (Pending, Running, Succeeded, Failed, Unknown).
	Status string `json:"status"`
	// Reason is what `kubectl get pods` shows when it differs from the phase:
	// CrashLoopBackOff, ImagePullBackOff, Terminating, Completed, …
	Reason     string         `json:"reason,omitempty"`
	Ready      bool           `json:"ready"`
	Containers []PodContainer `json:"containers"`
	Node       string         `json:"node,omitempty"`
	IP         string         `json:"ip,omitempty"`
	// Created is RFC 3339; empty when the API didn't report it.
	Created string `json:"created,omitempty"`
	// Owner is the workload that manages the pod (a ReplicaSet is resolved to
	// its Deployment). Nil for bare pods.
	Owner  *WorkloadRef      `json:"owner,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	// QOSClass is Guaranteed, Burstable or BestEffort.
	QOSClass string `json:"qos_class,omitempty"`
	// Protected marks Pulse's own pods; the agent refuses to delete them.
	Protected       bool   `json:"protected,omitempty"`
	ProtectedReason string `json:"protected_reason,omitempty"`
}

// Kubernetes workload kinds Pulse can list, scale and restart.
const (
	WorkloadDeployment  = "Deployment"
	WorkloadStatefulSet = "StatefulSet"
	WorkloadDaemonSet   = "DaemonSet"
)

type WorkloadRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// WorkloadInfo is a Deployment, StatefulSet or DaemonSet. For a DaemonSet,
// Replicas is the number of nodes it should run on and it can't be scaled.
type WorkloadInfo struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Replicas  int32    `json:"replicas"`
	Ready     int32    `json:"ready"`
	Updated   int32    `json:"updated"`
	Available int32    `json:"available"`
	Images    []string `json:"images"`
	Created   string   `json:"created,omitempty"`
	// Protected marks Pulse's own workloads; the agent refuses to scale or restart them.
	Protected       bool   `json:"protected,omitempty"`
	ProtectedReason string `json:"protected_reason,omitempty"`
}

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// NodeInfo is a cluster node. CPU/Memory/Pods are capacity; the Alloc*
// fields are what is left for pods after system reservations.
type NodeInfo struct {
	Name string `json:"name"`
	// Status is Ready, NotReady or Unknown.
	Status        string   `json:"status"`
	Unschedulable bool     `json:"unschedulable,omitempty"`
	Roles         []string `json:"roles"`
	// Version is the kubelet version.
	Version          string `json:"version"`
	OSImage          string `json:"os_image"`
	KernelVersion    string `json:"kernel_version"`
	Arch             string `json:"arch"`
	ContainerRuntime string `json:"container_runtime"`
	InternalIP       string `json:"internal_ip,omitempty"`
	CPU              string `json:"cpu"`
	Memory           string `json:"memory"`
	Pods             string `json:"pods"`
	AllocCPU         string `json:"alloc_cpu"`
	AllocMemory      string `json:"alloc_memory"`
	// Pressure lists the node conditions that are on (MemoryPressure, DiskPressure, …).
	Pressure []string `json:"pressure,omitempty"`
	Created  string   `json:"created,omitempty"`
}

type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	NodePort   int32  `json:"node_port,omitempty"`
}

type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Type is ClusterIP, NodePort, LoadBalancer or ExternalName.
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip,omitempty"`
	// External holds load balancer ingress addresses, external IPs, or the
	// ExternalName target.
	External []string          `json:"external,omitempty"`
	Ports    []ServicePort     `json:"ports"`
	Selector map[string]string `json:"selector,omitempty"`
	Created  string            `json:"created,omitempty"`
}

// EventInfo is a Kubernetes event: what happened to an object and why.
type EventInfo struct {
	Namespace string `json:"namespace"`
	// Type is Normal or Warning.
	Type       string `json:"type"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	ObjectKind string `json:"object_kind"`
	ObjectName string `json:"object_name"`
	Count      int32  `json:"count"`
	// LastSeen is RFC 3339.
	LastSeen string `json:"last_seen,omitempty"`
	Source   string `json:"source,omitempty"`
}
