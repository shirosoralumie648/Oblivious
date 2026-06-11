package billing

import (
	"context"
	"log"

	"oblivious/server/internal/agent"
	"oblivious/server/pkg/events"
)

type AgentEventConsumer struct {
	service *Service
}

func NewAgentEventConsumer(service *Service) *AgentEventConsumer {
	return &AgentEventConsumer{
		service: service,
	}
}

func (c *AgentEventConsumer) Subscribe(bus *events.Bus) {
	bus.Subscribe(agent.EventTypeRunCompleted, c.handleRunCompleted)
}

func (c *AgentEventConsumer) handleRunCompleted(ctx context.Context, evt events.Event) {
	runEvt, ok := evt.Payload.(agent.AgentRunEvent)
	if !ok {
		return
	}
	_, err := c.service.HandleUsageEvent(
		ctx,
		runEvt.UserID,
		runEvt.OrganizationID,
		runEvt.ConversationID,
		"",
		"agent_run",
		runEvt.InputTokens,
		runEvt.OutputTokens,
		0,
		0,
		runEvt.EventID,
	)
	if err != nil {
		log.Printf("[Billing] Failed to record agent usage: %v", err)
	}
}
