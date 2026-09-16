// Ports protocol/src/data/system.rs — host system and hardware metrics.
package protocol

type SystemInfo struct {
	DockerVersion     string `json:"docker_version"`
	APIVersion        string `json:"api_version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	Kernel            string `json:"kernel"`
	TotalMemory       int64  `json:"total_memory"`
	CPUs              int64  `json:"cpus"`
	ContainersRunning int64  `json:"containers_running"`
	ContainersStopped int64  `json:"containers_stopped"`
	Images            int64  `json:"images"`
}

type DiskInfo struct {
	Name  string  `json:"name"`
	Mount string  `json:"mount"`
	Total uint64  `json:"total"`
	Used  uint64  `json:"used"`
	Pct   float32 `json:"pct"`
}

type HardwareMetrics struct {
	MemUsed  uint64     `json:"mem_used"`
	MemTotal uint64     `json:"mem_total"`
	MemPct   float32    `json:"mem_pct"`
	Disks    []DiskInfo `json:"disks"`
}
