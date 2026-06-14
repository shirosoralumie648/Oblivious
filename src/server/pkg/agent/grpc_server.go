package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	internalagent "oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentRuntimeService interface {
	StartRun(context.Context, auth.Session, internalagent.StartRunRequest) (*internalagent.RunWithMessages, error)
	ListRuns(context.Context, auth.Session, string) ([]*internalagent.Run, error)
	GetRunWithMessages(context.Context, auth.Session, string) (*internalagent.RunWithMessages, error)
	ContinueRunWithTokenBudget(context.Context, auth.Session, string, int) (*internalagent.RunResult, error)
	ApproveToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error)
	RejectToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error)
	ContinuePlanningRun(context.Context, auth.Session, string) (*internalagent.RunWithMessages, error)
	AdjustPlanSteps(context.Context, auth.Session, string, string) (*internalagent.RunWithMessages, error)
	ApprovePlanStep(context.Context, auth.Session, string, string) (*internalagent.PlanStep, error)
	ExecutePlanStep(context.Context, auth.Session, string) (*internalagent.PlanStep, error)
	SkipPlanStep(context.Context, auth.Session, string, string) (*internalagent.PlanStep, error)
	RetryPlanStep(context.Context, auth.Session, string) (*internalagent.PlanStep, error)
}

type RuntimeGateway interface {
	CreateRun(context.Context, CreateRunInput) (RunState, error)
	ExecuteReAct(context.Context, ExecuteReActInput) (RunExecutionState, error)
	ContinueBudget(context.Context, ContinueBudgetInput) (ContinueBudgetState, error)
	ApproveToolCall(context.Context, ToolApprovalInput) (ToolApprovalState, error)
	ContinuePlan(context.Context, PlanRunInput) (PlanRunState, error)
	AdjustPlan(context.Context, AdjustPlanInput) (PlanRunState, error)
	ApprovePlanStep(context.Context, PlanStepActionInput) (PlanStepActionState, error)
	ExecutePlanStep(context.Context, PlanStepActionInput) (PlanStepActionState, error)
	SkipPlanStep(context.Context, PlanStepActionInput) (PlanStepActionState, error)
	RetryPlanStep(context.Context, PlanStepActionInput) (PlanStepActionState, error)
}

type CreateRunInput struct {
	AgentID        string
	ConversationID string
	UserContent    string
	OrganizationID string
	UserID         string
	RecursionDepth int32
	MaxDepth       int32
}

type ExecuteReActInput struct {
	RunID          string
	OrganizationID string
	UserID         string
}

type ContinueBudgetInput struct {
	RunID          string
	OrganizationID string
	UserID         string
	TokenBudget    int32
}

type ToolApprovalInput struct {
	RunID          string
	ToolCallID     string
	Approved       bool
	OrganizationID string
	UserID         string
	Reason         string
}

type PlanRunInput struct {
	RunID          string
	OrganizationID string
	UserID         string
}

type AdjustPlanInput struct {
	RunID          string
	OrganizationID string
	UserID         string
	Reason         string
}

type PlanStepActionInput struct {
	RunID          string
	PlanStepID     string
	OrganizationID string
	UserID         string
	Reason         string
}

type RunState struct {
	RunID  string
	Status string
}

type RunExecutionState struct {
	RunID            string
	Status           string
	Result           string
	PendingToolCalls []PendingToolCall
}

type ContinueBudgetState struct {
	RunID            string
	Status           string
	Result           string
	PendingToolCalls []PendingToolCall
	PlanSteps        []PlanStepState
}

type PendingToolCall struct {
	ID     string
	Name   string
	Input  string
	Status string
}

type ToolApprovalState struct {
	RunID      string
	ToolCallID string
	Status     string
}

type PlanRunState struct {
	RunID     string
	Status    string
	Result    string
	PlanSteps []PlanStepState
}

type PlanStepState struct {
	ID             string
	RunID          string
	Index          int32
	Title          string
	Description    string
	Status         string
	ApprovalStatus string
	ToolName       string
	Input          string
	DependsOn      []int32
	Result         string
	Error          string
}

