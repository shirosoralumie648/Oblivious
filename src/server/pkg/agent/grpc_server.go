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
	ApproveToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error)
	RejectToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error)
}

type RuntimeGateway interface {
	CreateRun(context.Context, CreateRunInput) (RunState, error)
	ExecuteReAct(context.Context, ExecuteReActInput) (RunExecutionState, error)
	ApproveToolCall(context.Context, ToolApprovalInput) (ToolApprovalState, error)
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

type ToolApprovalInput struct {
	RunID          string
	ToolCallID     string
	Approved       bool
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

func (input CreateRunInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input ExecuteReActInput) session() auth.Session {
	return auth.Session{OrganizationID: input.OrganizationID, User: auth.User{ID: input.UserID}}
}

func (input ToolApprovalInput) session() auth.Session {
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

func toolRunArgumentsJSON(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(encoded)
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
		strings.Contains(message, "not in planning mode"):
		return status.Error(codes.FailedPrecondition, message)
	default:
		return status.Error(codes.Internal, fmt.Sprintf("agent runtime failed: %s", message))
	}
}
