package gateway

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/webhooks.rs — webhook token management + token-guarded
// actions (recreate / redeploy / swarm update|scale / k8s rollout) on the agent.

// ── Token management (entity_type = container | swarm | k8s) ─────────────────

func (st *State) handleGetWebhookToken(w http.ResponseWriter, r *http.Request) {
	token, err := st.DB.GetWebhookToken(r.Context(), r.PathValue("entity_type"), r.PathValue("entity_key"), r.PathValue("host_id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": token != nil, "token": token})
}

func (st *State) handleSetWebhookToken(w http.ResponseWriter, r *http.Request) {
	token := uuid.NewString()
	if err := st.DB.SetWebhookToken(r.Context(), r.PathValue("entity_type"), r.PathValue("entity_key"), r.PathValue("host_id"), token); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, protocol.TokenResponse{Token: token})
}

func (st *State) handleDeleteWebhookToken(w http.ResponseWriter, r *http.Request) {
	if err := st.DB.DeleteWebhookToken(r.Context(), r.PathValue("entity_type"), r.PathValue("entity_key"), r.PathValue("host_id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// verifyWebhookToken checks the provided ?token against the stored token. Writes
// the HTTP error and returns false on failure.
func (st *State) verifyWebhookToken(w http.ResponseWriter, r *http.Request, entityType, entityKey, hostID string) bool {
	required, err := st.DB.GetWebhookToken(r.Context(), entityType, entityKey, hostID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return false
	}
	if required == nil {
		writeErr(w, http.StatusForbidden, "webhook not enabled")
		return false
	}
	if r.URL.Query().Get("token") != *required {
		writeErr(w, http.StatusUnauthorized, "invalid or missing token")
		return false
	}
	return true
}

func boolParam(r *http.Request, key string, def bool) bool {
	switch r.URL.Query().Get(key) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}

// ── Container: pull latest and recreate (name-based, token-protected) ─────────

func (st *State) handleWebhookDeployContainer(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("host_id")
	name := r.PathValue("name")
	if !st.verifyWebhookToken(w, r, "container", name, hostID) {
		return
	}
	agentID, dockerHost, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return
	}

	// Find the container id by name via the agent.
	listResp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqListContainers, DockerHost: dockerHost})
	if listResp.Kind != protocol.RespKindContainers {
		writeUnexpected(w, listResp)
		return
	}
	var containers []protocol.ContainerInfo
	_ = listResp.Decode(&containers)
	targetID := ""
	for _, c := range containers {
		if c.Name == name || c.Name == "/"+name {
			targetID = c.ID
			break
		}
	}
	if targetID == "" {
		writeErr(w, http.StatusNotFound, "no container named '"+name+"'")
		return
	}

	pull := boolParam(r, "pull", true)
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind: protocol.ReqRecreateLatest, DockerHost: dockerHost, ID: targetID, Pull: pull,
	})
	if resp.Kind != protocol.RespKindID {
		writeUnexpected(w, resp)
		return
	}
	var id string
	_ = resp.Decode(&id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "pulled": pull})
}

// recreate_container: legacy endpoint (id-based, no token).
func (st *State) handleWebhookRecreateContainer(w http.ResponseWriter, r *http.Request) {
	agentID, dockerHost, ok := st.resolveTarget(w, r, r.PathValue("host_id"))
	if !ok {
		return
	}
	pull := boolParam(r, "pull", false)
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind: protocol.ReqRecreateLatest, DockerHost: dockerHost, ID: r.PathValue("container_id"), Pull: pull,
	})
	if resp.Kind != protocol.RespKindID {
		writeUnexpected(w, resp)
		return
	}
	var id string
	_ = resp.Decode(&id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "pulled": pull})
}

// ── Stack: pull and redeploy ──────────────────────────────────────────────────

func (st *State) handleWebhookRedeployStack(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("host_id")
	stackID := r.PathValue("stack_id")

	s, err := st.DB.GetStack(r.Context(), stackID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		writeErr(w, http.StatusNotFound, "stack "+stackID+" not found")
		return
	}
	if s.HostID != hostID {
		writeErr(w, http.StatusForbidden, "stack does not belong to this host")
		return
	}
	// Token is required — if not set the webhook is disabled.
	if s.WebhookToken == nil {
		writeErr(w, http.StatusForbidden, "webhook not enabled for this stack")
		return
	}
	if r.URL.Query().Get("token") != *s.WebhookToken {
		writeErr(w, http.StatusUnauthorized, "invalid or missing token")
		return
	}

	agentID, dockerHost, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return
	}
	compose := composeStack(s)
	var output string

	if boolParam(r, "pull", false) {
		pullResp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqComposePull, DockerHost: dockerHost, Stack: compose})
		output += "=== pull ===\n"
		if pullResp.Kind == protocol.RespKindText {
			var out string
			_ = pullResp.Decode(&out)
			output += out + "\n"
		} else {
			_, msg := unexpected(pullResp)
			output += "(warn) " + msg + "\n"
		}
	}

	upResp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqComposeUp, DockerHost: dockerHost, Stack: compose})
	if upResp.Kind != protocol.RespKindText {
		writeUnexpected(w, upResp)
		return
	}
	var upOut string
	_ = upResp.Decode(&upOut)
	output += "=== up ===\n" + upOut

	writeJSON(w, http.StatusOK, map[string]any{"id": stackID, "status": "ok", "output": output})
}

// ── Swarm service: force update / scale ───────────────────────────────────────

func (st *State) handleWebhookUpdateSwarm(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("host_id")
	serviceID := r.PathValue("service_id")
	if !st.verifyWebhookToken(w, r, "swarm", serviceID, hostID) {
		return
	}
	agentID, dockerHost, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqUpdateSwarmService, DockerHost: dockerHost, ServiceID: serviceID})
	if resp.Kind != protocol.RespKindOk {
		writeUnexpected(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": serviceID, "updated": true})
}

func (st *State) handleWebhookScaleSwarm(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("host_id")
	serviceID := r.PathValue("service_id")
	if !st.verifyWebhookToken(w, r, "swarm", serviceID, hostID) {
		return
	}
	agentID, dockerHost, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return
	}
	replicas := uint64(clampInt(atoiDefault(r.URL.Query().Get("target_replicas"), 1), 0, 1<<20))
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind: protocol.ReqScaleSwarmService, DockerHost: dockerHost, ServiceID: serviceID, TargetReplicas: replicas,
	})
	if resp.Kind != protocol.RespKindOk {
		writeUnexpected(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": serviceID, "updated": true})
}

// ── Kubernetes: rollout restart ───────────────────────────────────────────────

func (st *State) handleWebhookK8sRollout(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("host_id")
	namespace := r.PathValue("namespace")
	deployment := r.PathValue("deployment")
	if !st.verifyWebhookToken(w, r, "k8s", namespace+"/"+deployment, hostID) {
		return
	}
	agentID, _, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind: protocol.ReqK8sRollout, Namespace: namespace, Deployment: deployment,
	})
	if resp.Kind != protocol.RespKindOk {
		writeUnexpected(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"namespace": namespace, "deployment": deployment, "restarted": true})
}
