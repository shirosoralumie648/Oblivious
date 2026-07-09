package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	internalagent "oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	agentv1 "oblivious/server/internal/grpc/agentv1"
	"oblivious/server/internal/workflow"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestSelectAgentDatabaseURLUsesAgentServiceDatabase(t *testing.T) {
	cfg := config.Config{
		DatabaseURL: "postgres://main",
		DBMode:      "microservices",
		DBURLAgent:  "postgres://agent",
	}
	if got := selectAgentDatabaseURL(cfg); got != "postgres://agent" {
		t.Fatalf("selectAgentDatabaseURL = %q, want agent database", got)
	}
	cfg.DBMode = "monolith"
	if got := selectAgentDatabaseURL(cfg); got != "postgres://main" {
		t.Fatalf("selectAgentDatabaseURL monolith = %q, want main database", got)
	}
}

func TestAgentRelayBaseURLUsesDedicatedRuntimeURL(t *testing.T) {
	cfg := config.Config{AgentRelayBaseURL: " http://gateway.oblivious.svc.cluster.local:8080/v1/ "}
	if got := agentRelayBaseURL(cfg); got != "http://gateway.oblivious.svc.cluster.local:8080/v1" {
		t.Fatalf("agentRelayBaseURL = %q", got)
	}
	if got := agentRelayBaseURL(config.Config{}); got != "http://localhost:8080/v1" {
		t.Fatalf("default agentRelayBaseURL = %q", got)
	}
}

func TestBuildAgentGatewayProductionRelayDisabledFailsClosed(t *testing.T) {
	gateway := buildAgentGateway(agentGatewayConfig("production", false))
	messages := []chat.Message{{Role: "user", Content: "hello"}}

	reply, err := gateway.GenerateReply(context.Background(), messages, chat.ConversationConfig{})
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production agent gateway to fail closed when Relay is disabled, got reply=%q err=%v", reply, err)
	}
	if strings.Contains(reply, "Assistant reply") {
		t.Fatalf("production Relay-disabled agent gateway must not use demo text, got %q", reply)
	}

	var stream strings.Builder
	err = gateway.GenerateReplyStream(context.Background(), messages, chat.ConversationConfig{}, func(chunk string) error {
		_, _ = stream.WriteString(chunk)
		return nil
	})
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production agent stream to fail closed when Relay is disabled, got stream=%q err=%v", stream.String(), err)
	}
	if strings.Contains(stream.String(), "Assistant reply") {
		t.Fatalf("production Relay-disabled agent stream must not use demo text, got %q", stream.String())
	}
}

func TestBuildAgentGatewayDevelopmentRelayDisabledKeepsDemoFallback(t *testing.T) {
	gateway := buildAgentGateway(agentGatewayConfig("development", false))
	reply, err := gateway.GenerateReply(context.Background(), []chat.Message{{Role: "user", Content: "hello"}}, chat.ConversationConfig{})
	if err != nil {
		t.Fatalf("expected development agent gateway demo reply, got err=%v", err)
	}
	if reply != "Assistant reply: hello" {
		t.Fatalf("expected development demo reply, got %q", reply)
	}
}

func TestBuildAgentCustomPythonSandboxRunnerDisabledByDefault(t *testing.T) {
	if runner := buildAgentCustomPythonSandboxRunner(config.Config{}); runner != nil {
		t.Fatalf("expected nil runner when workflow sandbox disabled, got %#v", runner)
	}
}

func TestBuildAgentCustomPythonSandboxRunnerEnabled(t *testing.T) {
	runner := buildAgentCustomPythonSandboxRunner(config.Config{
		WorkflowSandboxEnabled:          true,
		WorkflowSandboxAllowedLanguages: "python, javascript",
		WorkflowSandboxMemoryMB:         512,
		WorkflowSandboxCPUs:             2,
		WorkflowSandboxDefaultTimeoutMS: 5000,
		WorkflowSandboxMaxTimeoutMS:     20000,
	})
	if runner == nil {
		t.Fatal("expected custom Python sandbox runner when workflow sandbox enabled")
	}
	if _, ok := runner.(agentCustomPythonSandboxRunner); !ok {
		t.Fatalf("expected agentCustomPythonSandboxRunner, got %T", runner)
	}
}

