package relay

import (
	"fmt"
	"sync"

	"oblivious/server/internal/relay/types"
)

type PricingStore struct {
	mu     sync.RWMutex
	prices map[string]map[types.APIType]map[types.UsageDimension]float64
}

func NewPricingStore() *PricingStore {
	return &PricingStore{
		prices: make(map[string]map[types.APIType]map[types.UsageDimension]float64),
	}
}

func NewPricingStoreWithDefaults() *PricingStore {
	store := NewPricingStore()
	// OpenAI defaults (approximate, per 1K tokens)
	defaults := map[types.APIType]map[types.UsageDimension]float64{
		types.APITypeChat: {
			types.DimPromptTokens:     0.002,
			types.DimCompletionTokens: 0.008,
		},
		types.APITypeCompletions: {
			types.DimPromptTokens:     0.002,
			types.DimCompletionTokens: 0.008,
		},
		types.APITypeEmbeddings: {
			types.DimPromptTokens: 0.0001,
		},
		types.APITypeImageGen: {
			types.DimImageCount: 0.004,
		},
	}
	for apiType, dims := range defaults {
		for dim, price := range dims {
			store.SetPrice("gpt-4o", apiType, dim, price)
			store.SetPrice("gpt-4o-mini", apiType, dim, price*0.1)
		}
	}
	return store
}

func (s *PricingStore) SetPrice(model string, apiType types.APIType, dim types.UsageDimension, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prices[model] == nil {
		s.prices[model] = make(map[types.APIType]map[types.UsageDimension]float64)
	}
	if s.prices[model][apiType] == nil {
		s.prices[model][apiType] = make(map[types.UsageDimension]float64)
	}
	s.prices[model][apiType][dim] = price
}

func (s *PricingStore) GetPrice(model string, apiType types.APIType, dim types.UsageDimension) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.prices[model] == nil || s.prices[model][apiType] == nil {
		return 0, fmt.Errorf("price not found for model=%s apiType=%s dim=%s", model, apiType, dim)
	}
	price := s.prices[model][apiType][dim]
	return price, nil
}

func (s *PricingStore) CalculateCost(model string, apiType types.APIType, usage *types.Usage) float64 {
	var total float64
	if usage.PromptTokens > 0 {
		if price, err := s.GetPrice(model, apiType, types.DimPromptTokens); err == nil {
			total += price * float64(usage.PromptTokens)
		}
	}
	if usage.CompletionTokens > 0 {
		if price, err := s.GetPrice(model, apiType, types.DimCompletionTokens); err == nil {
			total += price * float64(usage.CompletionTokens)
		}
	}
	if usage.ImageCount > 0 {
		if price, err := s.GetPrice(model, apiType, types.DimImageCount); err == nil {
			total += price * float64(usage.ImageCount)
		}
	}
	if usage.AudioSeconds > 0 {
		if price, err := s.GetPrice(model, apiType, types.DimAudioSeconds); err == nil {
			total += price * usage.AudioSeconds
		}
	}
	return total
}
