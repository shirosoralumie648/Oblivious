package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

func TestAgentRunsHandlerCreateRunStartsReactRunAndReturnsMessages(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Research Agent",
		Model:          "test-model",
		Tools:          []agent.Tool{{Name: "search", Type: "builtin", Enabled: true}},
		Config:         agent.Config{MaxIterations: 3, TokenBudget: 2_000},
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "done"}))

	recorder := httptest.NewRecorder()
	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", `{
		"agent_id":"agent_1",
		"conversation_id":"conv_1",
		"input":"summarize this",
		"mode":"react",
		"token_budget": 42,
		"max_iterations": 250
	}`)
	handler.createRun(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run.ID == "" || response.Data.Run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected completed run in response, got %+v", response.Data.Run)
	}
	if response.Data.Run.Mode != agent.ExecutionModeReact {
		t.Fatalf("expected react run mode in response, got %+v", response.Data.Run)
	}
	if response.Data.Status != agent.RunStatusCompleted || response.Data.ID != response.Data.Run.ID {
		t.Fatalf("expected facade id/status mirrors, got %+v", response.Data)
	}
	if len(response.Data.Messages) != 2 || response.Data.Messages[0].Role != "user" || response.Data.Messages[1].Content != "done" {
		t.Fatalf("expected user and assistant messages, got %+v", response.Data.Messages)
	}
	if store.agent.Config.MaxIterations != 3 {
		t.Fatalf("expected run-scoped max_iterations override not to mutate agent config, got %d", store.agent.Config.MaxIterations)
	}
	if store.agent.Config.TokenBudget != 2_000 {
		t.Fatalf("expected run-scoped token_budget override not to mutate agent config, got %d", store.agent.Config.TokenBudget)
	}
}

func TestAgentRunsHandlerGetRunReturnsDetailAndMessages(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusCompleted,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_1",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ToolName:       "search",
		Status:         agent.ToolRunStatusCompleted,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_1",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "assistant",
		Content:        "done",
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.getRun(recorder, newAgentRunsRequest(stdhttp.MethodGet, "/api/v1/agent/runs/run_1", ""), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run.ID != "run_1" || len(response.Data.ToolRuns) != 1 || len(response.Data.Messages) != 1 {
		t.Fatalf("expected run detail with tool run and message, got %+v", response.Data)
	}
}

func TestAgentRunsHandlerCreateRunStartsPlanningRun(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Planning Agent",
		Model:          "test-model",
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "1. Gather requirements\n2. Draft implementation plan"}))

	recorder := httptest.NewRecorder()
	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", `{
		"agent_id":"agent_1",
		"conversation_id":"conv_1",
		"input":"make a plan",
		"mode":"planning"
	}`)
	handler.createRun(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusPendingApproval {
		t.Fatalf("expected planning run to wait for plan step execution, got %+v", response.Data.Run)
	}
	if response.Data.Run.Mode != agent.ExecutionModePlanning {
		t.Fatalf("expected planning run mode in response, got %+v", response.Data.Run)
	}
	if len(response.Data.Messages) != 2 {
		t.Fatalf("expected user and planning messages, got %+v", response.Data.Messages)
	}
	if response.Data.Messages[0].Role != "user" || response.Data.Messages[0].Content != "make a plan" {
		t.Fatalf("expected original user request to be persisted, got %+v", response.Data.Messages[0])
	}
	if response.Data.Messages[1].Role != "assistant" || !strings.Contains(response.Data.Messages[1].Content, "Draft implementation plan") {
		t.Fatalf("expected assistant planning response, got %+v", response.Data.Messages[1])
	}
}

func TestAgentRunsHandlerCreateRunUsesAgentDefaultExecutionModePlanning(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Planning Default Agent",
		Model:          "test-model",
		Config: agent.Config{
			DefaultExecutionMode: agent.ExecutionModePlanning,
		},
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "1. Gather context\n2. Execute changes"}))

	recorder := httptest.NewRecorder()
	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", `{
		"agent_id":"agent_1",
		"conversation_id":"conv_1",
		"input":"plan by default"
	}`)
	handler.createRun(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.PlanSteps) != 2 {
		t.Fatalf("expected planning run to create plan steps from default mode, got %+v", response.Data)
	}
	if len(response.Data.Messages) != 2 || !strings.Contains(response.Data.Messages[1].Content, "Execute changes") {
		t.Fatalf("expected planning messages from default mode, got %+v", response.Data.Messages)
	}
}

func TestAgentRunsHandlerCreateRunRejectsExplicitBlankExecutionMode(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "blank string",
			body: `{
				"agent_id":"agent_1",
				"conversation_id":"conv_1",
				"input":"do not default",
				"mode":" "
			}`,
		},
		{
			name: "null",
			body: `{
				"agent_id":"agent_1",
				"conversation_id":"conv_1",
				"input":"do not default",
				"mode":null
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAgentRunsStore()
			store.agent = &agent.Agent{
				ID:             "agent_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
				Name:           "Planning Default Agent",
				Model:          "test-model",
				Config: agent.Config{
					DefaultExecutionMode: agent.ExecutionModePlanning,
				},
			}
			store.conversation = &agent.Conversation{
				ID:             "conv_1",
				AgentID:        "agent_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
			}
			handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "should not run"}))

			recorder := httptest.NewRecorder()
			handler.createRun(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", tt.body))

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "mode must be react or planning") {
				t.Fatalf("expected mode validation error, got %s", recorder.Body.String())
			}
			if len(store.runs) != 0 || len(store.planSteps) != 0 || len(store.messages) != 0 {
				t.Fatalf("expected explicit blank mode to stop before starting run, got runs=%+v planSteps=%+v messages=%+v", store.runs, store.planSteps, store.messages)
			}
		})
	}
}

func TestAgentRunsHandlerCreateRunRequestModeOverridesAgentDefaultExecutionMode(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Planning Default Agent",
		Model:          "test-model",
		Config: agent.Config{
			DefaultExecutionMode: agent.ExecutionModePlanning,
		},
	}
	store.conversation = &agent.Conversation{
		ID:             "conv_1",
		AgentID:        "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "react answer"}))

	recorder := httptest.NewRecorder()
	request := newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs", `{
		"agent_id":"agent_1",
		"conversation_id":"conv_1",
		"input":"answer directly",
		"mode":"react"
	}`)
	handler.createRun(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.PlanSteps) != 0 {
		t.Fatalf("expected explicit react mode to skip planning steps, got %+v", response.Data.PlanSteps)
	}
	if len(response.Data.Messages) != 2 || response.Data.Messages[1].Content != "react answer" {
		t.Fatalf("expected react response despite planning default, got %+v", response.Data.Messages)
	}
}

func TestAgentRunsHandlerApprovePlanStepReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
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
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approvePlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-plan-step", `{"planStepId":"step_1","reason":"ready"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.ID != "run_1" {
		t.Fatalf("expected run detail, got %+v", response.Data.Run)
	}
	if len(response.Data.PlanSteps) != 1 {
		t.Fatalf("expected plan step detail, got %+v", response.Data)
	}
	step := response.Data.PlanSteps[0]
	if step.ID != "step_1" || step.Status != agent.PlanStepStatusApproved || step.ApprovalStatus != agent.ApprovalStatusApproved {
		t.Fatalf("expected approved plan step, got %+v", step)
	}
}

func TestAgentRunsHandlerAdjustPlanReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(-time.Minute)
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Planning Agent",
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
		Mode:           agent.ExecutionModePlanning,
		Status:         agent.RunStatusFailed,
		Error:          "old failure",
		CompletedAt:    &completedAt,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_1",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "adjust this plan",
		CreatedAt:      now.Add(-2 * time.Minute),
	}}
	store.planSteps = []*agent.PlanStep{
		{
			ID:             "step_done",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Gather evidence",
			Status:         agent.PlanStepStatusCompleted,
			ApprovalStatus: agent.ApprovalStatusNotRequired,
			ResultContent:  "evidence collected",
			CompletedAt:    &completedAt,
			CreatedAt:      now.Add(-3 * time.Minute),
			UpdatedAt:      now.Add(-3 * time.Minute),
		},
		{
			ID:             "step_pending",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Old remaining step",
			Status:         agent.PlanStepStatusPending,
			ApprovalStatus: agent.ApprovalStatusPending,
			CreatedAt:      now.Add(-2 * time.Minute),
			UpdatedAt:      now.Add(-2 * time.Minute),
		},
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: `[{"title":"Adjusted implementation"},{"title":"Adjusted verification"}]`}))

	recorder := httptest.NewRecorder()
	handler.adjustPlan(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/adjust-plan", `{"reason":"new result changed next steps"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusPendingApproval || response.Data.Run.Error != "" || response.Data.Run.CompletedAt != nil {
		t.Fatalf("expected reopened run detail, got %+v", response.Data.Run)
	}
	if response.Data.Run.FinalMessageID == "" {
		t.Fatalf("expected adjusted planning reply final message, got %+v", response.Data.Run)
	}
	if len(response.Data.Messages) != 2 {
		t.Fatalf("expected refreshed messages to include adjusted planning reply, got %+v", response.Data.Messages)
	}
	adjustedReply := response.Data.Messages[1]
	if adjustedReply.ID != response.Data.Run.FinalMessageID || adjustedReply.Role != "assistant" || adjustedReply.Content != `[{"title":"Adjusted implementation"},{"title":"Adjusted verification"}]` {
		t.Fatalf("expected persisted adjusted assistant reply, run=%+v messages=%+v", response.Data.Run, response.Data.Messages)
	}
	if len(response.Data.PlanSteps) != 3 {
		t.Fatalf("expected preserved prefix plus adjusted suffix, got %+v", response.Data.PlanSteps)
	}
	if response.Data.PlanSteps[0].ID != "step_done" || response.Data.PlanSteps[0].ResultContent != "evidence collected" {
		t.Fatalf("expected completed prefix to remain, got %+v", response.Data.PlanSteps[0])
	}
	if response.Data.PlanSteps[1].Title != "Adjusted implementation" || response.Data.PlanSteps[1].Index != 2 {
		t.Fatalf("expected adjusted first remaining step, got %+v", response.Data.PlanSteps[1])
	}
	if response.Data.PlanSteps[2].Title != "Adjusted verification" || response.Data.PlanSteps[2].Index != 3 {
		t.Fatalf("expected adjusted second remaining step, got %+v", response.Data.PlanSteps[2])
	}
}

func TestAgentRunsHandlerUpdatePlanStepReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
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
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.updatePlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPatch, "/api/v1/agent/runs/run_1/update-plan-step", `{"planStepId":"step_1","title":"Read safer file","toolName":"read_file","input":{"path":"new.go"}}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.PlanSteps) != 1 {
		t.Fatalf("expected plan step detail, got %+v", response.Data)
	}
	step := response.Data.PlanSteps[0]
	if step.Title != "Read safer file" || step.ToolName != "read_file" || step.Input["path"] != "new.go" {
		t.Fatalf("expected edited plan step payload, got %+v", step)
	}
	if step.Status != agent.PlanStepStatusPending || step.ApprovalStatus != agent.ApprovalStatusPending {
		t.Fatalf("expected edited approved step to require fresh review, got %+v", step)
	}
}

func TestAgentRunsHandlerExecutePlanStepAcceptsSnakeCaseID(t *testing.T) {
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
		CreatedAt:      now,
		UpdatedAt:      now,
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

	recorder := httptest.NewRecorder()
	handler.executePlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/execute-plan-step", `{"plan_step_id":"step_1"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.PlanSteps) != 1 {
		t.Fatalf("expected plan step detail, got %+v", response.Data)
	}
	step := response.Data.PlanSteps[0]
	if step.ID != "step_1" || step.Status != agent.PlanStepStatusCompleted || step.ResultContent != "step executed" {
		t.Fatalf("expected completed plan step result, got %+v", step)
	}
}

func TestAgentRunsHandlerApprovePlanStepVerifiesStepBelongsToRun(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{
		{ID: "run_1", OrganizationID: "org_1", UserID: "user_1"},
		{ID: "run_2", OrganizationID: "org_1", UserID: "user_1"},
	}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_other",
		RunID:          "run_2",
		OrganizationID: "org_1",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approvePlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-plan-step", `{"planStepId":"step_other"}`), "run_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "planStepId does not belong to run") {
		t.Fatalf("expected membership error, got %s", recorder.Body.String())
	}
}

