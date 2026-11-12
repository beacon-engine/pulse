package api

import "net/http"

// Ports gateway/src/api/events.rs and the list_agents handler from api.rs.

// handleListEvents serves recent container lifecycle events from the DB.
func (st *State) handleListEvents(w http.ResponseWriter, r *http.Request) {
	evs, err := st.DB.GetRecentEvents(r.Context(), 100)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(evs))
}

// handleListAgents lists agents currently connected via a push session (the
// default pull agent is always reachable and not listed).
func (st *State) handleListAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, orEmpty(st.Agent.ConnectedAgentIDs()))
}

// handleHealth and handleStatus are trivial liveness endpoints.
func (st *State) handleHealth(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func (st *State) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "running",
		"version": "0.1.0",
	})
}
