package api

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

// Ports the browser-facing half of gateway/src/observability/websocket.rs: a UI
// client connects to /ws and receives live event broadcasts from the events.Hub.
func (st *State) handleUIWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Detect client close; cancel the write loop when the socket goes away.
	go func() {
		for {
			if _, _, err := c.Read(ctx); err != nil {
				cancel()
				return
			}
		}
	}()

	ch, unsubscribe := st.Events.Subscribe()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := c.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
				return
			}
		}
	}
}
