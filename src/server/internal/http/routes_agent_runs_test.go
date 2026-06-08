package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/agent"
)

func TestRegisterAgentRunRoutesDispatchesCreate(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Research Agent",
		Model:          "test-model",
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "done"}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", `{"agent_id":"agent_1","conversation_id":"conv_1","input":"summarize"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected completed run response, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesApproveTool(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Model:          "test-model",
		Tools:          []agent.Tool{{Name: "datetime", Type: "builtin", Enabled: true, RequiresApproval: true}},
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_pending",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime_pending",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool", `{"toolRunId":"tool_run_pending","reason":"ok"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"approvalStatus":"approved"`) {
		t.Fatalf("expected approved tool response, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected completed run detail, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesRejectTool(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_pending",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime_pending",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/reject-tool", `{"toolRunId":"tool_run_pending","reason":"unsafe command"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"approvalStatus":"rejected"`) {
		t.Fatalf("expected rejected tool response, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"failed"`) {
		t.Fatalf("expected failed run detail after rejected tool, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesRetryTool(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Model:          "test-model",
		Tools:          []agent.Tool{{Name: "datetime", Type: "builtin", Enabled: true}},
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusFailed,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_failed",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime_failed",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusFailed,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		AttemptCount:   1,
		Error:          "failed",
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-tool", `{"tool_run_id":"tool_run_failed"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected retry tool response, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesApprovePlanStep(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusCompleted,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Gather requirements",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-plan-step", `{"planStepId":"step_1","reason":"ok"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"planSteps"`) || !strings.Contains(recorder.Body.String(), `"status":"approved"`) {
		t.Fatalf("expected approved plan step response, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesUpdatePlanStep(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusRunning,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Original step",
		Status:         agent.PlanStepStatusApproved,
		ApprovalStatus: agent.ApprovalStatusApproved,
		ToolName:       "write_file",
		Input:          map[string]any{"path": "old.go"},
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPatch, "/api/v1/agent/runs/run_1/update-plan-step", `{"planStepId":"step_1","title":"Read safer file","toolName":"read_file","input":{"path":"new.go"}}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"title":"Read safer file"`) || !strings.Contains(recorder.Body.String(), `"toolName":"read_file"`) {
		t.Fatalf("expected updated plan step response, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"approvalStatus":"pending"`) {
		t.Fatalf("expected updated approved step to require fresh review, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesExecutePlanStep(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Implementation Agent",
		Model:          "test-model",
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusCompleted,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Run implementation",
		Status:         agent.PlanStepStatusApproved,
		ApprovalStatus: agent.ApprovalStatusApproved,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_1",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "implement the approved step",
		CreatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "step executed"}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/execute-plan-step", `{"planStepId":"step_1"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"resultContent":"step executed"`) {
		t.Fatalf("expected executed plan step response, got %s", recorder.Body.String())
	}
}

func TestRegisterAgentRunRoutesDispatchesListTools(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Tools: []agent.Tool{{
			Name:        "custom_tool",
			Type:        "mcp",
			Description: "Custom MCP tool",
			Enabled:     true,
		}},
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentRunRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := newAgentRunsRequest(stdhttp.MethodGet, "/api/v1/agent/tools?agentId=agent_1", "")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"name":"custom_tool"`) {
		t.Fatalf("expected tools response, got %s", recorder.Body.String())
	}
}
