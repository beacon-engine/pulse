package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"sort"
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
}

func NewAgentRouter(def *AgentClient) *AgentRouter {
	return &AgentRouter{def: def, sessions: make(map[string]AgentTransport)}
}

// CallOn prefers the named agent's live push session, falling back to the default
// pull client. agentID comes from the target host's record once host→agent
// association lands; today it is usually nil.
func (r *AgentRouter) CallOn(ctx context.Context, agentID *string, req protocol.Request) protocol.Response {
	if agentID != nil {
		r.mu.RLock()
		session, ok := r.sessions[*agentID]
		r.mu.RUnlock()
		if ok {
			return session.Call(ctx, req)
		}
	}
	return r.def.Call(ctx, req)
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
