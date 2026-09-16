// Package protocol defines the shared wire vocabulary for the Pulse gateway <->
// agent protocol. It is the Go port of the Rust `protocol` crate; the JSON
// encoding is kept byte-for-byte compatible so a Go agent and the existing Rust
// gateway interoperate during the migration.
//
// This file ports protocol/src/data/container.rs — container, image, volume and
// network models.
package protocol

// ContainerInfo is the list-view of a container.
type ContainerInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Image  string  `json:"image"`
	Status string  `json:"status"`
	State  string  `json:"state"`
	Stack  *string `json:"stack"`
}

type ContainerPort struct {
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
	HostPort      string `json:"host_port"`
}

type ContainerMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

type ContainerDetail struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Image         string           `json:"image"`
	Created       string           `json:"created"`
	State         string           `json:"state"`
	StartedAt     string           `json:"started_at"`
	FinishedAt    string           `json:"finished_at"`
	RestartCount  int64            `json:"restart_count"`
	Ports         []ContainerPort  `json:"ports"`
	Mounts        []ContainerMount `json:"mounts"`
	Env           []string         `json:"env"`
	Cmd           []string         `json:"cmd"`
	NetworkMode   string           `json:"network_mode"`
	RestartPolicy string           `json:"restart_policy"`
}

// Stack is a group of containers detected from compose labels.
type Stack struct {
	Name     string          `json:"name"`
	Services []ContainerInfo `json:"services"`
}

// PortMap is a port-remapping input for container recreate.
type PortMap struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type ImageInfo struct {
	ID       string   `json:"id"`
	Tags     []string `json:"tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
	Dangling bool     `json:"dangling"`
}

type VolumeInfo struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	Created    string `json:"created"`
}

type HostNetwork struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Scope    string `json:"scope"`
	Internal bool   `json:"internal"`
}

type ContainerNetworkInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address"`
	Gateway    string `json:"gateway"`
	MacAddress string `json:"mac_address"`
}

type ContainerEvent struct {
	ID            string `json:"id"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Event         string `json:"event"`
	OccurredAt    string `json:"occurred_at"`
}
