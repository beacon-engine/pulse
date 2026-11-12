// Package events moves events through the gateway: the live feed to the
// dashboard, the Docker event watcher, and Slack alerts.
package events

import "sync"

// Ports gateway/src/observability/websocket.rs — the live-event fan-out to
// connected UI WebSocket clients. Rust used a tokio broadcast channel; this is
// the Go equivalent: subscribers get a buffered channel, and a slow subscriber
// drops messages rather than blocking the broadcaster.
type Hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[chan string]struct{})}
}

// Subscribe returns a receive channel for broadcasts and a cancel func to
// unsubscribe (which closes the channel).
func (h *Hub) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, ch)
			h.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

// Broadcast delivers msg to every subscriber, dropping it for any whose buffer
// is full (a lagging client must not stall the others).
func (h *Hub) Broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}
