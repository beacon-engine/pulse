// Package metrics keeps the gateway's Prometheus counters and serves them on
// /metrics.
package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Ports gateway/src/observability/metrics.rs — a minimal Prometheus text
// exposition, hand-rolled to avoid a metrics-client dependency (keeping the
// gateway a lean static binary).

var startTime = time.Now()

var appMetrics = struct {
	httpRequests atomic.Int64
	mu           sync.Mutex
	events       map[string]int64
}{events: map[string]int64{}}

func RecordEvent(event string) {
	appMetrics.mu.Lock()
	appMetrics.events[event]++
	appMetrics.mu.Unlock()
}

func recordHTTPRequest() { appMetrics.httpRequests.Add(1) }

// Handler serves the counters in the Prometheus text format.
func Handler(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	b.WriteString("# HELP pulse_uptime_seconds Seconds since Pulse started\n")
	b.WriteString("# TYPE pulse_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "pulse_uptime_seconds %.3f\n", time.Since(startTime).Seconds())

	b.WriteString("# HELP pulse_http_requests_total Total number of HTTP requests\n")
	b.WriteString("# TYPE pulse_http_requests_total counter\n")
	fmt.Fprintf(&b, "pulse_http_requests_total %d\n", appMetrics.httpRequests.Load())

	b.WriteString("# HELP pulse_container_events_total Container lifecycle events recorded by Pulse\n")
	b.WriteString("# TYPE pulse_container_events_total counter\n")
	appMetrics.mu.Lock()
	for ev, n := range appMetrics.events {
		fmt.Fprintf(&b, "pulse_container_events_total{event=%q} %d\n", ev, n)
	}
	appMetrics.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// CountRequests is middleware that tallies every HTTP request for /metrics.
func CountRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordHTTPRequest()
		next.ServeHTTP(w, r)
	})
}
