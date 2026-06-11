package agent

import (
	"context"

	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	agentv1.UnimplementedAgentServiceServer
}

func NewServer() *Server {
	return &Server{}
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

	return &agentv1.CreateRunResponse{
		RunId:  "run_" + req.AgentId,
		Status: "created",
	}, nil
}

func (s *Server) ExecuteReAct(ctx context.Context, req *agentv1.ExecuteReActRequest) (*agentv1.ExecuteReActResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}

	return &agentv1.ExecuteReActResponse{
		RunId:  req.RunId,
		Status: "completed",
		Result: "execution completed",
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

	statusStr := "rejected"
	if req.Approved {
		statusStr = "approved"
	}

	return &agentv1.ApproveToolCallResponse{
		RunId:      req.RunId,
		ToolCallId: req.ToolCallId,
		Status:     statusStr,
	}, nil
}
