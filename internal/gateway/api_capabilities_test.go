package gateway

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// fakePullAgent serves the framed TCP protocol, answering Capabilities with
// resp, and returns its address.
func fakePullAgent(t *testing.T, resp protocol.Response) string {
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
					out := protocol.RespError(400, "unexpected "+req.Kind)
					if req.Kind == protocol.ReqCapabilities {
						out = resp
					}
					if protocol.WriteFrame(c, out) != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func getCapabilities(t *testing.T, c *http.Client, url string) HostCapabilities {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET capabilities: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("capabilities status = %d, want 200", resp.StatusCode)
	}
	var out HostCapabilities
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestAPIHostCapabilities(t *testing.T) {
	srv, st := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)
	ctx := context.Background()

	// A Kubernetes-only agent: no container runtime, which is a valid environment.
	k8sOnly := fakePullAgent(t, protocol.RespCapabilities(protocol.Capabilities{
		Runtimes:    []protocol.RuntimeInfo{{Kind: protocol.RuntimeKubernetes, Version: "v1.36.5"}},
		Unavailable: []protocol.RuntimeProbe{{Kind: "container", Error: "no docker socket"}},
	}))
	id, _ := st.DB.CreateEnvironment(ctx, "cluster", "", nil, &k8sOnly)
	got := getCapabilities(t, c, srv.URL+"/api/hosts/"+id+"/capabilities")
	if !got.Reachable || len(got.Runtimes) != 1 || got.Runtimes[0].Kind != protocol.RuntimeKubernetes {
		t.Errorf("k8s-only capabilities = %+v, want reachable with only kubernetes", got)
	}
	if got.HasContainerRuntime() {
		t.Errorf("k8s-only agent must not report a container runtime")
	}

	// An agent from before Capabilities existed answers "unknown request": the
	// gateway assumes the Docker(+k8s) agent it always was, so nothing disappears.
	legacy := fakePullAgent(t, protocol.RespError(400, "unknown request: Capabilities"))
	id, _ = st.DB.CreateEnvironment(ctx, "old", "", nil, &legacy)
	got = getCapabilities(t, c, srv.URL+"/api/hosts/"+id+"/capabilities")
	if !got.Reachable || !got.HasContainerRuntime() {
		t.Errorf("legacy agent capabilities = %+v, want reachable docker", got)
	}

	// The default agent is a dead port: still 200, reported as unreachable.
	id, _ = st.DB.CreateEnvironment(ctx, "dead", "", nil, nil)
	got = getCapabilities(t, c, srv.URL+"/api/hosts/"+id+"/capabilities")
	if got.Reachable || got.Error == "" {
		t.Errorf("dead agent capabilities = %+v, want unreachable with an error", got)
	}
}

func TestAPICreateHostValidation(t *testing.T) {
	srv, _ := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)

	cases := []struct {
		body string
		want int
	}{
		{`{"name":"","docker_host":"tcp://h:2375"}`, http.StatusBadRequest},
		{`{"name":"nothing","docker_host":"  "}`, http.StatusBadRequest},
		{`{"name":"agent only","docker_host":"","agent_addr":"10.0.0.5:9000"}`, http.StatusOK},
		{`{"name":"endpoint only","docker_host":"tcp://h:2375"}`, http.StatusOK},
	}
	for _, tc := range cases {
		resp, err := c.Post(srv.URL+"/api/hosts", "application/json", strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("POST hosts: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("POST %s = %d, want %d", tc.body, resp.StatusCode, tc.want)
		}
	}
}
