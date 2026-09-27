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
