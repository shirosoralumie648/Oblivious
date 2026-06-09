package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/chat"
)

type planningRuntimeFakeGateway struct {
	responses    []*chat.CompletionResponse
	errors       []error
	calls        int
	lastMessages []chat.Message
}

func (g *planningRuntimeFakeGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.calls++
	g.lastMessages = append([]chat.Message(nil), messages...)
	if g.calls <= len(g.errors) && g.errors[g.calls-1] != nil {
		return nil, g.errors[g.calls-1]
	}
	if g.calls <= len(g.responses) {
		return g.responses[g.calls-1], nil
	}
	return &chat.CompletionResponse{Content: "done"}, nil
}

func TestNormalizePlanningConfigClampsTokenBudget(t *testing.T) {
	tests := []struct {
		name string
		in   PlanningConfig
		want PlanningConfig
	}{
		{
			name: "defaults",
			in:   PlanningConfig{},
			want: PlanningConfig{MaxSteps: 20, MaxIterations: 10, TokenBudget: 0},
		},
		{
			name: "negative budget is unlimited",
			in:   PlanningConfig{MaxSteps: 5, MaxIterations: 3, TokenBudget: -1},
			want: PlanningConfig{MaxSteps: 5, MaxIterations: 3, TokenBudget: 0},
		},
		{
			name: "low positive budget clamps to minimum",
			in:   PlanningConfig{MaxSteps: 5, MaxIterations: 3, TokenBudget: 1},
			want: PlanningConfig{MaxSteps: 5, MaxIterations: 3, TokenBudget: 1000},
		},
		{
			name: "oversized budget clamps to maximum",
			in:   PlanningConfig{MaxSteps: 100, MaxIterations: 200, TokenBudget: 2_000_000},
			want: PlanningConfig{MaxSteps: 50, MaxIterations: 100, TokenBudget: 1_000_000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePlanningConfig(tt.in)
			if got != tt.want {
				t.Fatalf("NormalizePlanningConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlanningEngineGeneratePlanClampsMaxSteps(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{responses: []*chat.CompletionResponse{{
		Content: `{
			"goal": "ship the runtime slice",
			"steps": [
				{"index": 1, "title": "Inspect"},
				{"index": 2, "title": "Patch"},
				{"index": 3, "title": "Verify"}
			]
		}`,
	}}}
	engine := NewPlanningEngine(gateway, nil)

	plan, err := engine.GeneratePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, nil, "ship it", PlanningConfig{MaxSteps: 2})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}
	if gateway.calls != 1 {
		t.Fatalf("expected one planning gateway call, got %d", gateway.calls)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected plan to be clamped to 2 steps, got %+v", plan.Steps)
	}
	if plan.Steps[0].Title != "Inspect" || plan.Steps[1].Title != "Patch" {
		t.Fatalf("expected clamped plan to preserve the first two steps, got %+v", plan.Steps)
	}
}

func TestPlanningEngineExecutePlanHandlesOneBasedStepIndexes(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{responses: []*chat.CompletionResponse{{
		Content: "first step complete",
		Usage:   &chat.CompletionUsage{TotalTokens: 120},
	}}}
	engine := NewPlanningEngine(gateway, nil)
	plan := Plan{
		Goal: "handle one-based index",
		Steps: []PlanStep{{
			Index: 1,
			Title: "First",
		}},
	}

	result, err := engine.ExecutePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, "conv_1", nil, plan, PlanningConfig{})
	if err != nil {
		t.Fatalf("ExecutePlan returned error: %v", err)
	}
	if result.StopReason != "plan_completed" {
		t.Fatalf("expected one-based plan to complete, got %+v", result)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].Index != 1 || result.StepResults[0].Status != "completed" {
		t.Fatalf("expected completed one-based step result, got %+v", result.StepResults)
	}
	if gateway.calls != 1 {
		t.Fatalf("expected one step gateway call, got %d", gateway.calls)
	}
}

func TestPlanningEngineAdjustPlanClampsMaxSteps(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{responses: []*chat.CompletionResponse{{
		Content: `{
			"goal": "finish remaining work",
			"steps": [
				{"index": 1, "title": "Recover"},
				{"index": 2, "title": "Verify"},
				{"index": 3, "title": "Report"}
			]
		}`,
	}}}
	engine := NewPlanningEngine(gateway, nil)
	original := Plan{
		Goal: "ship dynamic planning",
		Steps: []PlanStep{{
			Index: 1,
			Title: "Inspect",
		}},
	}
	completed := []StepResult{{
		Index:  1,
		Title:  "Inspect",
		Status: "completed",
		Result: "found a budget boundary",
	}}

	plan, err := engine.AdjustPlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, original, completed, "step budget changed", PlanningConfig{MaxSteps: 2})
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}
	if gateway.calls != 1 {
		t.Fatalf("expected one adjustment gateway call, got %d", gateway.calls)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected adjusted plan to be clamped to 2 steps, got %+v", plan.Steps)
	}
	if plan.Steps[0].Title != "Recover" || plan.Steps[1].Title != "Verify" {
		t.Fatalf("expected adjusted plan to preserve the first two revised steps, got %+v", plan.Steps)
	}
	if len(gateway.lastMessages) != 1 || !strings.Contains(gateway.lastMessages[0].Content, "step budget changed") || !strings.Contains(gateway.lastMessages[0].Content, "found a budget boundary") {
		t.Fatalf("expected adjustment prompt to include reason and completed-step evidence, got %+v", gateway.lastMessages)
	}
}

