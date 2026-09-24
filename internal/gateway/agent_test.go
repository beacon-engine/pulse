package gateway

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// The mirror of the Rust session_ws_tests, from the gateway side: an agent dials
// the Go gateway's /agent/ws, the session registers, and a routed CallOn(Ping)
// correlates to Pong. Proves the Go gateway accepts the (Go) agent's push session.
func TestAgentWSAcceptsPushSessionAndCorrelates(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(http.HandlerFunc(st.AgentWSHandler))
	defer srv.Close()

	ctx := context.Background()
	c := dialAgent(t, ctx, srv.URL, "s3cret")
	defer c.CloseNow()

	// Fake agent: reply to each RequestFrame with Pong on the same id.
	go func() {
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			var rf protocol.RequestFrame
			if json.Unmarshal(data, &rf) != nil {
				continue
			}
			reply, _ := json.Marshal(protocol.ResponseFrame{ID: rf.ID, Resp: protocol.RespPong()})
			if c.Write(ctx, websocket.MessageText, reply) != nil {
				return
			}
		}
	}()

	waitRegistered(t, st, "prod", true)

	id := "prod"
	resp := st.Agent.CallOn(ctx, &id, nil, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("CallOn(prod, Ping) = %s, want Pong", resp.Kind)
	}
}

func TestAgentWSRejectsBadToken(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(http.HandlerFunc(st.AgentWSHandler))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer nope"}},
	})
	if err == nil {
		t.Fatal("expected the upgrade to be rejected for an invalid token")
	}
	if ids := st.Agent.ConnectedAgentIDs(); len(ids) != 0 {
		t.Errorf("no session should be registered, got %v", ids)
	}
}

func TestAgentWSDisconnectUnregisters(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(http.HandlerFunc(st.AgentWSHandler))
	defer srv.Close()

	ctx := context.Background()
	c := dialAgent(t, ctx, srv.URL, "s3cret")
	waitRegistered(t, st, "prod", true)

	c.CloseNow() // agent drops → gateway tears the session down
	waitRegistered(t, st, "prod", false)

	// With the session gone, a routed call falls back to the (dead) default
	// pull client and fails fast rather than hanging.
	id := "prod"
	resp := st.Agent.CallOn(ctx, &id, nil, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindError {
		t.Errorf("post-disconnect CallOn = %s, want Error (fallback)", resp.Kind)
	}
}

func testState() *State {
	// The default pull client points at a dead port; push tests never hit it,
	// and the disconnect test relies on that fallback failing.
	return &State{
		Agent:       NewAgentRouter(NewAgentClient("127.0.0.1:1")),
		AgentTokens: map[string]string{"s3cret": "prod"},
		Events:      NewHub(),
	}
}

func dialAgent(t *testing.T, ctx context.Context, srvURL, token string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srvURL, "http")
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + token}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func waitRegistered(t *testing.T, st *State, id string, present bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		found := false
		for _, x := range st.Agent.ConnectedAgentIDs() {
			if x == id {
				found = true
			}
		}
		if found == present {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("agent %q registration != %v within timeout", id, present)
}

// Proves the architectural fix: a host with its own agent_addr routes to THAT
// pull agent, not the default AGENT_ADDR. The default here points at a dead port,
// so a correct route must reach the live per-host agent.
func TestCallOnRoutesToPerHostAgentAddr(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// A minimal pull agent that answers Ping→Pong over the framed protocol.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				for {
					var req protocol.Request
					if protocol.ReadFrame(c, &req) != nil {
						return
					}
					resp := protocol.RespOk()
					if req.Kind == protocol.ReqPing {
						resp = protocol.RespPong()
					}
					if protocol.WriteFrame(c, resp) != nil {
						return
					}
				}
			}(conn)
		}
	}()

	// Default points at a dead port; the host supplies its own agent_addr.
	router := NewAgentRouter(NewAgentClient("127.0.0.1:1"))
	addr := ln.Addr().String()

	resp := router.CallOn(context.Background(), nil, &addr, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("per-host agent_addr route = %s, want Pong (must not hit the dead default)", resp.Kind)
	}

	// No agent_addr → falls back to the dead default → transport error.
	resp = router.CallOn(context.Background(), nil, nil, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindError {
		t.Errorf("nil agent_addr should fall back to the (dead) default → Error, got %s", resp.Kind)
	}
}

// Proves the firewall-friendly pull path: a host whose agent_addr is a ws:// URL
// is routed over WebSocket (not raw TCP), reaching a WS-serving agent.
func TestCallOnRoutesToWSAgent(t *testing.T) {
	// A fake WS agent: replies Pong to each RequestFrame on any path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			var f protocol.RequestFrame
			if json.Unmarshal(data, &f) != nil {
				continue
			}
			resp := protocol.RespOk()
			if f.Req.Kind == protocol.ReqPing {
				resp = protocol.RespPong()
			}
			reply, _ := json.Marshal(protocol.ResponseFrame{ID: f.ID, Resp: resp})
			if c.Write(r.Context(), websocket.MessageText, reply) != nil {
				return
			}
		}
	}))
	defer srv.Close()

	// Bare ws://host (no path) — the router appends /agent/ws. Default is a dead
	// TCP port, so success proves the WS transport was chosen and used.
	wsAddr := "ws" + strings.TrimPrefix(srv.URL, "http")
	router := NewAgentRouter(NewAgentClient("127.0.0.1:1"))

	resp := router.CallOn(context.Background(), nil, &wsAddr, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("ws:// agent_addr route = %s, want Pong (should use the WebSocket transport)", resp.Kind)
	}
}
