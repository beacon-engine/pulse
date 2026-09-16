package gateway

import "github.com/beacon-engine/pulse/internal/gateway/storage"

// State is the gateway's shared runtime (ports gateway/src/state.rs).
type State struct {
	// Agent routes each request to the right transport (pull client or a live
	// push session).
	Agent *AgentRouter
	// AgentTokens maps bearer token -> agent_id for authenticating push agents.
	AgentTokens map[string]string
	// DB is the gateway-owned database: hosts, stacks, webhook tokens, events, users.
	DB storage.Repository
	// Events fans out live events to connected UI WebSocket clients.
	Events *Hub
	// SlackWebhook is an optional incoming-webhook URL for lifecycle alerts.
	SlackWebhook string
	// StaticDir is the built frontend directory served as the SPA ("" → dev page).
	StaticDir string
}

// NewState builds the gateway state from config and an opened database.
func NewState(cfg Config, db storage.Repository) *State {
	return &State{
		Agent:        NewAgentRouter(NewAgentClient(cfg.AgentAddr)),
		AgentTokens:  cfg.AgentTokens,
		DB:           db,
		Events:       NewHub(),
		SlackWebhook: cfg.SlackWebhookURL,
		StaticDir:    cfg.StaticDir,
	}
}
