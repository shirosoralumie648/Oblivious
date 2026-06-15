package workflow

import (
	"context"
	"fmt"
	"strings"
)

type WorkflowAgentRunner interface {
	StartAgentRun(ctx context.Context, req WorkflowAgentRunRequest) (*WorkflowAgentRunResult, error)
	ApproveAgentToolRun(ctx context.Context, req WorkflowAgentApprovalRequest) (*WorkflowAgentRunResult, error)
}

type WorkflowAgentPlanningController interface {
	ContinueAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	AdjustAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	ContinueAgentRunWithTokenBudget(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	ApproveAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	ExecuteAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	SkipAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
	RetryAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
}

type WorkflowAgentRunRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	RequestID      string
	AgentID        string
	ConversationID string
	Input          string
	Mode           string
	MaxIterations  *int
	TokenBudget    *int
}

type WorkflowAgentApprovalRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	RequestID      string
	RunID          string
	ToolRunID      string
	Reason         string
}

type WorkflowAgentControlRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	RequestID      string
	RunID          string
	PlanStepID     string
	Reason         string
	TokenBudget    int
}

type WorkflowAgentRunResult struct {
	RunID          string
	Status         string
	FinalMessageID string
	FinalMessage   string
	Messages       []WorkflowAgentMessage
	ToolRuns       []WorkflowAgentToolRun
	PlanSteps      []WorkflowAgentPlanStep
}

type WorkflowAgentMessage struct {
	ID      string
	Role    string
	Content string
}

type WorkflowAgentToolRun struct {
	ID             string
	ToolName       string
	Status         string
	ApprovalStatus string
}

type WorkflowAgentPlanStep struct {
	ID             string
	Title          string
	Status         string
	ApprovalStatus string
	ToolName       string
	ResultContent  string
	Error          string
}

type AgentNodeExecutor struct {
	runner WorkflowAgentRunner
}

func NewAgentNodeExecutor(runner WorkflowAgentRunner) *AgentNodeExecutor {
	return &AgentNodeExecutor{runner: runner}
}

func (e *AgentNodeExecutor) Type() string { return "agent" }

func (e *AgentNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: agent runner is required", ErrInvalidInput)
	}
	req, err := workflowAgentRunRequestFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.StartAgentRun(ctx, req)
	if err != nil {
		return nil, err
	}
	output := workflowAgentRunOutput(result)
	if workflowAgentRunNeedsResume(result) {
		return output, ErrWorkflowUserInputRequired
	}
	return output, nil
}

func (e *AgentNodeExecutor) ApproveToolRun(ctx context.Context, input NodeExecutorInput, pending WorkflowNodeExecution, submitted map[string]any) (map[string]any, error) {
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: agent runner is required", ErrInvalidInput)
	}
	if action := workflowAgentResumeAction(submitted); action != "approve_tool" {
		return e.controlAgentRun(ctx, input, pending, submitted, action)
	}
	req, err := workflowAgentApprovalRequestFromInput(input, pending, submitted)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.ApproveAgentToolRun(ctx, req)
	if err != nil {
		return nil, err
	}
	output := workflowAgentRunOutput(result)
	if workflowAgentRunNeedsResume(result) {
		return output, fmt.Errorf("%w: agent run is still pending approval", ErrInvalidInput)
	}
	return output, nil
}

