package relay

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestPricing_GetPrice(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 0.002)
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 0.008)

	price, err := store.GetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0.002 {
		t.Fatalf("expected 0.002, got %f", price)
	}
}

func TestPricing_GetPrice_NotFound(t *testing.T) {
	store := NewPricingStore()
	_, err := store.GetPrice("unknown-model", types.APITypeChat, types.DimPromptTokens)
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestPricing_CalculateCost(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)     // $2 per 1K tokens
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0) // $8 per 1K tokens

	cost := store.CalculateCost("gpt-4o", types.APITypeChat, &types.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	})

	// 1000 * 2.0 + 500 * 8.0 = 2000 + 4000 = 6000
	expected := 6000.0
	if cost != expected {
		t.Fatalf("expected %f, got %f", expected, cost)
	}
}

func TestPricing_DefaultPricing(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	price, err := store.GetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price <= 0 {
		t.Fatal("default pricing should return positive price")
	}
}

func TestPricing_DefaultPricingCoversCommercialSupportedBillingDimensions(t *testing.T) {
	store := NewPricingStoreWithDefaults()

	tests := []struct {
		name    string
		apiType types.APIType
		usage   *types.Usage
	}{
		{name: "responses", apiType: types.APITypeResponses, usage: &types.Usage{PromptTokens: 100, CompletionTokens: 50}},
		{name: "image edit", apiType: types.APITypeImageEdit, usage: &types.Usage{ImageCount: 1}},
		{name: "image variation", apiType: types.APITypeImageVar, usage: &types.Usage{ImageCount: 1}},
		{name: "audio speech", apiType: types.APITypeAudioSpeech, usage: &types.Usage{AudioSeconds: 3}},
		{name: "audio transcription", apiType: types.APITypeAudioSTT, usage: &types.Usage{AudioSeconds: 3}},
		{name: "audio translation", apiType: types.APITypeAudioTranslate, usage: &types.Usage{AudioSeconds: 3}},
		{name: "moderation", apiType: types.APITypeModeration, usage: &types.Usage{PromptTokens: 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := store.CalculateCost("gpt-4o", tt.apiType, tt.usage)
			if cost <= 0 {
				t.Fatalf("expected positive default cost for %s, got %f", tt.apiType.String(), cost)
			}
		})
	}
}
