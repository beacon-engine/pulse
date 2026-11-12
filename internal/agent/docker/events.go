package docker

import (
	"context"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/events.rs — streams container lifecycle events.
//
// WatchEvents streams Docker container events, calling write with a
// Response::Event for each until the context is cancelled or the stream errors.
// The gateway decides which events to persist / alert on. Used by the TCP pull
// path, where a WatchEvents request takes over its connection.
func WatchEvents(ctx context.Context, cli *client.Client, write func(protocol.Response) error) error {
	res := cli.Events(ctx, client.EventsListOptions{Filters: client.Filters{}.Add("type", "container")})
	msgs, errs := res.Messages, res.Err
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
