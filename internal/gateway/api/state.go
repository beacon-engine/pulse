// Package api is the gateway's HTTP API: the handlers behind every dashboard
// action, the routes, and the static dashboard itself.
package api

import (
	"net/http"

	"github.com/beacon-engine/pulse/internal/gateway/agents"
	"github.com/beacon-engine/pulse/internal/gateway/events"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
)

// State is what the handlers share.
type State struct {
	// Agent routes each request to the right transport (pull client or a live
	// push session).
	Agent *agents.Router
	// AgentWS accepts push agents on /agent/ws.
	AgentWS http.Handler
	// DB is the gateway-owned database: hosts, stacks, webhook tokens, events, users.
	DB storage.Repository
	// Events fans out live events to connected UI WebSocket clients.
	Events *events.Hub
	// Notifier records container lifecycle events and sends their alerts.
	Notifier *events.Notifier
	// StaticDir is the built frontend directory served as the SPA ("" → dev page).
	StaticDir string
}
