package workflow

import (
	"context"
	"fmt"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
)

type AgentClient struct {
	agentService *agent.Service
}

func NewAgentClient(agentService *agent.Service) *AgentClient {
	return &AgentClient{agentService: agentService}
}

func (c *AgentClient) StartAgentRun(ctx context.Context, req WorkflowAgentRunRequest) (*WorkflowAgentRunResult, error) {
	session := auth.Session{
		OrganizationID: req.OrganizationID,
		User:           auth.User{ID: req.UserID},
	}
	runReq := agent.StartRunRequest{
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		Input:          req.Input,
		MaxIterations:  req.MaxIterations,
		TokenBudget:    req.TokenBudget,
	}
	result, err := c.agentService.StartRun(ctx, session, runReq)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func (c *AgentClient) ApproveAgentToolRun(ctx context.Context, req WorkflowAgentApprovalRequest) (*WorkflowAgentRunResult, error) {
	session := auth.Session{
		OrganizationID: req.OrganizationID,
		User:           auth.User{ID: req.UserID},
	}
	_, err := c.agentService.ApproveToolRun(ctx, session, req.ToolRunID, req.Reason)
	if err != nil {
		return nil, err
	}
	result, err := c.agentService.GetRunWithMessages(ctx, session, req.RunID)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func toWorkflowAgentRunResult(result *agent.RunWithMessages) *WorkflowAgentRunResult {
	if result == nil || result.Run == nil {
		return &WorkflowAgentRunResult{}
	}
	messages := make([]WorkflowAgentMessage, 0, len(result.Messages))
	for _, msg := range result.Messages {
		messages = append(messages, WorkflowAgentMessage{
			ID:      msg.ID,
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	toolRuns := make([]WorkflowAgentToolRun, 0, len(result.ToolRuns))
	for _, tr := range result.ToolRuns {
		toolRuns = append(toolRuns, WorkflowAgentToolRun{
			ID:             tr.ID,
			ToolName:       tr.ToolName,
			Status:         tr.Status,
			ApprovalStatus: tr.ApprovalStatus,
		})
	}
	planSteps := make([]WorkflowAgentPlanStep, 0, len(result.PlanSteps))
	for _, ps := range result.PlanSteps {
		planSteps = append(planSteps, WorkflowAgentPlanStep{
			ID:     ps.ID,
			Title:  ps.Title,
			Status: ps.Status,
		})
	}
	finalMessage := ""
	finalMessageID := ""
	if result.Run.FinalMessageID != "" {
		finalMessageID = result.Run.FinalMessageID
		for _, msg := range result.Messages {
			if msg.ID == result.Run.FinalMessageID {
				finalMessage = msg.Content
				break
			}
		}
	}
	return &WorkflowAgentRunResult{
		RunID:          result.Run.ID,
		Status:         result.Run.Status,
		FinalMessageID: finalMessageID,
		FinalMessage:   finalMessage,
		Messages:       messages,
		ToolRuns:       toolRuns,
		PlanSteps:      planSteps,
	}
}
