package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Stands up an in-process "gateway" WS endpoint, has the agent dial into it
// (push mode), then drives a Ping over the session and checks the correlated
// reply — the same flow the real Rust gateway's /agent/ws uses.
func TestUplinkServesPushSession(t *testing.T) {
	gotAuth := make(chan string, 1)
	gotReply := make(chan protocol.ResponseFrame, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth <- r.Header.Get("Authorization")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		// Act as the gateway: send a Ping request frame down the session.
		req, _ := json.Marshal(protocol.RequestFrame{ID: 1, Req: protocol.Request{Kind: protocol.ReqPing}})
		if err := c.Write(ctx, websocket.MessageText, req); err != nil {
			return
		}
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var rf protocol.ResponseFrame
		_ = json.Unmarshal(data, &rf)
		gotReply <- rf
		c.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	st := NewState(Config{Mode: ModeDial})
	go dialAndServe(context.Background(), st, Config{GatewayURL: url, GatewayToken: "s3cret"})

	select {
	case auth := <-gotAuth:
		if auth != "Bearer s3cret" {
			t.Errorf("auth header = %q, want %q", auth, "Bearer s3cret")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent never connected")
	}

	select {
	case rf := <-gotReply:
		if rf.ID != 1 {
			t.Errorf("reply id = %d, want 1 (correlation)", rf.ID)
		}
		if rf.Resp.Kind != protocol.RespKindPong {
			t.Errorf("reply = %s, want Pong", rf.Resp.Kind)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no reply over push session")
	}
}

// On shutdown (SIGTERM in k8s/ECS) the agent must get its Deregister onto the
// wire before the socket closes, so a redeploy isn't reported as an outage.
// Reading with the shutdown context used to close the socket first.
func TestUplinkDeregistersOnShutdown(t *testing.T) {
	connected := make(chan struct{})
	got := make(chan string, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		// Round-trip a Ping so we know the agent is serving (handshake done)
		// before the test triggers shutdown.
		ping, _ := json.Marshal(protocol.RequestFrame{ID: 1, Req: protocol.Request{Kind: protocol.ReqPing}})
		if c.Write(r.Context(), websocket.MessageText, ping) != nil {
			return
		}
		if _, _, err := c.Read(r.Context()); err != nil {
			return
		}
		close(connected)
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			var rf protocol.ResponseFrame
			if json.Unmarshal(data, &rf) == nil {
				got <- rf.Resp.Kind
			}
		}
	}))
	defer srv.Close()

	ctx, shutdown := context.WithCancel(context.Background())
	cfg := Config{
		GatewayURL:        "ws" + strings.TrimPrefix(srv.URL, "http"),
		AutoRegister:      true,
		InstanceID:        "i-1",
		HeartbeatInterval: time.Hour,
	}
	done := make(chan struct{})
	st := NewState(Config{Mode: ModeDial, InstanceID: cfg.InstanceID})
	go func() { _, _ = dialAndServe(ctx, st, cfg); close(done) }()

	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("agent never connected")
	}
	shutdown()

	select {
	case kind := <-got:
		if kind != protocol.RespKindDeregister {
			t.Errorf("first frame after shutdown = %s, want Deregister", kind)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("gateway never received Deregister on shutdown")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("uplink did not exit after shutdown")
	}
}

// A gateway that stops answering pings (a silently dead link) makes the agent
// drop the session, so its run loop can reconnect.
func TestUplinkDropsSessionWhenPingsGoUnanswered(t *testing.T) {
	defer func(i, to time.Duration) { pingInterval, pingTimeout = i, to }(pingInterval, pingTimeout)
	pingInterval, pingTimeout = 50*time.Millisecond, 100*time.Millisecond

	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		<-release // never reads, so never answers a ping
	}))
	defer srv.Close()

	type result struct {
		connected bool
		err       error
	}
	done := make(chan result, 1)
	st := NewState(Config{Mode: ModeDial})
	go func() {
		connected, err := dialAndServe(context.Background(), st, Config{GatewayURL: "ws" + strings.TrimPrefix(srv.URL, "http")})
		done <- result{connected, err}
	}()

	select {
	case r := <-done:
		if !r.connected || r.err == nil {
			t.Errorf("dialAndServe = (%t, %v), want (true, error)", r.connected, r.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent kept a session whose pings went unanswered")
	}
}

func TestJitterStaysInRange(t *testing.T) {
	for range 1000 {
		if d := jitter(4 * time.Second); d < 2*time.Second || d > 4*time.Second {
			t.Fatalf("jitter(4s) = %s, want within [2s, 4s]", d)
		}
	}
}