func TestAgentRunsHandlerPlanStepActionsRejectExplicitNonMatchingStepIDs(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	tests := []struct {
		name string
		call func(agentRunsHandler, stdhttp.ResponseWriter, *stdhttp.Request, string)
		body string
	}{
		{
			name: "approve completed",
			call: agentRunsHandler.approvePlanStep,
			body: `{"planStepId":"step_completed","reason":"late approval"}`,
		},
		{
			name: "skip completed",
			call: agentRunsHandler.skipPlanStep,
			body: `{"planStepId":"step_completed","reason":"late skip"}`,
		},
		{
			name: "retry completed",
			call: agentRunsHandler.retryPlanStep,
			body: `{"planStepId":"step_completed"}`,
		},
		{
			name: "update completed",
			call: agentRunsHandler.updatePlanStep,
			body: `{"planStepId":"step_completed","title":"mutated title"}`,
		},
		{
			name: "delete completed",
			call: agentRunsHandler.deletePlanStep,
			body: `{"planStepId":"step_completed"}`,
		},
		{
			name: "execute completed",
			call: agentRunsHandler.executePlanStep,
			body: `{"planStepId":"step_completed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAgentRunsStore()
			store.runs = []*agent.Run{{
				ID:             "run_1",
				OrganizationID: "org_1",
				ConversationID: "conv_1",
				AgentID:        "agent_1",
				UserID:         "user_1",
				Status:         agent.RunStatusCompleted,
				CreatedAt:      now,
				UpdatedAt:      now,
			}}
			store.planSteps = []*agent.PlanStep{{
				ID:             "step_completed",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Already completed",
				Status:         agent.PlanStepStatusCompleted,
				ApprovalStatus: agent.ApprovalStatusApproved,
				ResultContent:  "verified output",
				Error:          "",
				CompletedAt:    &completedAt,
				CreatedAt:      now,
				UpdatedAt:      now,
			}}
			handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "should not execute"}))

			recorder := httptest.NewRecorder()
			tt.call(handler, recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/plan-step-action", tt.body), "run_1")

			if recorder.Code != stdhttp.StatusConflict {
				t.Fatalf("expected 409 for explicit non-matching plan step, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "planStepId is not valid for this action") {
				t.Fatalf("expected explicit plan-step state gate error, got %s", recorder.Body.String())
			}
			if len(store.planSteps) != 1 {
				t.Fatalf("explicit non-matching plan-step action deleted guarded step: %+v", store.planSteps)
			}
			step := store.planSteps[0]
			if step.Status != agent.PlanStepStatusCompleted || step.ApprovalStatus != agent.ApprovalStatusApproved ||
				step.Title != "Already completed" || step.ResultContent != "verified output" || step.CompletedAt != &completedAt {
				t.Fatalf("explicit non-matching plan-step action mutated guarded evidence: %+v", step)
			}
		})
	}
}

func TestAgentRunsHandlerSkipPlanStepReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Optional discovery",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, {
		ID:             "step_2",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          2,
		Title:          "Verify patch",
		Status:         agent.PlanStepStatusCompleted,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.skipPlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/skip-plan-step", `{"plan_step_id":"step_1","reason":"not required"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected run to complete after skipped/completed steps, got %+v", response.Data.Run)
	}
	if len(response.Data.PlanSteps) != 2 {
		t.Fatalf("expected plan step detail, got %+v", response.Data)
	}
	step := response.Data.PlanSteps[0]
	if step.ID != "step_1" || step.Status != agent.PlanStepStatusSkipped || step.Error != "not required" || step.CompletedAt == nil {
		t.Fatalf("expected skipped plan step with reason, got %+v", step)
	}
}

func TestAgentRunsHandlerSkipPlanStepRejectsOutOfOrderWithoutClearingEvidence(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Still pending",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, {
		ID:             "step_2",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          2,
		Title:          "Future optional cleanup",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.skipPlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/skip-plan-step", `{"plan_step_id":"step_2","reason":"not required"}`), "run_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "prior plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected prior-step error body, got %s", recorder.Body.String())
	}
	if store.runs[0].Status != agent.RunStatusPendingApproval || store.runs[0].CompletedAt != nil {
		t.Fatalf("expected rejected HTTP skip to preserve run state, got %+v", store.runs[0])
	}
	step := store.planSteps[1]
	if step.Status != agent.PlanStepStatusPending || step.Error != "" || step.CompletedAt != nil {
		t.Fatalf("expected rejected HTTP skip to preserve future step evidence, got %+v", step)
	}
}

func TestAgentRunsHandlerRetryPlanStepReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
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
		Status:         agent.RunStatusTokenBudgetExceeded,
		Error:          "token_budget_exceeded: old budget",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Retry implementation",
		Status:         agent.PlanStepStatusFailed,
		ApprovalStatus: agent.ApprovalStatusApproved,
		ResultContent:  "stale output",
		Error:          "old failure",
		StartedAt:      &now,
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_1",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "retry the failed step",
		CreatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "retry passed"}))

	recorder := httptest.NewRecorder()
	handler.retryPlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-plan-step", `{"plan_step_id":"step_1"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusCompleted || response.Data.Run.Error != "" || response.Data.Run.CompletedAt == nil {
		t.Fatalf("expected retried run to complete cleanly, got %+v", response.Data.Run)
	}
	if len(response.Data.PlanSteps) != 1 {
		t.Fatalf("expected plan step detail, got %+v", response.Data)
	}
	step := response.Data.PlanSteps[0]
	if step.ID != "step_1" || step.Status != agent.PlanStepStatusCompleted || step.ResultContent != "retry passed" || step.Error != "" {
		t.Fatalf("expected retried plan step result, got %+v", step)
	}
}

func TestAgentRunsHandlerRetryPlanStepRejectsOutOfOrderWithoutClearingFailure(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
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
		Status:         agent.RunStatusFailed,
		Error:          "step 2 failed",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Still pending",
		Status:         agent.PlanStepStatusPending,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, {
		ID:             "step_2",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          2,
		Title:          "Failed retry target",
		Status:         agent.PlanStepStatusFailed,
		ApprovalStatus: agent.ApprovalStatusApproved,
		ResultContent:  "partial output",
		Error:          "old failure",
		StartedAt:      &now,
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "should not run"}))

	recorder := httptest.NewRecorder()
	handler.retryPlanStep(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-plan-step", `{"plan_step_id":"step_2"}`), "run_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "prior plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected prior-step error body, got %s", recorder.Body.String())
	}
	if store.runs[0].Status != agent.RunStatusFailed || store.runs[0].Error != "step 2 failed" || store.runs[0].CompletedAt != &completedAt {
		t.Fatalf("expected rejected HTTP retry to preserve run failure evidence, got %+v", store.runs[0])
	}
	step := store.planSteps[1]
	if step.Status != agent.PlanStepStatusFailed || step.ApprovalStatus != agent.ApprovalStatusApproved ||
		step.ResultContent != "partial output" || step.Error != "old failure" || step.CompletedAt != &completedAt {
		t.Fatalf("expected rejected HTTP retry to preserve failed step evidence, got %+v", step)
	}
}

func TestAgentRunsHandlerContinueBudgetReturnsUpdatedRunDetail(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Implementation Agent",
		Model:          "test-model",
		Config:         agent.Config{TokenBudget: 1000},
		Tools:          []agent.Tool{{Name: "datetime", Type: "builtin", Enabled: true}},
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
		Status:         agent.RunStatusTokenBudgetExceeded,
		IterationCount: 1,
		ToolCallCount:  1,
		Error:          "token_budget_exceeded: used 1200 tokens exceeds budget 1000",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_user",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "continue this run",
		CreatedAt:      now,
	}, {
		ID:             "msg_tool",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "tool",
		Content:        "Current time: noon",
		ToolCallID:     "call_datetime",
		CreatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{structured: []*chat.CompletionResponse{
		{Content: "continued after budget increase", Usage: &chat.CompletionUsage{TotalTokens: 1200}},
	}}))

	recorder := httptest.NewRecorder()
	handler.continueBudget(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget", `{"token_budget":2500}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusCompleted || response.Data.Run.Error != "" || response.Data.Run.CompletedAt == nil {
		t.Fatalf("expected continued run to complete cleanly, got %+v", response.Data.Run)
	}
	if len(response.Data.Messages) == 0 || response.Data.Messages[len(response.Data.Messages)-1].Content != "continued after budget increase" {
		t.Fatalf("expected final assistant message in response, got %+v", response.Data.Messages)
	}
	if store.agent.Config.TokenBudget != 1000 {
		t.Fatalf("continue budget should not mutate agent config, got %d", store.agent.Config.TokenBudget)
	}
}

