package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/beacon-engine/pulse/internal/agent/docker"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// ServeTCP binds 0.0.0.0:port and serves gateway connections (pull / ModeListen).
func ServeTCP(ctx context.Context, st *State, port uint16) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("bind tcp port %d: %w", port, err)
	}
	return Serve(ctx, st, ln)
}

// ServeWS serves gateway connections in Listen mode over WebSocket (HTTP), so
// the pull path rides 80/443 and traverses firewalls/proxies that block raw TCP
// — the same reason push mode uses WSS. The gateway dials `/agent/ws` and drives
// the exact same request/response loop as the push uplink.
func ServeWS(ctx context.Context, st *State, port uint16) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		c.SetReadLimit(protocol.MaxFrame)
		// Pull mode: the gateway drives; no self-registration/heartbeat here.
		_ = serve(r.Context(), st, c, Config{})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("Pulse agent listening on ws://%s/agent/ws", addr)
	srv := &http.Server{Addr: addr, Handler: mux}

	// Graceful shutdown on ctx cancel (SIGTERM).
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Serve accepts connections on ln and serves each until it closes. Split from
// ServeTCP so tests can supply an ephemeral loopback listener.
func Serve(ctx context.Context, st *State, ln net.Listener) error {
	// Unblock Accept and stop the loop when ctx is cancelled (SIGTERM).
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	log.Printf("Pulse agent listening on tcp://%s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed → clean shutdown.
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConn(ctx, st, conn)
	}
}

// handleConn reads framed requests from one connection until EOF, answering each
// in turn. (A WatchEvents request will eventually take over the connection and
// stream events; today it returns a 501 like the other stubs.)
func handleConn(ctx context.Context, st *State, conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		var req protocol.Request
		if err := protocol.ReadFrame(r, &req); err != nil {
			switch {
			// Peer closed cleanly between frames — normal.
			case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF):
			// A non-protocol client spoke to this port: an HTTP request, a k8s
			// httpGet probe (use tcpSocket instead), or a Docker client pointed
			// here by mistake. This port speaks the binary gateway protocol only.
			case errors.Is(err, protocol.ErrFrameTooLarge):
				log.Printf("ignoring non-protocol connection from %s (this port speaks the binary gateway protocol, not HTTP — e.g. a k8s httpGet probe or a mis-set docker_host)", conn.RemoteAddr())
			default:
				log.Printf("connection error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		// WatchEvents takes over this connection and streams event frames until
		// the daemon stream ends or the gateway closes the socket.
		if req.Kind == protocol.ReqWatchEvents {
			cli, err := st.Docker.ClientFor(req.DockerHost)
			if err != nil {
				_ = protocol.WriteFrame(conn, connError(err.Error()))
				return
			}
			_ = docker.WatchEvents(ctx, cli, func(r protocol.Response) error {
				return protocol.WriteFrame(conn, r)
			})
			return
		}

		resp := Dispatch(ctx, st, req)
		if err := protocol.WriteFrame(conn, resp); err != nil {
			return
		}
	}
}