func TestPlanningEngineExecutePlanStopsWhenStepExceedsTokenBudget(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{responses: []*chat.CompletionResponse{{
		Content: "expensive step complete",
		Usage:   &chat.CompletionUsage{TotalTokens: 1200},
	}}}
	engine := NewPlanningEngine(gateway, nil)
	plan := Plan{
		Goal: "respect budget after every step",
		Steps: []PlanStep{{
			Index: 1,
			Title: "Expensive",
		}},
	}

	result, err := engine.ExecutePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, "conv_1", nil, plan, PlanningConfig{TokenBudget: 1000})
	if err != nil {
		t.Fatalf("ExecutePlan returned error: %v", err)
	}
	if result.StopReason != "token_budget_exceeded" {
		t.Fatalf("expected token budget stop instead of plan completion, got %+v", result)
	}
	if result.TotalTokens != 1200 || result.FinalAnswer != "" {
		t.Fatalf("expected budget evidence without final answer, got %+v", result)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].TokensUsed != 1200 {
		t.Fatalf("expected one expensive step result, got %+v", result.StepResults)
	}
	if gateway.calls != 1 {
		t.Fatalf("expected one step gateway call, got %d", gateway.calls)
	}
}

func TestPlanningEngineExecutePlanMarksDependencyFailureIncomplete(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{
		errors: []error{errors.New("gateway offline")},
	}
	engine := NewPlanningEngine(gateway, nil)
	plan := Plan{
		Goal: "do dependent work",
		Steps: []PlanStep{
			{
				Index: 1,
				Title: "Fetch source",
			},
			{
				Index:     2,
				Title:     "Use source",
				DependsOn: []int{1},
			},
		},
	}

	result, err := engine.ExecutePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, "conv_1", nil, plan, PlanningConfig{})
	if err != nil {
		t.Fatalf("ExecutePlan returned error: %v", err)
	}
	if result.StopReason != "plan_incomplete" || result.FinalAnswer != "" {
		t.Fatalf("expected incomplete plan without final answer, got %+v", result)
	}
	if result.IterationCount != 2 {
		t.Fatalf("expected failed step and skipped dependent step to be counted, got %+v", result)
	}
	if len(result.StepResults) != 2 {
		t.Fatalf("expected two step results, got %+v", result.StepResults)
	}
	if result.StepResults[0].Index != 1 || result.StepResults[0].Status != "failed" || !strings.Contains(result.StepResults[0].Error, "gateway offline") {
		t.Fatalf("expected failed dependency evidence, got %+v", result.StepResults[0])
	}
	if result.StepResults[1].Index != 2 || result.StepResults[1].Status != "skipped" || !strings.Contains(result.StepResults[1].Error, "dependency step 1 not completed") {
		t.Fatalf("expected skipped dependent-step evidence, got %+v", result.StepResults[1])
	}
	if gateway.calls != 1 {
		t.Fatalf("expected only the failed dependency to call the gateway, got %d", gateway.calls)
	}
}

func TestPlanningEngineExecutePlanMarksMissingDependencyIncomplete(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{}
	engine := NewPlanningEngine(gateway, nil)
	plan := Plan{
		Goal: "skip missing dependency",
		Steps: []PlanStep{{
			Index:     2,
			Title:     "Use missing dependency",
			DependsOn: []int{1},
		}},
	}

	result, err := engine.ExecutePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, "conv_1", nil, plan, PlanningConfig{})
	if err != nil {
		t.Fatalf("ExecutePlan returned error: %v", err)
	}
	if result.StopReason != "plan_incomplete" || result.FinalAnswer != "" {
		t.Fatalf("expected missing dependency to keep plan incomplete, got %+v", result)
	}
	if result.IterationCount != 1 || len(result.StepResults) != 1 {
		t.Fatalf("expected one skipped step result, got %+v", result)
	}
	if result.StepResults[0].Index != 2 || result.StepResults[0].Status != "skipped" || !strings.Contains(result.StepResults[0].Error, "dependency step 1 not completed") {
		t.Fatalf("expected missing dependency skip evidence, got %+v", result.StepResults[0])
	}
	if gateway.calls != 0 {
		t.Fatalf("expected missing dependency to avoid gateway calls, got %d", gateway.calls)
	}
}

func TestPlanningEngineGenerateAndExecutePlanAppliesConfig(t *testing.T) {
	gateway := &planningRuntimeFakeGateway{responses: []*chat.CompletionResponse{
		{
			Content: `{
				"goal": "ship the configured runtime",
				"steps": [
					{"index": 1, "title": "Inspect"},
					{"index": 2, "title": "Patch"},
					{"index": 3, "title": "Verify"}
				]
			}`,
		},
		{
			Content: "expensive inspect complete",
			Usage:   &chat.CompletionUsage{TotalTokens: 1200},
		},
		{
			Content: "patch should not run after budget stop",
			Usage:   &chat.CompletionUsage{TotalTokens: 100},
		},
	}}
	engine := NewPlanningEngine(gateway, nil)

	result, err := engine.GenerateAndExecutePlan(context.Background(), &agent.Agent{Model: "gpt-4o-mini"}, "conv_1", nil, "ship it", PlanningConfig{
		MaxSteps:    2,
		TokenBudget: 1000,
	})
	if err != nil {
		t.Fatalf("GenerateAndExecutePlan returned error: %v", err)
	}
	if len(result.Plan.Steps) != 2 {
		t.Fatalf("expected generated plan to honor max-step config, got %+v", result.Plan.Steps)
	}
	if result.StopReason != "token_budget_exceeded" || result.TotalTokens != 1200 {
		t.Fatalf("expected execution to honor token-budget config, got %+v", result)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].Index != 1 {
		t.Fatalf("expected execution to stop after first over-budget step, got %+v", result.StepResults)
	}
	if gateway.calls != 2 {
		t.Fatalf("expected one planning call and one execution call, got %d", gateway.calls)
	}
}