type PlanStepActionState struct {
	RunID          string
	PlanStepID     string
	Index          int32
	Status         string
	ApprovalStatus string
	Result         string
	Error          string
	RunDetail      PlanRunState
}

type Server struct {
	agentv1.UnimplementedAgentServiceServer
	runtime RuntimeGateway
}

func NewServer() *Server {
	return &Server{}
}

func NewServerWithRuntime(runtime RuntimeGateway) *Server {
	return &Server{runtime: runtime}
}

func NewServerWithAgentService(service AgentRuntimeService) *Server {
	return NewServerWithRuntime(NewServiceRuntimeGateway(service))
}

func NewServiceRuntimeGateway(service AgentRuntimeService) RuntimeGateway {
	return serviceRuntimeGateway{service: service}
}

func (s *Server) CreateRun(ctx context.Context, req *agentv1.CreateRunRequest) (*agentv1.CreateRunResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	run, err := s.runtime.CreateRun(ctx, CreateRunInput{
		AgentID:        req.AgentId,
		ConversationID: req.ConversationId,
		UserContent:    req.UserContent,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		RecursionDepth: req.RecursionDepth,
		MaxDepth:       req.MaxDepth,
	})
	if err != nil {
		return nil, err
	}
	return &agentv1.CreateRunResponse{
		RunId:  run.RunID,
		Status: run.Status,
	}, nil
}

func (s *Server) ExecuteReAct(ctx context.Context, req *agentv1.ExecuteReActRequest) (*agentv1.ExecuteReActResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	run, err := s.runtime.ExecuteReAct(ctx, ExecuteReActInput{
		RunID:          req.RunId,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
	})
	if err != nil {
		return nil, err
	}
	pendingToolCalls := make([]*agentv1.ToolCall, 0, len(run.PendingToolCalls))
	for _, toolCall := range run.PendingToolCalls {
		pendingToolCalls = append(pendingToolCalls, &agentv1.ToolCall{
			Id:     toolCall.ID,
			Name:   toolCall.Name,
			Input:  toolCall.Input,
			Status: toolCall.Status,
		})
	}
	return &agentv1.ExecuteReActResponse{
		RunId:            run.RunID,
		Status:           run.Status,
		Result:           run.Result,
		PendingToolCalls: pendingToolCalls,
	}, nil
}