func TestAgentCustomPythonSandboxRunnerMapsWorkflowResult(t *testing.T) {
	workflowRunner := &recordingAgentWorkflowCodeRunner{
		result: &workflow.WorkflowCodeResult{
			Output: map[string]any{
				"stdout":   `{"ok":true}`,
				"stderr":   "warning",
				"exitCode": float64(3),
			},
			Logs: []string{"warning"},
			Raw:  map[string]any{"image": "python:3.12-alpine"},
		},
	}
	runner := agentCustomPythonSandboxRunner{runner: workflowRunner}

	result, err := runner.RunCustomPython(context.Background(), internalagent.CustomPythonSandboxRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		AgentID:        "agent_1",
		RunID:          "run_1",
		ToolRunID:      "tool_run_1",
		ToolCallID:     "tool_call_1",
		ToolName:       "sum_order",
		RequestID:      "req_1",
		Code:           "print('ok')",
		Inputs:         map[string]any{"x": 1},
		TimeoutMS:      1500,
	})
	if err != nil {
		t.Fatalf("RunCustomPython returned error: %v", err)
	}
	if workflowRunner.calls != 1 {
		t.Fatalf("workflow runner calls = %d, want 1", workflowRunner.calls)
	}
	if workflowRunner.request.OrganizationID != "org_1" || workflowRunner.request.UserID != "user_1" {
		t.Fatalf("workflow identity = %q/%q", workflowRunner.request.OrganizationID, workflowRunner.request.UserID)
	}
	if workflowRunner.request.AgentID != "agent_1" ||
		workflowRunner.request.RunID != "run_1" ||
		workflowRunner.request.ToolRunID != "tool_run_1" ||
		workflowRunner.request.ToolCallID != "tool_call_1" ||
		workflowRunner.request.ToolName != "sum_order" ||
		workflowRunner.request.RequestID != "req_1" {
		t.Fatalf("workflow execution context = agent:%q run:%q toolRun:%q toolCall:%q tool:%q request:%q",
			workflowRunner.request.AgentID,
			workflowRunner.request.RunID,
			workflowRunner.request.ToolRunID,
			workflowRunner.request.ToolCallID,
			workflowRunner.request.ToolName,
			workflowRunner.request.RequestID,
		)
	}
	if workflowRunner.request.Language != "python" || workflowRunner.request.Code != "print('ok')" {
		t.Fatalf("workflow language/code = %q/%q", workflowRunner.request.Language, workflowRunner.request.Code)
	}
	if workflowRunner.request.TimeoutMS != 1500 || workflowRunner.request.Inputs["x"] != 1 {
		t.Fatalf("workflow timeout/inputs = %d/%+v", workflowRunner.request.TimeoutMS, workflowRunner.request.Inputs)
	}
	if result.Stdout != `{"ok":true}` || result.Stderr != "warning" || result.ExitCode != 3 {
		t.Fatalf("mapped sandbox result = %+v", result)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "warning" || result.Raw["image"] != "python:3.12-alpine" {
		t.Fatalf("mapped logs/raw = %+v/%+v", result.Logs, result.Raw)
	}
}

func agentGatewayConfig(env string, relayEnabled bool) config.Config {
	return config.Config{
		Env:               env,
		RelayEnabled:      relayEnabled,
		RelayDefaultModel: "gpt-4o-mini",
		ModelDefaultName:  "demo-reply",
		LLMTimeoutMS:      30000,
	}
}

