package relay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	if !errors.Is(err, ErrRelayPriceNotConfigured) {
		t.Fatalf("expected ErrRelayPriceNotConfigured, got %v", err)
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

func TestPricingCalculateCostTreatsNilUsageAsZero(t *testing.T) {
	store := NewPricingStoreWithDefaults()

	if cost := store.CalculateCost("gpt-4o", types.APITypeBatch, nil); cost != 0 {
		t.Fatalf("expected nil usage cost to be zero, got %f", cost)
	}
}

func TestPricingCalculateCostForGroupAppliesGroupMultiplier(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0)
	store.ApplyMultipliers(nil, map[string]float64{"vip": 0.5})

	usage := &types.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}

	baseCost := store.CalculateCost("gpt-4o", types.APITypeChat, usage)
	if baseCost != 6000 {
		t.Fatalf("expected base cost 6000, got %f", baseCost)
	}

	groupCost := store.CalculateCostForGroup("gpt-4o", types.APITypeChat, usage, "vip")
	if groupCost != 3000 {
		t.Fatalf("expected vip group cost 3000, got %f", groupCost)
	}

	if cost := store.CalculateCostForGroup("gpt-4o", types.APITypeChat, usage, "standard"); cost != 6000 {
		t.Fatalf("expected default group cost 6000, got %f", cost)
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

func TestPricingDefaultsCoverSupportedAudioAndModerationRoutes(t *testing.T) {
	store := NewPricingStoreWithDefaults()

	for _, tt := range []struct {
		name    string
		apiType types.APIType
		usage   *types.Usage
	}{
		{name: "audio speech", apiType: types.APITypeAudioSpeech, usage: &types.Usage{AudioSeconds: 2}},
		{name: "audio transcription", apiType: types.APITypeAudioSTT, usage: &types.Usage{AudioSeconds: 60}},
		{name: "audio translation", apiType: types.APITypeAudioTranslate, usage: &types.Usage{AudioSeconds: 60}},
		{name: "moderation", apiType: types.APITypeModeration, usage: &types.Usage{PromptTokens: 4, TotalTokens: 4}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := store.CalculateCostStrict("gpt-4o", tt.apiType, tt.usage)
			if err != nil {
				t.Fatalf("expected default price for %s: %v", tt.apiType, err)
			}
			if cost <= 0 {
				t.Fatalf("expected positive default cost, got %f", cost)
			}
		})
	}
}

func TestPricingCalculateCostStrictRejectsMissingDimension(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)

	_, err := store.CalculateCostStrict("gpt-4o", types.APITypeChat, &types.Usage{
		PromptTokens:     100,
		CompletionTokens: 10,
	})
	if !errors.Is(err, ErrRelayPriceNotConfigured) {
		t.Fatalf("expected missing completion price error, got %v", err)
	}
}

func TestPricingCalculateCostStrictUsesTotalTokensWhenPromptBreakdownMissing(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimTotalTokens, 3.0)

	cost, err := store.CalculateCostStrict("gpt-4o", types.APITypeChat, &types.Usage{TotalTokens: 25})
	if err != nil {
		t.Fatalf("calculate total-token cost: %v", err)
	}
	if cost != 75 {
		t.Fatalf("expected total-token cost 75, got %f", cost)
	}
}

func TestPricingCalculateCostStrictAllowsModelAgnosticFileStoragePricing(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("", types.APITypeFiles, types.DimStorageBytes, 0.000000001)

	cost, err := store.CalculateCostStrict("", types.APITypeFiles, &types.Usage{StorageBytes: 1024})
	if err != nil {
		t.Fatalf("calculate file storage cost: %v", err)
	}
	if cost != 0.000001024 {
		t.Fatalf("expected file storage cost 0.000001024, got %.12f", cost)
	}
}

