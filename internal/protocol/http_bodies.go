// Bodies of the gateway's HTTP API: what the dashboard sends (ported from
// protocol/src/protocol/request.rs) and the envelopes some endpoints return
// (from response.rs).
package protocol

type CreateHostBody struct {
	Name       string  `json:"name"`
	DockerHost string  `json:"docker_host"`
	AgentID    *string `json:"agent_id"`
	AgentAddr  *string `json:"agent_addr"`
}

type RenameHostBody struct {
	Name string `json:"name"`
}

type LogsQuery struct {
	Tail *uint32 `json:"tail"`
}

type RecreateBody struct {
	Ports []PortMap `json:"ports"`
}

type EnvBody struct {
	Env []string `json:"env"`
}

type ExecBody struct {
	Cmd []string `json:"cmd"`
}

type NetworkBody struct {
	NetworkID string `json:"network_id"`
}

type CreateStackBody struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UpdateStackBody struct {
	Content string   `json:"content"`
	EnvVars []EnvVar `json:"env_vars"`
}

// ScaleQuery is a gateway-side query for scaling a swarm service.
type ScaleQuery struct {
	TargetReplicas *uint64 `json:"target_replicas"`
	Token          *string `json:"token"`
}

type StackOpResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output string `json:"output"`
}

type TokenResponse struct {
	Token string `json:"token"`
}
