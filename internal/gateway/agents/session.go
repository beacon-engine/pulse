package agents

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

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
// pumps requests out and replies/events in, and cleans up on disconnect. When
// auto is set the agent self-registered an environment (id == agentID): its
// heartbeats keep that row alive and, on disconnect, the row is marked offline —
// gracefully if the agent sent a deregister first (a clean shutdown / scale-down)
// so no down alert fires.
func (s *Server) runSession(ctx context.Context, socket *websocket.Conn, agentID string, auto bool) {
	p := newPending()
	outbound := make(chan protocol.RequestFrame, 64)
	done := make(chan struct{})

	transport := &SessionTransport{pending: p, outbound: outbound, done: done}
	s.Router.Register(agentID, transport)
	if auto {
		// Overrides an "offline" written by the previous session's teardown
		// before this session registered (see markSessionOffline).
		_ = s.DB.TouchEnvironment(ctx, agentID)
	}
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

	// Keepalive: an unanswered ping closes the socket, which ends the reader.
	go func() {
		t := time.NewTicker(sessionPingInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
			}
			pctx, cancel := context.WithTimeout(ctx, sessionPingTimeout)
			err := socket.Ping(pctx)
			cancel()
			if err != nil {
				select {
				case <-done:
				default:
					log.Printf("Agent '%s' didn't answer a ping within %s; dropping the session", agentID, sessionPingTimeout)
					socket.CloseNow()
				}
				return
			}
		}
	}()

	// Reader: route replies to their waiters, and handle pushes on EventStreamID
	// — Docker events (to the watcher), liveness heartbeats, and the deregister
	// notice. graceful is only touched by this loop and read after it, so no lock.
	graceful := false
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
			switch frame.Resp.Kind {
			case protocol.RespKindHeartbeat:
				if auto {
					_ = s.DB.TouchEnvironment(ctx, agentID)
				}
			case protocol.RespKindDeregister:
				graceful = true
			default:
				s.OnEvent(ctx, frame.Resp)
			}
		} else {
			p.complete(frame.ID, frame.Resp)
		}
	}

	// Teardown: stop routing to this agent and unblock any pending calls. For an
	// auto-registered agent, mark its environment offline (a detached context, so
	// the write survives the cancelled session). graceful → no alert.
	current := s.Router.UnregisterIf(agentID, transport)
	close(done)
	p.failAll()
	if !current {
		// A newer session for the same agent id took over (a redeploy with a
		// stable PULSE_INSTANCE_ID): the environment is still online through it.
		log.Printf("Agent '%s' handed over to a new session", agentID)
		return
	}
	if auto && !s.markSessionOffline(context.WithoutCancel(ctx), agentID, graceful) {
		log.Printf("Agent '%s' reconnected while its old session closed", agentID)
		return
	}
	log.Printf("Agent '%s' disconnected (graceful=%t)", agentID, graceful)
}

// markSessionOffline marks an auto-registered environment offline after its
// session ended, unless the agent has reconnected meanwhile. A fast reconnect
// can register its new session between this session's unregister and the
// database write; re-checking afterwards puts the environment back online.
// Reports whether the environment stayed offline.
func (s *Server) markSessionOffline(ctx context.Context, agentID string, graceful bool) bool {
	_ = s.DB.MarkEnvironmentOffline(ctx, agentID, graceful)
	if s.Router.IsConnected(agentID) {
		_ = s.DB.TouchEnvironment(ctx, agentID)
		return false
	}
	return true
}
