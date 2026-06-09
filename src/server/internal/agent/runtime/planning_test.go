package runtime

import (
	"context"
	"testing"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/chat"
)

type planningRuntimeFakeGateway struct {
	responses []*chat.CompletionResponse
	calls     int
}

func (g *planningRuntimeFakeGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.calls++
	if g.calls <= len(g.responses) {
		return g.responses[g.calls-1], nil
	}
	return &chat.CompletionResponse{Content: "done"}, nil
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
