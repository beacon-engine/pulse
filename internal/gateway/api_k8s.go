package gateway

import (
	"net/http"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/k8s.rs — per-host Kubernetes reads, routed to the agent
// that represents that host's cluster.

func (st *State) handleK8sPods(w http.ResponseWriter, r *http.Request) {
	agentID, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqK8sPods})
	if resp.Kind != protocol.RespKindPods {
		writeUnexpected(w, resp)
		return
	}
	var pods []protocol.PodInfo
	if err := resp.Decode(&pods); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(pods))
}

func (st *State) handleK8sNamespaces(w http.ResponseWriter, r *http.Request) {
	agentID, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{Kind: protocol.ReqK8sNamespaces})
	if resp.Kind != protocol.RespKindNamespaces {
		writeUnexpected(w, resp)
		return
	}
	var ns []protocol.NamespaceInfo
	if err := resp.Decode(&ns); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(ns))
}

func (st *State) handleK8sDeletePod(w http.ResponseWriter, r *http.Request) {
	agentID, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, protocol.Request{
		Kind:      protocol.ReqK8sDeletePod,
		Namespace: r.PathValue("namespace"),
		Name:      r.PathValue("name"),
	})
	if resp.Kind != protocol.RespKindOk {
		writeUnexpected(w, resp)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
