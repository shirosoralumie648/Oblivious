package agent

import (
	"strings"
	"testing"
)

func TestModelRouter_SelectModel_NoRules(t *testing.T) {
	router := &ModelRouter{Fallback: "fallback"}
	ctx := IterationContext{InputText: "hello", Iteration: 1}
	if got := router.SelectModel(ctx); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
}

func TestModelRouter_SelectModel_RuleMatches(t *testing.T) {
	cases := []struct {
		name     string
		rules    []ModelRoutingRule
		ctx      IterationContext
		expected string
	}{
		{
			name:     "min input chars",
			rules:    []ModelRoutingRule{{TargetModel: "large", MinInputChars: 100}},
			ctx:      IterationContext{InputText: strings.Repeat("x", 100), Iteration: 1},
			expected: "large",
		},
		{
			name:     "min input chars not met",
			rules:    []ModelRoutingRule{{TargetModel: "large", MinInputChars: 100}},
			ctx:      IterationContext{InputText: "short", Iteration: 1},
			expected: "fallback",
		},
		{
			name:     "max input chars",
			rules:    []ModelRoutingRule{{TargetModel: "small", MaxInputChars: 10}},
			ctx:      IterationContext{InputText: "short", Iteration: 1},
			expected: "small",
		},
		{
			name:     "max input chars exceeded",
			rules:    []ModelRoutingRule{{TargetModel: "small", MaxInputChars: 10}},
			ctx:      IterationContext{InputText: strings.Repeat("x", 20), Iteration: 1},
			expected: "fallback",
		},
		{
			name:     "min iteration",
			rules:    []ModelRoutingRule{{TargetModel: "advanced", MinIteration: 3}},
			ctx:      IterationContext{InputText: "query", Iteration: 3},
			expected: "advanced",
		},
		{
			name:     "min iteration not met",
			rules:    []ModelRoutingRule{{TargetModel: "advanced", MinIteration: 3}},
			ctx:      IterationContext{InputText: "query", Iteration: 2},
			expected: "fallback",
		},
		{
			name:     "requires tool result",
			rules:    []ModelRoutingRule{{TargetModel: "refiner", RequiresToolResult: true}},
			ctx:      IterationContext{InputText: "refine", Iteration: 2, HasToolResult: true},
			expected: "refiner",
		},
		{
			name:     "requires tool result not met",
			rules:    []ModelRoutingRule{{TargetModel: "refiner", RequiresToolResult: true}},
			ctx:      IterationContext{InputText: "refine", Iteration: 2, HasToolResult: false},
			expected: "fallback",
		},
		{
			name:     "keyword match case insensitive",
			rules:    []ModelRoutingRule{{TargetModel: "coding", Keywords: []string{"code", "debug"}}},
			ctx:      IterationContext{InputText: "Please CODE this task", Iteration: 1},
			expected: "coding",
		},
		{
			name:     "keyword not present",
			rules:    []ModelRoutingRule{{TargetModel: "coding", Keywords: []string{"code", "debug"}}},
			ctx:      IterationContext{InputText: "general query", Iteration: 1},
			expected: "fallback",
		},
		{
			name: "first matching rule wins",
			rules: []ModelRoutingRule{
				{TargetModel: "first", MinInputChars: 10},
				{TargetModel: "second", MinInputChars: 5},
			},
			ctx:      IterationContext{InputText: strings.Repeat("x", 15), Iteration: 1},
			expected: "first",
		},
		{
			name: "compound rule all conditions",
			rules: []ModelRoutingRule{
				{TargetModel: "compound", MinInputChars: 10, MinIteration: 2, Keywords: []string{"complex"}},
			},
			ctx:      IterationContext{InputText: "handle this complex situation carefully", Iteration: 2},
			expected: "compound",
		},
		{
			name: "compound rule one condition fails",
			rules: []ModelRoutingRule{
				{TargetModel: "compound", MinInputChars: 10, MinIteration: 2, Keywords: []string{"complex"}},
			},
			ctx:      IterationContext{InputText: "handle this complex situation carefully", Iteration: 1},
			expected: "fallback",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := &ModelRouter{Rules: tc.rules, Fallback: "fallback"}
			got := router.SelectModel(tc.ctx)
			if got != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestModelRouter_SelectModel_InputCharLengthComputed(t *testing.T) {
	router := &ModelRouter{
		Rules:    []ModelRoutingRule{{TargetModel: "large", MinInputChars: 5}},
		Fallback: "fallback",
	}
	ctx := IterationContext{InputText: "hello world", Iteration: 1}
	if got := router.SelectModel(ctx); got != "large" {
		t.Fatalf("expected large when computed length meets threshold, got %s", got)
	}
}
