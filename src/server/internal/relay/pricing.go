package relay

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"oblivious/server/internal/relay/types"
)

var ErrRelayPriceNotConfigured = fmt.Errorf("relay price not configured")

type PricingQuoteDimension struct {
	PricingEntryID string               `json:"pricingEntryId,omitempty"`
	Model          string               `json:"model"`
	APIType        string               `json:"apiType"`
	Dimension      types.UsageDimension `json:"dimension"`
	UnitCost       float64              `json:"unitCost"`
	Markup         float64              `json:"markup"`
	UnitPrice      float64              `json:"unitPrice"`
	Quantity       float64              `json:"quantity"`
	Amount         float64              `json:"amount"`
	Currency       string               `json:"currency"`
	Source         string               `json:"source"`
	EffectiveFrom  *time.Time           `json:"effectiveFrom,omitempty"`
}

type PricingQuote struct {
	Model             string                  `json:"model"`
	APIType           string                  `json:"apiType"`
	Currency          string                  `json:"currency,omitempty"`
	Source            string                  `json:"source,omitempty"`
	EffectiveFrom     *time.Time              `json:"effectiveFrom,omitempty"`
	Subtotal          float64                 `json:"subtotal"`
	GroupMultiplier   float64                 `json:"groupMultiplier"`
	ChannelMultiplier float64                 `json:"channelMultiplier"`
	TotalCost         float64                 `json:"totalCost"`
	Dimensions        []PricingQuoteDimension `json:"dimensions"`
}

type pricingCatalogEntry struct {
	ID            string
	APIType       types.APIType
	Currency      string
	Dimension     types.UsageDimension
	EffectiveFrom *time.Time
	Markup        float64
	Model         string
	Source        string
	UnitCost      float64
	UnitPrice     float64
}

type PricingStore struct {
	mu               sync.RWMutex
	prices           map[string]map[types.APIType]map[types.UsageDimension]pricingCatalogEntry
	groupMultipliers map[string]float64
}

func NewPricingStore() *PricingStore {
	return &PricingStore{
		prices:           make(map[string]map[types.APIType]map[types.UsageDimension]pricingCatalogEntry),
		groupMultipliers: make(map[string]float64),
	}
}

func NewPricingStoreWithDefaults() *PricingStore {
	store := NewPricingStore()
	// OpenAI defaults (approximate, per 1K tokens)
	defaults := map[types.APIType]map[types.UsageDimension]float64{
		types.APITypeChat: {
			types.DimPromptTokens:     0.002,
			types.DimCompletionTokens: 0.008,
			types.DimTotalTokens:      0.008,
		},
		types.APITypeCompletions: {
			types.DimPromptTokens:     0.002,
			types.DimCompletionTokens: 0.008,
			types.DimTotalTokens:      0.008,
		},
		types.APITypeEmbeddings: {
			types.DimPromptTokens: 0.0001,
		},
		types.APITypeImageGen: {
			types.DimImageCount: 0.004,
		},
		types.APITypeAudioSpeech: {
			types.DimAudioSeconds: 0.000015,
		},
		types.APITypeAudioSTT: {
			types.DimAudioSeconds: 0.0001,
		},
		types.APITypeAudioTranslate: {
			types.DimAudioSeconds: 0.0001,
		},
		types.APITypeModeration: {
			types.DimPromptTokens: 0.0001,
			types.DimTotalTokens:  0.0001,
		},
	}
	for apiType, dims := range defaults {
		for dim, price := range dims {
			store.SetPrice("gpt-4o", apiType, dim, price)
			store.SetPrice("gpt-4o-mini", apiType, dim, price*0.1)
		}
	}
	store.SetPrice("", types.APITypeFiles, types.DimStorageBytes, 0.000000001)
	return store
}

func (s *PricingStore) ApplyMultipliers(modelMultipliers, groupMultipliers map[string]float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for model, multiplier := range modelMultipliers {
		if multiplier < 0 {
			continue
		}
		modelPrices := s.prices[model]
		if len(modelPrices) == 0 {
			continue
		}
		for apiType, dimensions := range modelPrices {
			if dimensions == nil {
				continue
			}
			for dimension, entry := range dimensions {
				entry.UnitPrice *= multiplier
				s.prices[model][apiType][dimension] = entry
			}
		}
	}
	appliedGroupMultipliers := make(map[string]float64)
	for group, multiplier := range groupMultipliers {
		if multiplier < 0 {
			continue
		}
		appliedGroupMultipliers[group] = multiplier
	}
	s.groupMultipliers = appliedGroupMultipliers
}

