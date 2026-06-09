package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/chat"
)

// PlanStep represents a single step in an execution plan.
type PlanStep struct {
	Index       int            `json:"index"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	ToolName    string         `json:"toolName,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	DependsOn   []int          `json:"dependsOn,omitempty"`
}

// Plan represents a complete execution plan.
type Plan struct {
	Goal     string         `json:"goal"`
	Steps    []PlanStep     `json:"steps"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PlanExecutionResult is the outcome of executing a plan.
type PlanExecutionResult struct {
	Plan           Plan         `json:"plan"`
	StepResults    []StepResult `json:"stepResults"`
	FinalAnswer    string       `json:"finalAnswer,omitempty"`
	StopReason     string       `json:"stopReason"`
	IterationCount int          `json:"iterationCount"`
	TotalTokens    int          `json:"totalTokens"`
	PlanAdjusted   bool         `json:"planAdjusted,omitempty"`
}

// StepResult records the outcome of executing one plan step.
type StepResult struct {
	Index      int    `json:"index"`
	Title      string `json:"title"`
	Status     string `json:"status"` // "pending", "running", "completed", "failed", "skipped"
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	TokensUsed int    `json:"tokensUsed,omitempty"`
}

// PlanningConfig controls planning engine behaviour.
type PlanningConfig struct {
	MaxSteps        int  // maximum number of plan steps allowed
	MaxIterations   int  // maximum ReAct-style iterations during step execution
	RequireApproval bool // whether the plan must be approved before execution
	TokenBudget     int  // total token budget (0 = unlimited)
}

// DefaultPlanningConfig returns sensible defaults.
func DefaultPlanningConfig() PlanningConfig {
	return PlanningConfig{
		MaxSteps:        20,
		MaxIterations:   10,
		RequireApproval: false,
		TokenBudget:     0,
	}
}

// NormalizePlanningConfig applies limits.
func NormalizePlanningConfig(cfg PlanningConfig) PlanningConfig {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 20
	}
	if cfg.MaxSteps > 50 {
		cfg.MaxSteps = 50
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if cfg.MaxIterations > 100 {
		cfg.MaxIterations = 100
	}
	if cfg.TokenBudget < 0 {
		cfg.TokenBudget = 0
	}
	return cfg
}

// PlanningEngine generates and optionally executes plans.
type PlanningEngine struct {
	gateway  ChatGateway
	runner   ToolRunner
	reactEng *ReActEngine
}

// NewPlanningEngine creates a new planning engine.
func NewPlanningEngine(gateway ChatGateway, runner ToolRunner) *PlanningEngine {
	return &PlanningEngine{
		gateway:  gateway,
		runner:   runner,
		reactEng: NewReActEngine(gateway, runner),
	}
}

const planningSystemPrompt = `You are a planning agent. Given a user task, produce a JSON execution plan.
Respond in EXACTLY this format:
{
  "goal": "one sentence summary of what the plan achieves",
  "steps": [
    {
      "index": 1,
      "title": "short step title",
      "description": "what this step does",
      "toolName": "optional tool name",
      "input": {"key": "value"},
      "dependsOn": []
    }
  ]
}

Rules:
- Steps must be in execution order.
- Use toolName only when a tool is needed; omit for reasoning/text steps.
- dependsOn lists step indices this step requires to be completed first.
- Keep the plan minimal: every step must be necessary.
- Maximum 20 steps.`

// GeneratePlan asks the LLM to produce a plan for the given task.
func (e *PlanningEngine) GeneratePlan(ctx context.Context, agentInstance *agent.Agent, messages []chat.Message, userContent string, cfg PlanningConfig) (*Plan, error) {
	if e.gateway == nil {
		return nil, fmt.Errorf("planning engine: gateway not configured")
	}
	cfg = NormalizePlanningConfig(cfg)

	config := chat.ConversationConfig{
		ModelID: agentInstance.Model,
		SystemPromptOverride: strings.Join([]string{
			agentInstance.SystemPrompt,
			planningSystemPrompt,
		}, "\n\n"),
		Temperature:     agentInstance.Config.Temperature,
		MaxOutputTokens: agentInstance.Config.MaxTokens,
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}

	planMessages := make([]chat.Message, 0, len(messages)+1)
	planMessages = append(planMessages, messages...)
	planMessages = append(planMessages, chat.Message{
		Role:    "user",
		Content: userContent,
	})

	reply, err := e.gateway.GenerateStructuredReply(ctx, planMessages, config, nil)
	if err != nil {
		return nil, fmt.Errorf("planning engine: generate plan: %w", err)
	}

	plan, err := parsePlan(reply.Content)
	if err != nil {
		return nil, fmt.Errorf("planning engine: parse plan: %w", err)
	}

	// Enforce max steps.
	if len(plan.Steps) > cfg.MaxSteps {
		plan.Steps = plan.Steps[:cfg.MaxSteps]
	}

	return &plan, nil
}

// ExecutePlan runs each step in the plan sequentially (or respecting dependencies).
// It returns intermediate results and can dynamically adjust the plan.
func (e *PlanningEngine) ExecutePlan(ctx context.Context, agentInstance *agent.Agent, conversationID string, messages []chat.Message, plan Plan, cfg PlanningConfig) (*PlanExecutionResult, error) {
	if e.gateway == nil {
		return nil, fmt.Errorf("planning engine: gateway not configured")
	}
	cfg = NormalizePlanningConfig(cfg)

	result := &PlanExecutionResult{
		Plan: plan,
	}
	completed := make(map[int]bool)
	totalTokens := 0
	stepByIndex := make(map[int]PlanStep, len(plan.Steps))
	for _, step := range plan.Steps {
		stepByIndex[step.Index] = step
	}

	// Build an execution order respecting dependsOn.
	order := topologicalOrder(plan.Steps)

	for _, stepIndex := range order {
		step, ok := stepByIndex[stepIndex]
		if !ok {
			continue
		}

		// Check all dependencies are completed.
		skip := false
		for _, dep := range step.DependsOn {
			if !completed[dep] {
				result.StepResults = append(result.StepResults, StepResult{
					Index:  step.Index,
					Title:  step.Title,
					Status: "skipped",
					Error:  fmt.Sprintf("dependency step %d not completed", dep),
				})
				completed[step.Index] = false
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		if cfg.TokenBudget > 0 && totalTokens > cfg.TokenBudget {
			result.StopReason = "token_budget_exceeded"
			result.TotalTokens = totalTokens
			result.IterationCount = len(result.StepResults)
			return result, nil
		}

		stepResult := e.executeStep(ctx, agentInstance, messages, plan, step, cfg)
		totalTokens += stepResult.TokensUsed
		result.TotalTokens = totalTokens
		result.StepResults = append(result.StepResults, stepResult)
		result.IterationCount = len(result.StepResults)

		if cfg.TokenBudget > 0 && totalTokens > cfg.TokenBudget {
			result.StopReason = "token_budget_exceeded"
			return result, nil
		}

		if stepResult.Status == "completed" {
			completed[step.Index] = true
		} else {
			completed[step.Index] = false
		}
	}

	// Determine if all steps completed.
	allDone := true
	for _, sr := range result.StepResults {
		if sr.Status != "completed" && sr.Status != "skipped" {
			allDone = false
			break
		}
	}
	if allDone {
		result.StopReason = "plan_completed"
		// Synthesize a final answer from step results.
		result.FinalAnswer = synthesizeFinalAnswer(plan, result.StepResults)
	} else {
		result.StopReason = "plan_incomplete"
	}

	return result, nil
}

// GenerateAndExecutePlan is a convenience method that plans then executes.
func (e *PlanningEngine) GenerateAndExecutePlan(ctx context.Context, agentInstance *agent.Agent, conversationID string, messages []chat.Message, userContent string, cfg PlanningConfig) (*PlanExecutionResult, error) {
	plan, err := e.GeneratePlan(ctx, agentInstance, messages, userContent, cfg)
	if err != nil {
		return nil, err
	}
	return e.ExecutePlan(ctx, agentInstance, conversationID, messages, *plan, cfg)
}

// AdjustPlan dynamically re-plans based on current execution state.
func (e *PlanningEngine) AdjustPlan(ctx context.Context, agentInstance *agent.Agent, originalPlan Plan, completedSteps []StepResult, reason string) (*Plan, error) {
	if e.gateway == nil {
		return nil, fmt.Errorf("planning engine: gateway not configured")
	}

	adjustmentPrompt := fmt.Sprintf("The original plan has been partially executed and needs adjustment. Reason: %s\n\nOriginal plan: %s\n\nCompleted steps: %s\n\nProduce a revised plan for the remaining work only.", reason, mustJSON(originalPlan), mustJSON(completedSteps))

	config := chat.ConversationConfig{
		ModelID: agentInstance.Model,
		SystemPromptOverride: strings.Join([]string{
			agentInstance.SystemPrompt,
			planningSystemPrompt,
		}, "\n\n"),
		Temperature:     agentInstance.Config.Temperature,
		MaxOutputTokens: agentInstance.Config.MaxTokens,
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}

	reply, err := e.gateway.GenerateStructuredReply(ctx, []chat.Message{{
		Role:    "user",
		Content: adjustmentPrompt,
	}}, config, nil)
	if err != nil {
		return nil, fmt.Errorf("planning engine: adjust plan: %w", err)
	}

	newPlan, err := parsePlan(reply.Content)
	if err != nil {
		return nil, fmt.Errorf("planning engine: parse adjusted plan: %w", err)
	}
	return &newPlan, nil
}

func (e *PlanningEngine) executeStep(ctx context.Context, agentInstance *agent.Agent, messages []chat.Message, plan Plan, step PlanStep, cfg PlanningConfig) StepResult {
	sr := StepResult{
		Index:  step.Index,
		Title:  step.Title,
		Status: "running",
	}

	// If the step has a tool, use the ReAct engine for that single step.
	if step.ToolName != "" && e.reactEng != nil {
		stepPrompt := step.Description
		if stepPrompt == "" {
			stepPrompt = step.Title
		}

		reactCfg := ReActConfig{
			MaxIterations: cfg.MaxIterations,
			TokenBudget:   cfg.TokenBudget,
		}
		stepMessages := make([]chat.Message, 0, len(messages)+1)
		stepMessages = append(stepMessages, messages...)
		stepMessages = append(stepMessages, chat.Message{
			Role:    "user",
			Content: fmt.Sprintf("Execute plan step %d: %s\n\nContext: %s", step.Index, stepPrompt, stepContext(plan, step)),
		})

		reactResult, err := e.reactEng.Run(ctx, agentInstance, "", stepMessages, reactCfg)
		if err != nil {
			sr.Status = "failed"
			sr.Error = err.Error()
			return sr
		}
		sr.TokensUsed = reactResult.TotalTokens
		sr.Result = reactResult.FinalAnswer
		if reactResult.StopReason == "final_answer" || reactResult.StopReason == "model_stop" {
			sr.Status = "completed"
		} else {
			sr.Status = "failed"
			sr.Error = fmt.Sprintf("react stopped: %s", reactResult.StopReason)
		}
		return sr
	}

	// Plain LLM step without tools.
	stepPrompt := step.Description
	if stepPrompt == "" {
		stepPrompt = step.Title
	}
	config := chat.ConversationConfig{
		ModelID:              agentInstance.Model,
		SystemPromptOverride: agentInstance.SystemPrompt,
		Temperature:          agentInstance.Config.Temperature,
		MaxOutputTokens:      agentInstance.Config.MaxTokens,
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}

	stepMessages := make([]chat.Message, 0, len(messages)+1)
	stepMessages = append(stepMessages, messages...)
	stepMessages = append(stepMessages, chat.Message{
		Role:    "user",
		Content: fmt.Sprintf("Execute plan step %d: %s\n\nContext: %s", step.Index, stepPrompt, stepContext(plan, step)),
	})

	reply, err := e.gateway.GenerateStructuredReply(ctx, stepMessages, config, nil)
	if err != nil {
		sr.Status = "failed"
		sr.Error = err.Error()
		return sr
	}
	sr.TokensUsed = completionTokens(reply)
	sr.Result = reply.Content
	sr.Status = "completed"
	return sr
}

// parsePlan extracts a Plan from an LLM response.
func parsePlan(content string) (Plan, error) {
	content = strings.TrimSpace(content)
	var plan Plan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		// Try to find JSON block in markdown.
		if start := strings.Index(content, "{"); start >= 0 {
			if end := strings.LastIndex(content, "}"); end > start {
				if err2 := json.Unmarshal([]byte(content[start:end+1]), &plan); err2 != nil {
					return Plan{}, fmt.Errorf("failed to parse plan: %w", err2)
				}
				return plan, nil
			}
		}
		return Plan{}, fmt.Errorf("failed to parse plan: %w", err)
	}
	return plan, nil
}

// topologicalOrder returns step indices in dependency-respecting order.
func topologicalOrder(steps []PlanStep) []int {
	if len(steps) == 0 {
		return nil
	}
	// Build index -> step mapping and dependency sets.
	stepByIndex := make(map[int]*PlanStep, len(steps))
	for i := range steps {
		stepByIndex[steps[i].Index] = &steps[i]
	}

	order := make([]int, 0, len(steps))
	visited := make(map[int]bool)

	var visit func(idx int)
	visit = func(idx int) {
		if visited[idx] {
			return
		}
		visited[idx] = true
		if s, ok := stepByIndex[idx]; ok {
			for _, dep := range s.DependsOn {
				visit(dep)
			}
		}
		order = append(order, idx)
	}

	for i := range steps {
		visit(steps[i].Index)
	}
	return order
}

func stepContext(plan Plan, step PlanStep) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Goal: %s\n", plan.Goal))
	if len(step.DependsOn) > 0 {
		builder.WriteString(fmt.Sprintf("This step depends on steps: %v\n", step.DependsOn))
	}
	if len(step.Input) > 0 {
		inputJSON, _ := json.Marshal(step.Input)
		builder.WriteString(fmt.Sprintf("Input: %s\n", string(inputJSON)))
	}
	return builder.String()
}

func synthesizeFinalAnswer(plan Plan, results []StepResult) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Plan completed: %s\n\n", plan.Goal))
	for _, r := range results {
		if r.Status == "completed" && r.Result != "" {
			builder.WriteString(fmt.Sprintf("Step %d (%s): %s\n", r.Index, r.Title, r.Result))
		}
	}
	return builder.String()
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
