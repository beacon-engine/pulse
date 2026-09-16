package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// RunWatcher consumes the default (pull) agent's Docker event stream, turning
// each event into a persisted record, a UI broadcast and a Slack alert. It
// reconnects with backoff. Push agents deliver their own events over their
// session (see runSession), so this watches only the pull agent's local daemon.
func RunWatcher(ctx context.Context, st *State) {
	for {
		conn, err := st.Agent.WatchEvents(ctx, "")
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
					st.handleEvent(ctx, resp)
				case protocol.RespKindError:
					_, msg := unexpected(resp)
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

// Ports the event-handling half of gateway/src/watcher.rs: turns an agent's
// Docker event into a persisted event (TODO: once storage lands), a UI broadcast
// and a Slack alert.
func (st *State) handleEvent(ctx context.Context, resp protocol.Response) {
	if resp.Kind != protocol.RespKindEvent {
		return
	}
	raw, ok := resp.Payload.(json.RawMessage)
	if !ok {
		return
	}
	var ev protocol.EventFrame
	if err := json.Unmarshal(raw, &ev); err != nil {
		return
	}

	var eventType, alertMsg string
	switch ev.Action {
	case "die":
		exit := "?"
		if ev.ExitCode != nil {
			exit = *ev.ExitCode
		}
		eventType = "crash"
		if exit != "0" {
			alertMsg = fmt.Sprintf(":red_circle: *Pulse* — container `%s` crashed (exit code %s)", ev.ContainerName, exit)
		}
	case "oom":
		eventType = "oom"
		alertMsg = fmt.Sprintf(":rotating_light: *Pulse* — container `%s` was OOM-killed", ev.ContainerName)
	case "start":
		eventType = "start"
	case "stop":
		eventType = "stop"
	case "restart":
		eventType = "restart"
	default:
		return
	}

	if st.DB != nil {
		// Best-effort persist (detached from the read-loop context so teardown
		// doesn't cancel the write).
		_ = st.DB.RecordEvent(context.WithoutCancel(ctx), ev.ContainerID, ev.ContainerName, eventType)
	}
	recordEvent(eventType)
	st.Events.Broadcast(fmt.Sprintf(`{"event":"%s","container":"%s"}`, eventType, ev.ContainerName))

	if st.SlackWebhook != "" && alertMsg != "" {
		go sendSlack(context.WithoutCancel(ctx), st.SlackWebhook, alertMsg)
	}
}

// alertContainer fires a Slack lifecycle alert for a container action (start /
// stop / restart), matching gateway/src/observability/alerts.rs. No-op without a
// configured webhook.
func (st *State) alertContainer(action, containerID string) {
	if st.SlackWebhook == "" {
		return
	}
	var msg string
	switch action {
	case "start":
		msg = fmt.Sprintf(":white_check_mark: *Pulse* — container `%s` started", containerID)
	case "stop":
		msg = fmt.Sprintf(":stop_sign: *Pulse* — container `%s` stopped", containerID)
	case "restart":
		msg = fmt.Sprintf(":arrows_counterclockwise: *Pulse* — container `%s` restarted", containerID)
	default:
		return
	}
	go sendSlack(context.Background(), st.SlackWebhook, msg)
}

// sendSlack posts a message to a Slack incoming webhook (best effort).
func sendSlack(ctx context.Context, url, msg string) {
	body, _ := json.Marshal(map[string]string{"text": msg})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