func (s *PricingStore) SetPrice(model string, apiType types.APIType, dim types.UsageDimension, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setPriceLocked(model, apiType, dim, price)
}

func (s *PricingStore) setPriceLocked(model string, apiType types.APIType, dim types.UsageDimension, price float64) {
	s.setPriceEntryLocked(pricingCatalogEntry{
		APIType:   apiType,
		Currency:  "quota",
		Dimension: dim,
		Markup:    1,
		Model:     model,
		Source:    "runtime",
		UnitCost:  price,
		UnitPrice: price,
	})
}

func (s *PricingStore) setPriceEntryLocked(entry pricingCatalogEntry) {
	if s.prices[entry.Model] == nil {
		s.prices[entry.Model] = make(map[types.APIType]map[types.UsageDimension]pricingCatalogEntry)
	}
	if s.prices[entry.Model][entry.APIType] == nil {
		s.prices[entry.Model][entry.APIType] = make(map[types.UsageDimension]pricingCatalogEntry)
	}
	s.prices[entry.Model][entry.APIType][entry.Dimension] = entry
}

func (s *PricingStore) GetPrice(model string, apiType types.APIType, dim types.UsageDimension) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, err := s.priceEntryLocked(model, apiType, dim)
	if err != nil {
		return 0, err
	}
	return entry.UnitPrice, nil
}

func (s *PricingStore) priceEntryLocked(model string, apiType types.APIType, dim types.UsageDimension) (pricingCatalogEntry, error) {
	if s.prices[model] == nil || s.prices[model][apiType] == nil {
		return pricingCatalogEntry{}, fmt.Errorf("%w: model=%s apiType=%s dim=%s", ErrRelayPriceNotConfigured, model, apiType, dim)
	}
	entry := s.prices[model][apiType][dim]
	if entry.UnitPrice <= 0 {
		return pricingCatalogEntry{}, fmt.Errorf("%w: model=%s apiType=%s dim=%s", ErrRelayPriceNotConfigured, model, apiType, dim)
	}
	return entry, nil
}

