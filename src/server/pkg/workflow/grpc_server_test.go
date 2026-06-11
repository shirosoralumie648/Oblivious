package workflow

import (
	"context"
	"errors"
	"testing"

	workflowv1 "oblivious/server/internal/grpc/workflowv1"
	"oblivious/server/internal/workflow"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockService interface {
	StartExecution(ctx context.Context, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error)
	TestNode(ctx context.Context, req workflow.TestNodeRequest) (*workflow.TestNodeResult, error)
}

type mockExec struct {
	exec *workflow.WorkflowExecution
	err  error
}

func (m *mockExec) StartExecution(ctx context.Context, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	return m.exec, m.err
}

func (m *mockExec) TestNode(ctx context.Context, req workflow.TestNodeRequest) (*workflow.TestNodeResult, error) {
	return nil, errors.New("not implemented")
}

type mockTest struct {
	result *workflow.TestNodeResult
	err    error
}

func (m *mockTest) StartExecution(ctx context.Context, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTest) TestNode(ctx context.Context, req workflow.TestNodeRequest) (*workflow.TestNodeResult, error) {
	return m.result, m.err
}

func TestServer_Execute(t *testing.T) {
	tests := []struct {
		name     string
		req      *workflowv1.ExecuteRequest
		mock     func() mockService
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "success",
			req: &workflowv1.ExecuteRequest{
				WorkflowId:     "wf-123",
				OrganizationId: "org-456",
				UserId:         "user-789",
				Inputs:         map[string]string{"key": "value"},
			},
			mock: func() mockService {
				return &mockExec{
					exec: &workflow.WorkflowExecution{
						ID:     "exec-001",
						Status: workflow.ExecutionStatusRunning,
						Output: map[string]any{"result": "ok"},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "missing workflow_id",
			req: &workflowv1.ExecuteRequest{
				OrganizationId: "org-456",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &workflowv1.ExecuteRequest{
				WorkflowId: "wf-123",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "service error",
			req: &workflowv1.ExecuteRequest{
				WorkflowId:     "wf-123",
				OrganizationId: "org-456",
			},
			mock: func() mockService {
				return &mockExec{err: errors.New("service error")}
			},
			wantErr:  true,
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m mockService
			if tt.mock != nil {
				m = tt.mock()
			}
			s := &Server{service: m}
			resp, err := s.Execute(context.Background(), tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if st, ok := status.FromError(err); ok {
					if st.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
		})
	}
}

func TestServer_TestNode(t *testing.T) {
	tests := []struct {
		name     string
		req      *workflowv1.TestNodeRequest
		mock     func() mockService
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "success",
			req: &workflowv1.TestNodeRequest{
				NodeId:         "node-123",
				OrganizationId: "org-456",
				Inputs:         map[string]string{"input": "test"},
			},
			mock: func() mockService {
				return &mockTest{
					result: &workflow.TestNodeResult{
						Status: workflow.ExecutionStatusCompleted,
						Output: map[string]any{"output": "success"},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "missing node_id",
			req: &workflowv1.TestNodeRequest{
				OrganizationId: "org-456",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &workflowv1.TestNodeRequest{
				NodeId: "node-123",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "service error returns failed status",
			req: &workflowv1.TestNodeRequest{
				NodeId:         "node-123",
				OrganizationId: "org-456",
			},
			mock: func() mockService {
				return &mockTest{err: errors.New("test node failed")}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m mockService
			if tt.mock != nil {
				m = tt.mock()
			}
			s := &Server{service: m}
			resp, err := s.TestNode(context.Background(), tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if st, ok := status.FromError(err); ok {
					if st.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
		})
	}
}
