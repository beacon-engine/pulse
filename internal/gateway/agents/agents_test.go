package agents

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

	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// The mirror of the Rust session_ws_tests, from the gateway side: an agent dials
// the Go gateway's /agent/ws, the session registers, and a routed CallOn(Ping)
// correlates to Pong. Proves the Go gateway accepts the (Go) agent's push session.
func TestAgentWSAcceptsPushSessionAndCorrelates(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(st)
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
	resp := st.Router.CallOn(ctx, &id, nil, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("CallOn(prod, Ping) = %s, want Pong", resp.Kind)
	}
}

func TestAgentWSRejectsBadToken(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(st)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer nope"}},
	})
	if err == nil {
		t.Fatal("expected the upgrade to be rejected for an invalid token")
	}
	if ids := st.Router.ConnectedAgentIDs(); len(ids) != 0 {
		t.Errorf("no session should be registered, got %v", ids)
	}
}

func TestAgentWSDisconnectUnregisters(t *testing.T) {
	st := testState()
	srv := httptest.NewServer(st)
	defer srv.Close()

	ctx := context.Background()
	c := dialAgent(t, ctx, srv.URL, "s3cret")
	waitRegistered(t, st, "prod", true)

	c.CloseNow() // agent drops → gateway tears the session down
	waitRegistered(t, st, "prod", false)

	// With the session gone, a routed call fails fast with "not connected"
	// rather than hanging or answering from the default agent.
	id := "prod"
	resp := st.Router.CallOn(ctx, &id, nil, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindError {
		t.Errorf("post-disconnect CallOn = %s, want Error (not connected)", resp.Kind)
	}
}

// An agent that declares an instance id on the upgrade is auto-registered as an
// environment keyed by "<token_id>:<instance>", and marked offline when its
// session drops.
func TestAgentWSAutoRegisters(t *testing.T) {
	ctx := context.Background()
	repo, err := storage.Init(ctx, "sqlite::memory:", false)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}
	st := &Server{
		Router: NewRouter(NewPullClient("127.0.0.1:1")),
		Tokens: map[string]string{"s3cret": "prod"},
		DB:     repo,
	}
	srv := httptest.NewServer(st)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization":       {"Bearer s3cret"},
			"X-Pulse-Instance":    {"inst-1"},
			"X-Pulse-Name":        {"web-1"},
			"X-Pulse-Docker-Host": {"unix:///var/run/docker.sock"},
		},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Registers under the composite identity, not just the token id.
	waitRegistered(t, st, "prod:inst-1", true)

	envs, _ := st.DB.ListEnvironmentsPage(ctx, "web-1", 10, 0)
	if len(envs) != 1 || envs[0].Source != "auto" || envs[0].Status != "online" {
		t.Fatalf("auto env not created correctly: %+v", envs)
	}
	envID := envs[0].ID

	// Abrupt disconnect → session gone and the environment marked offline.
	c.CloseNow()
	waitRegistered(t, st, "prod:inst-1", false)

	deadline := time.Now().Add(2 * time.Second)
	for {
		e, _ := st.DB.GetEnvironment(ctx, envID)
		if e != nil && e.Status == "offline" && e.OfflineSince != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("env not marked offline after disconnect: %+v", e)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func testState() *Server {
	// The default pull client points at a dead port; push tests never hit it,
	// and the disconnect test relies on that fallback failing.
	return &Server{
		Router: NewRouter(NewPullClient("127.0.0.1:1")),
		Tokens: map[string]string{"s3cret": "prod"},
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

func waitRegistered(t *testing.T, st *Server, id string, present bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		found := false
		for _, x := range st.Router.ConnectedAgentIDs() {
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
	router := NewRouter(NewPullClient("127.0.0.1:1"))
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
	router := NewRouter(NewPullClient("127.0.0.1:1"))

	resp := router.CallOn(context.Background(), nil, &wsAddr, protocol.Request{Kind: protocol.ReqPing})
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("ws:// agent_addr route = %s, want Pong (should use the WebSocket transport)", resp.Kind)
	}
}

// A host owned by a push agent that isn't connected must not be answered by the
// default agent (a different machine). Here the default is a live agent that
// would happily reply, so only an explicit refusal passes.
func TestCallOnOfflinePushAgentDoesNotFallBack(t *testing.T) {
	live := fakePullAgent(t, protocol.RespCapabilities(protocol.Capabilities{}))
	router := NewRouter(NewPullClient(live))

	id := "k8s:gone"
	resp := router.CallOn(context.Background(), &id, nil, protocol.Request{Kind: protocol.ReqCapabilities})
	status, msg, isErr := resp.AsError()
	if !isErr || status != 503 || !strings.Contains(msg, "not connected") {
		t.Errorf("offline push agent = %s %d %q, want 503 not connected", resp.Kind, status, msg)
	}
}

// With a stable agent id, a redeploy's new session registers before the old one
// leaves. The old session's teardown must not evict the replacement.
func TestUnregisterIfKeepsReplacementSession(t *testing.T) {
	router := NewRouter(NewPullClient("127.0.0.1:1"))
	oldT := &SessionTransport{done: make(chan struct{})}
	newT := &SessionTransport{done: make(chan struct{})}

	router.Register("k8s:cluster", oldT)
	router.Register("k8s:cluster", newT) // new pod connects first

	if router.UnregisterIf("k8s:cluster", oldT) {
		t.Errorf("old session's teardown removed the replacement")
	}
	if ids := router.ConnectedAgentIDs(); len(ids) != 1 {
		t.Errorf("connected = %v, want the replacement still registered", ids)
	}
	if !router.UnregisterIf("k8s:cluster", newT) {
		t.Errorf("current session should unregister")
	}
}

// An agent whose link dies silently (it stops reading, so pings go unanswered)
// is dropped by the keepalive instead of keeping a session that swallows
// requests until they time out.
func TestAgentSessionDroppedWhenPingsGoUnanswered(t *testing.T) {
	defer func(i, to time.Duration) { sessionPingInterval, sessionPingTimeout = i, to }(sessionPingInterval, sessionPingTimeout)
	sessionPingInterval, sessionPingTimeout = 50*time.Millisecond, 100*time.Millisecond

	st := testState()
	srv := httptest.NewServer(st)
	defer srv.Close()

	c := dialAgent(t, context.Background(), srv.URL, "s3cret")
	defer c.CloseNow()
	// The fake agent never reads, so it never answers the gateway's pings.
	waitRegistered(t, st, "prod", true)
	waitRegistered(t, st, "prod", false)
}

// A session's teardown that loses the race with the agent's reconnect must not
// leave the environment offline while the new session is live.
func TestOldSessionTeardownDoesNotOfflineReconnectedAgent(t *testing.T) {
	ctx := context.Background()
	repo, err := storage.Init(ctx, "sqlite::memory:", false)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}
	st := &Server{Router: NewRouter(NewPullClient("127.0.0.1:1")), DB: repo}
	agentID := "prod:i-1"
	if _, err := st.DB.UpsertAutoEnvironment(ctx, agentID, "cluster", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	status := func() string {
		envs, _ := st.DB.ListEnvironmentsPage(ctx, "cluster", 10, 0)
		if len(envs) != 1 {
			t.Fatalf("envs = %+v", envs)
		}
		return envs[0].Status
	}

	// The reconnect registered before the old session's offline write landed.
	st.Router.Register(agentID, &SessionTransport{})
	if st.markSessionOffline(ctx, agentID, false) {
		t.Error("markSessionOffline reported offline although the agent is connected")
	}
	if s := status(); s != "online" {
		t.Errorf("status = %s, want online (a live session exists)", s)
	}

	// With no live session it really goes offline.
	st.Router.UnregisterIf(agentID, st.Router.sessions[agentID])
	if !st.markSessionOffline(ctx, agentID, false) {
		t.Error("markSessionOffline kept a disconnected agent online")
	}
	if s := status(); s != "offline" {
		t.Errorf("status = %s, want offline", s)
	}
}

// fakePullAgent serves the framed TCP protocol, answering every request with
// resp, and returns its address.
func fakePullAgent(t *testing.T, resp protocol.Response) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req protocol.Request
				if protocol.ReadFrame(c, &req) != nil {
					return
				}
				_ = protocol.WriteFrame(c, resp)
			}(conn)
		}
	}()
	return ln.Addr().String()
}