func (s *PricingStore) IsEmpty() bool {
	if s == nil {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, byAPIType := range s.prices {
		for _, byDimension := range byAPIType {
			for _, entry := range byDimension {
				if entry.UnitPrice > 0 {
					return false
				}
			}
		}
	}
	return true
}

func (s *PricingStore) CalculateCost(model string, apiType types.APIType, usage *types.Usage) float64 {
	cost, err := s.CalculateCostStrict(model, apiType, usage)
	if err != nil {
		return 0
	}
	return cost
}

func (s *PricingStore) CalculateCostStrict(model string, apiType types.APIType, usage *types.Usage) (float64, error) {
	quote, err := s.QuoteUsageForGroupStrict(model, apiType, usage, "")
	if err != nil {
		return 0, err
	}
	return quote.TotalCost, nil
}

func (s *PricingStore) CalculateCostForGroup(model string, apiType types.APIType, usage *types.Usage, userGroup string) float64 {
	cost, err := s.CalculateCostForGroupStrict(model, apiType, usage, userGroup)
	if err != nil {
		return 0
	}
	return cost
}

func (s *PricingStore) CalculateCostForGroupStrict(model string, apiType types.APIType, usage *types.Usage, userGroup string) (float64, error) {
	quote, err := s.QuoteUsageForGroupStrict(model, apiType, usage, userGroup)
	if err != nil {
		return 0, err
	}
	return quote.TotalCost, nil
}

func (s *PricingStore) ApplyGroupMultiplier(cost float64, userGroup string) float64 {
	if s == nil || userGroup == "" {
		return cost
	}
	s.mu.RLock()
	multiplier, ok := s.groupMultipliers[userGroup]
	s.mu.RUnlock()
	if !ok || multiplier < 0 {
		return cost
	}
	return cost * multiplier
}

func (s *PricingStore) QuoteUsageForGroupStrict(model string, apiType types.APIType, usage *types.Usage, userGroup string) (*PricingQuote, error) {
	quote := &PricingQuote{
		Model:             model,
		APIType:           apiType.String(),
		GroupMultiplier:   1,
		ChannelMultiplier: 1,
		Dimensions:        []PricingQuoteDimension{},
	}
	if s == nil || usage == nil {
		return quote, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	hasPricedUsage := false
	addDimension := func(dim types.UsageDimension, quantity float64) error {
		entry, err := s.priceEntryLocked(model, apiType, dim)
		if err != nil {
			return err
		}
		amount := entry.UnitPrice * quantity
		quote.Subtotal += amount
		quote.Dimensions = append(quote.Dimensions, PricingQuoteDimension{
			PricingEntryID: entry.ID,
			Model:          entry.Model,
			APIType:        entry.APIType.String(),
			Dimension:      entry.Dimension,
			UnitCost:       entry.UnitCost,
			Markup:         entry.Markup,
			UnitPrice:      entry.UnitPrice,
			Quantity:       quantity,
			Amount:         amount,
			Currency:       entry.Currency,
			Source:         entry.Source,
			EffectiveFrom:  cloneTime(entry.EffectiveFrom),
		})
		hasPricedUsage = true
		return nil
	}

	if usage.PromptTokens > 0 {
		if err := addDimension(types.DimPromptTokens, float64(usage.PromptTokens)); err != nil {
			return nil, err
		}
	}
	if usage.CompletionTokens > 0 {
		if err := addDimension(types.DimCompletionTokens, float64(usage.CompletionTokens)); err != nil {
			return nil, err
		}
	}
	if !hasPricedUsage && usage.TotalTokens > 0 {
		if err := addDimension(types.DimTotalTokens, float64(usage.TotalTokens)); err != nil {
			return nil, err
		}
	}
	if usage.ImageCount > 0 {
		if err := addDimension(types.DimImageCount, float64(usage.ImageCount)); err != nil {
			return nil, err
		}
	}
	if usage.VideoCount > 0 {
		if err := addDimension(types.DimVideoCount, float64(usage.VideoCount)); err != nil {
			return nil, err
		}
	}
	if usage.AudioSeconds > 0 {
		if err := addDimension(types.DimAudioSeconds, usage.AudioSeconds); err != nil {
			return nil, err
		}
	}
	if usage.StorageBytes > 0 {
		if err := addDimension(types.DimStorageBytes, float64(usage.StorageBytes)); err != nil {
			return nil, err
		}
	}
	if usage.TrainingTokens > 0 {
		if err := addDimension(types.DimTrainingTokens, float64(usage.TrainingTokens)); err != nil {
			return nil, err
		}
	}
	if userGroup != "" {
		if multiplier, ok := s.groupMultipliers[userGroup]; ok && multiplier >= 0 {
			quote.GroupMultiplier = multiplier
		}
	}
	quote.TotalCost = quote.Subtotal * quote.GroupMultiplier
	quote.summarizeCatalogFields()
	return quote, nil
}

func (q *PricingQuote) summarizeCatalogFields() {
	if q == nil || len(q.Dimensions) == 0 {
		return
	}
	q.Currency = q.Dimensions[0].Currency
	q.Source = q.Dimensions[0].Source
	q.EffectiveFrom = cloneTime(q.Dimensions[0].EffectiveFrom)
	for _, dimension := range q.Dimensions[1:] {
		if dimension.Currency != q.Currency {
			q.Currency = "mixed"
		}
		if dimension.Source != q.Source {
			q.Source = "mixed"
		}
		if dimension.EffectiveFrom != nil && (q.EffectiveFrom == nil || dimension.EffectiveFrom.After(*q.EffectiveFrom)) {
			q.EffectiveFrom = cloneTime(dimension.EffectiveFrom)
		}
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type PricingEntry struct {
	APIType       types.APIType
	Currency      string
	Dimension     types.UsageDimension
	EffectiveFrom *time.Time
	ID            string
	Markup        float64
	Model         string
	Source        string
	UnitCost      float64
}

func NewPricingStoreFromEntries(entries []PricingEntry) (*PricingStore, error) {
	store := NewPricingStore()
	for _, entry := range entries {
		model := entry.Model
		if model == "" && !allowsModelAgnosticPricing(entry.APIType) {
			return nil, fmt.Errorf("relay pricing entry missing model")
		}
		if !validUsageDimension(entry.Dimension) {
			return nil, fmt.Errorf("relay pricing entry has unsupported dimension: %s", entry.Dimension)
		}
		if entry.APIType == types.APITypeUnknown {
			return nil, fmt.Errorf("relay pricing entry has unsupported api type")
		}
		if entry.UnitCost <= 0 {
			return nil, fmt.Errorf("relay pricing entry has non-positive unit cost for model=%s apiType=%s dim=%s", model, entry.APIType, entry.Dimension)
		}
		markup := entry.Markup
		if markup <= 0 {
			markup = 1
		}
		currency := entry.Currency
		if currency == "" {
			currency = "quota"
		}
		source := entry.Source
		if source == "" {
			source = "operator"
		}
		store.setPriceEntryLocked(pricingCatalogEntry{
			ID:            entry.ID,
			APIType:       entry.APIType,
			Currency:      currency,
			Dimension:     entry.Dimension,
			EffectiveFrom: cloneTime(entry.EffectiveFrom),
			Markup:        markup,
			Model:         model,
			Source:        source,
			UnitCost:      entry.UnitCost,
			UnitPrice:     entry.UnitCost * markup,
		})
	}
	return store, nil
}

func allowsModelAgnosticPricing(apiType types.APIType) bool {
	switch apiType {
	case types.APITypeFiles:
		return true
	default:
		return false
	}
}

func LoadPricingStoreFromSQL(ctx context.Context, db *sql.DB) (*PricingStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required for relay pricing catalog")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, api_type, model, dimension, unit_cost, COALESCE(markup, 1), currency, source, effective_from
		FROM relay_pricing_entries
		WHERE active = true
		ORDER BY model, api_type, dimension
	`)
	if err != nil {
		return nil, fmt.Errorf("load relay pricing catalog: %w", err)
	}
	defer rows.Close()

	entries := []PricingEntry{}
	for rows.Next() {
		var (
			apiTypeRaw string
			dimRaw     string
			effective  time.Time
			entry      PricingEntry
		)
		if err := rows.Scan(&entry.ID, &apiTypeRaw, &entry.Model, &dimRaw, &entry.UnitCost, &entry.Markup, &entry.Currency, &entry.Source, &effective); err != nil {
			return nil, fmt.Errorf("scan relay pricing catalog: %w", err)
		}
		apiType, ok := parsePricingAPIType(apiTypeRaw)
		if !ok {
			return nil, fmt.Errorf("relay pricing catalog contains unsupported api type: %s", apiTypeRaw)
		}
		dim := types.UsageDimension(dimRaw)
		if !validUsageDimension(dim) {
			return nil, fmt.Errorf("relay pricing catalog contains unsupported dimension: %s", dimRaw)
		}
		entry.APIType = apiType
		entry.Dimension = dim
		entry.EffectiveFrom = &effective
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read relay pricing catalog: %w", err)
	}
	store, err := NewPricingStoreFromEntries(entries)
	if err != nil {
		return nil, err
	}
	if store.IsEmpty() {
		return nil, fmt.Errorf("relay pricing catalog has no active prices")
	}
	return store, nil
}

func parsePricingAPIType(value string) (types.APIType, bool) {
	for apiType := types.APITypeUnknown + 1; apiType <= types.APITypeModels; apiType++ {
		if apiType.String() == value {
			return apiType, true
		}
	}
	return types.APITypeUnknown, false
}

func validUsageDimension(dim types.UsageDimension) bool {
	switch dim {
	case types.DimPromptTokens,
		types.DimCompletionTokens,
		types.DimTotalTokens,
		types.DimImageCount,
		types.DimVideoCount,
		types.DimAudioSeconds,
		types.DimStorageBytes,
		types.DimTrainingTokens:
		return true
	default:
		return false
	}
}
