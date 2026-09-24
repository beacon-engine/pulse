package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/agent.rs — the transport(s) to the agent.
//
// AgentClient is the pull transport (the gateway dials the agent, one short-lived
// framed TCP request/response per call). AgentRouter sits in front and picks a
// transport per call: a live push session (an agent that dialed into the gateway,
// registered by id) when available, otherwise the default pull client.

// requestTimeout bounds how long a routed request waits for its correlated reply.
const requestTimeout = 30 * time.Second

// AgentTransport executes a request and returns the agent's response.
type AgentTransport interface {
	Call(ctx context.Context, req protocol.Request) protocol.Response
}

// ── Router ───────────────────────────────────────────────────────────────────

type AgentRouter struct {
	def      *AgentClient
	mu       sync.RWMutex
	sessions map[string]AgentTransport
	pulls    map[string]AgentTransport
}

func NewAgentRouter(def *AgentClient) *AgentRouter {
	return &AgentRouter{
		def:      def,
		sessions: make(map[string]AgentTransport),
		pulls:    make(map[string]AgentTransport),
	}
}

// CallOn routes a request to the agent that serves a host, in priority order:
//  1. the host's live push session (by agentID), if connected;
//  2. the host's own pull agent at agentAddr, if set (each address gets a cached
//     client — the gateway can talk to many agents, not just one);
//  3. the default pull agent (AGENT_ADDR) as a fallback / seed.
func (r *AgentRouter) CallOn(ctx context.Context, agentID, agentAddr *string, req protocol.Request) protocol.Response {
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
	return r.def.Call(ctx, req)
}

// transportFor returns a cached transport for a per-host agent endpoint, chosen
// by scheme: ws://|wss:// → a WebSocket pull client (rides HTTP(S), so it
// traverses firewalls/proxies — same rationale as push); anything else (bare
// host:port or tcp://…) → the raw framed-TCP client. Mirrors how the agent pools
// Docker clients per docker_host.
func (r *AgentRouter) transportFor(addr string) AgentTransport {
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
		t = &wsAgentClient{url: wsURL(key)}
	} else {
		t = NewAgentClient(strings.TrimPrefix(key, "tcp://"))
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
func (r *AgentRouter) WatchEvents(ctx context.Context, dockerHost string) (net.Conn, error) {
	return r.def.WatchEvents(ctx, dockerHost)
}

// ConnectedAgentIDs lists the agents currently connected via a push session.
func (r *AgentRouter) ConnectedAgentIDs() []string {
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
func (r *AgentRouter) Register(agentID string, t AgentTransport) {
	r.mu.Lock()
	r.sessions[agentID] = t
	r.mu.Unlock()
}

// Unregister removes the push session for agentID (on disconnect).
func (r *AgentRouter) Unregister(agentID string) {
	r.mu.Lock()
	delete(r.sessions, agentID)
	r.mu.Unlock()
}

// ── Pull client ──────────────────────────────────────────────────────────────

type AgentClient struct {
	addr string
}

func NewAgentClient(addr string) *AgentClient {
	return &AgentClient{addr: addr}
}

// Call sends a request and returns the agent's response. Transport failures are
// surfaced as a 502 Response::Error so callers only ever match on Response.
func (c *AgentClient) Call(ctx context.Context, req protocol.Request) protocol.Response {
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	defer conn.Close()
	if err := protocol.WriteFrame(conn, req); err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	var resp protocol.Response
	if err := protocol.ReadFrame(conn, &resp); err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	return resp
}

// wsAgentClient is a pull transport that reaches the agent over WebSocket
// (ws://|wss://). Like AgentClient it is per-call: dial, send one RequestFrame,
// read the ResponseFrame, close. Rides HTTP(S), so it traverses firewalls/proxies
// that block raw TCP. TLS ("wss") verification uses the system roots.
type wsAgentClient struct {
	url string
}

func (c *wsAgentClient) Call(ctx context.Context, req protocol.Request) protocol.Response {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, c.url, nil)
	if err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	defer conn.CloseNow()
	conn.SetReadLimit(protocol.MaxFrame)

	out, err := json.Marshal(protocol.RequestFrame{ID: 1, Req: req})
	if err != nil {
		return protocol.RespError(500, err.Error())
	}
	if err := conn.Write(ctx, websocket.MessageText, out); err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		return protocol.RespError(502, "agent unreachable: "+err.Error())
	}
	var frame protocol.ResponseFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return protocol.RespError(502, "malformed agent response")
	}
	conn.Close(websocket.StatusNormalClosure, "")
	return frame.Resp
}