func TestRegisterAgentGRPCServiceServesGeneratedClient(t *testing.T) {
	runtime := &fakeAgentRuntimeService{
		runDetail: &internalagent.RunWithMessages{
			Run: &internalagent.Run{
				ID:             "run_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
				Status:         internalagent.RunStatusPendingApproval,
				Mode:           internalagent.ExecutionModePlanning,
			},
			PlanSteps: []*internalagent.PlanStep{{
				ID:             "step_1",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Verify runtime registration",
				Status:         internalagent.PlanStepStatusPending,
				ApprovalStatus: internalagent.ApprovalStatusPending,
				ToolName:       "read_file",
				Input:          map[string]any{"path": "README.md"},
				DependsOn:      []int{0},
			}},
		},
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	registerAgentGRPCService(grpcServer, runtime)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	defer conn.Close()

	client := agentv1.NewAgentServiceClient(conn)
	resp, err := client.ContinuePlan(ctx, &agentv1.PlanRunRequest{
		RunId:          "run_1",
		OrganizationId: "org_1",
		UserId:         "user_1",
	})
	if err != nil {
		t.Fatalf("ContinuePlan over generated client returned error: %v", err)
	}
	if runtime.continueRunID != "run_1" || runtime.continueSession.OrganizationID != "org_1" || runtime.continueSession.User.ID != "user_1" {
		t.Fatalf("runtime did not receive authenticated session/run: run=%q session=%+v", runtime.continueRunID, runtime.continueSession)
	}
	if resp.GetRunId() != "run_1" || resp.GetStatus() != internalagent.RunStatusPendingApproval || len(resp.GetPlanSteps()) != 1 {
		t.Fatalf("unexpected ContinuePlan response: %+v", resp)
	}
	step := resp.GetPlanSteps()[0]
	if step.GetId() != "step_1" || step.GetToolName() != "read_file" || step.GetDependsOn()[0] != 0 {
		t.Fatalf("unexpected mapped plan step: %+v", step)
	}

	budgetResp, err := client.ContinueBudget(ctx, &agentv1.ContinueBudgetRequest{
		RunId:          "run_1",
		OrganizationId: "org_1",
		UserId:         "user_1",
		TokenBudget:    5000,
	})
	if err != nil {
		t.Fatalf("ContinueBudget over generated client returned error: %v", err)
	}
	if runtime.continueBudgetRunID != "run_1" || runtime.continueBudgetValue != 5000 || runtime.continueBudgetSession.OrganizationID != "org_1" || runtime.continueBudgetSession.User.ID != "user_1" {
		t.Fatalf("runtime did not receive budget resume session/run: run=%q budget=%d session=%+v", runtime.continueBudgetRunID, runtime.continueBudgetValue, runtime.continueBudgetSession)
	}
	if budgetResp.GetRunId() != "run_1" || budgetResp.GetStatus() != internalagent.RunStatusPendingApproval || len(budgetResp.GetPlanSteps()) != 1 {
		t.Fatalf("unexpected ContinueBudget response: %+v", budgetResp)
	}

	adjustResp, err := client.AdjustPlan(ctx, &agentv1.AdjustPlanRequest{
		RunId:          "run_1",
		OrganizationId: "org_1",
		UserId:         "user_1",
		Reason:         "new production evidence",
	})
	if err != nil {
		t.Fatalf("AdjustPlan over generated client returned error: %v", err)
	}
	if runtime.adjustPlanRunID != "run_1" || runtime.adjustPlanReason != "new production evidence" || runtime.adjustPlanSession.OrganizationID != "org_1" || runtime.adjustPlanSession.User.ID != "user_1" {
		t.Fatalf("runtime did not receive adjust-plan session/run: run=%q reason=%q session=%+v", runtime.adjustPlanRunID, runtime.adjustPlanReason, runtime.adjustPlanSession)
	}
	if adjustResp.GetRunId() != "run_1" || adjustResp.GetStatus() != internalagent.RunStatusPendingApproval || len(adjustResp.GetPlanSteps()) != 1 {
		t.Fatalf("unexpected AdjustPlan response: %+v", adjustResp)
	}

	approvalResp, err := client.ApproveToolCall(ctx, &agentv1.ApproveToolCallRequest{
		RunId:          "run_1",
		ToolCallId:     "tool_run_1",
		Approved:       true,
		OrganizationId: "org_1",
		UserId:         "user_1",
		Reason:         "approved from generated client",
	})
	if err != nil {
		t.Fatalf("ApproveToolCall over generated client returned error: %v", err)
	}
	if runtime.approveToolRunID != "tool_run_1" || runtime.approveToolReason != "approved from generated client" || runtime.approveToolSession.OrganizationID != "org_1" || runtime.approveToolSession.User.ID != "user_1" {
		t.Fatalf("runtime did not receive approve-tool session/tool: tool=%q reason=%q session=%+v", runtime.approveToolRunID, runtime.approveToolReason, runtime.approveToolSession)
	}
	if approvalResp.GetRunId() != "run_1" || approvalResp.GetToolCallId() != "tool_run_1" || approvalResp.GetStatus() != internalagent.ToolRunStatusCompleted {
		t.Fatalf("unexpected ApproveToolCall response: %+v", approvalResp)
	}

	rejectResp, err := client.ApproveToolCall(ctx, &agentv1.ApproveToolCallRequest{
		RunId:          "run_1",
		ToolCallId:     "tool_run_2",
		Approved:       false,
		OrganizationId: "org_1",
		UserId:         "user_1",
		Reason:         "rejected from generated client",
	})
	if err != nil {
		t.Fatalf("RejectToolCall over generated client returned error: %v", err)
	}
	if runtime.rejectToolRunID != "tool_run_2" || runtime.rejectToolReason != "rejected from generated client" || runtime.rejectToolSession.OrganizationID != "org_1" || runtime.rejectToolSession.User.ID != "user_1" {
		t.Fatalf("runtime did not receive reject-tool session/tool: tool=%q reason=%q session=%+v", runtime.rejectToolRunID, runtime.rejectToolReason, runtime.rejectToolSession)
	}
	if rejectResp.GetRunId() != "run_1" || rejectResp.GetToolCallId() != "tool_run_2" || rejectResp.GetStatus() != internalagent.ToolRunStatusRejected {
		t.Fatalf("unexpected RejectToolCall response: %+v", rejectResp)
	}

	planStepActions := []struct {
		name       string
		reason     string
		call       func(context.Context, *agentv1.PlanStepActionRequest, ...grpc.CallOption) (*agentv1.PlanStepActionResponse, error)
		wantReason bool
	}{
		{name: "approve", reason: "approve step evidence", call: client.ApprovePlanStep, wantReason: true},
		{name: "execute", call: client.ExecutePlanStep},
		{name: "skip", reason: "not required now", call: client.SkipPlanStep, wantReason: true},
		{name: "retry", call: client.RetryPlanStep},
	}
	for _, action := range planStepActions {
		resp, err := action.call(ctx, &agentv1.PlanStepActionRequest{
			RunId:          "run_1",
			PlanStepId:     "step_1",
			OrganizationId: "org_1",
			UserId:         "user_1",
			Reason:         action.reason,
		})
		if err != nil {
			t.Fatalf("%s plan step over generated client returned error: %v", action.name, err)
		}
		record := runtime.planStepActions[action.name]
		if record.planStepID != "step_1" || record.session.OrganizationID != "org_1" || record.session.User.ID != "user_1" {
			t.Fatalf("runtime did not receive %s plan-step session/step: %+v", action.name, record)
		}
		if action.wantReason && record.reason != action.reason {
			t.Fatalf("runtime did not receive %s plan-step reason: got %q want %q", action.name, record.reason, action.reason)
		}
		if !action.wantReason && record.reason != "" {
			t.Fatalf("runtime received unexpected %s plan-step reason: %q", action.name, record.reason)
		}
		if resp.GetRunId() != "run_1" || resp.GetPlanStepId() != "step_1" || resp.GetRunDetail().GetRunId() != "run_1" || len(resp.GetRunDetail().GetPlanSteps()) != 1 {
			t.Fatalf("unexpected %s PlanStep response: %+v", action.name, resp)
		}
	}
}

type fakeAgentRuntimeService struct {
	runDetail             *internalagent.RunWithMessages
	continueSession       auth.Session
	continueRunID         string
	continueBudgetSession auth.Session
	continueBudgetRunID   string
	continueBudgetValue   int
	adjustPlanSession     auth.Session
	adjustPlanRunID       string
	adjustPlanReason      string
	approveToolSession    auth.Session
	approveToolRunID      string
	approveToolReason     string
	rejectToolSession     auth.Session
	rejectToolRunID       string
	rejectToolReason      string
	planStepActions       map[string]fakePlanStepAction
}

type fakePlanStepAction struct {
	session    auth.Session
	planStepID string
	reason     string
}

type recordingAgentWorkflowCodeRunner struct {
	calls   int
	request workflow.WorkflowCodeRequest
	result  *workflow.WorkflowCodeResult
	err     error
}

func (r *recordingAgentWorkflowCodeRunner) RunWorkflowCode(ctx context.Context, req workflow.WorkflowCodeRequest) (*workflow.WorkflowCodeResult, error) {
	r.calls++
	r.request = req
	return r.result, r.err
}

func (f *fakeAgentRuntimeService) StartRun(context.Context, auth.Session, internalagent.StartRunRequest) (*internalagent.RunWithMessages, error) {
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) ListRuns(context.Context, auth.Session, string) ([]*internalagent.Run, error) {
	return nil, nil
}

func (f *fakeAgentRuntimeService) GetRunWithMessages(context.Context, auth.Session, string) (*internalagent.RunWithMessages, error) {
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) ContinueRunWithTokenBudget(_ context.Context, session auth.Session, runID string, tokenBudget int) (*internalagent.RunResult, error) {
	f.continueBudgetSession = session
	f.continueBudgetRunID = runID
	f.continueBudgetValue = tokenBudget
	return &internalagent.RunResult{}, nil
}

func (f *fakeAgentRuntimeService) ApproveToolRun(_ context.Context, session auth.Session, toolRunID, reason string) (*internalagent.ToolRun, error) {
	f.approveToolSession = session
	f.approveToolRunID = toolRunID
	f.approveToolReason = reason
	return &internalagent.ToolRun{ID: toolRunID, RunID: f.runDetail.Run.ID, Status: internalagent.ToolRunStatusCompleted}, nil
}

func (f *fakeAgentRuntimeService) RejectToolRun(_ context.Context, session auth.Session, toolRunID, reason string) (*internalagent.ToolRun, error) {
	f.rejectToolSession = session
	f.rejectToolRunID = toolRunID
	f.rejectToolReason = reason
	return &internalagent.ToolRun{ID: toolRunID, RunID: f.runDetail.Run.ID, Status: internalagent.ToolRunStatusRejected}, nil
}

func (f *fakeAgentRuntimeService) ContinuePlanningRun(_ context.Context, session auth.Session, runID string) (*internalagent.RunWithMessages, error) {
	f.continueSession = session
	f.continueRunID = runID
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) AdjustPlanSteps(_ context.Context, session auth.Session, runID, reason string) (*internalagent.RunWithMessages, error) {
	f.adjustPlanSession = session
	f.adjustPlanRunID = runID
	f.adjustPlanReason = reason
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) ApprovePlanStep(_ context.Context, session auth.Session, planStepID, reason string) (*internalagent.PlanStep, error) {
	f.recordPlanStepAction("approve", session, planStepID, reason)
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) ExecutePlanStep(_ context.Context, session auth.Session, planStepID string) (*internalagent.PlanStep, error) {
	f.recordPlanStepAction("execute", session, planStepID, "")
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) SkipPlanStep(_ context.Context, session auth.Session, planStepID, reason string) (*internalagent.PlanStep, error) {
	f.recordPlanStepAction("skip", session, planStepID, reason)
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) RetryPlanStep(_ context.Context, session auth.Session, planStepID string) (*internalagent.PlanStep, error) {
	f.recordPlanStepAction("retry", session, planStepID, "")
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) recordPlanStepAction(action string, session auth.Session, planStepID, reason string) {
	if f.planStepActions == nil {
		f.planStepActions = make(map[string]fakePlanStepAction)
	}
	f.planStepActions[action] = fakePlanStepAction{session: session, planStepID: planStepID, reason: reason}
}
