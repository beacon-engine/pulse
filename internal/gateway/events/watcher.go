package events

import (
	"context"
	"log"
	"time"

	"github.com/beacon-engine/pulse/internal/gateway/agents"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// RunWatcher consumes the default (pull) agent's Docker event stream, turning
// each event into a persisted record, a UI broadcast and a Slack alert. It
// reconnects with backoff. Push agents deliver their own events over their
// session (see agents.Server), so this watches only the pull agent's local daemon.
func RunWatcher(ctx context.Context, router *agents.Router, n *Notifier) {
	for {
		conn, err := router.WatchEvents(ctx, "")
		if err != nil {
			log.Printf("Cannot reach agent for events: %v", err)
		} else {
			log.Println("Event watcher connected to agent")
			for {
				var resp protocol.Response
				if err := protocol.ReadFrame(conn, &resp); err != nil {
					break
				}
				switch resp.Kind {
				case protocol.RespKindEvent:
					n.HandleAgentEvent(ctx, resp)
				case protocol.RespKindError:
					_, msg, _ := resp.AsError()
					log.Printf("Event stream error from agent: %s", msg)
				}
			}
			_ = conn.Close()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
