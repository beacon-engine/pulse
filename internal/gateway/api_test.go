package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/protocol"
)

func apiTestServer(t *testing.T) (*httptest.Server, *State) {
	t.Helper()
	db, err := storage.Init(context.Background(), "sqlite::memory:", false)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}
	st := &State{
		Agent:       NewAgentRouter(NewAgentClient("127.0.0.1:1")),
		AgentTokens: map[string]string{},
		DB:          db,
		Events:      NewHub(),
	}
	return httptest.NewServer(st.Routes()), st
}

// authedClient creates the first admin via /api/auth/setup and returns a client
// carrying the resulting session cookie.
func authedClient(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	resp, err := c.Post(srv.URL+"/api/auth/setup", "application/json",
		strings.NewReader(`{"username":"admin","password":"password123"}`))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup status = %d", resp.StatusCode)
	}
	return c
}

func TestAuthGate(t *testing.T) {
	srv, _ := apiTestServer(t)
	defer srv.Close()

	// Unauthenticated protected route → 401.
	resp, _ := http.Get(srv.URL + "/api/hosts")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthed /api/hosts = %d, want 401", resp.StatusCode)
	}

	// Public setup-status → 200 with needs_setup=true.
	resp, _ = http.Get(srv.URL + "/api/auth/setup")
	var status map[string]bool
	json.NewDecoder(resp.Body).Decode(&status)
	resp.Body.Close()
	if !status["needs_setup"] {
		t.Errorf("needs_setup = %v, want true", status["needs_setup"])
	}
}

func TestAPIHostsCRUD(t *testing.T) {
	srv, _ := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)

	resp, err := c.Post(srv.URL+"/api/hosts", "application/json",
		strings.NewReader(`{"name":"prod","docker_host":"tcp://10.0.0.5:2375"}`))
	if err != nil {
		t.Fatalf("POST hosts: %v", err)
	}
	var created protocol.Environment
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.Name != "prod" {
		t.Errorf("created host wrong: %+v", created)
	}

	resp, _ = c.Get(srv.URL + "/api/hosts?search=prod")
	var page Page[protocol.Environment]
	json.NewDecoder(resp.Body).Decode(&page)
	resp.Body.Close()
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "prod" {
		t.Errorf("list?search=prod = %+v", page)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/hosts/"+created.ID, nil)
	resp, _ = c.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE host status = %d, want 204", resp.StatusCode)
	}
}

func TestAPIEventsAndAgentsEmpty(t *testing.T) {
	srv, _ := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)

	for _, path := range []string{"/api/events", "/api/agents"} {
		resp, err := c.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body := readBody(resp)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d", path, resp.StatusCode)
		}
		if strings.TrimSpace(body) != "[]" {
			t.Errorf("GET %s = %q, want []", path, body)
		}
	}
}

func TestAPIK8sUnreachableAgent(t *testing.T) {
	srv, st := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)

	id, _ := st.DB.CreateEnvironment(context.Background(), "k8s", "", nil)
	resp, err := c.Get(srv.URL + "/api/hosts/" + id + "/k8s/pods")
	if err != nil {
		t.Fatalf("GET pods: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("k8s pods with unreachable agent = %d, want 502", resp.StatusCode)
	}
}

func TestAPIStacksCRUD(t *testing.T) {
	srv, st := apiTestServer(t)
	defer srv.Close()
	c := authedClient(t, srv)

	hostID, _ := st.DB.CreateEnvironment(context.Background(), "h", "", nil)
	resp, err := c.Post(srv.URL+"/api/hosts/"+hostID+"/stacks", "application/json",
		strings.NewReader(`{"name":"web","content":"services: {}"}`))
	if err != nil {
		t.Fatalf("create stack: %v", err)
	}
	var stack protocol.SavedStack
	json.NewDecoder(resp.Body).Decode(&stack)
	resp.Body.Close()
	if stack.Name != "web" || stack.Status != "created" {
		t.Errorf("stack wrong: %+v", stack)
	}

	// Set a webhook token, then confirm it's returned.
	resp, _ = c.Post(srv.URL+"/api/stacks/"+stack.ID+"/token", "application/json", nil)
	var tok protocol.TokenResponse
	json.NewDecoder(resp.Body).Decode(&tok)
	resp.Body.Close()
	if tok.Token == "" {
		t.Error("expected a webhook token")
	}
}

func readBody(resp *http.Response) string {
	buf := make([]byte, 8192)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}