func TestAgentRunsHandlerContinueBudgetRetriesPlanningStepAndReturnsRunDetail(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	largeContext := strings.Repeat("tenant migration risk ", 900)
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Planning Agent",
		Model:          "test-model",
		Config:         agent.Config{TokenBudget: 1000},
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Mode:           agent.ExecutionModePlanning,
		Status:         agent.RunStatusTokenBudgetExceeded,
		IterationCount: 1,
		Error:          "token_budget_exceeded: estimated 1800 prompt tokens exceeds budget 1000",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_user",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        largeContext,
		CreatedAt:      now,
	}}
	store.planSteps = []*agent.PlanStep{{
		ID:             "step_1",
		RunID:          "run_1",
		OrganizationID: "org_1",
		Index:          1,
		Title:          "Summarize oversized context",
		Status:         agent.PlanStepStatusFailed,
		ApprovalStatus: agent.ApprovalStatusApproved,
		ResultContent:  "stale oversized result",
		Error:          "token_budget_exceeded: estimated 1800 prompt tokens exceeds budget 1000",
		StartedAt:      &now,
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{reply: "Completed plan step after budget increase."}))

	recorder := httptest.NewRecorder()
	handler.continueBudget(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget", `{"tokenBudget":100000}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusCompleted || response.Data.Run.Error != "" || response.Data.Run.CompletedAt == nil {
		t.Fatalf("expected planning continuation to complete run detail, got %+v", response.Data.Run)
	}
	if len(response.Data.PlanSteps) != 1 {
		t.Fatalf("expected one plan step in response, got %+v", response.Data.PlanSteps)
	}
	step := response.Data.PlanSteps[0]
	if step.ID != "step_1" || step.Status != agent.PlanStepStatusCompleted || step.ResultContent != "Completed plan step after budget increase." || step.Error != "" {
		t.Fatalf("expected completed planning step detail, got %+v", step)
	}
	if store.agent.Config.TokenBudget != 1000 {
		t.Fatalf("continue budget should not mutate agent config, got %d", store.agent.Config.TokenBudget)
	}
}

func TestAgentRunsHandlerContinueBudgetRejectsOutOfRangeBudget(t *testing.T) {
	handler := newAgentRunsHandler(agent.NewService(newFakeAgentRunsStore(), &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.continueBudget(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget", `{"tokenBudget":1000001}`), "run_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "tokenBudget must be between 1000 and 1000000") {
		t.Fatalf("expected budget range error, got %s", recorder.Body.String())
	}
}

func TestAgentRunsHandlerContinueBudgetReturnsConflictWhenBudgetStillExceeded(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Implementation Agent",
		Model:          "test-model",
		Config:         agent.Config{TokenBudget: 1000},
		Tools:          []agent.Tool{{Name: "datetime", Type: "builtin", Enabled: true}},
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
		Status:         agent.RunStatusTokenBudgetExceeded,
		IterationCount: 1,
		ToolCallCount:  1,
		Error:          "token_budget_exceeded: used 1200 tokens exceeds budget 1000",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_user",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "continue this run",
		CreatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{structured: []*chat.CompletionResponse{
		{Content: "still too large", Usage: &chat.CompletionUsage{TotalTokens: 3000}},
	}}))

	recorder := httptest.NewRecorder()
	handler.continueBudget(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget", `{"tokenBudget":2500}`), "run_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "token budget exceeded") {
		t.Fatalf("expected token budget exceeded error, got %s", recorder.Body.String())
	}
	if store.runs[0].Status != agent.RunStatusTokenBudgetExceeded || !strings.Contains(store.runs[0].Error, "token_budget_exceeded") {
		t.Fatalf("expected run to remain token_budget_exceeded, got %+v", store.runs[0])
	}
}

func TestAgentRunsHandlerContinueBudgetReturnsPendingApprovalDetail(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Implementation Agent",
		Model:          "test-model",
		Config:         agent.Config{TokenBudget: 1000},
		Tools:          []agent.Tool{{Name: "write_file", Type: "builtin", Enabled: true, RequiresApproval: true}},
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusTokenBudgetExceeded,
		IterationCount: 1,
		ToolCallCount:  1,
		Error:          "token_budget_exceeded: used 1200 tokens exceeds budget 1000",
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.messages = []*agent.Message{{
		ID:             "msg_user",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "user",
		Content:        "continue and write the result",
		CreatedAt:      now,
	}, {
		ID:             "msg_tool",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "tool",
		Content:        "Result ready",
		ToolCallID:     "call_previous",
		CreatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{structured: []*chat.CompletionResponse{
		{
			ToolCalls: []chat.ToolCall{
				{ID: "call_write_file", Type: "function", Function: chat.ToolFunction{Name: "write_file", Arguments: `{"path":"result.md","content":"ready"}`}},
			},
			FinishReason: "tool_calls",
			Usage:        &chat.CompletionUsage{TotalTokens: 300},
		},
	}}))

	recorder := httptest.NewRecorder()
	handler.continueBudget(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget", `{"tokenBudget":2500}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusPendingApproval || response.Data.Run.Error != "" || response.Data.Run.CompletedAt != nil {
		t.Fatalf("expected pending approval run detail, got %+v", response.Data.Run)
	}
	if len(response.Data.ToolRuns) != 1 {
		t.Fatalf("expected one pending tool run in response, got %+v", response.Data.ToolRuns)
	}
	toolRun := response.Data.ToolRuns[0]
	if toolRun.ToolName != "write_file" || toolRun.Status != agent.ToolRunStatusPendingApproval || toolRun.ApprovalStatus != agent.ApprovalStatusPending || toolRun.AttemptCount != 0 {
		t.Fatalf("expected write_file approval detail, got %+v", toolRun)
	}
	if len(response.Data.Messages) == 0 || len(response.Data.Messages[len(response.Data.Messages)-1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool-call message in response, got %+v", response.Data.Messages)
	}
}

func TestAgentRunsHandlerApproveToolSelectsOnlyPendingApprovalWhenOmitted(t *testing.T) {
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
	store.toolRuns = []*agent.ToolRun{
		{
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
		},
		{
			ID:             "tool_run_completed",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ToolName:       "datetime",
			Status:         agent.ToolRunStatusCompleted,
			ApprovalStatus: agent.ApprovalStatusNotRequired,
		},
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approveTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool", `{"reason":"reviewed"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.ToolRuns) == 0 {
		t.Fatalf("expected tool run detail, got %+v", response.Data)
	}
	toolRun := response.Data.ToolRuns[0]
	if toolRun.ID != "tool_run_pending" || toolRun.Status != agent.ToolRunStatusCompleted || toolRun.ApprovalStatus != agent.ApprovalStatusApproved || toolRun.ApprovalDecisionReason != "reviewed" {
		t.Fatalf("expected pending tool run approved and completed, got %+v", toolRun)
	}
}

func TestAgentRunsHandlerRejectToolReturnsFailedRunDetail(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_pending",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{"timezone": "UTC"},
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.rejectTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/reject-tool", `{"toolRunId":"tool_run_pending","reason":"unsafe command"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusFailed || !strings.Contains(response.Data.Run.Error, "unsafe command") {
		t.Fatalf("expected failed run detail with reject reason, got %+v", response.Data.Run)
	}
	if len(response.Data.ToolRuns) != 1 {
		t.Fatalf("expected one tool run detail, got %+v", response.Data.ToolRuns)
	}
	toolRun := response.Data.ToolRuns[0]
	if toolRun.ID != "tool_run_pending" || toolRun.Status != agent.ToolRunStatusRejected || toolRun.ApprovalStatus != agent.ApprovalStatusRejected || toolRun.ApprovalDecisionReason != "unsafe command" {
		t.Fatalf("expected rejected tool run detail, got %+v", toolRun)
	}
}

