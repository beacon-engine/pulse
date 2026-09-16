package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

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

// Serve accepts connections on ln and serves each until it closes. Split from
// ServeTCP so tests can supply an ephemeral loopback listener.
func Serve(ctx context.Context, st *State, ln net.Listener) error {
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
			// Peer closed cleanly between frames — normal.
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return
			}
			log.Printf("connection error from %s: %v", conn.RemoteAddr(), err)
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
			_ = watchEvents(ctx, cli, func(r protocol.Response) error {
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