func (s *Server) ContinueBudget(ctx context.Context, req *agentv1.ContinueBudgetRequest) (*agentv1.ContinueBudgetResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.TokenBudget < 1000 || req.TokenBudget > 1000000 {
		return nil, status.Error(codes.InvalidArgument, "token_budget must be between 1000 and 1000000")
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	run, err := s.runtime.ContinueBudget(ctx, ContinueBudgetInput{
		RunID:          req.RunId,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		TokenBudget:    req.TokenBudget,
	})
	if err != nil {
		return nil, err
	}
	pendingToolCalls := make([]*agentv1.ToolCall, 0, len(run.PendingToolCalls))
	for _, toolCall := range run.PendingToolCalls {
		pendingToolCalls = append(pendingToolCalls, &agentv1.ToolCall{
			Id:     toolCall.ID,
			Name:   toolCall.Name,
			Input:  toolCall.Input,
			Status: toolCall.Status,
		})
	}
	return &agentv1.ContinueBudgetResponse{
		RunId:            run.RunID,
		Status:           run.Status,
		Result:           run.Result,
		PendingToolCalls: pendingToolCalls,
		PlanSteps:        planRunResponse(PlanRunState{PlanSteps: run.PlanSteps}).PlanSteps,
	}, nil
}

func (s *Server) ApproveToolCall(ctx context.Context, req *agentv1.ApproveToolCallRequest) (*agentv1.ApproveToolCallResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.ToolCallId == "" {
		return nil, status.Error(codes.InvalidArgument, "tool_call_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	approval, err := s.runtime.ApproveToolCall(ctx, ToolApprovalInput{
		RunID:          req.RunId,
		ToolCallID:     req.ToolCallId,
		Approved:       req.Approved,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		Reason:         req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &agentv1.ApproveToolCallResponse{
		RunId:      approval.RunID,
		ToolCallId: approval.ToolCallID,
		Status:     approval.Status,
	}, nil
}

func (s *Server) ContinuePlan(ctx context.Context, req *agentv1.PlanRunRequest) (*agentv1.PlanRunResponse, error) {
	input, err := planRunInputFromRequest(req)
	if err != nil {
		return nil, err
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}
	run, err := s.runtime.ContinuePlan(ctx, input)
	if err != nil {
		return nil, err
	}
	return planRunResponse(run), nil
}

func (s *Server) AdjustPlan(ctx context.Context, req *agentv1.AdjustPlanRequest) (*agentv1.PlanRunResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}
	run, err := s.runtime.AdjustPlan(ctx, AdjustPlanInput{
		RunID:          req.RunId,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		Reason:         req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return planRunResponse(run), nil
}

func (s *Server) ApprovePlanStep(ctx context.Context, req *agentv1.PlanStepActionRequest) (*agentv1.PlanStepActionResponse, error) {
	return s.planStepAction(ctx, req, "approve")
}

func (s *Server) ExecutePlanStep(ctx context.Context, req *agentv1.PlanStepActionRequest) (*agentv1.PlanStepActionResponse, error) {
	return s.planStepAction(ctx, req, "execute")
}

func (s *Server) SkipPlanStep(ctx context.Context, req *agentv1.PlanStepActionRequest) (*agentv1.PlanStepActionResponse, error) {
	return s.planStepAction(ctx, req, "skip")
}

func (s *Server) RetryPlanStep(ctx context.Context, req *agentv1.PlanStepActionRequest) (*agentv1.PlanStepActionResponse, error) {
	return s.planStepAction(ctx, req, "retry")
}

func (s *Server) planStepAction(ctx context.Context, req *agentv1.PlanStepActionRequest, action string) (*agentv1.PlanStepActionResponse, error) {
	input, err := planStepActionInputFromRequest(req)
	if err != nil {
		return nil, err
	}
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	var state PlanStepActionState
	switch action {
	case "approve":
		state, err = s.runtime.ApprovePlanStep(ctx, input)
	case "execute":
		state, err = s.runtime.ExecutePlanStep(ctx, input)
	case "skip":
		state, err = s.runtime.SkipPlanStep(ctx, input)
	case "retry":
		state, err = s.runtime.RetryPlanStep(ctx, input)
	default:
		err = status.Error(codes.Internal, "unknown plan step action")
	}
	if err != nil {
		return nil, err
	}
	return &agentv1.PlanStepActionResponse{
		RunId:          state.RunID,
		PlanStepId:     state.PlanStepID,
		Index:          state.Index,
		Status:         state.Status,
		ApprovalStatus: state.ApprovalStatus,
		Result:         state.Result,
		Error:          state.Error,
		RunDetail:      planRunResponse(state.RunDetail),
	}, nil
}

type serviceRuntimeGateway struct {
	service AgentRuntimeService
}

func (g serviceRuntimeGateway) CreateRun(ctx context.Context, input CreateRunInput) (RunState, error) {
	if g.service == nil {
		return RunState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	req := internalagent.StartRunRequest{
		AgentID:        input.AgentID,
		ConversationID: input.ConversationID,
		Input:          input.UserContent,
	}
	if input.MaxDepth > 0 {
		maxIterations := int(input.MaxDepth)
		req.MaxIterations = &maxIterations
	}
	session := input.session()
	result, err := g.service.StartRun(ctx, session, req)
	if errors.Is(err, internalagent.ErrToolApprovalRequired) {
		result, err = g.latestRunWithMessages(ctx, session, input.ConversationID)
	}
	if err != nil {
		return RunState{}, mapAgentRuntimeError(err)
	}
	if result == nil || result.Run == nil {
		return RunState{}, status.Error(codes.Internal, "agent runtime did not return a run")
	}
	return RunState{RunID: result.Run.ID, Status: result.Run.Status}, nil
}

func (g serviceRuntimeGateway) latestRunWithMessages(ctx context.Context, session auth.Session, conversationID string) (*internalagent.RunWithMessages, error) {
	runs, err := g.service.ListRuns(ctx, session, conversationID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 || runs[len(runs)-1] == nil {
		return nil, fmt.Errorf("agent runtime paused for approval without a persisted run")
	}
	return g.service.GetRunWithMessages(ctx, session, runs[len(runs)-1].ID)
}

func (g serviceRuntimeGateway) ExecuteReAct(ctx context.Context, input ExecuteReActInput) (RunExecutionState, error) {
	if g.service == nil {
		return RunExecutionState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	result, err := g.service.GetRunWithMessages(ctx, input.session(), input.RunID)
	if err != nil {
		return RunExecutionState{}, mapAgentRuntimeError(err)
	}
	if result == nil || result.Run == nil {
		return RunExecutionState{}, status.Error(codes.Internal, "agent runtime did not return a run")
	}
	return RunExecutionState{
		RunID:            result.Run.ID,
		Status:           result.Run.Status,
		Result:           finalAssistantContent(result),
		PendingToolCalls: pendingToolCalls(result.ToolRuns),
	}, nil
}

func (g serviceRuntimeGateway) ContinueBudget(ctx context.Context, input ContinueBudgetInput) (ContinueBudgetState, error) {
	if g.service == nil {
		return ContinueBudgetState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	session := input.session()
	if _, err := g.service.ContinueRunWithTokenBudget(ctx, session, input.RunID, int(input.TokenBudget)); err != nil {
		return ContinueBudgetState{}, mapAgentRuntimeError(err)
	}
	result, err := g.service.GetRunWithMessages(ctx, session, input.RunID)
	if err != nil {
		return ContinueBudgetState{}, mapAgentRuntimeError(err)
	}
	if result == nil || result.Run == nil {
		return ContinueBudgetState{}, status.Error(codes.Internal, "agent runtime did not return a run")
	}
	return ContinueBudgetState{
		RunID:            result.Run.ID,
		Status:           result.Run.Status,
		Result:           finalAssistantContent(result),
		PendingToolCalls: pendingToolCalls(result.ToolRuns),
		PlanSteps:        planStepStates(result.PlanSteps),
	}, nil
}

func (g serviceRuntimeGateway) ApproveToolCall(ctx context.Context, input ToolApprovalInput) (ToolApprovalState, error) {
	if g.service == nil {
		return ToolApprovalState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	session := input.session()
	var (
		toolRun *internalagent.ToolRun
		err     error
	)
	if input.Approved {
		toolRun, err = g.service.ApproveToolRun(ctx, session, input.ToolCallID, input.Reason)
	} else {
		toolRun, err = g.service.RejectToolRun(ctx, session, input.ToolCallID, input.Reason)
	}
	if err != nil {
		return ToolApprovalState{}, mapAgentRuntimeError(err)
	}
	if toolRun == nil {
		return ToolApprovalState{}, status.Error(codes.Internal, "agent runtime did not return a tool run")
	}
	if input.RunID != "" && toolRun.RunID != input.RunID {
		return ToolApprovalState{}, status.Error(codes.InvalidArgument, "tool_call_id does not belong to run_id")
	}
	return ToolApprovalState{RunID: toolRun.RunID, ToolCallID: toolRun.ID, Status: toolRun.Status}, nil
}

func (g serviceRuntimeGateway) ContinuePlan(ctx context.Context, input PlanRunInput) (PlanRunState, error) {
	if g.service == nil {
		return PlanRunState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	result, err := g.service.ContinuePlanningRun(ctx, input.session(), input.RunID)
	if err != nil {
		return PlanRunState{}, mapAgentRuntimeError(err)
	}
	return planRunStateFromResult(result)
}

func (g serviceRuntimeGateway) AdjustPlan(ctx context.Context, input AdjustPlanInput) (PlanRunState, error) {
	if g.service == nil {
		return PlanRunState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	result, err := g.service.AdjustPlanSteps(ctx, input.session(), input.RunID, input.Reason)
	if err != nil {
		return PlanRunState{}, mapAgentRuntimeError(err)
	}
	return planRunStateFromResult(result)
}

func (g serviceRuntimeGateway) ApprovePlanStep(ctx context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	if g.service == nil {
		return PlanStepActionState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	step, err := g.service.ApprovePlanStep(ctx, input.session(), input.PlanStepID, input.Reason)
	if err != nil {
		return PlanStepActionState{}, mapAgentRuntimeError(err)
	}
	return g.planStepActionStateFromResult(ctx, input, step)
}

func (g serviceRuntimeGateway) ExecutePlanStep(ctx context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	if g.service == nil {
		return PlanStepActionState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	step, err := g.service.ExecutePlanStep(ctx, input.session(), input.PlanStepID)
	if err != nil {
		return PlanStepActionState{}, mapAgentRuntimeError(err)
	}
	return g.planStepActionStateFromResult(ctx, input, step)
}

func (g serviceRuntimeGateway) SkipPlanStep(ctx context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	if g.service == nil {
		return PlanStepActionState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	step, err := g.service.SkipPlanStep(ctx, input.session(), input.PlanStepID, input.Reason)
	if err != nil {
		return PlanStepActionState{}, mapAgentRuntimeError(err)
	}
	return g.planStepActionStateFromResult(ctx, input, step)
}

func (g serviceRuntimeGateway) RetryPlanStep(ctx context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	if g.service == nil {
		return PlanStepActionState{}, status.Error(codes.FailedPrecondition, "agent service is not configured")
	}
	step, err := g.service.RetryPlanStep(ctx, input.session(), input.PlanStepID)
	if err != nil {
		return PlanStepActionState{}, mapAgentRuntimeError(err)
	}
	return g.planStepActionStateFromResult(ctx, input, step)
}

func (g serviceRuntimeGateway) planStepActionStateFromResult(ctx context.Context, input PlanStepActionInput, step *internalagent.PlanStep) (PlanStepActionState, error) {
	state, err := planStepActionStateFromResult(input.RunID, step)
	if err != nil {
		return PlanStepActionState{}, err
	}
	result, err := g.service.GetRunWithMessages(ctx, input.session(), step.RunID)
	if err != nil {
		return PlanStepActionState{}, mapAgentRuntimeError(err)
	}
	runState, err := planRunStateFromResult(result)
	if err != nil {
		return PlanStepActionState{}, err
	}
	state.RunDetail = runState
	return state, nil
}

func (input CreateRunInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input ExecuteReActInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input ContinueBudgetInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input ToolApprovalInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input PlanRunInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input AdjustPlanInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input PlanStepActionInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func finalAssistantContent(result *internalagent.RunWithMessages) string {
	if result == nil || result.Run == nil {
		return ""
	}
	if result.Run.FinalMessageID != "" {
		for _, message := range result.Messages {
			if message != nil && message.ID == result.Run.FinalMessageID {
				return message.Content
			}
		}
	}
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if result.Messages[i] != nil && result.Messages[i].Role == "assistant" {
			return result.Messages[i].Content
		}
	}
	return ""
}

func pendingToolCalls(toolRuns []*internalagent.ToolRun) []PendingToolCall {
	pending := make([]PendingToolCall, 0, len(toolRuns))
	for _, toolRun := range toolRuns {
		if toolRun == nil || toolRun.Status != internalagent.ToolRunStatusPendingApproval {
			continue
		}
		pending = append(pending, PendingToolCall{
			ID:     toolRun.ID,
			Name:   toolRun.ToolName,
			Input:  toolRunArgumentsJSON(toolRun.Arguments),
			Status: toolRun.Status,
		})
	}
	return pending
}

func planRunInputFromRequest(req *agentv1.PlanRunRequest) (PlanRunInput, error) {
	if req.RunId == "" {
		return PlanRunInput{}, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.OrganizationId == "" {
		return PlanRunInput{}, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return PlanRunInput{}, status.Error(codes.InvalidArgument, "user_id is required")
	}
	return PlanRunInput{RunID: req.RunId, OrganizationID: req.OrganizationId, UserID: req.UserId}, nil
}

func planStepActionInputFromRequest(req *agentv1.PlanStepActionRequest) (PlanStepActionInput, error) {
	if req.RunId == "" {
		return PlanStepActionInput{}, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.PlanStepId == "" {
		return PlanStepActionInput{}, status.Error(codes.InvalidArgument, "plan_step_id is required")
	}
	if req.OrganizationId == "" {
		return PlanStepActionInput{}, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if req.UserId == "" {
		return PlanStepActionInput{}, status.Error(codes.InvalidArgument, "user_id is required")
	}
	return PlanStepActionInput{
		RunID:          req.RunId,
		PlanStepID:     req.PlanStepId,
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		Reason:         req.Reason,
	}, nil
}

func planRunStateFromResult(result *internalagent.RunWithMessages) (PlanRunState, error) {
	if result == nil || result.Run == nil {
		return PlanRunState{}, status.Error(codes.Internal, "agent runtime did not return a run")
	}
	return PlanRunState{
		RunID:     result.Run.ID,
		Status:    result.Run.Status,
		Result:    finalAssistantContent(result),
		PlanSteps: planStepStates(result.PlanSteps),
	}, nil
}

func planStepActionStateFromResult(requestRunID string, step *internalagent.PlanStep) (PlanStepActionState, error) {
	if step == nil {
		return PlanStepActionState{}, status.Error(codes.Internal, "agent runtime did not return a plan step")
	}
	if requestRunID != "" && step.RunID != requestRunID {
		return PlanStepActionState{}, status.Error(codes.InvalidArgument, "plan_step_id does not belong to run_id")
	}
	return PlanStepActionState{
		RunID:          step.RunID,
		PlanStepID:     step.ID,
		Index:          int32(step.Index),
		Status:         step.Status,
		ApprovalStatus: step.ApprovalStatus,
		Result:         step.ResultContent,
		Error:          step.Error,
	}, nil
}

func planStepStates(steps []*internalagent.PlanStep) []PlanStepState {
	states := make([]PlanStepState, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			continue
		}
		states = append(states, PlanStepState{
			ID:             step.ID,
			RunID:          step.RunID,
			Index:          int32(step.Index),
			Title:          step.Title,
			Description:    step.Description,
			Status:         step.Status,
			ApprovalStatus: step.ApprovalStatus,
			ToolName:       step.ToolName,
			Input:          mapJSON(step.Input),
			DependsOn:      intSliceToInt32(step.DependsOn),
			Result:         step.ResultContent,
			Error:          step.Error,
		})
	}
	return states
}

func planRunResponse(state PlanRunState) *agentv1.PlanRunResponse {
	steps := make([]*agentv1.PlanStep, 0, len(state.PlanSteps))
	for _, step := range state.PlanSteps {
		steps = append(steps, &agentv1.PlanStep{
			Id:             step.ID,
			RunId:          step.RunID,
			Index:          step.Index,
			Title:          step.Title,
			Description:    step.Description,
			Status:         step.Status,
			ApprovalStatus: step.ApprovalStatus,
			ToolName:       step.ToolName,
			Input:          step.Input,
			DependsOn:      step.DependsOn,
			Result:         step.Result,
			Error:          step.Error,
		})
	}
	return &agentv1.PlanRunResponse{
		RunId:     state.RunID,
		Status:    state.Status,
		Result:    state.Result,
		PlanSteps: steps,
	}
}

func toolRunArgumentsJSON(args map[string]any) string {
	return mapJSON(args)
}

func mapJSON(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func intSliceToInt32(values []int) []int32 {
	if len(values) == 0 {
		return nil
	}
	converted := make([]int32, 0, len(values))
	for _, value := range values {
		converted = append(converted, int32(value))
	}
	return converted
}

func mapAgentRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "required"):
		return status.Error(codes.InvalidArgument, message)
	case strings.Contains(message, "not found"):
		return status.Error(codes.NotFound, message)
	case strings.Contains(message, "access denied"):
		return status.Error(codes.PermissionDenied, message)
	case strings.Contains(message, "not pending approval"),
		strings.Contains(message, "pending approval"),
		strings.Contains(message, "invalid_state"),
		strings.Contains(message, "not in planning mode"),
		strings.Contains(message, "not approved"),
		strings.Contains(message, "requires approval"),
		strings.Contains(message, "not failed"),
		strings.Contains(message, "cannot be"),
		strings.Contains(message, "cannot "),
		strings.Contains(message, "prior plan step"):
		return status.Error(codes.FailedPrecondition, message)
	default:
		return status.Error(codes.Internal, fmt.Sprintf("agent runtime failed: %s", message))
	}
}
