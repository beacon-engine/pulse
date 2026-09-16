package gateway

import (
	"encoding/json"
	"net/http"

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
	id, err := st.DB.CreateEnvironment(r.Context(), body.Name, body.DockerHost, body.AgentID)
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
