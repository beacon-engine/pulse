package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/stacks.rs — persisted stacks. Records live in the DB;
// up/down run `docker compose` on the agent.

func composeStack(s *protocol.SavedStack) *protocol.ComposeStack {
	return &protocol.ComposeStack{ID: s.ID, Name: s.Name, Content: s.Content, EnvVars: s.EnvVars}
}

// loadStack fetches a stack or writes 404/500 and returns nil.
func (st *State) loadStack(w http.ResponseWriter, r *http.Request, id string) *protocol.SavedStack {
	s, err := st.DB.GetStack(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return nil
	}
	if s == nil {
		writeErr(w, http.StatusNotFound, "stack "+id+" not found")
		return nil
	}
	return s
}

func (st *State) handleListStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := st.DB.ListStacks(r.Context(), r.PathValue("host_id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(stacks))
}

func (st *State) handleCreateStack(w http.ResponseWriter, r *http.Request) {
	var body protocol.CreateStackBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	id, err := st.DB.CreateStack(r.Context(), r.PathValue("host_id"), body.Name, body.Content)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s := st.loadStack(w, r, id); s != nil {
		writeJSON(w, http.StatusOK, s)
	}
}

func (st *State) handleGetStack(w http.ResponseWriter, r *http.Request) {
	if s := st.loadStack(w, r, r.PathValue("stack_id")); s != nil {
		writeJSON(w, http.StatusOK, s)
	}
}

func (st *State) handleUpdateStack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("stack_id")
	var body protocol.UpdateStackBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	var envJSON *string
	if body.EnvVars != nil {
		if b, err := json.Marshal(body.EnvVars); err == nil {
			s := string(b)
			envJSON = &s
		}
	}
	if err := st.DB.UpdateStack(r.Context(), id, body.Content, envJSON); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s := st.loadStack(w, r, id); s != nil {
		writeJSON(w, http.StatusOK, s)
	}
}

func (st *State) handleDeleteStack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("stack_id")
	// Best-effort teardown before removing the record.
	if s, err := st.DB.GetStack(r.Context(), id); err == nil && s != nil {
		if env, derr := st.DB.GetEnvironment(r.Context(), s.HostID); derr == nil && env != nil {
			_ = st.Agent.CallOn(r.Context(), env.AgentID, protocol.Request{
				Kind: protocol.ReqComposeDown, DockerHost: env.DockerHost, Stack: composeStack(s),
			})
		}
	}
	if err := st.DB.DeleteStack(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (st *State) handleStackUp(w http.ResponseWriter, r *http.Request) {
	st.stackLifecycle(w, r, protocol.ReqComposeUp, "running")
}

func (st *State) handleStackDown(w http.ResponseWriter, r *http.Request) {
	st.stackLifecycle(w, r, protocol.ReqComposeDown, "created")
}

// stackLifecycle runs compose up/down on the agent and records the new status.
func (st *State) stackLifecycle(w http.ResponseWriter, r *http.Request, kind, newStatus string) {
	id := r.PathValue("stack_id")
	s := st.loadStack(w, r, id)
	if s == nil {
		return
	}
	agentID, dockerHost, ok := st.resolveTarget(w, r, s.HostID)
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind: kind, DockerHost: dockerHost, Stack: composeStack(s),
	})
	if resp.Kind != protocol.RespKindText {
		if kind == protocol.ReqComposeUp {
			_ = st.DB.UpdateStackStatus(r.Context(), id, "error")
		}
		writeUnexpected(w, resp)
		return
	}
	var output string
	_ = resp.Decode(&output)
	if err := st.DB.UpdateStackStatus(r.Context(), id, newStatus); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, protocol.StackOpResponse{ID: id, Status: newStatus, Output: output})
}

func (st *State) handleSetStackToken(w http.ResponseWriter, r *http.Request) {
	token := uuid.NewString()
	if err := st.DB.SetStackWebhookToken(r.Context(), r.PathValue("stack_id"), &token); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, protocol.TokenResponse{Token: token})
}

func (st *State) handleDeleteStackToken(w http.ResponseWriter, r *http.Request) {
	if err := st.DB.SetStackWebhookToken(r.Context(), r.PathValue("stack_id"), nil); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
