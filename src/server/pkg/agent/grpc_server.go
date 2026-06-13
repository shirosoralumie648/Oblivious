package agent

import (
	"context"

	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
}

type ExecuteReActInput struct {
	RunID          string
	OrganizationID string
}

type ToolApprovalInput struct {
	RunID          string
	ToolCallID     string
	Approved       bool
	OrganizationID string
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
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	run, err := s.runtime.ExecuteReAct(ctx, ExecuteReActInput{
		RunID:          req.RunId,
		OrganizationID: req.OrganizationId,
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
	if s.runtime == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent runtime is not configured")
	}

	approval, err := s.runtime.ApproveToolCall(ctx, ToolApprovalInput{
		RunID:          req.RunId,
		ToolCallID:     req.ToolCallId,
		Approved:       req.Approved,
		OrganizationID: req.OrganizationId,
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
