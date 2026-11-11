package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/hosts.rs — host (environment) CRUD, served from the DB.

func (st *State) handleListHosts(w http.ResponseWriter, r *http.Request) {
	page, perPage := pageBounds(r)
	search := searchTerm(r)

	total, err := st.DB.CountEnvironments(r.Context(), search)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	offset := int64((page - 1) * perPage)
	items, err := st.DB.ListEnvironmentsPage(r.Context(), search, int64(perPage), offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Page[protocol.Environment]{
		Items:   orEmpty(items),
		Total:   int(total),
		Page:    page,
		PerPage: perPage,
	})
}

func (st *State) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var body protocol.CreateHostBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.DockerHost = strings.TrimSpace(body.DockerHost)
	body.AgentAddr = trimmedOrNil(body.AgentAddr)
	body.AgentID = trimmedOrNil(body.AgentID)
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	// An environment is reached through an agent (agent_addr / agent_id) or a
	// container API the default agent can dial (docker_host). Neither is
	// required on its own: a Kubernetes-only agent has no docker_host.
	if body.DockerHost == "" && body.AgentAddr == nil && body.AgentID == nil {
		writeErr(w, http.StatusBadRequest, "provide an agent address or a container runtime endpoint")
		return
	}
	id, err := st.DB.CreateEnvironment(r.Context(), body.Name, body.DockerHost, body.AgentID, body.AgentAddr)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	env, err := st.DB.GetEnvironment(r.Context(), id)
	if err != nil || env == nil {
		writeErr(w, http.StatusInternalServerError, "not found after insert")
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (st *State) handleRenameHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body protocol.RenameHostBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := st.DB.RenameEnvironment(r.Context(), id, body.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	env, err := st.DB.GetEnvironment(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if env == nil {
		writeErr(w, http.StatusNotFound, "host "+id+" not found")
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (st *State) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := st.DB.GetEnvironment(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if env == nil {
		writeErr(w, http.StatusNotFound, "host "+id+" not found")
		return
	}
	if env.IsLocal {
		writeErr(w, http.StatusBadRequest, "cannot delete the local host")
		return
	}
	if err := st.DB.DeleteEnvironment(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HostCapabilities is the capabilities answer for one environment, plus whether
// its agent could be reached at all.
type HostCapabilities struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
	protocol.Capabilities
}

// handleHostCapabilities asks the environment's agent which runtimes it can
// drive. It always answers 200: "agent unreachable" is a state the UI renders,
// not a failure of this endpoint.
func (st *State) handleHostCapabilities(w http.ResponseWriter, r *http.Request) {
	agentID, agentAddr, dockerHost, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, agentAddr, protocol.Request{
		Kind: protocol.ReqCapabilities, DockerHost: dockerHost,
	})
	out := HostCapabilities{Capabilities: protocol.Capabilities{
		Runtimes: []protocol.RuntimeInfo{}, Unavailable: []protocol.RuntimeProbe{},
	}}
	switch {
	case resp.Kind == protocol.RespKindCapabilities:
		if err := resp.Decode(&out.Capabilities); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out.Reachable = true
	case isUnknownRequest(resp):
		// An agent from before capabilities existed: it can only have been a
		// Docker agent (optionally with Kubernetes), so offer what it always did.
		out.Reachable = true
		out.Runtimes = []protocol.RuntimeInfo{
			{Kind: protocol.RuntimeDocker, Features: []string{protocol.FeatureSwarm, protocol.FeatureCompose}},
			{Kind: protocol.RuntimeKubernetes, Features: []string{}},
		}
	default:
		_, msg := unexpected(resp)
		out.Error = msg
	}
	writeJSON(w, http.StatusOK, out)
}

func isUnknownRequest(resp protocol.Response) bool {
	status, msg, ok := resp.AsError()
	return ok && status == http.StatusBadRequest && strings.HasPrefix(msg, "unknown request")
}

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
