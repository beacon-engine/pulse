package api

import (
	"net/http"
	"strconv"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/k8s.rs — per-host Kubernetes reads, routed to the agent
// that represents that host's cluster.

func (st *State) handleK8sPods(w http.ResponseWriter, r *http.Request) {
	agentID, agentAddr, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, agentAddr, protocol.Request{Kind: protocol.ReqK8sPods})
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
	agentID, agentAddr, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, agentAddr, protocol.Request{Kind: protocol.ReqK8sNamespaces})
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
	agentID, agentAddr, _, ok := st.resolveTarget(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	resp := st.Agent.CallOn(r.Context(), agentID, agentAddr, protocol.Request{
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

func (st *State) handleK8sWorkloads(w http.ResponseWriter, r *http.Request) {
	var v []protocol.WorkloadInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request { return protocol.Request{Kind: protocol.ReqK8sWorkloads} },
		protocol.RespKindWorkloads, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleK8sPodLogs(w http.ResponseWriter, r *http.Request) {
	tail := uint32(clampInt(atoiDefault(r.URL.Query().Get("tail"), 200), 1, 2000))
	var out string
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request {
			return protocol.Request{
				Kind:      protocol.ReqK8sPodLogs,
				Namespace: r.PathValue("namespace"),
				Name:      r.PathValue("name"),
				Container: r.URL.Query().Get("container"),
				Tail:      tail,
			}
		},
		protocol.RespKindText, &out) {
		writeText(w, out)
	}
}

// handleK8sScale sets a Deployment's or StatefulSet's replica count
// (?replicas=N).
func (st *State) handleK8sScale(w http.ResponseWriter, r *http.Request) {
	replicas, err := strconv.ParseUint(r.URL.Query().Get("replicas"), 10, 32)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "replicas must be a non-negative integer")
		return
	}
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request {
			return protocol.Request{
				Kind:           protocol.ReqK8sScale,
				Namespace:      r.PathValue("namespace"),
				WorkloadKind:   r.PathValue("kind"),
				Name:           r.PathValue("name"),
				TargetReplicas: replicas,
			}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleK8sRestart does a rolling restart of a Deployment, StatefulSet or DaemonSet.
func (st *State) handleK8sRestart(w http.ResponseWriter, r *http.Request) {
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request {
			return protocol.Request{
				Kind:         protocol.ReqK8sRestart,
				Namespace:    r.PathValue("namespace"),
				WorkloadKind: r.PathValue("kind"),
				Name:         r.PathValue("name"),
			}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (st *State) handleK8sNodes(w http.ResponseWriter, r *http.Request) {
	var v []protocol.NodeInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request { return protocol.Request{Kind: protocol.ReqK8sNodes} },
		protocol.RespKindNodes, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleK8sServices(w http.ResponseWriter, r *http.Request) {
	var v []protocol.ServiceInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request { return protocol.Request{Kind: protocol.ReqK8sServices} },
		protocol.RespKindServices, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

// handleK8sEvents returns the cluster's most recent events, newest first.
func (st *State) handleK8sEvents(w http.ResponseWriter, r *http.Request) {
	var v []protocol.EventInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request { return protocol.Request{Kind: protocol.ReqK8sEvents} },
		protocol.RespKindK8sEvents, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}