func TestAgentRunsHandlerApproveToolRejectsAmbiguousOmittedToolRunID(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Status:         agent.RunStatusPendingApproval,
	}}
	store.toolRuns = []*agent.ToolRun{
		{ID: "tool_run_1", OrganizationID: "org_1", RunID: "run_1", Status: agent.ToolRunStatusPendingApproval, ApprovalStatus: agent.ApprovalStatusPending},
		{ID: "tool_run_2", OrganizationID: "org_1", RunID: "run_1", Status: agent.ToolRunStatusPendingApproval, ApprovalStatus: agent.ApprovalStatusPending},
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approveTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool", `{}`), "run_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "toolRunId is required") {
		t.Fatalf("expected ambiguous tool id message, got %s", recorder.Body.String())
	}
}

func TestAgentRunsHandlerApproveToolVerifiesToolBelongsToRun(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.runs = []*agent.Run{
		{ID: "run_1", OrganizationID: "org_1", UserID: "user_1"},
		{ID: "run_2", OrganizationID: "org_1", UserID: "user_1"},
	}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_other",
		OrganizationID: "org_1",
		RunID:          "run_2",
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approveTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool", `{"toolRunId":"tool_run_other"}`), "run_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "does not belong to run") {
		t.Fatalf("expected membership error, got %s", recorder.Body.String())
	}
}

func TestAgentRunsHandlerToolDecisionsVerifyToolRunBelongsToURLRun(t *testing.T) {
	tests := []struct {
		name            string
		call            func(agentRunsHandler, stdhttp.ResponseWriter, *stdhttp.Request, string)
		path            string
		body            string
		initialStatus   string
		initialApproval string
		initialError    string
	}{
		{
			name:            "reject",
			call:            agentRunsHandler.rejectTool,
			path:            "/api/v1/agent/runs/run_1/reject-tool",
			body:            `{"toolRunId":"tool_run_other","reason":"wrong run"}`,
			initialStatus:   agent.ToolRunStatusPendingApproval,
			initialApproval: agent.ApprovalStatusPending,
		},
		{
			name:            "retry",
			call:            agentRunsHandler.retryTool,
			path:            "/api/v1/agent/runs/run_1/retry-tool",
			body:            `{"toolRunId":"tool_run_other"}`,
			initialStatus:   agent.ToolRunStatusFailed,
			initialApproval: agent.ApprovalStatusNotRequired,
			initialError:    "temporary failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAgentRunsStore()
			store.runs = []*agent.Run{
				{ID: "run_1", OrganizationID: "org_1", UserID: "user_1"},
				{ID: "run_2", OrganizationID: "org_1", UserID: "user_1"},
			}
			store.toolRuns = []*agent.ToolRun{{
				ID:             "tool_run_other",
				OrganizationID: "org_1",
				RunID:          "run_2",
				Status:         tt.initialStatus,
				ApprovalStatus: tt.initialApproval,
				Error:          tt.initialError,
			}}
			handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

			recorder := httptest.NewRecorder()
			tt.call(handler, recorder, newAgentRunsRequest(stdhttp.MethodPost, tt.path, tt.body), "run_1")

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "toolRunId does not belong to run") {
				t.Fatalf("expected membership error, got %s", recorder.Body.String())
			}
			toolRun := store.toolRuns[0]
			if toolRun.RunID != "run_2" || toolRun.Status != tt.initialStatus || toolRun.ApprovalStatus != tt.initialApproval || toolRun.Error != tt.initialError {
				t.Fatalf("cross-run tool decision mutated guarded tool run: %+v", toolRun)
			}
		})
	}
}

func TestAgentRunsHandlerToolDecisionsRejectAmbiguousOmittedToolRunID(t *testing.T) {
	tests := []struct {
		name            string
		call            func(agentRunsHandler, stdhttp.ResponseWriter, *stdhttp.Request, string)
		path            string
		initialStatus   string
		initialApproval string
	}{
		{
			name:            "reject",
			call:            agentRunsHandler.rejectTool,
			path:            "/api/v1/agent/runs/run_1/reject-tool",
			initialStatus:   agent.ToolRunStatusPendingApproval,
			initialApproval: agent.ApprovalStatusPending,
		},
		{
			name:            "retry",
			call:            agentRunsHandler.retryTool,
			path:            "/api/v1/agent/runs/run_1/retry-tool",
			initialStatus:   agent.ToolRunStatusFailed,
			initialApproval: agent.ApprovalStatusNotRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAgentRunsStore()
			store.runs = []*agent.Run{{
				ID:             "run_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
			}}
			store.toolRuns = []*agent.ToolRun{
				{ID: "tool_run_1", OrganizationID: "org_1", RunID: "run_1", Status: tt.initialStatus, ApprovalStatus: tt.initialApproval},
				{ID: "tool_run_2", OrganizationID: "org_1", RunID: "run_1", Status: tt.initialStatus, ApprovalStatus: tt.initialApproval},
			}
			handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

			recorder := httptest.NewRecorder()
			tt.call(handler, recorder, newAgentRunsRequest(stdhttp.MethodPost, tt.path, `{}`), "run_1")

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "toolRunId is required when multiple matching tool runs exist") {
				t.Fatalf("expected ambiguous tool id message, got %s", recorder.Body.String())
			}
			for _, toolRun := range store.toolRuns {
				if toolRun.Status != tt.initialStatus || toolRun.ApprovalStatus != tt.initialApproval {
					t.Fatalf("ambiguous omitted tool decision mutated guarded tool run: %+v", toolRun)
				}
			}
		})
	}
}

