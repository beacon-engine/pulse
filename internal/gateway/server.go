package gateway

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

// AgentWSHandler is the endpoint agents dial into (push mode). It authenticates
// the bearer token to an agent id, then hands the socket to runSession for its
// lifetime. Ports the /agent/ws route + authenticate_agent from
// gateway/src/{main,api}.rs.
func (st *State) AgentWSHandler(w http.ResponseWriter, r *http.Request) {
	agentID, ok := authenticateAgent(st, r)
	if !ok {
		http.Error(w, "invalid or missing agent token", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	runSession(r.Context(), c, st, agentID)
}

// authenticateAgent resolves the Authorization: Bearer <token> header to an agent
// id via the configured token map. Returns ok=false when absent or unknown.
func authenticateAgent(st *State, r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	id, ok := st.AgentTokens[strings.TrimSpace(strings.TrimPrefix(h, prefix))]
	return id, ok
}
