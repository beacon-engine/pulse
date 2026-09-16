package agent

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Starts the agent on a loopback listener and drives it over the real wire
// protocol, proving framing + dispatch work end to end.
func TestServeDispatchesOverWire(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	st := NewState(Config{Mode: ModeListen})
	go Serve(context.Background(), st, ln)

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Ping → Pong (implemented).
	if err := protocol.WriteFrame(conn, protocol.Request{Kind: protocol.ReqPing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	var resp protocol.Response
	if err := protocol.ReadFrame(conn, &resp); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if resp.Kind != protocol.RespKindPong {
		t.Errorf("Ping => %s, want Pong", resp.Kind)
	}

	// An unknown request kind → Error (deterministic, no Docker needed). Sent as
	// a raw frame since MarshalJSON rightly refuses to encode an unknown kind.
	if err := protocol.WriteFrame(conn, json.RawMessage(`{"NotARealRequest":{}}`)); err != nil {
		t.Fatalf("write req: %v", err)
	}
	if err := protocol.ReadFrame(conn, &resp); err != nil {
		t.Fatalf("read resp: %v", err)
	}
	if resp.Kind != protocol.RespKindError {
		t.Errorf("unknown request => %s, want Error", resp.Kind)
	}
}
