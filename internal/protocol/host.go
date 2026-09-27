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
	// AgentAddr is a per-host pull agent endpoint (host:port). When set, the
	// gateway dials this agent for this environment instead of the default
	// AGENT_ADDR. nil uses the default.
	AgentAddr *string `json:"agent_addr"`
	// Source is "manual" (created via the API or the seeded Local host) or
	// "auto" (self-registered by a dial-in agent). Only "auto" rows go through
	// the liveness lifecycle below.
	Source string `json:"source"`
	// Status is "online" or "offline" for auto rows; manual rows stay "online".
	Status string `json:"status"`
	// LastSeen is the RFC3339 UTC time of the last connect/heartbeat (auto rows).
	LastSeen *string `json:"last_seen"`
	// OfflineSince is when an auto row was marked offline; nil while online.
	OfflineSince *string `json:"offline_since"`
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
