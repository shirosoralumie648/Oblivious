package observability

import (
	"context"
	"log"
	"oblivious/server/pkg/event"
	eventpb "oblivious/server/pkg/event/proto"

	"google.golang.org/protobuf/proto"
)

type EventConsumer struct {
	consumer *event.Consumer
	handler  WorkflowEventHandler
}

type WorkflowEventHandler interface {
	HandleWorkflowEvent(ctx context.Context, evt *eventpb.WorkflowEvent) error
}

func NewEventConsumer(brokers []string, topic, groupID string, handler WorkflowEventHandler) *EventConsumer {
	return &EventConsumer{
		consumer: event.NewConsumer(brokers, topic, groupID),
		handler:  handler,
	}
}

func (c *EventConsumer) Start(ctx context.Context) error {
	return c.consumer.Subscribe(ctx, func(key string, msg []byte) error {
		var evt eventpb.WorkflowEvent
		if err := proto.Unmarshal(msg, &evt); err != nil {
			log.Printf("failed to unmarshal workflow event: %v", err)
			return err
		}
		return c.handler.HandleWorkflowEvent(ctx, &evt)
	})
}

func (c *EventConsumer) Close() error {
	return c.consumer.Close()
}
