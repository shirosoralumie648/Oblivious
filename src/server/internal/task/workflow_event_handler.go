package task

import (
	"context"
	"log"
	eventpb "oblivious/server/pkg/event/proto"
	"time"
)

type WorkflowEventHandlerImpl struct{}

func NewWorkflowEventHandler() *WorkflowEventHandlerImpl {
	return &WorkflowEventHandlerImpl{}
}

func (h *WorkflowEventHandlerImpl) HandleWorkflowEvent(ctx context.Context, evt *eventpb.WorkflowEvent) error {
	log.Printf("[Task] Received workflow event: type=%s workflow_id=%s execution_id=%s timestamp=%s",
		evt.EventType, evt.WorkflowId, evt.ExecutionId, time.Unix(evt.Timestamp, 0))
	return nil
}
