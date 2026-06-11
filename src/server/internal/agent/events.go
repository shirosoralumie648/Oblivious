package agent

import (
	"context"
	"time"

	"oblivious/server/pkg/event"
	"oblivious/server/pkg/events"
)

const (
	EventTypeRunStarted   = "agent.run.started"
	EventTypeRunCompleted = "agent.run.completed"
	EventTypeToolExecuted = "agent.tool.executed"
)

type EventPublisher struct {
	producer *event.Producer
	bus      *events.Bus
}

func NewEventPublisher(producer *event.Producer, bus *events.Bus) *EventPublisher {
	return &EventPublisher{
		producer: producer,
		bus:      bus,
	}
}

type AgentRunEvent struct {
	EventID        string
	Timestamp      time.Time
	OrganizationID string
	UserID         string
	AgentID        string
	ConversationID string
	RunID          string
	Status         string
	IterationCount int
	ToolCallCount  int
	InputTokens    int
	OutputTokens   int
}

type AgentToolEvent struct {
	EventID        string
	Timestamp      time.Time
	OrganizationID string
	AgentID        string
	ToolRunID      string
	ToolName       string
	Status         string
	DurationMs     int64
}

func (p *EventPublisher) PublishRunCompleted(ctx context.Context, evt AgentRunEvent) error {
	if p == nil {
		return nil
	}
	if p.bus != nil {
		p.bus.Publish(ctx, events.Event{
			Type:    EventTypeRunCompleted,
			Payload: evt,
		})
	}
	return nil
}

func (p *EventPublisher) PublishToolExecuted(ctx context.Context, evt AgentToolEvent) error {
	if p == nil {
		return nil
	}
	if p.bus != nil {
		p.bus.Publish(ctx, events.Event{
			Type:    EventTypeToolExecuted,
			Payload: evt,
		})
	}
	return nil
}
