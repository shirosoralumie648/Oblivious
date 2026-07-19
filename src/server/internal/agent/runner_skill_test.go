package agent

import (
	"context"
	"strings"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

func TestExecuteReActWithSkillSelection(t *testing.T) {
	// Setup test skills
	skills := []Skill{
		{
			Name:         "Weather",
			Instructions: "Provide weather information",
			Triggers:     []string{"weather", "forecast", "temperature"},
			ToolNames:    []string{"get_weather", "get_forecast"},
		},
		{
			Name:         "Calculator",
			Instructions: "Perform mathematical calculations",
			Triggers:     []string{"calculate", "math", "compute"},
			ToolNames:    []string{"calculate"},
		},
		{
			Name:         "Database",
			Instructions: "Query database records",
			Triggers:     []string{"database", "query", "search records"},
			ToolNames:    []string{"db_query"},
		},
	}

	tests := []struct {
		name              string
		input             string
		maxSkills         int
		expectedSkillsMin int
		expectedSkillsMax int
	}{
		{
			name:              "Weather query selects weather skill",
			input:             "What's the weather like today?",
			maxSkills:         3,
			expectedSkillsMin: 1,
			expectedSkillsMax: 1,
		},
		{
			name:              "Math query selects calculator skill",
			input:             "Calculate 25 * 47",
			maxSkills:         3,
			expectedSkillsMin: 1,
			expectedSkillsMax: 1,
		},
		{
			name:              "Multiple triggers select top skills",
			input:             "Calculate the weather forecast",
			maxSkills:         2,
			expectedSkillsMin: 2,
			expectedSkillsMax: 2,
		},
		{
			name:              "No matching triggers",
			input:             "Hello, how are you?",
			maxSkills:         3,
			expectedSkillsMin: 0,
			expectedSkillsMax: 0,
		},
		{
			name:              "MaxSkills limits results",
			input:             "Calculate weather forecast and query database records",
			maxSkills:         2,
			expectedSkillsMin: 2,
			expectedSkillsMax: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := &SkillSelector{}
			scored := selector.SelectSkills(tt.input, skills, tt.maxSkills)

			if len(scored) < tt.expectedSkillsMin || len(scored) > tt.expectedSkillsMax {
				t.Errorf("Expected %d-%d skills, got %d", tt.expectedSkillsMin, tt.expectedSkillsMax, len(scored))
			}

			// Verify scores are in descending order
			for i := 1; i < len(scored); i++ {
				if scored[i].Score > scored[i-1].Score {
					t.Errorf("Skills not sorted by score: %d > %d", scored[i].Score, scored[i-1].Score)
				}
			}
		})
	}
}

func TestBuildToolsFromSkills(t *testing.T) {
	baseTools := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "get_weather",
				"description": "Get weather information",
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "calculate",
				"description": "Perform calculation",
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "db_query",
				"description": "Query database",
			},
		},
	}

	tests := []struct {
		name          string
		activeSkills  []Skill
		expectedTools int
		expectedNames []string
	}{
		{
			name: "Single skill filters tools",
			activeSkills: []Skill{
				{
					Name:      "Weather",
					ToolNames: []string{"get_weather"},
				},
			},
			expectedTools: 1,
			expectedNames: []string{"get_weather"},
		},
		{
			name: "Multiple skills combine tools",
			activeSkills: []Skill{
				{
					Name:      "Weather",
					ToolNames: []string{"get_weather"},
				},
				{
					Name:      "Calculator",
					ToolNames: []string{"calculate"},
				},
			},
			expectedTools: 2,
			expectedNames: []string{"get_weather", "calculate"},
		},
		{
			name:          "No skills returns all tools",
			activeSkills:  []Skill{},
			expectedTools: 3,
			expectedNames: []string{"get_weather", "calculate", "db_query"},
		},
		{
			name: "Non-existent tool names filter nothing",
			activeSkills: []Skill{
				{
					Name:      "Unknown",
					ToolNames: []string{"nonexistent"},
				},
			},
			expectedTools: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := buildToolsFromSkills(nil, tt.activeSkills, baseTools)

			if len(filtered) != tt.expectedTools {
				t.Errorf("Expected %d tools, got %d", tt.expectedTools, len(filtered))
			}

			// Verify expected tool names are present
			toolNames := make(map[string]bool)
			for _, tool := range filtered {
				if fn, ok := tool["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok {
						toolNames[name] = true
					}
				}
			}

			for _, expectedName := range tt.expectedNames {
				if !toolNames[expectedName] {
					t.Errorf("Expected tool %s not found", expectedName)
				}
			}
		})
	}
}

