package agent

import (
	"context"
	"errors"
	"testing"

	internalagent "oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	agentv1 "oblivious/server/internal/grpc/agentv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRuntimeGateway struct {
	createInput         CreateRunInput
	executeInput        ExecuteReActInput
	approvalInput       ToolApprovalInput
	continuePlanInput   PlanRunInput
	adjustPlanInput     AdjustPlanInput
	planStepActionInput PlanStepActionInput
	planStepAction      string

	createResponse         RunState
	executeResponse        RunExecutionState
	approvalResponse       ToolApprovalState
	planRunResponse        PlanRunState
	planStepActionResponse PlanStepActionState
	err                    error
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

func (f *fakeRuntimeGateway) ContinuePlan(_ context.Context, input PlanRunInput) (PlanRunState, error) {
	f.continuePlanInput = input
	if f.err != nil {
		return PlanRunState{}, f.err
	}
	return f.planRunResponse, nil
}

func (f *fakeRuntimeGateway) AdjustPlan(_ context.Context, input AdjustPlanInput) (PlanRunState, error) {
	f.adjustPlanInput = input
	if f.err != nil {
		return PlanRunState{}, f.err
	}
	return f.planRunResponse, nil
}

func (f *fakeRuntimeGateway) ApprovePlanStep(_ context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	f.planStepAction = "approve"
	f.planStepActionInput = input
	if f.err != nil {
		return PlanStepActionState{}, f.err
	}
	return f.planStepActionResponse, nil
}

func (f *fakeRuntimeGateway) ExecutePlanStep(_ context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	f.planStepAction = "execute"
	f.planStepActionInput = input
	if f.err != nil {
		return PlanStepActionState{}, f.err
	}
	return f.planStepActionResponse, nil
}

func (f *fakeRuntimeGateway) SkipPlanStep(_ context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	f.planStepAction = "skip"
	f.planStepActionInput = input
	if f.err != nil {
		return PlanStepActionState{}, f.err
	}
	return f.planStepActionResponse, nil
}

func (f *fakeRuntimeGateway) RetryPlanStep(_ context.Context, input PlanStepActionInput) (PlanStepActionState, error) {
	f.planStepAction = "retry"
	f.planStepActionInput = input
	if f.err != nil {
		return PlanStepActionState{}, f.err
	}
	return f.planStepActionResponse, nil
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
		RecursionDepth: 1,
		MaxDepth:       6,
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
		runtime.createInput.UserID != "user1" ||
		runtime.createInput.RecursionDepth != 1 ||
		runtime.createInput.MaxDepth != 6 {
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
		UserId:         "user1",
	})
	if err != nil {
		t.Fatalf("ExecuteReAct returned error: %v", err)
	}
	if runtime.executeInput.RunID != "run1" ||
		runtime.executeInput.OrganizationID != "org1" ||
		runtime.executeInput.UserID != "user1" {
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
				UserId:         "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ExecuteReActRequest{
				RunId:  "run1",
				UserId: "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing user_id",
			req: &agentv1.ExecuteReActRequest{
				RunId:          "run1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "runtime not configured",
			req: &agentv1.ExecuteReActRequest{
				RunId:          "run1",
				OrganizationId: "org1",
				UserId:         "user1",
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
		UserId:         "user1",
		Reason:         "operator approved",
	})
	if err != nil {
		t.Fatalf("ApproveToolCall returned error: %v", err)
	}
	if runtime.approvalInput.RunID != "run1" ||
		runtime.approvalInput.ToolCallID != "tool_run_1" ||
		!runtime.approvalInput.Approved ||
		runtime.approvalInput.OrganizationID != "org1" ||
		runtime.approvalInput.UserID != "user1" ||
		runtime.approvalInput.Reason != "operator approved" {
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
				UserId:         "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing tool_call_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				OrganizationId: "org1",
				UserId:         "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing organization_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:      "run1",
				ToolCallId: "tool1",
				UserId:     "user1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing user_id",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				ToolCallId:     "tool1",
				OrganizationId: "org1",
			},
			code: codes.InvalidArgument,
		},
		{
			name: "runtime not configured",
			req: &agentv1.ApproveToolCallRequest{
				RunId:          "run1",
				ToolCallId:     "tool1",
				OrganizationId: "org1",
				UserId:         "user1",
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

func TestContinuePlanReturnsRunDetail(t *testing.T) {
	runtime := &fakeRuntimeGateway{
		planRunResponse: PlanRunState{
			RunID:  "run_plan",
			Status: internalagent.RunStatusPendingApproval,
			Result: "waiting for approval",
			PlanSteps: []PlanStepState{{
				ID:             "step_1",
				RunID:          "run_plan",
				Index:          1,
				Title:          "Inspect",
				Description:    "inspect current state",
				Status:         internalagent.PlanStepStatusCompleted,
				ApprovalStatus: internalagent.ApprovalStatusApproved,
				ToolName:       "shell",
				Input:          `{"cmd":"pwd"}`,
				DependsOn:      []int32{0},
				Result:         "done",
			}},
		},
	}
	s := NewServerWithRuntime(runtime)

	resp, err := s.ContinuePlan(context.Background(), &agentv1.PlanRunRequest{
		RunId:          "run_plan",
		OrganizationId: "org1",
		UserId:         "user1",
	})
	if err != nil {
		t.Fatalf("ContinuePlan returned error: %v", err)
	}
	if runtime.continuePlanInput.RunID != "run_plan" ||
		runtime.continuePlanInput.OrganizationID != "org1" ||
		runtime.continuePlanInput.UserID != "user1" {
		t.Fatalf("expected continue request fields to be forwarded, got %+v", runtime.continuePlanInput)
	}
	if resp.RunId != "run_plan" || resp.Status != internalagent.RunStatusPendingApproval || resp.Result != "waiting for approval" {
		t.Fatalf("expected run detail response, got %+v", resp)
	}
	if len(resp.PlanSteps) != 1 ||
		resp.PlanSteps[0].Id != "step_1" ||
		resp.PlanSteps[0].Input != `{"cmd":"pwd"}` ||
		len(resp.PlanSteps[0].DependsOn) != 1 ||
		resp.PlanSteps[0].DependsOn[0] != 0 {
		t.Fatalf("expected plan step detail to round trip, got %+v", resp.PlanSteps)
	}
}

func TestAdjustPlanRequiresReasonAndForwardsRequest(t *testing.T) {
	runtime := &fakeRuntimeGateway{
		planRunResponse: PlanRunState{RunID: "run_plan", Status: internalagent.RunStatusPendingApproval},
	}
	s := NewServerWithRuntime(runtime)

	if _, err := s.AdjustPlan(context.Background(), &agentv1.AdjustPlanRequest{
		RunId:          "run_plan",
		OrganizationId: "org1",
		UserId:         "user1",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected missing reason to fail invalid argument, got %v", err)
	}

	resp, err := s.AdjustPlan(context.Background(), &agentv1.AdjustPlanRequest{
		RunId:          "run_plan",
		OrganizationId: "org1",
		UserId:         "user1",
		Reason:         "new evidence changed the suffix",
	})
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}
	if resp.RunId != "run_plan" || resp.Status != internalagent.RunStatusPendingApproval {
		t.Fatalf("expected adjusted run detail response, got %+v", resp)
	}
	if runtime.adjustPlanInput.RunID != "run_plan" ||
		runtime.adjustPlanInput.OrganizationID != "org1" ||
		runtime.adjustPlanInput.UserID != "user1" ||
		runtime.adjustPlanInput.Reason != "new evidence changed the suffix" {
		t.Fatalf("expected adjust request fields to be forwarded, got %+v", runtime.adjustPlanInput)
	}
}

func TestPlanStepActionsForwardRequestAndReturnRunDetail(t *testing.T) {
	tests := []struct {
		name   string
		action string
		call   func(*Server, context.Context, *agentv1.PlanStepActionRequest) (*agentv1.PlanStepActionResponse, error)
	}{
		{name: "approve", action: "approve", call: (*Server).ApprovePlanStep},
		{name: "execute", action: "execute", call: (*Server).ExecutePlanStep},
		{name: "skip", action: "skip", call: (*Server).SkipPlanStep},
		{name: "retry", action: "retry", call: (*Server).RetryPlanStep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := &fakeRuntimeGateway{
				planStepActionResponse: PlanStepActionState{
					RunID:          "run_plan",
					PlanStepID:     "step_1",
					Index:          2,
					Status:         internalagent.PlanStepStatusCompleted,
					ApprovalStatus: internalagent.ApprovalStatusApproved,
					Result:         "step result",
					RunDetail: PlanRunState{
						RunID:  "run_plan",
						Status: internalagent.RunStatusCompleted,
						PlanSteps: []PlanStepState{{
							ID:             "step_1",
							RunID:          "run_plan",
							Index:          2,
							Title:          "Execute approved step",
							Status:         internalagent.PlanStepStatusCompleted,
							ApprovalStatus: internalagent.ApprovalStatusApproved,
							Result:         "step result",
						}},
					},
				},
			}
			s := NewServerWithRuntime(runtime)

			resp, err := tt.call(s, context.Background(), &agentv1.PlanStepActionRequest{
				RunId:          "run_plan",
				PlanStepId:     "step_1",
				OrganizationId: "org1",
				UserId:         "user1",
				Reason:         "operator evidence",
			})
			if err != nil {
				t.Fatalf("%s returned error: %v", tt.name, err)
			}
			if runtime.planStepAction != tt.action ||
				runtime.planStepActionInput.RunID != "run_plan" ||
				runtime.planStepActionInput.PlanStepID != "step_1" ||
				runtime.planStepActionInput.OrganizationID != "org1" ||
				runtime.planStepActionInput.UserID != "user1" ||
				runtime.planStepActionInput.Reason != "operator evidence" {
				t.Fatalf("expected %s request fields to be forwarded, action=%q input=%+v", tt.name, runtime.planStepAction, runtime.planStepActionInput)
			}
			if resp.RunId != "run_plan" ||
				resp.PlanStepId != "step_1" ||
				resp.Index != 2 ||
				resp.Status != internalagent.PlanStepStatusCompleted ||
				resp.ApprovalStatus != internalagent.ApprovalStatusApproved ||
				resp.Result != "step result" {
				t.Fatalf("expected plan-step action response, got %+v", resp)
			}
			if resp.RunDetail == nil ||
				resp.RunDetail.RunId != "run_plan" ||
				resp.RunDetail.Status != internalagent.RunStatusCompleted ||
				len(resp.RunDetail.PlanSteps) != 1 ||
				resp.RunDetail.PlanSteps[0].Id != "step_1" {
				t.Fatalf("expected refreshed run detail in action response, got %+v", resp.RunDetail)
			}
		})
	}
}

func TestPlanningRPCValidationAndRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name string
		call func(*Server) error
		code codes.Code
	}{
		{
			name: "continue missing run_id",
			call: func(s *Server) error {
				_, err := s.ContinuePlan(context.Background(), &agentv1.PlanRunRequest{OrganizationId: "org1", UserId: "user1"})
				return err
			},
			code: codes.InvalidArgument,
		},
		{
			name: "continue missing organization_id",
			call: func(s *Server) error {
				_, err := s.ContinuePlan(context.Background(), &agentv1.PlanRunRequest{RunId: "run1", UserId: "user1"})
				return err
			},
			code: codes.InvalidArgument,
		},
		{
			name: "continue missing user_id",
			call: func(s *Server) error {
				_, err := s.ContinuePlan(context.Background(), &agentv1.PlanRunRequest{RunId: "run1", OrganizationId: "org1"})
				return err
			},
			code: codes.InvalidArgument,
		},
		{
			name: "continue runtime not configured",
			call: func(s *Server) error {
				_, err := s.ContinuePlan(context.Background(), &agentv1.PlanRunRequest{RunId: "run1", OrganizationId: "org1", UserId: "user1"})
				return err
			},
			code: codes.FailedPrecondition,
		},
		{
			name: "plan step missing id",
			call: func(s *Server) error {
				_, err := s.ApprovePlanStep(context.Background(), &agentv1.PlanStepActionRequest{RunId: "run1", OrganizationId: "org1", UserId: "user1"})
				return err
			},
			code: codes.InvalidArgument,
		},
		{
			name: "plan step runtime not configured",
			call: func(s *Server) error {
				_, err := s.ExecutePlanStep(context.Background(), &agentv1.PlanStepActionRequest{RunId: "run1", PlanStepId: "step1", OrganizationId: "org1", UserId: "user1"})
				return err
			},
			code: codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(NewServer())
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

type fakeAgentRuntimeService struct {
	startSession      auth.Session
	startRequest      internalagent.StartRunRequest
	listSession       auth.Session
	listConversation  string
	detailSession     auth.Session
	detailRunID       string
	approveSession    auth.Session
	approveToolRunID  string
	approveReason     string
	rejectSession     auth.Session
	rejectToolRunID   string
	rejectReason      string
	continueSession   auth.Session
	continueRunID     string
	adjustSession     auth.Session
	adjustRunID       string
	adjustReason      string
	planStepSession   auth.Session
	planStepID        string
	planStepReason    string
	planStepAction    string
	startResponse     *internalagent.RunWithMessages
	startError        error
	listResponse      []*internalagent.Run
	detailResponse    *internalagent.RunWithMessages
	detailError       error
	approveResponse   *internalagent.ToolRun
	rejectResponse    *internalagent.ToolRun
	continueResponse  *internalagent.RunWithMessages
	adjustResponse    *internalagent.RunWithMessages
	planStepResponse  *internalagent.PlanStep
	toolDecisionError error
	planningError     error
	planStepError     error
}

func (f *fakeAgentRuntimeService) StartRun(_ context.Context, session auth.Session, req internalagent.StartRunRequest) (*internalagent.RunWithMessages, error) {
	f.startSession = session
	f.startRequest = req
	return f.startResponse, f.startError
}

func (f *fakeAgentRuntimeService) ListRuns(_ context.Context, session auth.Session, conversationID string) ([]*internalagent.Run, error) {
	f.listSession = session
	f.listConversation = conversationID
	return f.listResponse, nil
}

func (f *fakeAgentRuntimeService) GetRunWithMessages(_ context.Context, session auth.Session, runID string) (*internalagent.RunWithMessages, error) {
	f.detailSession = session
	f.detailRunID = runID
	return f.detailResponse, f.detailError
}

func (f *fakeAgentRuntimeService) ApproveToolRun(_ context.Context, session auth.Session, toolRunID, reason string) (*internalagent.ToolRun, error) {
	f.approveSession = session
	f.approveToolRunID = toolRunID
	f.approveReason = reason
	return f.approveResponse, f.toolDecisionError
}

func (f *fakeAgentRuntimeService) RejectToolRun(_ context.Context, session auth.Session, toolRunID, reason string) (*internalagent.ToolRun, error) {
	f.rejectSession = session
	f.rejectToolRunID = toolRunID
	f.rejectReason = reason
	return f.rejectResponse, f.toolDecisionError
}

func (f *fakeAgentRuntimeService) ContinuePlanningRun(_ context.Context, session auth.Session, runID string) (*internalagent.RunWithMessages, error) {
	f.continueSession = session
	f.continueRunID = runID
	return f.continueResponse, f.planningError
}

func (f *fakeAgentRuntimeService) AdjustPlanSteps(_ context.Context, session auth.Session, runID, reason string) (*internalagent.RunWithMessages, error) {
	f.adjustSession = session
	f.adjustRunID = runID
	f.adjustReason = reason
	return f.adjustResponse, f.planningError
}

func (f *fakeAgentRuntimeService) ApprovePlanStep(_ context.Context, session auth.Session, planStepID, reason string) (*internalagent.PlanStep, error) {
	f.planStepSession = session
	f.planStepID = planStepID
	f.planStepReason = reason
	f.planStepAction = "approve"
	return f.planStepResponse, f.planStepError
}

func (f *fakeAgentRuntimeService) ExecutePlanStep(_ context.Context, session auth.Session, planStepID string) (*internalagent.PlanStep, error) {
	f.planStepSession = session
	f.planStepID = planStepID
	f.planStepAction = "execute"
	return f.planStepResponse, f.planStepError
}

func (f *fakeAgentRuntimeService) SkipPlanStep(_ context.Context, session auth.Session, planStepID, reason string) (*internalagent.PlanStep, error) {
	f.planStepSession = session
	f.planStepID = planStepID
	f.planStepReason = reason
	f.planStepAction = "skip"
	return f.planStepResponse, f.planStepError
}

func (f *fakeAgentRuntimeService) RetryPlanStep(_ context.Context, session auth.Session, planStepID string) (*internalagent.PlanStep, error) {
	f.planStepSession = session
	f.planStepID = planStepID
	f.planStepAction = "retry"
	return f.planStepResponse, f.planStepError
}

func TestServiceRuntimeGatewayCreateRunUsesAgentServiceSessionAndRequest(t *testing.T) {
	service := &fakeAgentRuntimeService{
		startResponse: &internalagent.RunWithMessages{
			Run: &internalagent.Run{ID: "run_service", Status: internalagent.RunStatusCompleted},
		},
	}

	run, err := NewServiceRuntimeGateway(service).CreateRun(context.Background(), CreateRunInput{
		AgentID:        "agent1",
		ConversationID: "conv1",
		UserContent:    "ship it",
		OrganizationID: "org1",
		UserID:         "user1",
		MaxDepth:       7,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if run.RunID != "run_service" || run.Status != internalagent.RunStatusCompleted {
		t.Fatalf("expected service run state, got %+v", run)
	}
	if service.startSession.OrganizationID != "org1" || service.startSession.User.ID != "user1" {
		t.Fatalf("expected auth session from gRPC request, got %+v", service.startSession)
	}
	if service.startRequest.AgentID != "agent1" ||
		service.startRequest.ConversationID != "conv1" ||
		service.startRequest.Input != "ship it" ||
		service.startRequest.MaxIterations == nil ||
		*service.startRequest.MaxIterations != 7 {
		t.Fatalf("expected StartRun request from gRPC request, got %+v", service.startRequest)
	}
}

func TestServiceRuntimeGatewayCreateRunReturnsPausedApprovalRun(t *testing.T) {
	service := &fakeAgentRuntimeService{
		startError:   internalagent.ErrToolApprovalRequired,
		listResponse: []*internalagent.Run{{ID: "run_pending", Status: internalagent.RunStatusPendingApproval}},
		detailResponse: &internalagent.RunWithMessages{
			Run: &internalagent.Run{ID: "run_pending", Status: internalagent.RunStatusPendingApproval},
		},
	}

	run, err := NewServiceRuntimeGateway(service).CreateRun(context.Background(), CreateRunInput{
		AgentID:        "agent1",
		ConversationID: "conv1",
		UserContent:    "needs a tool",
		OrganizationID: "org1",
		UserID:         "user1",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if run.RunID != "run_pending" || run.Status != internalagent.RunStatusPendingApproval {
		t.Fatalf("expected paused approval run from detail reload, got %+v", run)
	}
	if service.listConversation != "conv1" || service.detailRunID != "run_pending" {
		t.Fatalf("expected latest run detail reload, listConversation=%q detailRunID=%q", service.listConversation, service.detailRunID)
	}
}

func TestServiceRuntimeGatewayExecuteReActReturnsRunDetail(t *testing.T) {
	service := &fakeAgentRuntimeService{
		detailResponse: &internalagent.RunWithMessages{
			Run: &internalagent.Run{
				ID:             "run1",
				Status:         internalagent.RunStatusPendingApproval,
				FinalMessageID: "msg_final",
			},
			Messages: []*internalagent.Message{{
				ID:      "msg_final",
				Role:    "assistant",
				Content: "tool approval required",
			}},
			ToolRuns: []*internalagent.ToolRun{{
				ID:       "tool_run_pending",
				ToolName: "web_search",
				Arguments: map[string]any{
					"query": "release",
				},
				Status: internalagent.ToolRunStatusPendingApproval,
			}, {
				ID:       "tool_run_done",
				ToolName: "shell",
				Status:   internalagent.ToolRunStatusCompleted,
			}},
		},
	}

	state, err := NewServiceRuntimeGateway(service).ExecuteReAct(context.Background(), ExecuteReActInput{
		RunID:          "run1",
		OrganizationID: "org1",
		UserID:         "user1",
	})
	if err != nil {
		t.Fatalf("ExecuteReAct returned error: %v", err)
	}
	if service.detailSession.OrganizationID != "org1" || service.detailSession.User.ID != "user1" || service.detailRunID != "run1" {
		t.Fatalf("expected run detail session and ID, session=%+v runID=%q", service.detailSession, service.detailRunID)
	}
	if state.RunID != "run1" || state.Status != internalagent.RunStatusPendingApproval || state.Result != "tool approval required" {
		t.Fatalf("expected run detail state, got %+v", state)
	}
	if len(state.PendingToolCalls) != 1 ||
		state.PendingToolCalls[0].ID != "tool_run_pending" ||
		state.PendingToolCalls[0].Name != "web_search" ||
		state.PendingToolCalls[0].Input != `{"query":"release"}` {
		t.Fatalf("expected one pending tool call, got %+v", state.PendingToolCalls)
	}
}

func TestServiceRuntimeGatewayApproveToolCallUsesReasonAndRejectsCrossRunResult(t *testing.T) {
	service := &fakeAgentRuntimeService{
		approveResponse: &internalagent.ToolRun{
			ID:     "tool_run_1",
			RunID:  "run_other",
			Status: internalagent.ToolRunStatusCompleted,
		},
	}

	_, err := NewServiceRuntimeGateway(service).ApproveToolCall(context.Background(), ToolApprovalInput{
		RunID:          "run1",
		ToolCallID:     "tool_run_1",
		Approved:       true,
		OrganizationID: "org1",
		UserID:         "user1",
		Reason:         "operator approved",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected cross-run result to fail invalid argument, got %v", err)
	}
	if service.approveSession.OrganizationID != "org1" ||
		service.approveSession.User.ID != "user1" ||
		service.approveToolRunID != "tool_run_1" ||
		service.approveReason != "operator approved" {
		t.Fatalf("expected approve request forwarded to service, session=%+v toolRunID=%q reason=%q", service.approveSession, service.approveToolRunID, service.approveReason)
	}
}

func TestServiceRuntimeGatewayRejectToolCallUsesAgentService(t *testing.T) {
	service := &fakeAgentRuntimeService{
		rejectResponse: &internalagent.ToolRun{
			ID:     "tool_run_1",
			RunID:  "run1",
			Status: internalagent.ToolRunStatusRejected,
		},
	}

	state, err := NewServiceRuntimeGateway(service).ApproveToolCall(context.Background(), ToolApprovalInput{
		RunID:          "run1",
		ToolCallID:     "tool_run_1",
		Approved:       false,
		OrganizationID: "org1",
		UserID:         "user1",
		Reason:         "operator rejected",
	})
	if err != nil {
		t.Fatalf("ApproveToolCall returned error: %v", err)
	}
	if state.RunID != "run1" || state.ToolCallID != "tool_run_1" || state.Status != internalagent.ToolRunStatusRejected {
		t.Fatalf("expected rejected tool state, got %+v", state)
	}
	if service.rejectSession.OrganizationID != "org1" ||
		service.rejectSession.User.ID != "user1" ||
		service.rejectToolRunID != "tool_run_1" ||
		service.rejectReason != "operator rejected" {
		t.Fatalf("expected reject request forwarded to service, session=%+v toolRunID=%q reason=%q", service.rejectSession, service.rejectToolRunID, service.rejectReason)
	}
}

func TestServiceRuntimeGatewayContinuePlanUsesAgentServiceAndMapsPlanSteps(t *testing.T) {
	service := &fakeAgentRuntimeService{
		continueResponse: planningRunDetailFixture(),
	}

	state, err := NewServiceRuntimeGateway(service).ContinuePlan(context.Background(), PlanRunInput{
		RunID:          "run_plan",
		OrganizationID: "org1",
		UserID:         "user1",
	})
	if err != nil {
		t.Fatalf("ContinuePlan returned error: %v", err)
	}
	if service.continueSession.OrganizationID != "org1" ||
		service.continueSession.User.ID != "user1" ||
		service.continueRunID != "run_plan" {
		t.Fatalf("expected continue request forwarded to service, session=%+v runID=%q", service.continueSession, service.continueRunID)
	}
	if state.RunID != "run_plan" || state.Status != internalagent.RunStatusPendingApproval || state.Result != "continue stopped at next approval" {
		t.Fatalf("expected planning run state, got %+v", state)
	}
	if len(state.PlanSteps) != 2 ||
		state.PlanSteps[0].ID != "step_done" ||
		state.PlanSteps[0].Input != `{"cmd":"go test"}` ||
		len(state.PlanSteps[1].DependsOn) != 1 ||
		state.PlanSteps[1].DependsOn[0] != 1 {
		t.Fatalf("expected mapped plan steps with input and dependencies, got %+v", state.PlanSteps)
	}
}

func TestServiceRuntimeGatewayAdjustPlanUsesReasonAndMapsRunDetail(t *testing.T) {
	service := &fakeAgentRuntimeService{
		adjustResponse: planningRunDetailFixture(),
	}

	state, err := NewServiceRuntimeGateway(service).AdjustPlan(context.Background(), AdjustPlanInput{
		RunID:          "run_plan",
		OrganizationID: "org1",
		UserID:         "user1",
		Reason:         "new evidence changed the remaining work",
	})
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}
	if service.adjustSession.OrganizationID != "org1" ||
		service.adjustSession.User.ID != "user1" ||
		service.adjustRunID != "run_plan" ||
		service.adjustReason != "new evidence changed the remaining work" {
		t.Fatalf("expected adjust request forwarded to service, session=%+v runID=%q reason=%q", service.adjustSession, service.adjustRunID, service.adjustReason)
	}
	if state.RunID != "run_plan" || len(state.PlanSteps) != 2 {
		t.Fatalf("expected adjusted run detail state, got %+v", state)
	}
}

func TestServiceRuntimeGatewayPlanStepActionsUseAgentServiceAndReturnRefreshedRunDetail(t *testing.T) {
	tests := []struct {
		name   string
		action string
		call   func(RuntimeGateway, context.Context, PlanStepActionInput) (PlanStepActionState, error)
	}{
		{name: "approve", action: "approve", call: RuntimeGateway.ApprovePlanStep},
		{name: "execute", action: "execute", call: RuntimeGateway.ExecutePlanStep},
		{name: "skip", action: "skip", call: RuntimeGateway.SkipPlanStep},
		{name: "retry", action: "retry", call: RuntimeGateway.RetryPlanStep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAgentRuntimeService{
				planStepResponse: &internalagent.PlanStep{
					ID:             "step_pending",
					RunID:          "run_plan",
					Index:          2,
					Status:         internalagent.PlanStepStatusCompleted,
					ApprovalStatus: internalagent.ApprovalStatusApproved,
					ResultContent:  "step result",
				},
				detailResponse: planningRunDetailFixture(),
			}
			gateway := NewServiceRuntimeGateway(service)

			state, err := tt.call(gateway, context.Background(), PlanStepActionInput{
				RunID:          "run_plan",
				PlanStepID:     "step_pending",
				OrganizationID: "org1",
				UserID:         "user1",
				Reason:         "operator reason",
			})
			if err != nil {
				t.Fatalf("%s returned error: %v", tt.name, err)
			}
			if service.planStepAction != tt.action ||
				service.planStepSession.OrganizationID != "org1" ||
				service.planStepSession.User.ID != "user1" ||
				service.planStepID != "step_pending" {
				t.Fatalf("expected %s plan-step request forwarded, action=%q session=%+v stepID=%q", tt.name, service.planStepAction, service.planStepSession, service.planStepID)
			}
			if (tt.action == "approve" || tt.action == "skip") && service.planStepReason != "operator reason" {
				t.Fatalf("expected %s to forward reason, got %q", tt.name, service.planStepReason)
			}
			if service.detailSession.OrganizationID != "org1" || service.detailSession.User.ID != "user1" || service.detailRunID != "run_plan" {
				t.Fatalf("expected refreshed run detail request, session=%+v runID=%q", service.detailSession, service.detailRunID)
			}
			if state.RunID != "run_plan" ||
				state.PlanStepID != "step_pending" ||
				state.Status != internalagent.PlanStepStatusCompleted ||
				state.Result != "step result" {
				t.Fatalf("expected plan-step action state, got %+v", state)
			}
			if state.RunDetail.RunID != "run_plan" || len(state.RunDetail.PlanSteps) != 2 {
				t.Fatalf("expected refreshed run detail state, got %+v", state.RunDetail)
			}
		})
	}
}

func TestServiceRuntimeGatewayPlanStepActionRejectsCrossRunResult(t *testing.T) {
	service := &fakeAgentRuntimeService{
		planStepResponse: &internalagent.PlanStep{
			ID:    "step_pending",
			RunID: "run_other",
		},
		detailResponse: planningRunDetailFixture(),
	}

	_, err := NewServiceRuntimeGateway(service).ApprovePlanStep(context.Background(), PlanStepActionInput{
		RunID:          "run_plan",
		PlanStepID:     "step_pending",
		OrganizationID: "org1",
		UserID:         "user1",
		Reason:         "operator approved",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected cross-run plan step to fail invalid argument, got %v", err)
	}
	if service.detailRunID != "" {
		t.Fatalf("cross-run plan step should fail before refreshed detail lookup, got detail run %q", service.detailRunID)
	}
}

func TestServiceRuntimeGatewayPlanStepApprovalBoundaryErrorsMapToFailedPrecondition(t *testing.T) {
	service := &fakeAgentRuntimeService{
		planStepError: errors.New("plan step requires approval"),
	}

	_, err := NewServiceRuntimeGateway(service).ExecutePlanStep(context.Background(), PlanStepActionInput{
		RunID:          "run_plan",
		PlanStepID:     "step_pending",
		OrganizationID: "org1",
		UserID:         "user1",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected approval-boundary error to map to failed precondition, got %v", err)
	}
}

func planningRunDetailFixture() *internalagent.RunWithMessages {
	return &internalagent.RunWithMessages{
		Run: &internalagent.Run{
			ID:             "run_plan",
			Status:         internalagent.RunStatusPendingApproval,
			FinalMessageID: "msg_final",
		},
		Messages: []*internalagent.Message{{
			ID:      "msg_final",
			Role:    "assistant",
			Content: "continue stopped at next approval",
		}},
		PlanSteps: []*internalagent.PlanStep{{
			ID:             "step_done",
			RunID:          "run_plan",
			Index:          1,
			Title:          "Run tests",
			Status:         internalagent.PlanStepStatusCompleted,
			ApprovalStatus: internalagent.ApprovalStatusApproved,
			ToolName:       "shell",
			Input: map[string]any{
				"cmd": "go test",
			},
			ResultContent: "passed",
		}, {
			ID:             "step_pending",
			RunID:          "run_plan",
			Index:          2,
			Title:          "Deploy",
			Description:    "wait for approval",
			Status:         internalagent.PlanStepStatusPending,
			ApprovalStatus: internalagent.ApprovalStatusPending,
			DependsOn:      []int{1},
		}},
	}
}
