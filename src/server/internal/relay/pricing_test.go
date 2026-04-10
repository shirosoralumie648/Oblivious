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
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0)   // $8 per 1K tokens

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