func TestAgentRunsHandlerToolApprovalDecisionsRejectNonPendingToolRuns(t *testing.T) {
	tests := []struct {
		name            string
		call            func(agentRunsHandler, stdhttp.ResponseWriter, *stdhttp.Request, string)
		body            string
		initialStatus   string
		initialApproval string
	}{
		{
			name:            "approve completed",
			call:            agentRunsHandler.approveTool,
			body:            `{"toolRunId":"tool_run_guarded","reason":"late approval"}`,
			initialStatus:   agent.ToolRunStatusCompleted,
			initialApproval: agent.ApprovalStatusApproved,
		},
		{
			name:            "reject completed",
			call:            agentRunsHandler.rejectTool,
			body:            `{"toolRunId":"tool_run_guarded","reason":"late rejection"}`,
			initialStatus:   agent.ToolRunStatusCompleted,
			initialApproval: agent.ApprovalStatusApproved,
		},
		{
			name:            "approve rejected",
			call:            agentRunsHandler.approveTool,
			body:            `{"toolRunId":"tool_run_guarded","reason":"override rejection"}`,
			initialStatus:   agent.ToolRunStatusRejected,
			initialApproval: agent.ApprovalStatusRejected,
		},
		{
			name:            "retry completed",
			call:            agentRunsHandler.retryTool,
			body:            `{"toolRunId":"tool_run_guarded"}`,
			initialStatus:   agent.ToolRunStatusCompleted,
			initialApproval: agent.ApprovalStatusApproved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAgentRunsStore()
			store.runs = []*agent.Run{{
				ID:             "run_1",
				OrganizationID: "org_1",
				ConversationID: "conv_1",
				AgentID:        "agent_1",
				UserID:         "user_1",
				Status:         agent.RunStatusCompleted,
			}}
			store.toolRuns = []*agent.ToolRun{{
				ID:                     "tool_run_guarded",
				OrganizationID:         "org_1",
				RunID:                  "run_1",
				ConversationID:         "conv_1",
				AgentID:                "agent_1",
				ToolCallID:             "call_guarded",
				ToolName:               "datetime",
				ToolType:               "builtin",
				Arguments:              map[string]any{},
				Status:                 tt.initialStatus,
				ApprovalStatus:         tt.initialApproval,
				ApprovedByUserID:       "reviewer_1",
				ApprovalDecisionReason: "original decision",
			}}
			handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

			recorder := httptest.NewRecorder()
			tt.call(handler, recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/tool-decision", tt.body), "run_1")

			if recorder.Code != stdhttp.StatusConflict {
				t.Fatalf("expected 409 for non-pending tool approval decision, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "toolRunId is not valid for this action") {
				t.Fatalf("expected explicit tool-run state gate error, got %s", recorder.Body.String())
			}
			toolRun := store.toolRuns[0]
			if toolRun.Status != tt.initialStatus || toolRun.ApprovalStatus != tt.initialApproval ||
				toolRun.ApprovedByUserID != "reviewer_1" || toolRun.ApprovalDecisionReason != "original decision" {
				t.Fatalf("non-pending approval decision mutated guarded tool run: %+v", toolRun)
			}
		})
	}
}

func TestAgentRunsHandlerRetryToolAcceptsSnakeCaseID(t *testing.T) {
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

	recorder := httptest.NewRecorder()
	handler.retryTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-tool", `{"tool_run_id":"tool_run_failed"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.ToolRuns) == 0 {
		t.Fatalf("expected tool run detail, got %+v", response.Data)
	}
	toolRun := response.Data.ToolRuns[0]
	if toolRun.ID != "tool_run_failed" || toolRun.Status != agent.ToolRunStatusCompleted || toolRun.AttemptCount != 2 || toolRun.Error != "" {
		t.Fatalf("expected failed tool run retried and completed, got %+v", toolRun)
	}
}

func TestAgentRunsHandlerRetryToolReopensPendingApprovalDetail(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Model:          "test-model",
		Tools:          []agent.Tool{{Name: "write_file", Type: "builtin", Enabled: true, RequiresApproval: true}},
	}
	store.runs = []*agent.Run{{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusFailed,
		Error:          "approval required",
		CompletedAt:    &now,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_failed_pending",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_write",
		ToolName:       "write_file",
		ToolType:       "builtin",
		RiskLevel:      agent.ToolRiskDangerous,
		Arguments:      map[string]any{"path": "/tmp/forbidden", "content": "no"},
		Status:         agent.ToolRunStatusFailed,
		ApprovalStatus: agent.ApprovalStatusPending,
		AttemptCount:   0,
		Error:          "approval required",
		CompletedAt:    &now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.retryTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-tool", `{"toolRunId":"tool_run_failed_pending"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusPendingApproval || response.Data.Run.Error != "" || response.Data.Run.CompletedAt != nil {
		t.Fatalf("expected run detail to reopen pending approval, got %+v", response.Data.Run)
	}
	if len(response.Data.ToolRuns) != 1 {
		t.Fatalf("expected one tool run detail, got %+v", response.Data.ToolRuns)
	}
	toolRun := response.Data.ToolRuns[0]
	if toolRun.ID != "tool_run_failed_pending" || toolRun.Status != agent.ToolRunStatusPendingApproval || toolRun.ApprovalStatus != agent.ApprovalStatusPending || toolRun.AttemptCount != 0 || toolRun.Error != "" || toolRun.CompletedAt != nil {
		t.Fatalf("expected failed pending approval tool to reopen without execution, got %+v", toolRun)
	}
	if len(response.Data.Messages) != 0 {
		t.Fatalf("pending approval retry should not emit tool messages, got %+v", response.Data.Messages)
	}
}

func TestAgentRunsHandlerApproveToolReturnsRecoverableRunDetail(t *testing.T) {
	now := time.Now().UTC()
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
		IterationCount: 1,
		ToolCallCount:  1,
	}}
	store.toolRuns = []*agent.ToolRun{{
		ID:             "tool_run_pending",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.approveTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool", `{"toolRunId":"tool_run_pending","reason":"ok"}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data agentRunResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run == nil || response.Data.Run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected completed run detail, got %+v", response.Data.Run)
	}
	if len(response.Data.ToolRuns) != 1 || response.Data.ToolRuns[0].Status != agent.ToolRunStatusCompleted {
		t.Fatalf("expected completed tool run detail, got %+v", response.Data.ToolRuns)
	}
	if len(response.Data.Messages) != 2 {
		t.Fatalf("expected recoverable tool result and final assistant messages, got %+v", response.Data.Messages)
	}
	if response.Data.Messages[0].Role != "tool" || response.Data.Messages[0].ToolCallID != "call_datetime" {
		t.Fatalf("expected recoverable tool result message first, got %+v", response.Data.Messages)
	}
	if response.Data.Messages[1].Role != "assistant" || response.Data.Messages[1].Content == "" {
		t.Fatalf("expected final assistant message after approved tool, got %+v", response.Data.Messages)
	}
}

func TestAgentRunsHandlerRetryToolSelectsOnlyFailedWhenOmitted(t *testing.T) {
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
	store.toolRuns = []*agent.ToolRun{
		{ID: "tool_run_completed", OrganizationID: "org_1", RunID: "run_1", Status: agent.ToolRunStatusCompleted, ApprovalStatus: agent.ApprovalStatusNotRequired},
		{ID: "tool_run_failed", OrganizationID: "org_1", RunID: "run_1", ConversationID: "conv_1", AgentID: "agent_1", ToolCallID: "call_datetime_failed", ToolName: "datetime", ToolType: "builtin", Arguments: map[string]any{}, Status: agent.ToolRunStatusFailed, ApprovalStatus: agent.ApprovalStatusNotRequired, AttemptCount: 1, Error: "failed"},
	}
	handler := newAgentRunsHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.retryTool(recorder, newAgentRunsRequest(stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-tool", `{}`), "run_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"id":"tool_run_failed"`) {
		t.Fatalf("expected failed tool run response, got %s", recorder.Body.String())
	}
}

func TestAgentRunsHandlerListToolsRequiresAgentID(t *testing.T) {
	handler := newAgentRunsHandler(agent.NewService(newFakeAgentRunsStore(), &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.listTools(recorder, newAgentRunsRequest(stdhttp.MethodGet, "/api/v1/agent/tools", ""))

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestAgentRunsHandlerListToolsAcceptsAgentIDAlias(t *testing.T) {
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

	recorder := httptest.NewRecorder()
	handler.listTools(recorder, newAgentRunsRequest(stdhttp.MethodGet, "/api/v1/agent/tools?agent_id=agent_1", ""))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"name":"custom_tool"`) {
		t.Fatalf("expected listed tool, got %s", recorder.Body.String())
	}
}

func newAgentRunsRequest(method, path, body string) *stdhttp.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		ID:             "session_1",
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User: auth.User{
			ID:    "user_1",
			Email: "user@example.com",
			Role:  "user",
		},
	}))
}

