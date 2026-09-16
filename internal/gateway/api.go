package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports the shared HTTP helpers from gateway/src/api.rs: pagination, target
// resolution, and mapping an agent Response onto an HTTP status.

// Page is a page of results plus the total count (for page controls).
type Page[T any] struct {
	Items   []T `json:"items"`
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

// toStatus maps a wire status code onto a valid HTTP status, defaulting to 500.
func toStatus(code uint16) int {
	if code >= 100 && code <= 599 {
		return int(code)
	}
	return http.StatusInternalServerError
}

// unexpected turns an error/unexpected agent response into (HTTP status, message).
func unexpected(resp protocol.Response) (int, string) {
	if status, msg, ok := resp.AsError(); ok {
		return toStatus(status), msg
	}
	return http.StatusInternalServerError, fmt.Sprintf("unexpected agent response: %s", resp.Kind)
}

// writeUnexpected writes the HTTP error for an error/unexpected agent response.
func writeUnexpected(w http.ResponseWriter, resp protocol.Response) {
	status, msg := unexpected(resp)
	writeErr(w, status, msg)
}

// resolveTarget resolves a host id to (agent_id, docker_host). On failure it
// writes the HTTP error and returns ok=false. agent_id selects the push session
// to route to (nil → the default pull agent).
func (st *State) resolveTarget(w http.ResponseWriter, r *http.Request, hostID string) (agentID *string, dockerHost string, ok bool) {
	env, err := st.DB.GetEnvironment(r.Context(), hostID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return nil, "", false
	}
	if env == nil {
		writeErr(w, http.StatusNotFound, "host "+hostID+" not found")
		return nil, "", false
	}
	return env.AgentID, env.DockerHost, true
}

// pageBounds parses ?page & ?per_page with sane bounds (page ≥ 1, per_page 1..500).
func pageBounds(r *http.Request) (page, perPage int) {
	q := r.URL.Query()
	page = atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage = atoiDefault(q.Get("per_page"), 50)
	switch {
	case perPage < 1:
		perPage = 1
	case perPage > 500:
		perPage = 500
	}
	return page, perPage
}

// searchTerm returns the trimmed, lower-cased ?search term ("" when absent).
func searchTerm(r *http.Request) string {
	return strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

// orEmpty returns a non-nil slice so JSON encodes [] rather than null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// paginate slices an already-filtered in-memory list into the requested page.
func paginate[T any](items []T, page, perPage int) Page[T] {
	total := len(items)
	start := min((page-1)*perPage, total)
	end := min(start+perPage, total)
	return Page[T]{Items: orEmpty(items[start:end]), Total: total, Page: page, PerPage: perPage}
}

// proxy resolves the host to its agent, forwards the request, and on the expected
// response kind decodes the payload into out (nil to ignore). It writes the HTTP
// error itself and returns false on any failure, so callers just write success.
func (st *State) proxy(w http.ResponseWriter, r *http.Request, hostID string, buildReq func(dockerHost string) protocol.Request, wantKind string, out any) bool {
	agentID, dockerHost, ok := st.resolveTarget(w, r, hostID)
	if !ok {
		return false
	}
	resp := st.Agent.CallOn(r.Context(), agentID, buildReq(dockerHost))
	if resp.Kind != wantKind {
		writeUnexpected(w, resp)
		return false
	}
	if out != nil {
		if err := resp.Decode(out); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return false
		}
	}
	return true
}
