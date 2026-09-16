// Ports protocol/src/host.rs — persisted host and stack records.
package protocol

type Environment struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DockerHost string `json:"docker_host"`
	IsLocal    bool   `json:"is_local"`
	CreatedAt  string `json:"created_at"`
	// AgentID is the push agent that represents this environment; nil routes to
	// the default pull agent. See the gateway's AgentRouter.
	AgentID *string `json:"agent_id"`
}

type SavedStack struct {
	ID           string  `json:"id"`
	HostID       string  `json:"host_id"`
	Name         string  `json:"name"`
	Content      string  `json:"content"`
	EnvVars      *string `json:"env_vars"`
	WebhookToken *string `json:"webhook_token"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}
