package channel

import (
	"testing"
)

func TestClaudeAdapter_ConvertResponseExtractsUsage(t *testing.T) {
	adapter := NewClaudeAdapter("https://api.anthropic.com", "sk-claude")

	resp, err := adapter.ConvertResponse([]byte(`{
		"id":"msg_123",
		"type":"message",
		"usage":{"input_tokens":17,"output_tokens":9}
	}`), false)
	if err != nil {
		t.Fatalf("ConvertResponse returned error: %v", err)
	}
	if resp == nil || resp.Usage == nil {
		t.Fatalf("expected normalized usage, got %+v", resp)
	}
	if resp.Usage.PromptTokens != 17 || resp.Usage.CompletionTokens != 9 || resp.Usage.TotalTokens != 26 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}
