package docker

import (
	"strings"
	"sync"

	"github.com/docker/docker/client"
)

// Ports agent/src/manager.rs — caches one Docker client per docker_host string.
//
// The agent is stateless with respect to hosts: the gateway resolves a host to
// its docker_host and passes it on every request. An empty string or a unix://
// address means the local daemon; tcp://… is a remote daemon.
type Manager struct {
	mu    sync.Mutex
	conns map[string]*client.Client
}

func NewManager() *Manager {
	return &Manager{conns: make(map[string]*client.Client)}
}

// ClientFor returns a cached Docker client for the connection string, connecting
// on first use.
func (m *Manager) ClientFor(dockerHost string) (*client.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.conns[dockerHost]; ok {
		return c, nil
	}

	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if dockerHost == "" {
		// Local daemon via env / default socket.
		opts = append(opts, client.FromEnv)
	} else {
		// unix://… or tcp://… — the client accepts both directly.
		opts = append(opts, client.WithHost(strings.TrimSpace(dockerHost)))
	}

	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	m.conns[dockerHost] = c
	return c, nil
}
