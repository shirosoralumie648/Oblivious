package agent

import (
	"context"
	"errors"
	"testing"

	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRuntimeGateway struct {
	createInput   CreateRunInput
	executeInput  ExecuteReActInput
	approvalInput ToolApprovalInput

	createResponse   RunState
	executeResponse  RunExecutionState
	approvalResponse ToolApprovalState
	err              error
}

func (f *fakeRuntimeGateway) CreateRun(_ context.Context, input CreateRunInput) (RunState, error) {
	f.createInput = input
	if f.err != nil {
		return RunState{}, f.err
	}
	return f.createResponse, nil
}

func (f *fakeRuntimeGateway) ExecuteReAct(_ context.Context, input ExecuteReActInput) (RunExecutionState, error) {
	f.executeInput = input
	if f.err != nil {
		return RunExecutionState{}, f.err
	}
	return f.executeResponse, nil
}

func (f *fakeRuntimeGateway) ApproveToolCall(_ context.Context, input ToolApprovalInput) (ToolApprovalState, error) {
	f.approvalInput = input
	if f.err != nil {
		return ToolApprovalState{}, f.err
	}
	return f.approvalResponse, nil
}

func TestCreateRun(t *testing.T) {
	runtime := &fakeRuntimeGateway{
		createResponse: RunState{RunID: "run_real_agent", Status: "pending_approval"},
	}
	s := NewServerWithRuntime(runtime)

	resp, err := s.CreateRun(context.Background(), &agentv1.CreateRunRequest{
		AgentId:        "agent1",
		ConversationId: "conv1",
		UserContent:    "ship it",
		OrganizationId: "org1",
		UserId:         "user1",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if resp.RunId != "run_real_agent" || resp.Status != "pending_approval" {
		t.Fatalf("expected runtime response, got %+v", resp)
	}
	if runtime.createInput.AgentID != "agent1" ||
		runtime.createInput.ConversationID != "conv1" ||
		runtime.createInput.UserContent != "ship it" ||
		runtime.createInput.OrganizationID != "org1" ||
		runtime.createInput.UserID != "user1" {
		t.Fatalf("expected request fields to be forwarded, got %+v", runtime.createInput)
	}
}

func TestCreateRunValidationAndRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name string
		req  *agentv1.CreateRunRequest
		code codes.Code
	}{
		{
			name: "missing agent_id",
			req: &agentv1.CreateRunRequest{
				UserId:         "user1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing user_id",
			req: &agentv1.CreateRunRequest{
				AgentId:        "agent1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.CreateRunRequest{
				AgentId: "agent1",
				UserId:  "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "runtime not configured",
			req: &agentv1.CreateRunRequest{
				AgentId:        "agent1",
				UserId:         "user1",
				OrganizationId: "org1",
			},
			code: codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer().CreateRun(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if status.Code(err) != tt.code {
				t.Fatalf("expected %v, got %v", tt.code, status.Code(err))
			}
		})
	}
}

func TestExecuteReAct(t *testing.T) {
	runtime := &fakeRuntimeGateway{
		executeResponse: RunExecutionState{
			RunID:  "run1",
			Status: "pending_approval",
			Result: "approval required",
			PendingToolCalls: []PendingToolCall{{
				ID:     "tool_run_1",
				Name:   "web_search",
				Input:  `{"query":"release"}`,
				Status: "pending_approval",
			}},
		},
	}
	s := NewServerWithRuntime(runtime)

	resp, err := s.ExecuteReAct(context.Background(), &agentv1.ExecuteReActRequest{
		RunId:          "run1",
		OrganizationId: "org1",
	})
	if err != nil {
		t.Fatalf("ExecuteReAct returned error: %v", err)
	}
	if runtime.executeInput.RunID != "run1" || runtime.executeInput.OrganizationID != "org1" {
		t.Fatalf("expected request fields to be forwarded, got %+v", runtime.executeInput)
	}
	if resp.RunId != "run1" || resp.Status != "pending_approval" || resp.Result != "approval required" {
		t.Fatalf("expected runtime execution response, got %+v", resp)
	}
	if len(resp.PendingToolCalls) != 1 ||
		resp.PendingToolCalls[0].Id != "tool_run_1" ||
		resp.PendingToolCalls[0].Name != "web_search" ||
		resp.PendingToolCalls[0].Input != `{"query":"release"}` ||
		resp.PendingToolCalls[0].Status != "pending_approval" {
		t.Fatalf("expected pending tool call from runtime, got %+v", resp.PendingToolCalls)
	}
}

func TestExecuteReActValidationAndRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name string
		req  *agentv1.ExecuteReActRequest
		code codes.Code
	}{
		{
			name: "missing run_id",
			req: &agentv1.ExecuteReActRequest{
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ExecuteReActRequest{
				RunId: "run1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "runtime not configured",
			req: &agentv1.ExecuteReActRequest{
				RunId:          "run1",
				OrganizationId: "org1",
			},
			code: codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer().ExecuteReAct(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if status.Code(err) != tt.code {
				t.Fatalf("expected %v, got %v", tt.code, status.Code(err))
			}
		})
	}
}

func TestApproveToolCall(t *testing.T) {
	runtime := &fakeRuntimeGateway{
		approvalResponse: ToolApprovalState{
			RunID:      "run1",
			ToolCallID: "tool_run_1",
			Status:     "completed",
		},
	}
	s := NewServerWithRuntime(runtime)

	resp, err := s.ApproveToolCall(context.Background(), &agentv1.ApproveToolCallRequest{
		RunId:          "run1",
		ToolCallId:     "tool_run_1",
		Approved:       true,
		OrganizationId: "org1",
	})
	if err != nil {
		t.Fatalf("ApproveToolCall returned error: %v", err)
	}
	if runtime.approvalInput.RunID != "run1" ||
		runtime.approvalInput.ToolCallID != "tool_run_1" ||
		!runtime.approvalInput.Approved ||
		runtime.approvalInput.OrganizationID != "org1" {
		t.Fatalf("expected request fields to be forwarded, got %+v", runtime.approvalInput)
	}
	if resp.RunId != "run1" || resp.ToolCallId != "tool_run_1" || resp.Status != "completed" {
		t.Fatalf("expected runtime approval response, got %+v", resp)
	}
}

func TestApproveToolCallValidationAndRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name string
		req  *agentv1.ApproveToolCallRequest
		code codes.Code
	}{
		{
			name: "missing run_id",
			req: &agentv1.ApproveToolCallRequest{
				ToolCallId:     "tool1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing tool_call_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:      "run1",
				ToolCallId: "tool1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "runtime not configured",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				ToolCallId:     "tool1",
				OrganizationId: "org1",
			},
			code: codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer().ApproveToolCall(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if status.Code(err) != tt.code {
				t.Fatalf("expected %v, got %v", tt.code, status.Code(err))
			}
		})
	}
}

func TestRuntimeErrorsPropagate(t *testing.T) {
	runtimeErr := status.Error(codes.Unavailable, "runtime offline")
	s := NewServerWithRuntime(&fakeRuntimeGateway{err: runtimeErr})

	_, err := s.CreateRun(context.Background(), &agentv1.CreateRunRequest{
		AgentId:        "agent1",
		UserId:         "user1",
		OrganizationId: "org1",
	})
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("expected runtime error to propagate, got %v", err)
	}
}
