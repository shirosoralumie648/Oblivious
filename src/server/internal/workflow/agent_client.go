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
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
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
	result, err := agentService.StartRun(ctx, session, runReq)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func (c *AgentClient) ApproveAgentToolRun(ctx context.Context, req WorkflowAgentApprovalRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := auth.Session{
		OrganizationID: req.OrganizationID,
		User:           auth.User{ID: req.UserID},
	}
	_, err = agentService.ApproveToolRun(ctx, session, req.ToolRunID, req.Reason)
	if err != nil {
		return nil, err
	}
	result, err := agentService.GetRunWithMessages(ctx, session, req.RunID)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func (c *AgentClient) ContinueAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	result, err := agentService.ContinuePlanningRun(ctx, session, req.RunID)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func (c *AgentClient) AdjustAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	result, err := agentService.AdjustPlanSteps(ctx, session, req.RunID, req.Reason)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func (c *AgentClient) ContinueAgentRunWithTokenBudget(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	if _, err := agentService.ContinueRunWithTokenBudget(ctx, session, req.RunID, req.TokenBudget); err != nil {
		return nil, err
	}
	return c.reloadAgentRun(ctx, agentService, session, req.RunID)
}

func (c *AgentClient) ApproveAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	if err := c.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return agentService.ApprovePlanStep(ctx, session, req.PlanStepID, req.Reason)
	}); err != nil {
		return nil, err
	}
	return c.reloadAgentRun(ctx, agentService, session, req.RunID)
}

func (c *AgentClient) ExecuteAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	if err := c.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return agentService.ExecutePlanStep(ctx, session, req.PlanStepID)
	}); err != nil {
		return nil, err
	}
	return c.reloadAgentRun(ctx, agentService, session, req.RunID)
}

func (c *AgentClient) SkipAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	if err := c.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return agentService.SkipPlanStep(ctx, session, req.PlanStepID, req.Reason)
	}); err != nil {
		return nil, err
	}
	return c.reloadAgentRun(ctx, agentService, session, req.RunID)
}

func (c *AgentClient) RetryAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentClientSession(req)
	if err := c.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return agentService.RetryPlanStep(ctx, session, req.PlanStepID)
	}); err != nil {
		return nil, err
	}
	return c.reloadAgentRun(ctx, agentService, session, req.RunID)
}

func (c *AgentClient) requireAgentService() (*agent.Service, error) {
	if c == nil || c.agentService == nil {
		return nil, fmt.Errorf("%w: agent service is not configured", ErrInvalidInput)
	}
	return c.agentService, nil
}

func (c *AgentClient) applyAgentPlanStepAction(req WorkflowAgentControlRequest, action func() (*agent.PlanStep, error)) error {
	step, err := action()
	if err != nil {
		return err
	}
	if step != nil && step.RunID != "" && req.RunID != "" && step.RunID != req.RunID {
		return fmt.Errorf("%w: agent plan step does not belong to run", ErrInvalidInput)
	}
	return nil
}

func (c *AgentClient) reloadAgentRun(ctx context.Context, agentService *agent.Service, session auth.Session, runID string) (*WorkflowAgentRunResult, error) {
	result, err := agentService.GetRunWithMessages(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	return toWorkflowAgentRunResult(result), nil
}

func workflowAgentClientSession(req WorkflowAgentControlRequest) auth.Session {
	return auth.Session{
		OrganizationID: req.OrganizationID,
		WorkspaceID:    req.WorkspaceID,
		User:           auth.User{ID: req.UserID},
	}
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
			ID:             ps.ID,
			Title:          ps.Title,
			Status:         ps.Status,
			ApprovalStatus: ps.ApprovalStatus,
			ToolName:       ps.ToolName,
			ResultContent:  ps.ResultContent,
			Error:          ps.Error,
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
