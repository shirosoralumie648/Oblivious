package channel

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestOpenAIAdapter_EstimateUsageForSupportedRequests(t *testing.T) {
	adapter := &OpenAIAdapter{}

	tests := []struct {
		name   string
		req    *types.ProviderRequest
		assert func(t *testing.T, usage *types.Usage)
	}{
		{
			name: "chat tokens",
			req: &types.ProviderRequest{
				APIType: types.APITypeChat,
				Messages: []types.Message{
					{Role: "user", Content: "hello world"},
					{Role: "assistant", Content: "hi"},
				},
				MaxTokens: 25,
			},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.PromptTokens <= 0 || usage.CompletionTokens != 25 {
					t.Fatalf("unexpected chat usage: %+v", usage)
				}
			},
		},
		{
			name: "image count",
			req:  &types.ProviderRequest{APIType: types.APITypeImageGen},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.ImageCount != 1 {
					t.Fatalf("expected one image, got %+v", usage)
				}
			},
		},
		{
			name: "audio estimate",
			req:  &types.ProviderRequest{APIType: types.APITypeAudioSpeech, Input: "hello world"},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.AudioSeconds <= 0 {
					t.Fatalf("expected positive audio seconds, got %+v", usage)
				}
			},
		},
		{
			name: "moderation tokens",
			req:  &types.ProviderRequest{APIType: types.APITypeModeration, Input: "check this"},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.PromptTokens <= 0 {
					t.Fatalf("expected moderation prompt tokens, got %+v", usage)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := adapter.EstimateUsage(tt.req)
			if usage == nil {
				t.Fatal("EstimateUsage returned nil")
			}
			tt.assert(t, usage)
		})
	}
}
