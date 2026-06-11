package agent

import (
	"context"
	"testing"

	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateRun(t *testing.T) {
	s := NewServer()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *agentv1.CreateRunRequest
		wantErr codes.Code
	}{
		{
			name: "valid request",
			req: &agentv1.CreateRunRequest{
				AgentId:        "agent1",
				UserId:         "user1",
				OrganizationId: "org1",
				ConversationId: "conv1",
				UserContent:    "test",
			},
			wantErr: codes.OK,
		},
		{
			name: "missing agent_id",
			req: &agentv1.CreateRunRequest{
				UserId:         "user1",
				OrganizationId: "org1",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "missing user_id",
			req: &agentv1.CreateRunRequest{
				AgentId:        "agent1",
				OrganizationId: "org1",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.CreateRunRequest{
				AgentId: "agent1",
				UserId:  "user1",
			},
			wantErr: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := s.CreateRun(ctx, tt.req)
			if tt.wantErr != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if status.Code(err) != tt.wantErr {
					t.Errorf("expected error code %v, got %v", tt.wantErr, status.Code(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.RunId == "" {
				t.Error("expected non-empty run_id")
			}
			if resp.Status != "created" {
				t.Errorf("expected status 'created', got %q", resp.Status)
			}
		})
	}
}

func TestExecuteReAct(t *testing.T) {
	s := NewServer()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *agentv1.ExecuteReActRequest
		wantErr codes.Code
	}{
		{
			name: "valid request",
			req: &agentv1.ExecuteReActRequest{
				RunId:          "run1",
				OrganizationId: "org1",
			},
			wantErr: codes.OK,
		},
		{
			name: "missing run_id",
			req: &agentv1.ExecuteReActRequest{
				OrganizationId: "org1",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ExecuteReActRequest{
				RunId: "run1",
			},
			wantErr: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := s.ExecuteReAct(ctx, tt.req)
			if tt.wantErr != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if status.Code(err) != tt.wantErr {
					t.Errorf("expected error code %v, got %v", tt.wantErr, status.Code(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.RunId != tt.req.RunId {
				t.Errorf("expected run_id %q, got %q", tt.req.RunId, resp.RunId)
			}
			if resp.Status != "completed" {
				t.Errorf("expected status 'completed', got %q", resp.Status)
			}
		})
	}
}

func TestApproveToolCall(t *testing.T) {
	s := NewServer()
	ctx := context.Background()

	tests := []struct {
		name       string
		req        *agentv1.ApproveToolCallRequest
		wantStatus string
		wantErr    codes.Code
	}{
		{
			name: "approved",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				ToolCallId:     "tool1",
				Approved:       true,
				OrganizationId: "org1",
			},
			wantStatus: "approved",
			wantErr:    codes.OK,
		},
		{
			name: "rejected",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				ToolCallId:     "tool1",
				Approved:       false,
				OrganizationId: "org1",
			},
			wantStatus: "rejected",
			wantErr:    codes.OK,
		},
		{
			name: "missing run_id",
			req: &agentv1.ApproveToolCallRequest{
				ToolCallId:     "tool1",
				OrganizationId: "org1",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "missing tool_call_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				OrganizationId: "org1",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:      "run1",
				ToolCallId: "tool1",
			},
			wantErr: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := s.ApproveToolCall(ctx, tt.req)
			if tt.wantErr != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if status.Code(err) != tt.wantErr {
					t.Errorf("expected error code %v, got %v", tt.wantErr, status.Code(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Errorf("expected status %q, got %q", tt.wantStatus, resp.Status)
			}
			if resp.RunId != tt.req.RunId {
				t.Errorf("expected run_id %q, got %q", tt.req.RunId, resp.RunId)
			}
			if resp.ToolCallId != tt.req.ToolCallId {
				t.Errorf("expected tool_call_id %q, got %q", tt.req.ToolCallId, resp.ToolCallId)
			}
		})
	}
}
