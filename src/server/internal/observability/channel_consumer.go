package observability

import (
	"context"
	"time"

	"oblivious/server/pkg/events"
)

type ChannelConsumer struct {
	reporter Reporter
}

func NewChannelConsumer(reporter Reporter) *ChannelConsumer {
	return &ChannelConsumer{reporter: reporter}
}

func (c *ChannelConsumer) HandleMessageReceived(ctx context.Context, event events.Event) {
	payload, ok := event.Payload.(events.ChannelMessagePayload)
	if !ok {
		return
	}

	c.reporter.ReportError(ctx, Event{
		Component:      "channel",
		Event:          "message.received",
		ChannelID:      payload.ChannelID,
		Fields: map[string]any{
			"message_id":      payload.MessageID,
			"conversation_id": payload.ConversationID,
			"direction":       payload.Direction,
			"status":          payload.Status,
		},
	})
}

func (c *ChannelConsumer) HandleMessageSent(ctx context.Context, event events.Event) {
	payload, ok := event.Payload.(events.ChannelMessagePayload)
	if !ok {
		return
	}

	c.reporter.ReportError(ctx, Event{
		Component:      "channel",
		Event:          "message.sent",
		ChannelID:      payload.ChannelID,
		Fields: map[string]any{
			"message_id":      payload.MessageID,
			"conversation_id": payload.ConversationID,
			"direction":       payload.Direction,
			"status":          payload.Status,
		},
	})
}

func (c *ChannelConsumer) HandleMessageFailed(ctx context.Context, event events.Event) {
	payload, ok := event.Payload.(events.ChannelMessagePayload)
	if !ok {
		return
	}

	c.reporter.ReportError(ctx, Event{
		Component:     "channel",
		Event:         "message.failed",
		ChannelID:     payload.ChannelID,
		FailureReason: payload.Error,
		Fields: map[string]any{
			"message_id":      payload.MessageID,
			"conversation_id": payload.ConversationID,
			"direction":       payload.Direction,
			"status":          payload.Status,
		},
	})
}

func (c *ChannelConsumer) HandleChannelCreated(ctx context.Context, event events.Event) {
	payload, ok := event.Payload.(events.ChannelConfigPayload)
	if !ok {
		return
	}

	c.reporter.ReportError(ctx, Event{
		Component:      "channel",
		Event:          "channel.created",
		ChannelID:      payload.ChannelID,
		OrganizationID: payload.OrganizationID,
		Fields: map[string]any{
			"type":   payload.Type,
			"status": payload.Status,
			"ts":     time.Now().UTC(),
		},
	})
}

func (c *ChannelConsumer) Subscribe(bus *events.Bus) {
	bus.Subscribe(events.ChannelMessageReceived, c.HandleMessageReceived)
	bus.Subscribe(events.ChannelMessageSent, c.HandleMessageSent)
	bus.Subscribe(events.ChannelMessageFailed, c.HandleMessageFailed)
	bus.Subscribe(events.ChannelCreated, c.HandleChannelCreated)
}
