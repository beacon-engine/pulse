package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Ports gateway/src/observability/prometheus_proxy.rs — a pure HTTP passthrough
// that forwards a PromQL query (?url=&q=) to a Prometheus server. No agent needed.
func (st *State) handlePrometheusQuery(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(r.URL.Query().Get("url"), "/") + "/api/v1/query"
	q := r.URL.Query().Get("q")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(base + "?query=" + url.QueryEscape(q))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Cannot reach Prometheus: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Invalid Prometheus response: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