type fakeAgentRunsGateway struct {
	reply       string
	structured  []*chat.CompletionResponse
	structIndex int
}

func (g *fakeAgentRunsGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	if g.reply == "" {
		return "ok", nil
	}
	return g.reply, nil
}

func (g *fakeAgentRunsGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk(g.reply)
}

func (g *fakeAgentRunsGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	if g.structIndex < len(g.structured) {
		response := g.structured[g.structIndex]
		g.structIndex++
		return response, nil
	}
	if g.reply == "" {
		return &chat.CompletionResponse{Content: "ok"}, nil
	}
	return &chat.CompletionResponse{Content: g.reply}, nil
}

type fakeAgentRunsStore struct {
	agent                    *agent.Agent
	conversation             *agent.Conversation
	listMemoryAgentID        string
	listMemoryLimit          int
	listMemoryOrganizationID string
	listMemoryQuery          string
	listMemoryUserID         string
	memories                 []*agent.Memory
	messages                 []*agent.Message
	planSteps                []*agent.PlanStep
	runs                     []*agent.Run
	toolRuns                 []*agent.ToolRun
}

func newFakeAgentRunsStore() *fakeAgentRunsStore {
	return &fakeAgentRunsStore{}
}

func (s *fakeAgentRunsStore) CreateAgent(ctx context.Context, userID, organizationID string, req *agent.CreateAgentRequest) (*agent.Agent, error) {
	panic("not used")
}

func (s *fakeAgentRunsStore) GetAgent(ctx context.Context, id, organizationID string) (*agent.Agent, error) {
	if s.agent != nil && s.agent.ID == id && s.agent.OrganizationID == organizationID {
		return s.agent, nil
	}
	return nil, nil
}

func (s *fakeAgentRunsStore) ListAgents(ctx context.Context, userID, organizationID string) ([]*agent.Agent, error) {
	panic("not used")
}

func (s *fakeAgentRunsStore) UpdateAgent(ctx context.Context, id, organizationID string, req *agent.UpdateAgentRequest) (*agent.Agent, error) {
	panic("not used")
}

func (s *fakeAgentRunsStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	panic("not used")
}

func (s *fakeAgentRunsStore) CreateConversation(ctx context.Context, agentID, userID, organizationID string, title string) (*agent.Conversation, error) {
	panic("not used")
}

func (s *fakeAgentRunsStore) GetConversation(ctx context.Context, id, organizationID string) (*agent.Conversation, error) {
	if s.conversation != nil && s.conversation.ID == id && s.conversation.OrganizationID == organizationID {
		return s.conversation, nil
	}
	return nil, nil
}

func (s *fakeAgentRunsStore) ListConversations(ctx context.Context, agentID, userID, organizationID string) ([]*agent.Conversation, error) {
	panic("not used")
}

func (s *fakeAgentRunsStore) DeleteConversation(ctx context.Context, id, organizationID string) error {
	panic("not used")
}

func (s *fakeAgentRunsStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []agent.ToolCall, toolCallID string) (*agent.Message, error) {
	msg := &agent.Message{
		ID:             role + "_message",
		ConversationID: conversationID,
		OrganizationID: organizationID,
		Role:           role,
		Content:        content,
		ToolCalls:      append([]agent.ToolCall(nil), toolCalls...),
		ToolCallID:     toolCallID,
		CreatedAt:      time.Now().UTC(),
	}
	s.messages = append(s.messages, msg)
	return msg, nil
}