// WatchEvents opens an event-stream connection; the caller reads Response::Event
// frames until it closes.
func (c *AgentClient) WatchEvents(ctx context.Context, dockerHost string) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, err
	}
	if err := protocol.WriteFrame(conn, protocol.Request{Kind: protocol.ReqWatchEvents, DockerHost: dockerHost}); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// ── Push sessions ────────────────────────────────────────────────────────────
// An agent that dials into the gateway holds one multiplexed WebSocket session.
// The gateway sends RequestFrames and matches each reply by id; the agent also
// pushes Docker events on EventStreamID.

// pending correlates in-flight requests with their replies over one session.
type pending struct {
	nextID  atomic.Uint64
	mu      sync.Mutex
	waiters map[uint64]chan protocol.Response
}

func newPending() *pending {
	return &pending{waiters: make(map[uint64]chan protocol.Response)}
}

// begin allocates a fresh id (≥ 1, never colliding with EventStreamID) and the
// channel that resolves when its reply arrives.
func (p *pending) begin() (uint64, chan protocol.Response) {
	id := p.nextID.Add(1)
	ch := make(chan protocol.Response, 1)
	p.mu.Lock()
	p.waiters[id] = ch
	p.mu.Unlock()
	return id, ch
}

func (p *pending) complete(id uint64, resp protocol.Response) {
	p.mu.Lock()
	ch, ok := p.waiters[id]
	delete(p.waiters, id)
	p.mu.Unlock()
	if ok {
		ch <- resp
	}
}

// failAll closes every waiter; their Calls observe a closed channel → 503.
func (p *pending) failAll() {
	p.mu.Lock()
	for id, ch := range p.waiters {
		close(ch)
		delete(p.waiters, id)
	}
	p.mu.Unlock()
}

// SessionTransport is the push transport: it queues a request onto the session
// and awaits its correlated reply.
type SessionTransport struct {
	pending  *pending
	outbound chan protocol.RequestFrame
	done     chan struct{}
}

func (t *SessionTransport) Call(ctx context.Context, req protocol.Request) protocol.Response {
	id, ch := t.pending.begin()
	select {
	case t.outbound <- protocol.RequestFrame{ID: id, Req: req}:
	case <-t.done:
		return sessionError("agent session closed")
	case <-ctx.Done():
		return sessionError("agent session closed")
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return sessionError("agent session closed")
		}
		return resp
	case <-t.done:
		return sessionError("agent session closed")
	case <-time.After(requestTimeout):
		return protocol.RespError(504, "agent request timed out")
	case <-ctx.Done():
		return sessionError("agent session closed")
	}
}

func sessionError(msg string) protocol.Response {
	return protocol.RespError(503, msg)
}

// runSession drives one push session for its lifetime: registers the transport,
// pumps requests out and replies/events in, and cleans up on disconnect.
func runSession(ctx context.Context, socket *websocket.Conn, st *State, agentID string) {
	p := newPending()
	outbound := make(chan protocol.RequestFrame, 64)
	done := make(chan struct{})

	st.Agent.Register(agentID, &SessionTransport{pending: p, outbound: outbound, done: done})
	log.Printf("Agent '%s' connected (push session)", agentID)

	// Writer: serialize queued request frames to the socket.
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-outbound:
				data, err := json.Marshal(frame)
				if err != nil {
					continue
				}
				if err := socket.Write(ctx, websocket.MessageText, data); err != nil {
					return
				}
			}
		}
	}()

	// Reader: route replies to their waiters, pushed events to the watcher.
	for {
		typ, data, err := socket.Read(ctx)
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		var frame protocol.ResponseFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			log.Printf("malformed frame from agent '%s': %v", agentID, err)
			continue
		}
		if frame.ID == protocol.EventStreamID {
			st.handleEvent(ctx, frame.Resp)
		} else {
			p.complete(frame.ID, frame.Resp)
		}
	}

	// Teardown: stop routing to this agent and unblock any pending calls.
	st.Agent.Unregister(agentID)
	close(done)
	p.failAll()
	log.Printf("Agent '%s' disconnected", agentID)
}
