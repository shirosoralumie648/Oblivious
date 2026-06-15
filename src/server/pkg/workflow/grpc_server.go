package workflow

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	workflowv1 "oblivious/server/internal/grpc/workflowv1"
	"oblivious/server/internal/workflow"
)

type workflowService interface {
	StartExecution(ctx context.Context, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error)
	TestNode(ctx context.Context, req workflow.TestNodeRequest) (*workflow.TestNodeResult, error)
}

type Server struct {
	workflowv1.UnimplementedWorkflowServiceServer
	service workflowService
}

func NewServer(service *workflow.Service) *Server {
	return &Server{service: service}
}

func (s *Server) Execute(ctx context.Context, req *workflowv1.ExecuteRequest) (*workflowv1.ExecuteResponse, error) {
	if req.WorkflowId == "" {
		return nil, status.Error(codes.InvalidArgument, "workflow_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if s.service == nil {
		return nil, status.Error(codes.FailedPrecondition, "workflow service is not configured")
	}

	inputs := make(map[string]any, len(req.Inputs))
	for k, v := range req.Inputs {
		inputs[k] = v
	}

	exec, err := s.service.StartExecution(ctx, workflow.StartExecutionRequest{
		OrganizationID: req.OrganizationId,
		WorkflowID:     req.WorkflowId,
		Input:          inputs,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start execution: %v", err)
	}

	outputs := make(map[string]string)
	for k, v := range exec.Output {
		if s, ok := v.(string); ok {
			outputs[k] = s
		}
	}

	return &workflowv1.ExecuteResponse{
		ExecutionId: exec.ID,
		Status:      string(exec.Status),
		Outputs:     outputs,
	}, nil
}

func (s *Server) TestNode(ctx context.Context, req *workflowv1.TestNodeRequest) (*workflowv1.TestNodeResponse, error) {
	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if req.OrganizationId == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if s.service == nil {
		return nil, status.Error(codes.FailedPrecondition, "workflow service is not configured")
	}

	inputs := make(map[string]any, len(req.Inputs))
	for k, v := range req.Inputs {
		inputs[k] = v
	}

	result, err := s.service.TestNode(ctx, workflow.TestNodeRequest{
		OrganizationID: req.OrganizationId,
		NodeID:         req.NodeId,
		Input:          inputs,
	})
	if err != nil {
		return &workflowv1.TestNodeResponse{
			Status: "failed",
			Error:  err.Error(),
		}, nil
	}

	outputs := make(map[string]string)
	for k, v := range result.Output {
		if s, ok := v.(string); ok {
			outputs[k] = s
		}
	}

	return &workflowv1.TestNodeResponse{
		Status:  string(result.Status),
		Outputs: outputs,
	}, nil
}
