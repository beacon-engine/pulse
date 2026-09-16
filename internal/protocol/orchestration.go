// Ports protocol/src/data/orchestration.rs — Docker Swarm and Kubernetes models.
package protocol

type SwarmService struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	Replicas     uint64 `json:"replicas"`
	Running      uint64 `json:"running"`
	DesiredState string `json:"desired_state"`
}

type SwarmNode struct {
	ID           string `json:"id"`
	Hostname     string `json:"hostname"`
	Role         string `json:"role"`
	State        string `json:"state"`
	Availability string `json:"availability"`
	Addr         string `json:"addr"`
}

// SwarmTask is a single task (running container) of a swarm service.
type SwarmTask struct {
	ID          string `json:"id"`
	ServiceID   string `json:"service_id"`
	NodeID      string `json:"node_id"`
	Slot        int64  `json:"slot"`
	ContainerID string `json:"container_id"`
	Image       string `json:"image"`
	State       string `json:"state"`
}

type PodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	// running | waiting | terminated | unknown
	State string `json:"state"`
}

type PodInfo struct {
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace"`
	Status     string         `json:"status"`
	Ready      bool           `json:"ready"`
	Containers []PodContainer `json:"containers"`
}

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ScaleQuery is a gateway-side query for scaling a swarm service.
type ScaleQuery struct {
	TargetReplicas *uint64 `json:"target_replicas"`
	Token          *string `json:"token"`
}
