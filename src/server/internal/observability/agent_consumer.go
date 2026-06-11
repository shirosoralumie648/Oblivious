package observability

import (
	"context"
	"log"

	"oblivious/server/internal/agent"
	"oblivious/server/pkg/events"
)

type EventConsumer struct {
	reporter *MemoryReporter
}

func NewEventConsumer(reporter *MemoryReporter) *EventConsumer {
	return &EventConsumer{
		reporter: reporter,
	}
}

func (c *EventConsumer) Subscribe(bus *events.Bus) {
	bus.Subscribe(agent.EventTypeRunCompleted, c.handleRunCompleted)
	bus.Subscribe(agent.EventTypeToolExecuted, c.handleToolExecuted)
}

func (c *EventConsumer) handleRunCompleted(ctx context.Context, evt events.Event) {
	runEvt, ok := evt.Payload.(agent.AgentRunEvent)
	if !ok {
		return
	}
	log.Printf("[Observability] Agent run completed: run_id=%s status=%s iterations=%d tools=%d tokens=%d",
		runEvt.RunID, runEvt.Status, runEvt.IterationCount, runEvt.ToolCallCount,
		runEvt.InputTokens+runEvt.OutputTokens)
}

func (c *EventConsumer) handleToolExecuted(ctx context.Context, evt events.Event) {
	toolEvt, ok := evt.Payload.(agent.AgentToolEvent)
	if !ok {
		return
	}
	log.Printf("[Observability] Tool executed: tool=%s status=%s duration=%dms",
		toolEvt.ToolName, toolEvt.Status, toolEvt.DurationMs)
}
