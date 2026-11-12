package agent

import (
	"github.com/beacon-engine/pulse/internal/agent/docker"
	"github.com/beacon-engine/pulse/internal/agent/kubernetes"
	"github.com/beacon-engine/pulse/internal/agent/selfguard"
)

// State is the agent's shared runtime, passed to every request handler.
type State struct {
	Cfg    Config
	Docker *docker.Manager
	K8s    *kubernetes.Cluster
	Self   selfguard.Self

	id identity
}

// NewState builds the agent state from config.
func NewState(cfg Config) *State {
	return &State{
		Cfg:    cfg,
		Docker: docker.NewManager(),
		K8s:    kubernetes.NewCluster(cfg.KubeconfigYAML),
		Self:   selfguard.Detect(),
	}
}
