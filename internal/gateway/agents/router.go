// Package agents is how the gateway reaches agents: the router that picks a
// transport per environment, the pull client that dials an agent, and the
// push sessions agents open on /agent/ws.

package agents

import (
	"context"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/agent.rs — the transport(s) to the agent.
//
// PullClient is the pull transport (the gateway dials the agent, one short-lived
// framed TCP request/response per call). Router sits in front and picks a
// transport per call: a live push session (an agent that dialed into the gateway,
// registered by id) when available, otherwise the default pull client.

// requestTimeout bounds how long a routed request waits for its correlated reply.
const requestTimeout = 30 * time.Second

// Session keepalive: the gateway pings each push agent every sessionPingInterval
// and drops the session when a pong doesn't arrive within sessionPingTimeout, so
// a silently dead link stops taking requests (and the environment goes offline)
// within about half a minute. Variables so tests can shorten them.
var (
	sessionPingInterval = 20 * time.Second
	sessionPingTimeout  = 10 * time.Second
)

// Transport executes a request and returns the agent's response.
type Transport interface {
	Call(ctx context.Context, req protocol.Request) protocol.Response
}

// ── Router ───────────────────────────────────────────────────────────────────

type Router struct {
	def      *PullClient
	mu       sync.RWMutex
	sessions map[string]Transport
	pulls    map[string]Transport
}

func NewRouter(def *PullClient) *Router {
	return &Router{
		def:      def,
		sessions: make(map[string]Transport),
		pulls:    make(map[string]Transport),
	}
}

// CallOn routes a request to the agent that serves a host, in priority order:
//  1. the host's live push session (by agentID), if connected;
//  2. the host's own pull agent at agentAddr, if set (each address gets a cached
//     client — the gateway can talk to many agents, not just one);
//  3. the default pull agent (AGENT_ADDR) as a fallback / seed.
func (r *Router) CallOn(ctx context.Context, agentID, agentAddr *string, req protocol.Request) protocol.Response {
	if agentID != nil {
		r.mu.RLock()
		session, ok := r.sessions[*agentID]
		r.mu.RUnlock()
		if ok {
			return session.Call(ctx, req)
		}
	}
	if agentAddr != nil && strings.TrimSpace(*agentAddr) != "" {
		return r.transportFor(*agentAddr).Call(ctx, req)
	}
	if agentID != nil {
		// The environment belongs to a push agent that isn't connected. Never
		// fall through to the default agent: that is a different machine (or
		// cluster), and answering for it would show the wrong environment's data.
		return sessionError("agent " + *agentID + " is not connected")
	}
	return r.def.Call(ctx, req)
}

// transportFor returns a cached transport for a per-host agent endpoint, chosen
// by scheme: ws://|wss:// → a WebSocket pull client (rides HTTP(S), so it
// traverses firewalls/proxies — same rationale as push); anything else (bare
// host:port or tcp://…) → the raw framed-TCP client. Mirrors how the agent pools
// Docker clients per docker_host.
func (r *Router) transportFor(addr string) Transport {
	key := strings.TrimSpace(addr)
	r.mu.RLock()
	t, ok := r.pulls[key]
	r.mu.RUnlock()
	if ok {
		return t
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.pulls[key]; ok {
		return t
	}
	if isWSAddr(key) {
		t = &wsPullClient{url: wsURL(key)}
	} else {
		t = NewPullClient(strings.TrimPrefix(key, "tcp://"))
	}
	r.pulls[key] = t
	return t
}

func isWSAddr(addr string) bool {
	a := strings.ToLower(strings.TrimSpace(addr))
	return strings.HasPrefix(a, "ws://") || strings.HasPrefix(a, "wss://")
}

// wsURL ensures the endpoint has the agent's WS path. A bare ws://host:port
// gets /agent/ws appended; an explicit path is left as-is.
func wsURL(addr string) string {
	u, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return addr
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/agent/ws"
	}
	return u.String()
}

// WatchEvents opens the default agent's event stream (used by the watcher).
func (r *Router) WatchEvents(ctx context.Context, dockerHost string) (net.Conn, error) {
	return r.def.WatchEvents(ctx, dockerHost)
}

// ConnectedAgentIDs lists the agents currently connected via a push session.
func (r *Router) ConnectedAgentIDs() []string {
	r.mu.RLock()
	ids := make([]string, 0, len(r.sessions))
	for id := range r.sessions {
		ids = append(ids, id)
	}
	r.mu.RUnlock()
	sort.Strings(ids)
	return ids
}

// Register adds a live push session under agentID, replacing any previous one.
func (r *Router) Register(agentID string, t Transport) {
	r.mu.Lock()
	r.sessions[agentID] = t
	r.mu.Unlock()
}

// IsConnected reports whether agentID has a live push session.
func (r *Router) IsConnected(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.sessions[agentID]
	return ok
}

// UnregisterIf removes agentID's session only if it is still t, and reports
// whether it did. With a stable agent id (PULSE_INSTANCE_ID) a redeploy's new
// pod connects before the old one leaves; the old session's teardown must not
// evict its replacement.
func (r *Router) UnregisterIf(agentID string, t Transport) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions[agentID] != t {
		return false
	}
	delete(r.sessions, agentID)
	return true
}
