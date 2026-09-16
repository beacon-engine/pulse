package gateway

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
	resp := st.Agent.CallOn(ctx, &id, protocol.Request{Kind: protocol.ReqPing})
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
	resp := st.Agent.CallOn(ctx, &id, protocol.Request{Kind: protocol.ReqPing})
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
