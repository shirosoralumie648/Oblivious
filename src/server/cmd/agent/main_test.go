package main

import (
	"context"
	"net"
	"testing"
	"time"

	internalagent "oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/config"
	agentv1 "oblivious/server/internal/grpc/agentv1"

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
}

type fakeAgentRuntimeService struct {
	runDetail       *internalagent.RunWithMessages
	continueSession auth.Session
	continueRunID   string
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

func (f *fakeAgentRuntimeService) ApproveToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error) {
	return nil, nil
}

func (f *fakeAgentRuntimeService) RejectToolRun(context.Context, auth.Session, string, string) (*internalagent.ToolRun, error) {
	return nil, nil
}

func (f *fakeAgentRuntimeService) ContinuePlanningRun(_ context.Context, session auth.Session, runID string) (*internalagent.RunWithMessages, error) {
	f.continueSession = session
	f.continueRunID = runID
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) AdjustPlanSteps(context.Context, auth.Session, string, string) (*internalagent.RunWithMessages, error) {
	return f.runDetail, nil
}

func (f *fakeAgentRuntimeService) ApprovePlanStep(context.Context, auth.Session, string, string) (*internalagent.PlanStep, error) {
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) ExecutePlanStep(context.Context, auth.Session, string) (*internalagent.PlanStep, error) {
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) SkipPlanStep(context.Context, auth.Session, string, string) (*internalagent.PlanStep, error) {
	return f.runDetail.PlanSteps[0], nil
}

func (f *fakeAgentRuntimeService) RetryPlanStep(context.Context, auth.Session, string) (*internalagent.PlanStep, error) {
	return f.runDetail.PlanSteps[0], nil
}
