package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/beacon-engine/pulse/internal/gateway/metrics"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// Notifier fans out what happens to containers and agents: it records events,
// counts them for /metrics, broadcasts them to the dashboard and, with a
// webhook configured, alerts Slack.
type Notifier struct {
	DB           storage.Repository
	Hub          *Hub
	SlackWebhook string
}

// Slack sends msg to the configured webhook in the background; no-op without one.
func (n *Notifier) Slack(ctx context.Context, msg string) {
	if n.SlackWebhook == "" {
		return
	}
	go sendSlack(context.WithoutCancel(ctx), n.SlackWebhook, msg)
}

// Ports the event-handling half of gateway/src/watcher.rs: turns an agent's
// Docker event into a persisted event (TODO: once storage lands), a UI broadcast
// and a Slack alert.
func (n *Notifier) HandleAgentEvent(ctx context.Context, resp protocol.Response) {
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

	if n.DB != nil {
		// Best-effort persist (detached from the read-loop context so teardown
		// doesn't cancel the write).
		_ = n.DB.RecordEvent(context.WithoutCancel(ctx), ev.ContainerID, ev.ContainerName, eventType)
	}
	metrics.RecordEvent(eventType)
	n.Hub.Broadcast(fmt.Sprintf(`{"event":"%s","container":"%s"}`, eventType, ev.ContainerName))

	if n.SlackWebhook != "" && alertMsg != "" {
		go sendSlack(context.WithoutCancel(ctx), n.SlackWebhook, alertMsg)
	}
}

// AlertContainer fires a Slack lifecycle alert for a container action (start /
// stop / restart), matching gateway/src/observability/alerts.rs. No-op without a
// configured webhook.
func (n *Notifier) AlertContainer(action, containerID string) {
	if n.SlackWebhook == "" {
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
	go sendSlack(context.Background(), n.SlackWebhook, msg)
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
