package agent

// State is the agent's shared runtime, passed to every request handler.
type State struct {
	Cfg    Config
	Docker *DockerManager
	K8s    *K8s
}

// NewState builds the agent state from config.
func NewState(cfg Config) *State {
	return &State{
		Cfg:    cfg,
		Docker: NewDockerManager(),
		K8s:    NewK8s(cfg.KubeconfigYAML),
	}
}
