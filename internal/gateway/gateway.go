// Package gateway assembles the Pulse gateway from its parts:
//
//	api/       the HTTP API, routes and dashboard
//	agents/    how the gateway reaches agents (pull client, push sessions)
//	auth/      passwords, sessions, auth middleware
//	events/    the live feed, the Docker event watcher, Slack alerts
//	liveness/  online/offline tracking for agents
//	metrics/   Prometheus counters
//	storage/   the database
package gateway

import (
	"context"
	"net/http"

	"github.com/beacon-engine/pulse/internal/gateway/agents"
	"github.com/beacon-engine/pulse/internal/gateway/api"
	"github.com/beacon-engine/pulse/internal/gateway/events"
	"github.com/beacon-engine/pulse/internal/gateway/liveness"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
)

// Gateway is a wired-up gateway: its HTTP handler and its background loops.
type Gateway struct {
	api      *api.State
	router   *agents.Router
	notifier *events.Notifier
	monitor  *liveness.Monitor
}

// New wires the gateway's parts together around an opened database.
func New(cfg Config, db storage.Repository) *Gateway {
	router := agents.NewRouter(agents.NewPullClient(cfg.AgentAddr))
	hub := events.NewHub()
	notifier := &events.Notifier{DB: db, Hub: hub, SlackWebhook: cfg.SlackWebhookURL}
	return &Gateway{
		api: &api.State{
			Agent: router,
			AgentWS: &agents.Server{
				Router:  router,
				Tokens:  cfg.AgentTokens,
				DB:      db,
				OnEvent: notifier.HandleAgentEvent,
			},
			DB:        db,
			Events:    hub,
			Notifier:  notifier,
			StaticDir: cfg.StaticDir,
		},
		router:   router,
		notifier: notifier,
		monitor:  &liveness.Monitor{DB: db, Router: router, Notifier: notifier},
	}
}

// Handler serves the API, the dashboard and /agent/ws.
func (g *Gateway) Handler() http.Handler { return g.api.Routes() }

// Start runs the background loops until ctx ends: the default agent's Docker
// event watcher, the push-agent sweeper and the pull-agent prober.
func (g *Gateway) Start(ctx context.Context) {
	go events.RunWatcher(ctx, g.router, g.notifier)
	go g.monitor.RunSweeper(ctx)
	go g.monitor.RunPullProber(ctx)
}
