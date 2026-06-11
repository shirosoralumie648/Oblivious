package workflow

import (
	"context"
	"oblivious/server/pkg/event"
	eventpb "oblivious/server/pkg/event/proto"
	"time"
)

type EventPublisher struct {
	producer *event.Producer
}

func NewEventPublisher(brokers []string, topic string) *EventPublisher {
	return &EventPublisher{
		producer: event.NewProducer(brokers, topic),
	}
}

func (p *EventPublisher) PublishExecutionStarted(ctx context.Context, workflowID, executionID string) error {
	evt := &eventpb.WorkflowEvent{
		WorkflowId:  workflowID,
		ExecutionId: executionID,
		EventType:   "execution.started",
		Timestamp:   time.Now().Unix(),
	}
	return p.producer.Publish(ctx, workflowID, evt)
}

func (p *EventPublisher) PublishExecutionCompleted(ctx context.Context, workflowID, executionID string) error {
	evt := &eventpb.WorkflowEvent{
		WorkflowId:  workflowID,
		ExecutionId: executionID,
		EventType:   "execution.completed",
		Timestamp:   time.Now().Unix(),
	}
	return p.producer.Publish(ctx, workflowID, evt)
}

func (p *EventPublisher) PublishExecutionFailed(ctx context.Context, workflowID, executionID string) error {
	evt := &eventpb.WorkflowEvent{
		WorkflowId:  workflowID,
		ExecutionId: executionID,
		EventType:   "execution.failed",
		Timestamp:   time.Now().Unix(),
	}
	return p.producer.Publish(ctx, workflowID, evt)
}

func (p *EventPublisher) Close() error {
	return p.producer.Close()
}
