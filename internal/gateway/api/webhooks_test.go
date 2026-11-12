package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// recordingAgent is a pull agent that answers Ok to everything and passes each
// request to got.
func recordingAgent(t *testing.T, got chan<- protocol.Request) string {
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
				for {
					var req protocol.Request
					if protocol.ReadFrame(c, &req) != nil {
						return
					}
					got <- req
					if protocol.WriteFrame(c, protocol.RespOk()) != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func post(t *testing.T, c *http.Client, u string) (int, map[string]any) {
	t.Helper()
	resp, err := c.Post(u, "application/json", nil)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestWebhookK8sRestart(t *testing.T) {
	srv, st := apiTestServer(t)
	defer srv.Close()
	admin := authedClient(t, srv)
	anon := &http.Client{}

	got := make(chan protocol.Request, 4)
	addr := recordingAgent(t, got)
	hostID, _ := st.DB.CreateEnvironment(context.Background(), "cluster", "", nil, &addr)

	hook := srv.URL + "/api/webhooks/" + hostID + "/k8s/shop/StatefulSet/db/restart"
	tokenURL := srv.URL + "/api/webhooks/" + hostID + "/k8s/" + url.PathEscape("shop:StatefulSet:db") + "/token"

	// Not enabled yet: the action is reachable without a session but refused.
	if code, _ := post(t, anon, hook+"?token=x"); code != http.StatusForbidden {
		t.Errorf("disabled webhook = %d, want 403", code)
	}

	// Managing tokens still needs a session.
	if code, _ := post(t, anon, tokenURL); code != http.StatusUnauthorized {
		t.Errorf("anonymous token create = %d, want 401", code)
	}
	code, body := post(t, admin, tokenURL)
	if code != http.StatusOK {
		t.Fatalf("token create = %d", code)
	}
	token, _ := body["token"].(string)

	if code, _ := post(t, anon, hook+"?token=wrong"); code != http.StatusUnauthorized {
		t.Errorf("wrong token = %d, want 401", code)
	}
	// A token for one workload doesn't restart another.
	other := srv.URL + "/api/webhooks/" + hostID + "/k8s/shop/Deployment/db/restart?token=" + token
	if code, _ := post(t, anon, other); code != http.StatusForbidden {
		t.Errorf("token reused on another workload = %d, want 403", code)
	}

	code, body = post(t, anon, hook+"?token="+token)
	if code != http.StatusOK || body["restarted"] != true {
		t.Fatalf("restart = %d %v, want 200 restarted", code, body)
	}
	req := <-got
	if req.Kind != protocol.ReqK8sRestart || req.Namespace != "shop" || req.WorkloadKind != "StatefulSet" || req.Name != "db" {
		t.Errorf("agent got %+v, want K8sRestart shop/StatefulSet/db", req)
	}
}

func TestWebhookSessionRules(t *testing.T) {
	srv, _ := apiTestServer(t)
	defer srv.Close()
	authedClient(t, srv) // an admin exists, so 401 means "no session"
	anon := &http.Client{}

	for _, tc := range []struct {
		path string
		want int
	}{
		// Token-checked actions: reachable without a session, refused for the
		// unknown host or missing token rather than with 401 for no session.
		{"/api/webhooks/h/containers/web/deploy", http.StatusForbidden},
		{"/api/webhooks/h/swarm/svc/update", http.StatusForbidden},
		{"/api/webhooks/h/k8s/ns/web/rollout", http.StatusForbidden},
		{"/api/webhooks/h/stacks/s/redeploy", http.StatusNotFound},
		// No token of its own: session only.
		{"/api/webhooks/h/abc123/recreate", http.StatusUnauthorized},
		{"/api/webhooks/h/containers/web/token", http.StatusUnauthorized},
	} {
		if code, _ := post(t, anon, srv.URL+tc.path); code != tc.want {
			t.Errorf("anonymous POST %s = %d, want %d", tc.path, code, tc.want)
		}
	}
}
