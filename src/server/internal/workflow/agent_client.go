package workflow

import (
	"context"
	"fmt"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
)

type AgentClient struct {
	agentService workflowAgentService
}

func NewAgentClient(agentService *agent.Service) *AgentClient {
	if agentService == nil {
		return &AgentClient{}
	}
	return &AgentClient{agentService: agentService}
}

type workflowAgentService interface {
	StartRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error)
	StartPlanningRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error)
	ApproveToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*agent.ToolRun, error)
	GetRunWithMessages(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error)
	ContinuePlanningRun(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error)
	AdjustPlanSteps(ctx context.Context, session auth.Session, runID, reason string) (*agent.RunWithMessages, error)
	ContinueRunWithTokenBudget(ctx context.Context, session auth.Session, runID string, tokenBudget int) (*agent.RunResult, error)
	ApprovePlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error)
	ExecutePlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error)
	SkipPlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error)
	RetryPlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error)
}

func (c *AgentClient) StartAgentRun(ctx context.Context, req WorkflowAgentRunRequest) (*WorkflowAgentRunResult, error) {
	agentService, err := c.requireAgentService()
	if err != nil {
		return nil, err
	}
	session := workflowAgentRunClientSession(req)
	runReq := agent.StartRunRequest{
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		Input:          req.Input,
		MaxIterations:  req.MaxIterations,
		TokenBudget:    req.TokenBudget,
	}
	var result *agent.RunWithMessages
	mode := agent.NormalizeExecutionMode(req.Mode)
	if mode == agent.ExecutionModePlanning {
		result, err = agentService.StartPlanningRun(ctx, session, runReq)
	} else {
		result, err = agentService.StartRun(ctx, session, runReq)
	}
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
	session := workflowAgentApprovalClientSession(req)
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

func (c *AgentClient) requireAgentService() (workflowAgentService, error) {
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

func (c *AgentClient) reloadAgentRun(ctx context.Context, agentService workflowAgentService, session auth.Session, runID string) (*WorkflowAgentRunResult, error) {
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

func workflowAgentRunClientSession(req WorkflowAgentRunRequest) auth.Session {
	return auth.Session{
		OrganizationID: req.OrganizationID,
		WorkspaceID:    req.WorkspaceID,
		User:           auth.User{ID: req.UserID},
	}
}

func workflowAgentApprovalClientSession(req WorkflowAgentApprovalRequest) auth.Session {
	return auth.Session{
		OrganizationID: req.OrganizationID,
		WorkspaceID:    req.WorkspaceID,
		User:           auth.User{ID: req.UserID},
	}
}

func toWorkflowAgentRunResult(result *agent.RunWithMessages) *WorkflowAgentRunResult {
	if result == nil {
		return &WorkflowAgentRunResult{}
	}
	out := &WorkflowAgentRunResult{
		Messages:  workflowAgentMessages(result.Messages),
		ToolRuns:  workflowAgentToolRuns(result.ToolRuns),
		PlanSteps: workflowAgentPlanSteps(result.PlanSteps),
	}
	if result.Run != nil {
		out.RunID = result.Run.ID
		out.Status = result.Run.Status
		out.FinalMessageID = result.Run.FinalMessageID
	}
	out.FinalMessage = workflowAgentFinalMessage(result.Messages, out.FinalMessageID)
	return out
}

func workflowAgentFinalMessage(messages []*agent.Message, finalMessageID string) string {
	if finalMessageID != "" {
		for _, message := range messages {
			if message != nil && message.ID == finalMessageID {
				return message.Content
			}
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == "assistant" {
			return messages[i].Content
		}
	}
	return ""
}

func workflowAgentMessages(messages []*agent.Message) []WorkflowAgentMessage {
	out := make([]WorkflowAgentMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		out = append(out, WorkflowAgentMessage{
			ID:      message.ID,
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func workflowAgentToolRuns(toolRuns []*agent.ToolRun) []WorkflowAgentToolRun {
	out := make([]WorkflowAgentToolRun, 0, len(toolRuns))
	for _, toolRun := range toolRuns {
		if toolRun == nil {
			continue
		}
		out = append(out, WorkflowAgentToolRun{
			ID:             toolRun.ID,
			ToolName:       toolRun.ToolName,
			Status:         toolRun.Status,
			ApprovalStatus: toolRun.ApprovalStatus,
		})
	}
	return out
}

func workflowAgentPlanSteps(planSteps []*agent.PlanStep) []WorkflowAgentPlanStep {
	out := make([]WorkflowAgentPlanStep, 0, len(planSteps))
	for _, step := range planSteps {
		if step == nil {
			continue
		}
		out = append(out, WorkflowAgentPlanStep{
			ID:             step.ID,
			Title:          step.Title,
			Status:         step.Status,
			ApprovalStatus: step.ApprovalStatus,
			ToolName:       step.ToolName,
			ResultContent:  step.ResultContent,
			Error:          step.Error,
		})
	}
	return out
}
