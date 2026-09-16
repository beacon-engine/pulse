package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/uplink.rs — the push (dial) uplink.
//
// In ModeDial the agent connects *out* to the gateway over WSS and serves
// requests over that single session, reusing the same Dispatch as the TCP
// listener. This lets an agent that can only make outbound connections (443/WSS)
// still be driven by the gateway. The socket is bidirectional: the gateway sends
// RequestFrames down it and the agent replies with ResponseFrames on the same id.

// reconnectDelay is the wait between reconnect attempts after the session drops.
const reconnectDelay = 5 * time.Second

// RunUplink runs the uplink forever: (re)connect to url, serve until the session
// drops, then back off and retry.
func RunUplink(ctx context.Context, st *State, url, token string, insecureSkipTLSVerify bool) {
	for {
		if err := dialAndServe(ctx, st, url, token, insecureSkipTLSVerify); err != nil {
			log.Printf("Uplink to %s failed/closed: %v; reconnecting in %s", url, err, reconnectDelay)
		} else {
			log.Printf("Uplink to %s closed; reconnecting in %s", url, reconnectDelay)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// dialAndServe opens one WSS session and serves it until it closes or errors.
func dialAndServe(ctx context.Context, st *State, url, token string, insecure bool) error {
	opts := &websocket.DialOptions{}
	if token != "" {
		// Bearer token on the upgrade so the gateway can authenticate the agent
		// before accepting the session.
		opts.HTTPHeader = http.Header{"Authorization": {"Bearer " + token}}
	}
	if insecure {
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // opt-in via config
			},
		}
	}

	c, _, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		return err
	}
	defer c.CloseNow()
	// Responses (logs, inspect, image lists) can exceed the 32 KiB default.
	c.SetReadLimit(protocol.MaxFrame)

	log.Printf("Uplink connected to gateway %s", url)
	return serve(ctx, st, c)
}

// serve reads RequestFrames off the session, dispatches each, and writes the
// correlated ResponseFrame back. Keepalive ping/pong is handled by the library.
func serve(ctx context.Context, st *State, c *websocket.Conn) error {
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			// Clean close or transport error — either way the run loop reconnects.
			return err
		}
		if typ != websocket.MessageText {
			continue
		}

		var frame protocol.RequestFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			log.Printf("uplink: malformed request frame: %v", err)
			continue
		}

		resp := Dispatch(ctx, st, frame.Req)
		out, err := json.Marshal(protocol.ResponseFrame{ID: frame.ID, Resp: resp})
		if err != nil {
			log.Printf("uplink: failed to encode response: %v", err)
			continue
		}
		if err := c.Write(ctx, websocket.MessageText, out); err != nil {
			return err
		}
	}
}
