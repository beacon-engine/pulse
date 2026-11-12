package api

import (
	"net/http"
	"strings"

	"github.com/beacon-engine/pulse/internal/gateway/auth"
	"github.com/beacon-engine/pulse/internal/gateway/metrics"
)

// Routes builds the gateway's HTTP handler using the Go 1.22+ ServeMux (method +
// path patterns), so no router dependency is needed. Public paths (auth
// endpoints, health/metrics, agent WS, and the static SPA) are served directly;
// everything else goes through requireAuth → enforceRole. Every request is
// counted for /metrics.
func (st *State) Routes() http.Handler {
	mux := http.NewServeMux()
	st.registerRoutes(mux)

	protected := auth.RequireAuth(st.DB, auth.EnforceRole(mux))
	return metrics.CountRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) || isTokenWebhook(mux, r) {
			mux.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	}))
}

// isPublicPath reports whether a path bypasses session auth. Mirrors the Rust
// `public` router, plus the static SPA (so the login page loads). `/ws` (the UI
// event feed) and all other `/api/*` routes are protected. Webhook
// actions are public too but token-checked: see tokenWebhooks.
func isPublicPath(path string) bool {
	switch path {
	case "/health", "/metrics", "/api/status", "/agent/ws", "/api/auth/setup", "/api/auth/login":
		return true
	}
	if path == "/ws" {
		return false
	}
	// Anything not under /api/ is a static asset (SPA) → public.
	return !strings.HasPrefix(path, "/api/")
}

// tokenWebhooks are the webhook actions a CI job calls with only the
// webhook's ?token=, which the handler checks. They're matched by route
// pattern, not path prefix, so token management and the legacy id-based
// recreate (which has no token) still need a session.
var tokenWebhooks = map[string]bool{
	"POST /api/webhooks/{host_id}/containers/{name}/deploy":              true,
	"POST /api/webhooks/{host_id}/stacks/{stack_id}/redeploy":            true,
	"POST /api/webhooks/{host_id}/swarm/{service_id}/update":             true,
	"POST /api/webhooks/{host_id}/swarm/{service_id}/scale":              true,
	"POST /api/webhooks/{host_id}/k8s/{namespace}/{deployment}/rollout":  true,
	"POST /api/webhooks/{host_id}/k8s/{namespace}/{kind}/{name}/restart": true,
}

func isTokenWebhook(mux *http.ServeMux, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/webhooks/") {
		return false
	}
	_, pattern := mux.Handler(r)
	return tokenWebhooks[pattern]
}

