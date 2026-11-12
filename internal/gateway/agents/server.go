package agents

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/protocol"
	"github.com/coder/websocket"
)

// defaultDockerHost is the local daemon an auto-registered environment points at
// when the agent doesn't declare one.
const defaultDockerHost = "unix:///var/run/docker.sock"

// Server accepts push agents on /agent/ws.
type Server struct {
	Router *Router
	// Tokens maps a bearer token to its name (the token id).
	Tokens map[string]string
	DB     storage.Repository
	// OnEvent receives the Docker events an agent pushes over its session.
	OnEvent func(ctx context.Context, resp protocol.Response)
}

// ServeHTTP is the endpoint agents dial into (push mode). It authenticates
// the bearer token to a token id, optionally auto-registers the agent as an
// environment (when it declares an identity on the upgrade), then hands the
// socket to runSession for its lifetime. Ports the /agent/ws route +
// authenticate_agent from gateway/src/{main,api}.rs.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenID, ok := s.authenticate(r)
	if !ok {
		http.Error(w, "invalid or missing agent token", http.StatusUnauthorized)
		return
	}

	// Self-registration: an agent that declares an instance id on the upgrade is
	// auto-registered/updated as an environment keyed by "<token_id>:<instance>".
	// This lets duplicated agents that share one token each appear as their own
	// environment, and makes restarts/reconnects idempotent (upsert). Legacy
	// agents omit the header and keep the static behavior: routed by token id to
	// an environment an admin created.
	agentID := tokenID
	auto := false
	if instance := strings.TrimSpace(r.Header.Get("X-Pulse-Instance")); instance != "" {
		auto = true
		agentID = tokenID + ":" + instance
		name := strings.TrimSpace(r.Header.Get("X-Pulse-Name"))
		if name == "" {
			name = agentID
		}
		dockerHost := strings.TrimSpace(r.Header.Get("X-Pulse-Docker-Host"))
		if dockerHost == "" {
			dockerHost = defaultDockerHost
		}
		if _, err := s.DB.UpsertAutoEnvironment(r.Context(), agentID, name, dockerHost); err != nil {
			log.Printf("auto-register failed for agent %q: %v", agentID, err)
			http.Error(w, "registration failed", http.StatusInternalServerError)
			return
		}
		log.Printf("Agent %q auto-registered environment %q", agentID, name)
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	s.runSession(r.Context(), c, agentID, auto)
}

// authenticateAgent resolves the Authorization: Bearer <token> header to an agent
// id via the configured token map. Returns ok=false when absent or unknown.
func (s *Server) authenticate(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	id, ok := s.Tokens[strings.TrimSpace(strings.TrimPrefix(h, prefix))]
	return id, ok
}