func TestPricingQuoteIncludesCatalogSnapshotAndGroupMultiplier(t *testing.T) {
	effectiveFrom := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	store, err := NewPricingStoreFromEntries([]PricingEntry{
		{
			ID:            "rpe_prompt_v1",
			APIType:       types.APITypeChat,
			Model:         "gpt-4o",
			Dimension:     types.DimPromptTokens,
			UnitCost:      2,
			Markup:        1.5,
			Currency:      "quota",
			Source:        "initial_catalog",
			EffectiveFrom: &effectiveFrom,
		},
		{
			ID:            "rpe_completion_v1",
			APIType:       types.APITypeChat,
			Model:         "gpt-4o",
			Dimension:     types.DimCompletionTokens,
			UnitCost:      8,
			Markup:        1.5,
			Currency:      "quota",
			Source:        "initial_catalog",
			EffectiveFrom: &effectiveFrom,
		},
	})
	if err != nil {
		t.Fatalf("new pricing store: %v", err)
	}
	store.ApplyMultipliers(nil, map[string]float64{"vip": 0.5})

	quote, err := store.QuoteUsageForGroupStrict("gpt-4o", types.APITypeChat, &types.Usage{
		PromptTokens:     100,
		CompletionTokens: 10,
	}, "vip")
	if err != nil {
		t.Fatalf("quote usage: %v", err)
	}

	if quote.Subtotal != 420 || quote.GroupMultiplier != 0.5 || quote.TotalCost != 210 {
		t.Fatalf("unexpected quote totals: %+v", quote)
	}
	if quote.Currency != "quota" || quote.Source != "initial_catalog" || quote.EffectiveFrom == nil || !quote.EffectiveFrom.Equal(effectiveFrom) {
		t.Fatalf("unexpected quote catalog summary: %+v", quote)
	}
	if len(quote.Dimensions) != 2 {
		t.Fatalf("expected 2 quote dimensions, got %+v", quote.Dimensions)
	}
	if quote.Dimensions[0].PricingEntryID != "rpe_prompt_v1" || quote.Dimensions[0].UnitCost != 2 || quote.Dimensions[0].Markup != 1.5 || quote.Dimensions[0].UnitPrice != 3 || quote.Dimensions[0].Amount != 300 {
		t.Fatalf("unexpected prompt dimension snapshot: %+v", quote.Dimensions[0])
	}
}

func TestLoadPricingStoreFromSQLLoadsActiveCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	effectiveFrom := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, api_type, model, dimension, unit_cost").
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_type", "model", "dimension", "unit_cost", "markup", "currency", "source", "effective_from"}).
			AddRow("rpe_prompt_v1", "chat", "gpt-4o", "prompt_tokens", 2.0, 1.5, "quota", "initial_catalog", effectiveFrom).
			AddRow("rpe_completion_v1", "chat", "gpt-4o", "completion_tokens", 8.0, 1.5, "quota", "initial_catalog", effectiveFrom).
			AddRow("rpe_files_v1", "files", "", "storage_bytes", 0.000000001, 1.0, "quota", "initial_catalog", effectiveFrom))

	store, err := LoadPricingStoreFromSQL(context.Background(), db)
	if err != nil {
		t.Fatalf("load pricing store: %v", err)
	}
	cost, err := store.CalculateCostStrict("gpt-4o", types.APITypeChat, &types.Usage{
		PromptTokens:     100,
		CompletionTokens: 10,
	})
	if err != nil {
		t.Fatalf("calculate loaded cost: %v", err)
	}
	if cost != 420 {
		t.Fatalf("expected marked-up catalog cost 420, got %f", cost)
	}
	fileCost, err := store.CalculateCostStrict("", types.APITypeFiles, &types.Usage{StorageBytes: 2048})
	if err != nil {
		t.Fatalf("calculate loaded file storage cost: %v", err)
	}
	if fileCost != 0.000002048 {
		t.Fatalf("expected loaded file storage cost 0.000002048, got %.12f", fileCost)
	}
	quote, err := store.QuoteUsageForGroupStrict("gpt-4o", types.APITypeChat, &types.Usage{PromptTokens: 100}, "")
	if err != nil {
		t.Fatalf("quote loaded cost: %v", err)
	}
	if quote.Currency != "quota" || quote.Source != "initial_catalog" || len(quote.Dimensions) != 1 || quote.Dimensions[0].PricingEntryID != "rpe_prompt_v1" {
		t.Fatalf("expected loaded quote to preserve catalog metadata, got %+v", quote)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