func TestInjectSkillInstructions(t *testing.T) {
	tests := []struct {
		name         string
		basePrompt   string
		activeSkills []Skill
		expectPrompt string
	}{
		{
			name:       "Single skill appends instructions",
			basePrompt: "You are a helpful assistant.",
			activeSkills: []Skill{
				{
					Name:         "Weather",
					Instructions: "Provide weather information",
				},
			},
			expectPrompt: "You are a helpful assistant.\n\nActive Skills:\n- Weather: Provide weather information\n",
		},
		{
			name:       "Multiple skills append all instructions",
			basePrompt: "System prompt",
			activeSkills: []Skill{
				{
					Name:         "Weather",
					Instructions: "Weather info",
				},
				{
					Name:         "Calculator",
					Instructions: "Math operations",
				},
			},
			expectPrompt: "System prompt\n\nActive Skills:\n- Weather: Weather info\n- Calculator: Math operations\n",
		},
		{
			name:         "No skills returns original prompt",
			basePrompt:   "Original prompt",
			activeSkills: []Skill{},
			expectPrompt: "Original prompt",
		},
		{
			name:       "Empty base prompt still adds skills",
			basePrompt: "",
			activeSkills: []Skill{
				{
					Name:         "Test",
					Instructions: "Test instructions",
				},
			},
			expectPrompt: "Active Skills:\n- Test: Test instructions\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := chat.ConversationConfig{
				SystemPromptOverride: tt.basePrompt,
			}
			result := injectSkillInstructions(config, tt.activeSkills)

			if result.SystemPromptOverride != tt.expectPrompt {
				t.Errorf("Expected prompt:\n%s\nGot:\n%s", tt.expectPrompt, result.SystemPromptOverride)
			}
		})
	}
}

func TestRunnerSkillSelectorIntegration(t *testing.T) {
	store := &fakeStore{}
	mockGateway := &skillMockStructuredGateway{
		reply: &chat.CompletionResponse{
			Content: "Mocked response",
			Usage: &chat.CompletionUsage{
				TotalTokens: 100,
			},
		},
	}

	runner := &Runner{
		store:         store,
		gateway:       mockGateway,
		executor:      newAuthorizedToolExecutorForTest(t, nil),
		config:        DefaultRunnerConfig(),
		SkillSelector: &SkillSelector{},
	}

	skills := []Skill{
		{
			Name:         "Weather",
			Instructions: "Weather information",
			Triggers:     []string{"weather"},
			ToolNames:    []string{"get_weather"},
		},
	}

	req := &RunRequest{
		Session: auth.Session{
			OrganizationID: "org-1",
			User:           auth.User{ID: "user-1"},
		},
		Agent: &Agent{
			ID:           "agent-1",
			Model:        "gpt-4",
			SystemPrompt: "Test prompt",
			Config:       Config{},
		},
		ConversationID: "conv-1",
		InputText:      "What's the weather today?",
		Skills:         skills,
		MaxSkills:      3,
	}

	_, err := runner.ExecuteReAct(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteReAct returned error: %v", err)
	}
	if !strings.Contains(mockGateway.lastConfig.SystemPromptOverride, "Active Skills:") ||
		!strings.Contains(mockGateway.lastConfig.SystemPromptOverride, "Weather: Weather information") {
		t.Fatalf("expected selected skill instructions in prompt, got %q", mockGateway.lastConfig.SystemPromptOverride)
	}
}

type skillMockStructuredGateway struct {
	reply      *chat.CompletionResponse
	lastConfig chat.ConversationConfig
	lastTools  []map[string]any
}

func (m *skillMockStructuredGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return m.reply.Content, nil
}

func (m *skillMockStructuredGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	if onChunk == nil {
		return nil
	}
	return onChunk(m.reply.Content)
}

func (m *skillMockStructuredGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	m.lastConfig = config
	m.lastTools = tools
	return m.reply, nil
}
