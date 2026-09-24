// Ports protocol/src/protocol/request.rs — request payloads the gateway forwards
// to an agent (HTTP-API body types).
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