func (e *AgentNodeExecutor) controlAgentRun(ctx context.Context, input NodeExecutorInput, pending WorkflowNodeExecution, submitted map[string]any, action string) (map[string]any, error) {
	controller, ok := e.runner.(WorkflowAgentPlanningController)
	if !ok {
		return nil, fmt.Errorf("%w: agent runner does not support planning controls", ErrInvalidInput)
	}
	req, err := workflowAgentControlRequestFromInput(input, pending, submitted)
	if err != nil {
		return nil, err
	}
	var result *WorkflowAgentRunResult
	switch action {
	case "continue_plan":
		result, err = controller.ContinueAgentPlan(ctx, req)
	case "adjust_plan":
		if strings.TrimSpace(req.Reason) == "" {
			return nil, fmt.Errorf("%w: reason is required for agent plan adjustment", ErrInvalidInput)
		}
		result, err = controller.AdjustAgentPlan(ctx, req)
	case "continue_budget":
		if req.TokenBudget == 0 {
			return nil, fmt.Errorf("%w: tokenBudget is required for agent budget continuation", ErrInvalidInput)
		}
		result, err = controller.ContinueAgentRunWithTokenBudget(ctx, req)
	case "approve_plan_step":
		if req.PlanStepID == "" {
			return nil, fmt.Errorf("%w: planStepId is required for agent plan-step approval", ErrInvalidInput)
		}
		result, err = controller.ApproveAgentPlanStep(ctx, req)
	case "execute_plan_step":
		if req.PlanStepID == "" {
			return nil, fmt.Errorf("%w: planStepId is required for agent plan-step execution", ErrInvalidInput)
		}
		result, err = controller.ExecuteAgentPlanStep(ctx, req)
	case "skip_plan_step":
		if req.PlanStepID == "" {
			return nil, fmt.Errorf("%w: planStepId is required for agent plan-step skip", ErrInvalidInput)
		}
		result, err = controller.SkipAgentPlanStep(ctx, req)
	case "retry_plan_step":
		if req.PlanStepID == "" {
			return nil, fmt.Errorf("%w: planStepId is required for agent plan-step retry", ErrInvalidInput)
		}
		result, err = controller.RetryAgentPlanStep(ctx, req)
	default:
		return nil, fmt.Errorf("%w: unsupported agent resume action %s", ErrInvalidInput, action)
	}
	if err != nil {
		return nil, err
	}
	output := workflowAgentRunOutput(result)
	if workflowAgentRunNeedsResume(result) {
		return output, fmt.Errorf("%w: agent run is still pending approval", ErrInvalidInput)
	}
	return output, nil
}

