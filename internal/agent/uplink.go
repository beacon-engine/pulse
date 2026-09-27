package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"sync"
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
//
// When AutoRegister is on the agent also self-registers as an environment: it
// declares its identity on the WS upgrade (headers), then calls home with a
// heartbeat on an interval so the gateway can track liveness, and sends a
// deregister on clean shutdown so scale-down doesn't look like an outage.

// reconnectDelay is the wait between reconnect attempts after the session drops.
const reconnectDelay = 5 * time.Second

// deregisterTimeout bounds the best-effort goodbye sent on shutdown.
const deregisterTimeout = 2 * time.Second

// RunUplink runs the uplink forever: (re)connect to the gateway, serve until the
// session drops, then back off and retry.
func RunUplink(ctx context.Context, st *State, cfg Config) {
	for {
		if err := dialAndServe(ctx, st, cfg); err != nil {
			log.Printf("Uplink to %s failed/closed: %v; reconnecting in %s", cfg.GatewayURL, err, reconnectDelay)
		} else {
			log.Printf("Uplink to %s closed; reconnecting in %s", cfg.GatewayURL, reconnectDelay)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// dialAndServe opens one WSS session and serves it until it closes or errors.
func dialAndServe(ctx context.Context, st *State, cfg Config) error {
	opts := &websocket.DialOptions{}
	header := http.Header{}
	if cfg.GatewayToken != "" {
		// Bearer token on the upgrade so the gateway can authenticate the agent
		// before accepting the session.
		header.Set("Authorization", "Bearer "+cfg.GatewayToken)
	}
	if cfg.AutoRegister {
		// Self-registration identity, read by the gateway pre-upgrade. The
		// instance id makes registration idempotent across restarts/reconnects.
		header.Set("X-Pulse-Instance", cfg.InstanceID)
		header.Set("X-Pulse-Name", cfg.EnvName)
		header.Set("X-Pulse-Docker-Host", cfg.DockerHost)
	}
	if len(header) > 0 {
		opts.HTTPHeader = header
	}
	if cfg.InsecureSkipTLSVerify {
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // opt-in via config
			},
		}
	}

	c, _, err := websocket.Dial(ctx, cfg.GatewayURL, opts)
	if err != nil {
		return err
	}
	defer c.CloseNow()
	// Responses (logs, inspect, image lists) can exceed the 32 KiB default.
	c.SetReadLimit(protocol.MaxFrame)

	log.Printf("Uplink connected to gateway %s", cfg.GatewayURL)
	return serve(ctx, st, c, cfg)
}

// serve reads RequestFrames off the session, dispatches each, and writes the
// correlated ResponseFrame back. When auto-registering it also runs the
// heartbeat and the shutdown deregister. Keepalive ping/pong is handled by the
// library. Writes are serialized: the reply path, the heartbeat and the
// deregister all share one socket.
func serve(ctx context.Context, st *State, c *websocket.Conn, cfg Config) error {
	var writeMu sync.Mutex
	write := func(wctx context.Context, frame protocol.ResponseFrame) error {
		out, err := json.Marshal(frame)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return c.Write(wctx, websocket.MessageText, out)
	}

	if cfg.AutoRegister {
		go runHeartbeat(ctx, cfg.HeartbeatInterval, write)
		// On our own shutdown (ctx cancelled), say goodbye so the gateway marks
		// us offline gracefully and skips the down alert. Best-effort; the socket
		// is still open here (CloseNow runs in the caller's defer, after this).
		defer func() {
			if ctx.Err() == nil {
				return // a dropped session, not a deliberate shutdown — don't deregister
			}
			wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deregisterTimeout)
			defer cancel()
			_ = write(wctx, protocol.ResponseFrame{ID: protocol.EventStreamID, Resp: protocol.RespDeregister()})
		}()
	}

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
		if err := write(ctx, protocol.ResponseFrame{ID: frame.ID, Resp: resp}); err != nil {
			return err
		}
	}
}

// runHeartbeat pushes a heartbeat on EventStreamID every interval until the
// session ends. A write error means the socket is gone; the read loop will
// observe the same and drive reconnect, so we just stop.
func runHeartbeat(ctx context.Context, interval time.Duration, write func(context.Context, protocol.ResponseFrame) error) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := write(ctx, protocol.ResponseFrame{ID: protocol.EventStreamID, Resp: protocol.RespHeartbeat()}); err != nil {
				return
			}
		}
	}
}
