package agent

import (
	"testing"
)

// TestModelRouterIntegration verifies ModelRouter.SelectModel with iteration context
func TestModelRouterIntegration(t *testing.T) {
	rules := []ModelRoutingRule{
		{
			TargetModel:   "gpt-4",
			MinInputChars: 100,
		},
		{
			TargetModel:        "claude-3-sonnet",
			MinIteration:       2,
			RequiresToolResult: true,
		},
		{
			TargetModel: "gpt-3.5-turbo",
			Keywords:    []string{"summary", "summarize"},
		},
	}

	router := &ModelRouter{
		Rules:    rules,
		Fallback: "gpt-4o-mini",
	}

	tests := []struct {
		name     string
		ctx      IterationContext
		expected string
	}{
		{
			name: "fallback - no rules match",
			ctx: IterationContext{
				InputText:       "short",
				Iteration:       1,
				HasToolResult:   false,
				InputCharLength: 5,
			},
			expected: "gpt-4o-mini",
		},
		{
			name: "rule 1 - long input",
			ctx: IterationContext{
				InputText:       string(make([]byte, 150)),
				Iteration:       1,
				HasToolResult:   false,
				InputCharLength: 150,
			},
			expected: "gpt-4",
		},
		{
			name: "rule 2 - iteration 2 with tool result",
			ctx: IterationContext{
				InputText:       "test",
				Iteration:       2,
				HasToolResult:   true,
				InputCharLength: 4,
			},
			expected: "claude-3-sonnet",
		},
		{
			name: "rule 3 - keyword match",
			ctx: IterationContext{
				InputText:       "please summarize this document",
				Iteration:       1,
				HasToolResult:   false,
				InputCharLength: 31,
			},
			expected: "gpt-3.5-turbo",
		},
		{
			name: "first match wins - long input with keyword",
			ctx: IterationContext{
				InputText:       string(make([]byte, 150)) + " summarize",
				Iteration:       1,
				HasToolResult:   false,
				InputCharLength: 160,
			},
			expected: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.SelectModel(tt.ctx)
			if result != tt.expected {
				t.Errorf("SelectModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestModelRouterWithRunnerStruct verifies Runner has ModelRouter field
func TestModelRouterWithRunnerStruct(t *testing.T) {
	router := &ModelRouter{
		Rules:    []ModelRoutingRule{},
		Fallback: "gpt-4o-mini",
	}

	runner := &Runner{
		ModelRouter: router,
		config:      DefaultRunnerConfig(),
	}

	if runner.ModelRouter == nil {
		t.Error("Runner.ModelRouter should not be nil")
	}

	if runner.ModelRouter.Fallback != "gpt-4o-mini" {
		t.Errorf("Expected fallback gpt-4o-mini, got %s", runner.ModelRouter.Fallback)
	}
}

// TestIterationContextMatching verifies iteration context properties
func TestIterationContextMatching(t *testing.T) {
	tests := []struct {
		name        string
		rule        ModelRoutingRule
		ctx         IterationContext
		shouldMatch bool
	}{
		{
			name: "min chars not met",
			rule: ModelRoutingRule{
				MinInputChars: 100,
			},
			ctx: IterationContext{
				InputCharLength: 50,
			},
			shouldMatch: false,
		},
		{
			name: "min chars met",
			rule: ModelRoutingRule{
				MinInputChars: 100,
			},
			ctx: IterationContext{
				InputCharLength: 150,
			},
			shouldMatch: true,
		},
		{
			name: "max chars exceeded",
			rule: ModelRoutingRule{
				MaxInputChars: 100,
			},
			ctx: IterationContext{
				InputCharLength: 150,
			},
			shouldMatch: false,
		},
		{
			name: "iteration requirement not met",
			rule: ModelRoutingRule{
				MinIteration: 3,
			},
			ctx: IterationContext{
				Iteration: 2,
			},
			shouldMatch: false,
		},
		{
			name: "tool result required but absent",
			rule: ModelRoutingRule{
				RequiresToolResult: true,
			},
			ctx: IterationContext{
				HasToolResult: false,
			},
			shouldMatch: false,
		},
		{
			name: "keyword not present",
			rule: ModelRoutingRule{
				Keywords: []string{"urgent", "critical"},
			},
			ctx: IterationContext{
				InputText: "normal request",
			},
			shouldMatch: false,
		},
		{
			name: "keyword present",
			rule: ModelRoutingRule{
				Keywords: []string{"urgent", "critical"},
			},
			ctx: IterationContext{
				InputText: "this is URGENT",
			},
			shouldMatch: true,
		},
		{
			name: "all conditions met",
			rule: ModelRoutingRule{
				MinInputChars:      50,
				MinIteration:       2,
				RequiresToolResult: true,
				Keywords:           []string{"analyze"},
			},
			ctx: IterationContext{
				InputText:       "please analyze this data carefully",
				InputCharLength: 100,
				Iteration:       3,
				HasToolResult:   true,
			},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesRule(tt.rule, tt.ctx)
			if result != tt.shouldMatch {
				t.Errorf("matchesRule() = %v, want %v", result, tt.shouldMatch)
			}
		})
	}
}