func workflowAgentRunRequestFromInput(input NodeExecutorInput) (WorkflowAgentRunRequest, error) {
	nodeInput := input.Input
	if nodeInput == nil {
		nodeInput = map[string]any{}
	}
	req := WorkflowAgentRunRequest{
		OrganizationID: workflowAgentOrganizationID(input),
		UserID:         firstWorkflowString(nodeInput, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(nodeInput, "workspaceId", "workspaceID", "workspace_id"),
		RequestID:      firstWorkflowString(nodeInput, "requestId", "requestID", "request_id"),
		AgentID:        firstWorkflowString(nodeInput, "agentId", "agentID", "agent_id"),
		ConversationID: firstWorkflowString(nodeInput, "conversationId", "conversationID", "conversation_id"),
		Input:          firstWorkflowString(nodeInput, "input", "message", "prompt"),
		Mode:           strings.ToLower(strings.TrimSpace(firstWorkflowString(nodeInput, "mode", "executionMode", "execution_mode"))),
	}
	if input.Execution != nil {
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
		if req.RequestID == "" {
			req.RequestID = firstWorkflowString(input.Execution.Context, "requestId", "requestID", "request_id")
		}
	}
	if req.OrganizationID == "" {
		return WorkflowAgentRunRequest{}, fmt.Errorf("%w: organization ID is required for agent node", ErrInvalidInput)
	}
	if req.UserID == "" {
		return WorkflowAgentRunRequest{}, fmt.Errorf("%w: user ID is required for agent node", ErrInvalidInput)
	}
	if req.AgentID == "" {
		return WorkflowAgentRunRequest{}, fmt.Errorf("%w: agent node agentId is required", ErrInvalidInput)
	}
	if req.ConversationID == "" {
		return WorkflowAgentRunRequest{}, fmt.Errorf("%w: agent node conversationId is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.Input) == "" {
		return WorkflowAgentRunRequest{}, fmt.Errorf("%w: agent node input is required", ErrInvalidInput)
	}
	if value, ok := intFromWorkflowValue(nodeInput["maxIterations"]); ok {
		req.MaxIterations = &value
	} else if value, ok := intFromWorkflowValue(nodeInput["max_iterations"]); ok {
		req.MaxIterations = &value
	}
	if value, ok := intFromWorkflowValue(nodeInput["tokenBudget"]); ok {
		req.TokenBudget = &value
	} else if value, ok := intFromWorkflowValue(nodeInput["token_budget"]); ok {
		req.TokenBudget = &value
	}
	return req, nil
}

func workflowAgentOrganizationID(input NodeExecutorInput) string {
	if input.Execution != nil && strings.TrimSpace(input.Execution.OrganizationID) != "" {
		return strings.TrimSpace(input.Execution.OrganizationID)
	}
	if input.Workflow != nil {
		return strings.TrimSpace(input.Workflow.OrganizationID)
	}
	return ""
}

func workflowAgentApprovalRequestFromInput(input NodeExecutorInput, pending WorkflowNodeExecution, submitted map[string]any) (WorkflowAgentApprovalRequest, error) {
	if submitted == nil {
		submitted = map[string]any{}
	}
	req := WorkflowAgentApprovalRequest{
		OrganizationID: workflowAgentOrganizationID(input),
		UserID:         firstWorkflowString(input.Input, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(input.Input, "workspaceId", "workspaceID", "workspace_id"),
		RequestID:      firstWorkflowString(input.Input, "requestId", "requestID", "request_id"),
		RunID:          firstWorkflowString(submitted, "runId", "runID", "run_id"),
		ToolRunID:      firstWorkflowString(submitted, "toolRunId", "toolRunID", "tool_run_id"),
		Reason:         firstWorkflowString(submitted, "approvalReason", "reason", "approval_reason"),
	}
	if input.Execution != nil {
		if req.OrganizationID == "" {
			req.OrganizationID = strings.TrimSpace(input.Execution.OrganizationID)
		}
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
		if req.RequestID == "" {
			req.RequestID = firstWorkflowString(input.Execution.Context, "requestId", "requestID", "request_id")
		}
	}
	if req.RunID == "" {
		req.RunID = firstWorkflowString(pending.Output, "runId", "runID", "run_id")
	}
	if req.OrganizationID == "" {
		return WorkflowAgentApprovalRequest{}, fmt.Errorf("%w: organization ID is required for agent approval", ErrInvalidInput)
	}
	if req.UserID == "" {
		return WorkflowAgentApprovalRequest{}, fmt.Errorf("%w: user ID is required for agent approval", ErrInvalidInput)
	}
	if req.RunID == "" {
		return WorkflowAgentApprovalRequest{}, fmt.Errorf("%w: agent runId is required for approval", ErrInvalidInput)
	}
	if req.ToolRunID == "" {
		return WorkflowAgentApprovalRequest{}, fmt.Errorf("%w: agent toolRunId is required for approval", ErrInvalidInput)
	}
	return req, nil
}

func workflowAgentControlRequestFromInput(input NodeExecutorInput, pending WorkflowNodeExecution, submitted map[string]any) (WorkflowAgentControlRequest, error) {
	if submitted == nil {
		submitted = map[string]any{}
	}
	req := WorkflowAgentControlRequest{
		OrganizationID: workflowAgentOrganizationID(input),
		UserID:         firstWorkflowString(input.Input, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(input.Input, "workspaceId", "workspaceID", "workspace_id"),
		RequestID:      firstWorkflowString(input.Input, "requestId", "requestID", "request_id"),
		RunID:          firstWorkflowString(submitted, "runId", "runID", "run_id"),
		PlanStepID:     firstWorkflowString(submitted, "planStepId", "planStepID", "plan_step_id"),
		Reason:         firstWorkflowString(submitted, "approvalReason", "reason", "approval_reason"),
	}
	if value, ok := intFromWorkflowValue(submitted["tokenBudget"]); ok {
		req.TokenBudget = value
	} else if value, ok := intFromWorkflowValue(submitted["token_budget"]); ok {
		req.TokenBudget = value
	}
	if input.Execution != nil {
		if req.OrganizationID == "" {
			req.OrganizationID = strings.TrimSpace(input.Execution.OrganizationID)
		}
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
		if req.RequestID == "" {
			req.RequestID = firstWorkflowString(input.Execution.Context, "requestId", "requestID", "request_id")
		}
	}
	if req.RunID == "" {
		req.RunID = firstWorkflowString(pending.Output, "runId", "runID", "run_id")
	}
	if req.OrganizationID == "" {
		return WorkflowAgentControlRequest{}, fmt.Errorf("%w: organization ID is required for agent control", ErrInvalidInput)
	}
	if req.UserID == "" {
		return WorkflowAgentControlRequest{}, fmt.Errorf("%w: user ID is required for agent control", ErrInvalidInput)
	}
	if req.RunID == "" {
		return WorkflowAgentControlRequest{}, fmt.Errorf("%w: agent runId is required for control", ErrInvalidInput)
	}
	return req, nil
}

func workflowAgentResumeAction(submitted map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(firstWorkflowString(submitted, "action", "agentAction", "agent_action", "planStepAction", "plan_step_action")))
	action = strings.ReplaceAll(action, "-", "_")
	switch action {
	case "", "approve_tool", "approve_tool_run", "tool", "tool_approval":
		if firstWorkflowString(submitted, "planStepId", "planStepID", "plan_step_id") != "" {
			return "approve_plan_step"
		}
		if _, ok := intFromWorkflowValue(submitted["tokenBudget"]); ok {
			return "continue_budget"
		}
		if _, ok := intFromWorkflowValue(submitted["token_budget"]); ok {
			return "continue_budget"
		}
		if truthyWorkflowValue(submitted["continuePlan"]) || truthyWorkflowValue(submitted["continue_plan"]) {
			return "continue_plan"
		}
		return "approve_tool"
	case "continue", "continue_plan", "continue_planning", "continue_planning_run":
		return "continue_plan"
	case "adjust", "adjust_plan", "adjust_planning", "adjust_plan_steps":
		return "adjust_plan"
	case "continue_budget", "continue_token_budget", "token_budget", "budget":
		return "continue_budget"
	case "approve", "approve_plan_step", "plan_step_approve":
		return "approve_plan_step"
	case "execute", "execute_plan_step", "plan_step_execute":
		return "execute_plan_step"
	case "skip", "skip_plan_step", "plan_step_skip":
		return "skip_plan_step"
	case "retry", "retry_plan_step", "plan_step_retry":
		return "retry_plan_step"
	default:
		return action
	}
}

func truthyWorkflowValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y":
			return true
		}
	}
	return false
}