func (st *State) registerRoutes(mux *http.ServeMux) {
	// ── Public ──────────────────────────────────────────────────────────────────
	mux.Handle("/agent/ws", st.AgentWS)
	mux.HandleFunc("GET /health", st.handleHealth)
	mux.HandleFunc("GET /api/status", st.handleStatus)
	mux.HandleFunc("GET /api/auth/setup", st.handleSetupStatus)
	mux.HandleFunc("POST /api/auth/setup", st.handleSetup)
	mux.HandleFunc("POST /api/auth/login", st.handleLogin)

	// ── Auth (protected) ────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/auth/logout", st.handleLogout)
	mux.HandleFunc("GET /api/auth/me", st.handleMe)

	// ── Users (admin) ───────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/users", st.handleListUsers)
	mux.HandleFunc("POST /api/users", st.handleCreateUser)
	mux.HandleFunc("PUT /api/users/{id}", st.handleUpdateUser)
	mux.HandleFunc("DELETE /api/users/{id}", st.handleDeleteUser)

	// ── Agents + events ─────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/agents", st.handleListAgents)
	mux.HandleFunc("GET /api/events", st.handleListEvents)

	// ── Settings (GET: any authed user; PUT: admin) ───────────────────────────────
	mux.HandleFunc("GET /api/settings", st.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", st.handleUpdateSettings)

	// ── Hosts (environments) ────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts", st.handleListHosts)
	mux.HandleFunc("GET /api/hosts/{id}", st.handleGetHost)
	mux.HandleFunc("POST /api/hosts", st.handleCreateHost)
	mux.HandleFunc("PUT /api/hosts/{id}", st.handleRenameHost)
	mux.HandleFunc("DELETE /api/hosts/{id}", st.handleDeleteHost)
	mux.HandleFunc("GET /api/hosts/{id}/capabilities", st.handleHostCapabilities)

	// ── Kubernetes ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/k8s/pods", st.handleK8sPods)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/namespaces", st.handleK8sNamespaces)
	mux.HandleFunc("DELETE /api/hosts/{id}/k8s/pods/{namespace}/{name}", st.handleK8sDeletePod)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/pods/{namespace}/{name}/logs", st.handleK8sPodLogs)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/workloads", st.handleK8sWorkloads)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/nodes", st.handleK8sNodes)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/services", st.handleK8sServices)
	mux.HandleFunc("GET /api/hosts/{id}/k8s/events", st.handleK8sEvents)
	mux.HandleFunc("POST /api/hosts/{id}/k8s/workloads/{namespace}/{kind}/{name}/scale", st.handleK8sScale)
	mux.HandleFunc("POST /api/hosts/{id}/k8s/workloads/{namespace}/{kind}/{name}/restart", st.handleK8sRestart)

	// ── Containers ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/containers", st.handleListContainers)
	mux.HandleFunc("GET /api/hosts/{id}/containers/{cid}/inspect", st.handleInspectContainer)
	mux.HandleFunc("GET /api/hosts/{id}/containers/{cid}/raw", st.handleInspectRawContainer)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/recreate", st.handleRecreateContainer)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/env", st.handleUpdateContainerEnv)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/exec", st.handleExecContainer)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/start", st.handleStartContainer)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/stop", st.handleStopContainer)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/restart", st.handleRestartContainer)
	mux.HandleFunc("GET /api/hosts/{id}/containers/{cid}/logs", st.handleContainerLogs)
	mux.HandleFunc("GET /api/hosts/{id}/containers/{cid}/networks", st.handleContainerNetworks)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/networks/connect", st.handleNetworkConnect)
	mux.HandleFunc("POST /api/hosts/{id}/containers/{cid}/networks/disconnect", st.handleNetworkDisconnect)

	// ── System / hardware ────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/system", st.handleSystemInfo)
	mux.HandleFunc("GET /api/hosts/{id}/hardware", st.handleHardwareMetrics)

	// ── Images ────────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/images", st.handleListImages)
	mux.HandleFunc("POST /api/hosts/{id}/images/prune", st.handlePruneImages)
	mux.HandleFunc("DELETE /api/hosts/{id}/images/{image_id}", st.handleDeleteImage)

	// ── Volumes ─────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/volumes", st.handleListVolumes)
	mux.HandleFunc("POST /api/hosts/{id}/volumes/prune", st.handlePruneVolumes)
	mux.HandleFunc("DELETE /api/hosts/{id}/volumes/{vol_name}", st.handleDeleteVolume)

	// ── Networks + detected stacks ──────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{id}/networks", st.handleListHostNetworks)
	mux.HandleFunc("GET /api/hosts/{host_id}/stacks/detected", st.handleListDetectedStacks)

	// ── Swarm ────────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{host_id}/swarm/services", st.handleListServices)
	mux.HandleFunc("GET /api/hosts/{host_id}/swarm/nodes", st.handleListNodes)
	mux.HandleFunc("GET /api/hosts/{host_id}/swarm/services/{service_id}/tasks", st.handleListServiceTasks)
	mux.HandleFunc("POST /api/hosts/{host_id}/swarm/services/{service_id}/update", st.handleUpdateSwarmService)
	mux.HandleFunc("POST /api/hosts/{host_id}/swarm/services/{service_id}/scale", st.handleScaleSwarmService)

	// ── Stacks (persisted) ──────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/hosts/{host_id}/stacks", st.handleListStacks)
	mux.HandleFunc("POST /api/hosts/{host_id}/stacks", st.handleCreateStack)
	mux.HandleFunc("GET /api/stacks/{stack_id}", st.handleGetStack)
	mux.HandleFunc("PUT /api/stacks/{stack_id}", st.handleUpdateStack)
	mux.HandleFunc("DELETE /api/stacks/{stack_id}", st.handleDeleteStack)
	mux.HandleFunc("POST /api/stacks/{stack_id}/up", st.handleStackUp)
	mux.HandleFunc("POST /api/stacks/{stack_id}/down", st.handleStackDown)
	mux.HandleFunc("POST /api/stacks/{stack_id}/token", st.handleSetStackToken)
	mux.HandleFunc("DELETE /api/stacks/{stack_id}/token", st.handleDeleteStackToken)

	// ── Webhooks ─────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/webhooks/{host_id}/{entity_type}/{entity_key}/token", st.handleGetWebhookToken)
	mux.HandleFunc("POST /api/webhooks/{host_id}/{entity_type}/{entity_key}/token", st.handleSetWebhookToken)
	mux.HandleFunc("DELETE /api/webhooks/{host_id}/{entity_type}/{entity_key}/token", st.handleDeleteWebhookToken)
	mux.HandleFunc("POST /api/webhooks/{host_id}/containers/{name}/deploy", st.handleWebhookDeployContainer)
	mux.HandleFunc("POST /api/webhooks/{host_id}/{container_id}/recreate", st.handleWebhookRecreateContainer)
	mux.HandleFunc("POST /api/webhooks/{host_id}/stacks/{stack_id}/redeploy", st.handleWebhookRedeployStack)
	mux.HandleFunc("POST /api/webhooks/{host_id}/swarm/{service_id}/update", st.handleWebhookUpdateSwarm)
	mux.HandleFunc("POST /api/webhooks/{host_id}/swarm/{service_id}/scale", st.handleWebhookScaleSwarm)
	mux.HandleFunc("POST /api/webhooks/{host_id}/k8s/{namespace}/{deployment}/rollout", st.handleWebhookK8sRollout)
	mux.HandleFunc("POST /api/webhooks/{host_id}/k8s/{namespace}/{kind}/{name}/restart", st.handleWebhookK8sRestart)

	// ── Observability ────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /metrics", metrics.Handler)
	mux.HandleFunc("GET /ws", st.handleUIWebSocket)
	mux.HandleFunc("GET /api/prometheus/query", st.handlePrometheusQuery)

	// ── Static SPA (fallback for anything unmatched) ─────────────────────────────
	mux.HandleFunc("/", st.handleStatic)
}
