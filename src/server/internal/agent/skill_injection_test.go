package agent

import (
	"testing"

	"oblivious/server/internal/chat"
)

// TestSkillSelectionFiltering verifies that SkillSelector correctly filters
// skills based on input triggers and respects the maxSkills limit.
func TestSkillSelectionFiltering(t *testing.T) {
	selector := &SkillSelector{}

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

	// Test 1: Weather query selects weather skill
	scored := selector.SelectSkills("What's the weather today?", skills, 3)
	if len(scored) != 1 {
		t.Errorf("Test 1: Expected 1 skill, got %d", len(scored))
	}
	if len(scored) > 0 && scored[0].Skill.Name != "Weather" {
		t.Errorf("Test 1: Expected Weather skill, got %s", scored[0].Skill.Name)
	}

	// Test 2: Math query selects calculator skill
	scored = selector.SelectSkills("Calculate 25 * 47", skills, 3)
	if len(scored) != 1 {
		t.Errorf("Test 2: Expected 1 skill, got %d", len(scored))
	}
	if len(scored) > 0 && scored[0].Skill.Name != "Calculator" {
		t.Errorf("Test 2: Expected Calculator skill, got %s", scored[0].Skill.Name)
	}

	// Test 3: Multiple triggers select multiple skills
	scored = selector.SelectSkills("Calculate the weather forecast", skills, 3)
	if len(scored) != 2 {
		t.Errorf("Test 3: Expected 2 skills, got %d", len(scored))
	}

	// Test 4: MaxSkills limits results
	scored = selector.SelectSkills("Calculate weather forecast and query database", skills, 2)
	if len(scored) > 2 {
		t.Errorf("Test 4: Expected max 2 skills, got %d", len(scored))
	}

	// Test 5: No matching triggers returns empty
	scored = selector.SelectSkills("Hello, how are you?", skills, 3)
	if len(scored) != 0 {
		t.Errorf("Test 5: Expected 0 skills, got %d", len(scored))
	}
}

// TestBuildToolsFromSkillsFiltering verifies that buildToolsFromSkills
// correctly filters tools based on active skills' ToolNames.
func TestBuildToolsFromSkillsFiltering(t *testing.T) {
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

	// Test 1: Single skill filters to one tool
	activeSkills := []Skill{
		{Name: "Weather", ToolNames: []string{"get_weather"}},
	}
	filtered := buildToolsFromSkills(nil, activeSkills, baseTools)
	if len(filtered) != 1 {
		t.Errorf("Test 1: Expected 1 tool, got %d", len(filtered))
	}

	// Test 2: Multiple skills combine tools
	activeSkills = []Skill{
		{Name: "Weather", ToolNames: []string{"get_weather"}},
		{Name: "Calculator", ToolNames: []string{"calculate"}},
	}
	filtered = buildToolsFromSkills(nil, activeSkills, baseTools)
	if len(filtered) != 2 {
		t.Errorf("Test 2: Expected 2 tools, got %d", len(filtered))
	}

	// Test 3: No skills returns all tools
	filtered = buildToolsFromSkills(nil, []Skill{}, baseTools)
	if len(filtered) != 3 {
		t.Errorf("Test 3: Expected 3 tools (all), got %d", len(filtered))
	}

	// Test 4: Non-existent tool names filter to zero
	activeSkills = []Skill{
		{Name: "Unknown", ToolNames: []string{"nonexistent"}},
	}
	filtered = buildToolsFromSkills(nil, activeSkills, baseTools)
	if len(filtered) != 0 {
		t.Errorf("Test 4: Expected 0 tools, got %d", len(filtered))
	}
}

// TestInjectSkillInstructionsIntegration verifies that injectSkillInstructions
// correctly appends skill instructions to the system prompt.
func TestInjectSkillInstructionsIntegration(t *testing.T) {
	// Test 1: Single skill appends instructions
	config := chat.ConversationConfig{
		SystemPromptOverride: "You are a helpful assistant.",
	}
	activeSkills := []Skill{
		{Name: "Weather", Instructions: "Provide weather information"},
	}
	result := injectSkillInstructions(config, activeSkills)
	expected := "You are a helpful assistant.\n\nActive Skills:\n- Weather: Provide weather information\n"
	if result.SystemPromptOverride != expected {
		t.Errorf("Test 1: Expected:\n%s\nGot:\n%s", expected, result.SystemPromptOverride)
	}

	// Test 2: Multiple skills
	activeSkills = []Skill{
		{Name: "Weather", Instructions: "Weather info"},
		{Name: "Calculator", Instructions: "Math operations"},
	}
	config.SystemPromptOverride = "System prompt"
	result = injectSkillInstructions(config, activeSkills)
	expected = "System prompt\n\nActive Skills:\n- Weather: Weather info\n- Calculator: Math operations\n"
	if result.SystemPromptOverride != expected {
		t.Errorf("Test 2: Expected:\n%s\nGot:\n%s", expected, result.SystemPromptOverride)
	}

	// Test 3: No skills returns original
	config.SystemPromptOverride = "Original"
	result = injectSkillInstructions(config, []Skill{})
	if result.SystemPromptOverride != "Original" {
		t.Errorf("Test 3: Expected 'Original', got '%s'", result.SystemPromptOverride)
	}

	// Test 4: Empty base prompt
	config.SystemPromptOverride = ""
	activeSkills = []Skill{
		{Name: "Test", Instructions: "Test instructions"},
	}
	result = injectSkillInstructions(config, activeSkills)
	expected = "Active Skills:\n- Test: Test instructions\n"
	if result.SystemPromptOverride != expected {
		t.Errorf("Test 4: Expected:\n%s\nGot:\n%s", expected, result.SystemPromptOverride)
	}
}