func workflowAgentRunOutput(result *WorkflowAgentRunResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	output := map[string]any{
		"runId":          strings.TrimSpace(result.RunID),
		"status":         strings.TrimSpace(result.Status),
		"finalMessageId": strings.TrimSpace(result.FinalMessageID),
		"text":           result.FinalMessage,
		"content":        result.FinalMessage,
		"messages":       workflowAgentMessagesOutput(result.Messages),
		"toolRuns":       workflowAgentToolRunsOutput(result.ToolRuns),
		"planSteps":      workflowAgentPlanStepsOutput(result.PlanSteps),
	}
	return output
}

func resultStatus(result *WorkflowAgentRunResult) string {
	if result == nil {
		return ""
	}
	return result.Status
}

func workflowAgentRunNeedsResume(result *WorkflowAgentRunResult) bool {
	switch strings.ToLower(strings.TrimSpace(resultStatus(result))) {
	case "pending_approval", "token_budget_exceeded":
		return true
	default:
		return false
	}
}

func workflowAgentMessagesOutput(messages []WorkflowAgentMessage) []map[string]any {
	output := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		output = append(output, map[string]any{
			"id":      strings.TrimSpace(message.ID),
			"role":    strings.TrimSpace(message.Role),
			"content": message.Content,
		})
	}
	return output
}

func workflowAgentToolRunsOutput(toolRuns []WorkflowAgentToolRun) []map[string]any {
	output := make([]map[string]any, 0, len(toolRuns))
	for _, toolRun := range toolRuns {
		output = append(output, map[string]any{
			"id":             strings.TrimSpace(toolRun.ID),
			"toolName":       strings.TrimSpace(toolRun.ToolName),
			"status":         strings.TrimSpace(toolRun.Status),
			"approvalStatus": strings.TrimSpace(toolRun.ApprovalStatus),
		})
	}
	return output
}

func workflowAgentPlanStepsOutput(planSteps []WorkflowAgentPlanStep) []map[string]any {
	output := make([]map[string]any, 0, len(planSteps))
	for _, step := range planSteps {
		output = append(output, map[string]any{
			"id":             strings.TrimSpace(step.ID),
			"title":          strings.TrimSpace(step.Title),
			"status":         strings.TrimSpace(step.Status),
			"approvalStatus": strings.TrimSpace(step.ApprovalStatus),
			"toolName":       strings.TrimSpace(step.ToolName),
			"resultContent":  step.ResultContent,
			"error":          step.Error,
		})
	}
	return output
}
