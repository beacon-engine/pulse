package agent

import (
	"context"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/events.rs — streams container lifecycle events.
//
// watchEvents streams Docker container events, calling write with a
// Response::Event for each until the context is cancelled or the stream errors.
// The gateway decides which events to persist / alert on. Used by the TCP pull
// path, where a WatchEvents request takes over its connection.
func watchEvents(ctx context.Context, cli *client.Client, write func(protocol.Response) error) error {
	f := filters.NewArgs()
	f.Add("type", "container")

	msgs, errs := cli.Events(ctx, events.ListOptions{Filters: f})
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errs:
			return err
		case msg := <-msgs:
			if frame, ok := eventToFrame(msg); ok {
				if err := write(protocol.RespEvent(frame)); err != nil {
					return err
				}
			}
		}
	}
}

func eventToFrame(m events.Message) (protocol.EventFrame, bool) {
	action := string(m.Action)
	if action == "" {
		return protocol.EventFrame{}, false
	}
	containerID := m.Actor.ID
	name := m.Actor.Attributes["name"]
	if name == "" {
		name = containerID
	}
	var exit *string
	if ec, ok := m.Actor.Attributes["exitCode"]; ok {
		exit = &ec
	}
	return protocol.EventFrame{
		ContainerID:   containerID,
		ContainerName: name,
		Action:        action,
		ExitCode:      exit,
	}, true
}
