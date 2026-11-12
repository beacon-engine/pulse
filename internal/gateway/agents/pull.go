package agents

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// ── Pull client ──────────────────────────────────────────────────────────────

type PullClient struct {
	addr string
}

func NewPullClient(addr string) *PullClient {
	return &PullClient{addr: addr}
}

// Call sends a request and returns the agent's response. Transport failures are
// surfaced as a 502 Response::Error so callers only ever match on Response.
func (c *PullClient) Call(ctx context.Context, req protocol.Request) protocol.Response {
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

// wsPullClient is a pull transport that reaches the agent over WebSocket
// (ws://|wss://). Like PullClient it is per-call: dial, send one RequestFrame,
// read the ResponseFrame, close. Rides HTTP(S), so it traverses firewalls/proxies
// that block raw TCP. TLS ("wss") verification uses the system roots.
type wsPullClient struct {
	url string
}

func (c *wsPullClient) Call(ctx context.Context, req protocol.Request) protocol.Response {
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
func (c *PullClient) WatchEvents(ctx context.Context, dockerHost string) (net.Conn, error) {
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
