package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"math/rand/v2"
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

// Reconnect backoff: the wait starts at minReconnectDelay, doubles after each
// failed dial up to maxReconnectDelay, and resets once a session is
// established. Each wait is jittered so a fleet doesn't reconnect in lockstep
// after a gateway restart.
const (
	minReconnectDelay = 1 * time.Second
	maxReconnectDelay = 30 * time.Second
)

// Keepalive: the agent pings the gateway every pingInterval and drops the
// session when a pong doesn't arrive within pingTimeout. Without it a link
// that dies silently (network drop, gateway host crash) goes unnoticed until a
// write fails, which TCP can delay for many minutes. Variables so tests can
// shorten them.
var (
	pingInterval = 20 * time.Second
	pingTimeout  = 10 * time.Second
)

// deregisterTimeout bounds the best-effort goodbye sent on shutdown.
const deregisterTimeout = 2 * time.Second

// RunUplink runs the uplink forever: (re)connect to the gateway, serve until the
// session drops, then back off and retry.
func RunUplink(ctx context.Context, st *State, cfg Config) {
	delay := minReconnectDelay
	for {
		connected, err := dialAndServe(ctx, st, cfg)
		if ctx.Err() != nil {
			return
		}
		if connected {
			delay = minReconnectDelay
		}
		wait := jitter(delay)
		if err != nil {
			log.Printf("Uplink to %s failed/closed: %v; reconnecting in %s", cfg.GatewayURL, err, wait.Round(time.Millisecond))
		} else {
			log.Printf("Uplink to %s closed; reconnecting in %s", cfg.GatewayURL, wait.Round(time.Millisecond))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if !connected {
			delay = min(delay*2, maxReconnectDelay)
		}
	}
}

// jitter returns a random wait in [d/2, d].
func jitter(d time.Duration) time.Duration {
	return d/2 + rand.N(d/2+1)
}

// dialAndServe opens one WSS session and serves it until it closes or errors.
// connected reports whether the session was established, so the caller can
// reset its backoff.
func dialAndServe(ctx context.Context, st *State, cfg Config) (connected bool, err error) {
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
		header.Set("X-Pulse-Instance", st.InstanceID(ctx))
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
		return false, err
	}
	defer c.CloseNow()
	// Responses (logs, inspect, image lists) can exceed the 32 KiB default.
	c.SetReadLimit(protocol.MaxFrame)

	log.Printf("Uplink connected to gateway %s", cfg.GatewayURL)
	return true, serve(ctx, st, c, cfg)
}

// serve reads RequestFrames off the session, dispatches each, and writes the
// correlated ResponseFrame back. Requests run concurrently (the gateway
// correlates replies by id), so a slow one doesn't hold up the rest or starve
// the read loop, which is also what processes keepalive pongs. When
// auto-registering it also runs the heartbeat and the shutdown deregister.
// Writes are serialized: replies, the heartbeat and the deregister all share
// one socket.
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

	// Background work for this session stops when it ends.
	sessCtx, endSession := context.WithCancel(ctx)
	defer endSession()

	go keepalive(sessCtx, c, pingInterval, pingTimeout)
	if cfg.AutoRegister {
		go runHeartbeat(sessCtx, cfg.HeartbeatInterval, write)
	}

	// The read loop must not use ctx: the websocket library closes the
	// connection the moment a Read's context is cancelled, which would drop the
	// socket before we could say goodbye. Instead, on shutdown, send the
	// deregister (so the gateway marks us offline gracefully and skips the down
	// alert) and then close cleanly, which ends the read loop.
	stopShutdown := context.AfterFunc(ctx, func() {
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deregisterTimeout)
		defer cancel()
		if cfg.AutoRegister {
			_ = write(wctx, protocol.ResponseFrame{ID: protocol.EventStreamID, Resp: protocol.RespDeregister()})
		}
		_ = c.Close(websocket.StatusNormalClosure, "agent shutting down")
	})
	defer stopShutdown()
	readCtx := context.WithoutCancel(ctx)

	for {
		typ, data, err := c.Read(readCtx)
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

		go func() {
			// The request runs for the agent's lifetime, not the session's: a
			// dropped link shouldn't abort a half-done deploy. Only the reply
			// is lost.
			resp := Dispatch(ctx, st, frame.Req)
			if err := write(sessCtx, protocol.ResponseFrame{ID: frame.ID, Resp: resp}); err != nil && sessCtx.Err() == nil {
				// The socket is broken; closing it ends the read loop, which
				// reconnects.
				c.CloseNow()
			}
		}()
	}
}

// keepalive pings the gateway every interval and closes the socket when a pong
// doesn't come back within timeout; the read loop then fails and the run loop
// reconnects. It stops quietly when the session ends.
func keepalive(ctx context.Context, c *websocket.Conn, interval, timeout time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, timeout)
		err := c.Ping(pctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Uplink: gateway didn't answer a ping within %s; reconnecting", timeout)
			c.CloseNow()
			return
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