func (s *fakeAgentRunsStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]*agent.Message, error) {
	var messages []*agent.Message
	for _, msg := range s.messages {
		if msg.ConversationID == conversationID && msg.OrganizationID == organizationID {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (s *fakeAgentRunsStore) CreateRun(ctx context.Context, req *agent.CreateRunRequest) (*agent.Run, error) {
	now := time.Now().UTC()
	run := &agent.Run{
		ID:                "run_1",
		OrganizationID:    req.OrganizationID,
		ConversationID:    req.ConversationID,
		AgentID:           req.AgentID,
		UserID:            req.UserID,
		RequestID:         req.RequestID,
		Mode:              agent.NormalizeExecutionMode(req.Mode),
		Status:            req.Status,
		MemoryEnabled:     req.MemoryEnabled,
		MemorySearched:    req.MemorySearched,
		MemoryResultCount: req.MemoryResultCount,
		StartedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *fakeAgentRunsStore) GetRun(ctx context.Context, organizationID, id string) (*agent.Run, error) {
	for _, run := range s.runs {
		if run.OrganizationID == organizationID && run.ID == id {
			return run, nil
		}
	}
	return nil, nil
}

func (s *fakeAgentRunsStore) ListRuns(ctx context.Context, organizationID, conversationID string) ([]*agent.Run, error) {
	var runs []*agent.Run
	for _, run := range s.runs {
		if run.OrganizationID == organizationID && run.ConversationID == conversationID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *fakeAgentRunsStore) UpdateRun(ctx context.Context, organizationID, id string, req agent.UpdateRunRequest) (*agent.Run, error) {
	run, _ := s.GetRun(ctx, organizationID, id)
	if run == nil {
		return nil, errors.New("run not found")
	}
	if req.Status != nil {
		run.Status = *req.Status
	}
	if req.IterationCount != nil {
		run.IterationCount = *req.IterationCount
	}
	if req.ToolCallCount != nil {
		run.ToolCallCount = *req.ToolCallCount
	}
	if req.FinalMessageID != nil {
		run.FinalMessageID = *req.FinalMessageID
	}
	if req.Error != nil {
		run.Error = *req.Error
	}
	if req.ClearCompletedAt {
		run.CompletedAt = nil
	} else if req.CompletedAt != nil {
		run.CompletedAt = req.CompletedAt
	}
	run.UpdatedAt = time.Now().UTC()
	return run, nil
}

func (s *fakeAgentRunsStore) CreateToolRun(ctx context.Context, req *agent.CreateToolRunRequest) (*agent.ToolRun, error) {
	now := time.Now().UTC()
	status := req.Status
	if status == "" {
		status = agent.ToolRunStatusRunning
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = agent.ApprovalStatusNotRequired
	}
	arguments := req.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	toolRun := &agent.ToolRun{
		ID:             "tool_run_created_" + time.Now().UTC().Format("150405.000000"),
		OrganizationID: req.OrganizationID,
		RunID:          req.RunID,
		ConversationID: req.ConversationID,
		AgentID:        req.AgentID,
		ToolCallID:     req.ToolCallID,
		ToolName:       req.ToolName,
		ToolType:       req.ToolType,
		ServerID:       req.ServerID,
		RiskLevel:      req.RiskLevel,
		Arguments:      arguments,
		Status:         status,
		ApprovalStatus: approvalStatus,
		AttemptCount:   req.AttemptCount,
		StartedAt:      req.StartedAt,
		CompletedAt:    req.CompletedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.toolRuns = append(s.toolRuns, toolRun)
	return toolRun, nil
}

func (s *fakeAgentRunsStore) GetToolRun(ctx context.Context, organizationID, id string) (*agent.ToolRun, error) {
	for _, toolRun := range s.toolRuns {
		if toolRun.OrganizationID == organizationID && toolRun.ID == id {
			return toolRun, nil
		}
	}
	return nil, nil
}

func (s *fakeAgentRunsStore) ListToolRuns(ctx context.Context, organizationID, runID string) ([]*agent.ToolRun, error) {
	var toolRuns []*agent.ToolRun
	for _, toolRun := range s.toolRuns {
		if toolRun.OrganizationID == organizationID && toolRun.RunID == runID {
			toolRuns = append(toolRuns, toolRun)
		}
	}
	return toolRuns, nil
}

func (s *fakeAgentRunsStore) UpdateToolRun(ctx context.Context, organizationID, id string, req agent.UpdateToolRunRequest) (*agent.ToolRun, error) {
	toolRun, _ := s.GetToolRun(ctx, organizationID, id)
	if toolRun == nil {
		return nil, errors.New("tool run not found")
	}
	if req.Status != nil {
		toolRun.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		toolRun.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ApprovedByUserID != nil {
		toolRun.ApprovedByUserID = *req.ApprovedByUserID
	}
	if req.ApprovalDecisionReason != nil {
		toolRun.ApprovalDecisionReason = *req.ApprovalDecisionReason
	}
	if req.AttemptCount != nil {
		toolRun.AttemptCount = *req.AttemptCount
	}
	if req.ResultContent != nil {
		toolRun.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		toolRun.Error = *req.Error
	}
	if req.StartedAt != nil {
		toolRun.StartedAt = req.StartedAt
	}
	if req.CompletedAt != nil {
		toolRun.CompletedAt = req.CompletedAt
	}
	if req.ClearCompletedAt {
		toolRun.CompletedAt = nil
	}
	toolRun.UpdatedAt = time.Now().UTC()
	return toolRun, nil
}

func (s *fakeAgentRunsStore) CreatePlanStep(ctx context.Context, req *agent.CreatePlanStepRequest) (*agent.PlanStep, error) {
	now := time.Now().UTC()
	status := req.Status
	if status == "" {
		status = agent.PlanStepStatusPending
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = agent.ApprovalStatusNotRequired
	}
	input := map[string]any{}
	for key, value := range req.Input {
		input[key] = value
	}
	step := &agent.PlanStep{
		ID:             "plan_step_" + time.Now().UTC().Format("150405.000000"),
		RunID:          req.RunID,
		OrganizationID: req.OrganizationID,
		Index:          req.Index,
		Title:          req.Title,
		Status:         status,
		ApprovalStatus: approvalStatus,
		ToolName:       req.ToolName,
		Input:          input,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.planSteps = append(s.planSteps, step)
	return step, nil
}

func (s *fakeAgentRunsStore) ListPlanSteps(ctx context.Context, organizationID, runID string) ([]*agent.PlanStep, error) {
	var steps []*agent.PlanStep
	for _, step := range s.planSteps {
		if step.OrganizationID == organizationID && step.RunID == runID {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

func (s *fakeAgentRunsStore) GetPlanStep(ctx context.Context, organizationID, id string) (*agent.PlanStep, error) {
	for _, step := range s.planSteps {
		if step.ID == id && step.OrganizationID == organizationID {
			return step, nil
		}
	}
	return nil, nil
}

func (s *fakeAgentRunsStore) UpdatePlanStep(ctx context.Context, organizationID, id string, req agent.UpdatePlanStepRequest) (*agent.PlanStep, error) {
	step, _ := s.GetPlanStep(ctx, organizationID, id)
	if step == nil {
		return nil, errors.New("agent plan step not found")
	}
	if req.Index != nil {
		step.Index = *req.Index
	}
	if req.Title != nil {
		step.Title = *req.Title
	}
	if req.Status != nil {
		step.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		step.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ToolName != nil {
		step.ToolName = *req.ToolName
	}
	if req.ReplaceInput {
		step.Input = map[string]any{}
		for key, value := range req.Input {
			step.Input[key] = value
		}
	}
	if req.ResultContent != nil {
		step.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		step.Error = *req.Error
	}
	if req.StartedAt != nil {
		step.StartedAt = req.StartedAt
	}
	if req.ClearCompletedAt {
		step.CompletedAt = nil
	} else if req.CompletedAt != nil {
		step.CompletedAt = req.CompletedAt
	}
	step.UpdatedAt = time.Now().UTC()
	return step, nil
}

func (s *fakeAgentRunsStore) DeletePlanStep(ctx context.Context, organizationID, id string) (*agent.PlanStep, error) {
	for index, step := range s.planSteps {
		if step.ID == id && step.OrganizationID == organizationID {
			s.planSteps = append(s.planSteps[:index], s.planSteps[index+1:]...)
			return step, nil
		}
	}
	return nil, errors.New("agent plan step not found")
}

func (s *fakeAgentRunsStore) CreateMemory(ctx context.Context, req *agent.CreateMemoryStoreRequest) (*agent.Memory, error) {
	now := time.Now().UTC()
	metadata := map[string]any{}
	for key, value := range req.Metadata {
		metadata[key] = value
	}
	memory := &agent.Memory{
		ID:             "memory_" + time.Now().UTC().Format("150405.000000"),
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		AgentID:        req.AgentID,
		Type:           req.Type,
		Content:        req.Content,
		Metadata:       metadata,
		ExpiresAt:      req.ExpiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.memories = append(s.memories, memory)
	return memory, nil
}

func (s *fakeAgentRunsStore) ListMemories(ctx context.Context, organizationID, userID string, req agent.ListMemoriesRequest) ([]*agent.Memory, error) {
	s.listMemoryOrganizationID = organizationID
	s.listMemoryUserID = userID
	s.listMemoryAgentID = req.AgentID
	s.listMemoryQuery = req.Query
	s.listMemoryLimit = req.Limit

	var memories []*agent.Memory
	for _, memory := range s.memories {
		if memory.OrganizationID != organizationID || memory.UserID != userID {
			continue
		}
		if req.AgentID != "" && memory.AgentID != req.AgentID {
			continue
		}
		if req.Type != "" && memory.Type != req.Type {
			continue
		}
		if req.Query != "" && !strings.Contains(strings.ToLower(memory.Content), strings.ToLower(req.Query)) {
			continue
		}
		memories = append(memories, memory)
		if req.Limit > 0 && len(memories) >= req.Limit {
			break
		}
	}
	return memories, nil
}
