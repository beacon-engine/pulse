// Runtime capabilities — what an agent can actually manage for an environment.
//
// An environment is not "a Docker host": it is whatever runtimes its agent can
// reach. A Kubernetes-only agent has no container daemon; a Podman host speaks
// the Docker-compatible API but has no Swarm. The gateway and UI ask the agent
// (ReqCapabilities) and only offer the views a runtime backs.
package protocol

// Runtime kinds. Docker and Podman are both served by the agent's
// Docker-API-compatible client; containerd will be its own kind.
const (
	RuntimeDocker     = "docker"
	RuntimePodman     = "podman"
	RuntimeKubernetes = "kubernetes"
)

// Runtime features, reported alongside a runtime kind.
const (
	FeatureSwarm   = "swarm"   // Docker Swarm is active on this daemon
	FeatureCompose = "compose" // compose stacks can be deployed through it
)

// RuntimeInfo is one runtime the agent reached.
type RuntimeInfo struct {
	Kind     string   `json:"kind"`
	Version  string   `json:"version"`
	Endpoint string   `json:"endpoint,omitempty"`
	Features []string `json:"features"`
}

// RuntimeProbe records a runtime the agent looked for but could not reach, so
// the UI can say why a view is missing rather than just hiding it.
type RuntimeProbe struct {
	Kind  string `json:"kind"`
	Error string `json:"error"`
}

// Capabilities is the agent's answer to ReqCapabilities.
type Capabilities struct {
	Runtimes    []RuntimeInfo  `json:"runtimes"`
	Unavailable []RuntimeProbe `json:"unavailable"`
}

// HasContainerRuntime reports whether a Docker-API-compatible runtime is present.
func (c Capabilities) HasContainerRuntime() bool {
	for _, r := range c.Runtimes {
		if r.Kind == RuntimeDocker || r.Kind == RuntimePodman {
			return true
		}
	}
	return false
}
