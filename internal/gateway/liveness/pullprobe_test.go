package liveness

import (
	"context"
	"net"
	"testing"

	"github.com/beacon-engine/pulse/internal/gateway/agents"
	"github.com/beacon-engine/pulse/internal/gateway/events"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// pongAgent serves the framed protocol, answering Ping with Pong. Closing the
// listener takes the agent down.
func pongAgent(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req protocol.Request
				if protocol.ReadFrame(c, &req) != nil {
					return
				}
				resp := protocol.RespError(400, "unknown request")
				if req.Kind == protocol.ReqPing {
					resp = protocol.RespPong()
				}
				_ = protocol.WriteFrame(c, resp)
			}(conn)
		}
	}()
	return ln
}

func TestPullProberTracksReachability(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Init(ctx, "sqlite::memory:", false)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}
	router := agents.NewRouter(agents.NewPullClient("127.0.0.1:1"))
	st := &Monitor{DB: db, Router: router, Notifier: &events.Notifier{DB: db, Hub: events.NewHub()}}

	ln := pongAgent(t)
	addr := ln.Addr().String()
	pullID, _ := st.DB.CreateEnvironment(ctx, "k3s-pull", "", nil, &addr)
	// Not probed: no agent address, and a push agent that calls home itself.
	plainID, _ := st.DB.CreateEnvironment(ctx, "endpoint", "tcp://127.0.0.1:1", nil, nil)
	pushID := "tok:instance"
	if _, err := st.DB.UpsertAutoEnvironment(ctx, pushID, "push", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	status := func(id string) (string, *string, *string) {
		t.Helper()
		env, err := st.DB.GetEnvironment(ctx, id)
		if err != nil || env == nil {
			t.Fatalf("get %s: %v", id, err)
		}
		return env.Status, env.LastSeen, env.OfflineSince
	}

	st.probePullAgents(ctx)
	if s, seen, _ := status(pullID); s != "online" || seen == nil {
		t.Fatalf("reachable pull agent: status=%s last_seen=%v, want online with last_seen", s, seen)
	}

	ln.Close()
	st.probePullAgents(ctx)
	s, _, since := status(pullID)
	if s != "offline" || since == nil {
		t.Fatalf("dead pull agent: status=%s offline_since=%v, want offline with offline_since", s, since)
	}
	// A second failed round keeps the time it first went down.
	first := *since
	st.probePullAgents(ctx)
	if _, _, since := status(pullID); since == nil || *since != first {
		t.Errorf("offline_since moved on a repeat failure: %v, want %s", since, first)
	}

	// It counts for the down alert, and is never deleted by retention.
	pending, _ := st.DB.ListEnvironmentsPendingAlert(ctx, 0)
	if len(pending) != 1 || pending[0].ID != pullID {
		t.Errorf("pending alerts = %+v, want only the pull env", pending)
	}
	if _, err := st.DB.DeleteExpiredEnvironments(ctx, 0); err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if env, _ := st.DB.GetEnvironment(ctx, pullID); env == nil {
		t.Error("retention deleted a manually added pull environment")
	}

	if s, _, _ := status(plainID); s != "online" {
		t.Errorf("endpoint env was probed: status=%s", s)
	}
	envs, _ := st.DB.ListPullEnvironments(ctx)
	if len(envs) != 1 || envs[0].ID != pullID {
		t.Errorf("ListPullEnvironments = %+v, want only the pull env", envs)
	}
}
