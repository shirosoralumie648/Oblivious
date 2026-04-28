package agent

import (
	"testing"

	"oblivious/server/internal/chat"
)

func TestMarshalToolCallsRoundTrip(t *testing.T) {
	original := []ToolCall{
		{ID: "call_1", Name: "weather", Arguments: map[string]any{"city": "Beijing"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	data := MarshalToolCalls(original)
	if len(data) == 0 {
		t.Fatal("MarshalToolCalls should return non-empty JSON for non-empty input")
	}

	result := UnmarshalToolCalls(data)
	if len(result) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather" {
		t.Fatalf("unexpected first tool call: %+v", result[0])
	}
	if result[1].ID != "call_2" || result[1].Name != "datetime" {
		t.Fatalf("unexpected second tool call: %+v", result[1])
	}
}

func TestMarshalToolCallsEmpty(t *testing.T) {
	if data := MarshalToolCalls(nil); data != nil {
		t.Fatal("MarshalToolCalls should return nil for nil input")
	}
	if data := MarshalToolCalls([]ToolCall{}); data != nil {
		t.Fatal("MarshalToolCalls should return nil for empty slice")
	}
}

func TestUnmarshalToolCallsEmpty(t *testing.T) {
	if result := UnmarshalToolCalls(nil); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for nil data")
	}
	if result := UnmarshalToolCalls([]byte{}); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for empty data")
	}
}

func TestHasEnabledTools(t *testing.T) {
	if hasEnabledTools(nil) {
		t.Fatal("nil agent should not have enabled tools")
	}

	agent := &Agent{Tools: []Tool{}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with no tools should not have enabled tools")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "datetime", Type: "builtin", Enabled: false},
	}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with only disabled tools should not be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "disabled", Type: "builtin", Enabled: false},
		{Name: "enabled", Type: "builtin", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with at least one enabled tool should be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "mcp_tool", Type: "mcp", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with enabled MCP tool should be detected")
	}
}

func TestStreamContentSplitsWords(t *testing.T) {
	var chunks []string
	err := streamContent("hello world", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 'hello world', got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "hello " || chunks[1] != "world" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestStreamContentSingleWord(t *testing.T) {
	var chunks []string
	err := streamContent("ok", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for 'ok', got %d", len(chunks))
	}
	if chunks[0] != "ok" {
		t.Fatalf("unexpected chunk: %q", chunks[0])
	}
}

func TestStreamContentEmpty(t *testing.T) {
	err := streamContent("", func(chunk string) error {
		t.Fatal("should not call onChunk for empty content")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolCallsToChatToolCalls(t *testing.T) {
	input := []ToolCall{
		{ID: "call_1", Name: "weather.lookup", Arguments: map[string]any{"city": "Paris"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	result := toolCallsToChatToolCalls(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 chat tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Type != "function" {
		t.Fatalf("unexpected first chat tool call: %+v", result[0])
	}
	if result[0].Function.Name != "weather.lookup" {
		t.Fatalf("expected weather.lookup, got %q", result[0].Function.Name)
	}
	if result[0].Function.Arguments == "" {
		t.Fatal("arguments should be serialized JSON string")
	}
}

func TestToolCallsToChatToolCallsEmpty(t *testing.T) {
	if result := toolCallsToChatToolCalls(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := toolCallsToChatToolCalls([]ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestChatToolCallsToAgent(t *testing.T) {
	input := []chat.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "weather.lookup",
				Arguments: `{"city":"Paris"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "datetime",
				Arguments: "",
			},
		},
	}

	result := chatToolCallsToAgent(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 agent tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather.lookup" {
		t.Fatalf("unexpected first agent tool call: %+v", result[0])
	}
	city, ok := result[0].Arguments["city"].(string)
	if !ok || city != "Paris" {
		t.Fatalf("expected city=Paris in arguments, got %+v", result[0].Arguments)
	}
	if result[1].Arguments == nil {
		t.Fatal("empty arguments should result in empty map, not nil")
	}
}

func TestChatToolCallsToAgentEmpty(t *testing.T) {
	if result := chatToolCallsToAgent(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := chatToolCallsToAgent([]chat.ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestParseToolCallsFromResponse(t *testing.T) {
	// Simulates a raw LLM response map containing tool_calls.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id":   "call_abc",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
			map[string]any{
				"id":   "call_def",
				"type": "function",
				"function": map[string]any{
					"name":      "web_search",
					"arguments": `{"query":"golang news"}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_abc" || toolCalls[0].Name != "datetime" {
		t.Fatalf("unexpected first tool call: %+v", toolCalls[0])
	}
	if toolCalls[1].ID != "call_def" || toolCalls[1].Name != "web_search" {
		t.Fatalf("unexpected second tool call: %+v", toolCalls[1])
	}
	if toolCalls[1].Arguments["query"] != "golang news" {
		t.Fatalf("unexpected arguments: %+v", toolCalls[1].Arguments)
	}
}

func TestParseToolCallsFromResponseNoToolCalls(t *testing.T) {
	response := map[string]any{
		"content": "simple text response",
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatalf("expected nil tool calls, got %+v", toolCalls)
	}
}

func TestParseToolCallsFromResponseNilResponse(t *testing.T) {
	toolCalls, err := ParseToolCallsFromResponse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatal("expected nil tool calls for nil response")
	}
}

func TestParseToolCallsFromResponseMalformedEntries(t *testing.T) {
	// Missing function field should be skipped gracefully.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id": "call_skip",
			},
			map[string]any{
				"id":   "call_good",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call after skipping malformed, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_good" {
		t.Fatalf("expected call_good, got %q", toolCalls[0].ID)
	}
}
