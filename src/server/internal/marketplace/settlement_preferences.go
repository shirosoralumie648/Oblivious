package marketplace

import (
	"context"
	"fmt"
	"strings"
)

const (
	settlementCycleWeekly    = "weekly"
	settlementCycleMonthly   = "monthly"
	settlementCycleQuarterly = "quarterly"

	defaultSettlementCycle       = settlementCycleMonthly
	defaultMinimumPayoutAmount   = 100
	defaultSettlementEffectiveOn = "next_settlement_cycle"
)

// MarketplaceSettlementPreferences describes a publisher's settlement cadence.
type MarketplaceSettlementPreferences struct {
	Cycle                string  `json:"cycle"`
	Label                string  `json:"label"`
	Description          string  `json:"description"`
	PayoutBusinessDays   int     `json:"payoutBusinessDays"`
	ProcessingFeePercent float64 `json:"processingFeePercent"`
	MinimumPayoutAmount  float64 `json:"minimumPayoutAmount"`
	EffectiveFrom        string  `json:"effectiveFrom"`
}

// GetPublisherSettlementPreferences returns publisher settlement preferences with defaults applied.
func (s *Service) GetPublisherSettlementPreferences(ctx context.Context, organizationID string) (*MarketplaceSettlementPreferences, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("get settlement preferences: organization id is required")
	}
	stored, err := s.store.GetPublisherSettlementPreferences(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("get settlement preferences: %w", err)
	}
	cycle := defaultSettlementCycle
	if stored != nil && strings.TrimSpace(stored.Cycle) != "" {
		cycle = strings.TrimSpace(stored.Cycle)
	}
	return settlementPreferencesForCycle(cycle)
}

// UpdatePublisherSettlementPreferences stores a new cycle that takes effect next settlement cycle.
func (s *Service) UpdatePublisherSettlementPreferences(ctx context.Context, organizationID string, cycle string) (*MarketplaceSettlementPreferences, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("update settlement preferences: organization id is required")
	}
	normalizedCycle, err := normalizeSettlementCycle(cycle)
	if err != nil {
		return nil, fmt.Errorf("update settlement preferences: %w", err)
	}
	if _, err := s.store.UpdatePublisherSettlementPreferences(ctx, organizationID, normalizedCycle); err != nil {
		return nil, fmt.Errorf("update settlement preferences: %w", err)
	}
	return settlementPreferencesForCycle(normalizedCycle)
}

func settlementPreferencesForCycle(cycle string) (*MarketplaceSettlementPreferences, error) {
	normalizedCycle, err := normalizeSettlementCycle(cycle)
	if err != nil {
		return nil, err
	}
	prefs := MarketplaceSettlementPreferences{
		Cycle:               normalizedCycle,
		MinimumPayoutAmount: defaultMinimumPayoutAmount,
		EffectiveFrom:       defaultSettlementEffectiveOn,
	}
	switch normalizedCycle {
	case settlementCycleWeekly:
		prefs.Label = "Weekly"
		prefs.Description = "Settles the previous week's Marketplace income every Monday."
		prefs.PayoutBusinessDays = 3
		prefs.ProcessingFeePercent = 2
	case settlementCycleMonthly:
		prefs.Label = "Monthly"
		prefs.Description = "Settles the previous month's Marketplace income on the first day of each month."
		prefs.PayoutBusinessDays = 5
		prefs.ProcessingFeePercent = 1
	case settlementCycleQuarterly:
		prefs.Label = "Quarterly"
		prefs.Description = "Settles the previous quarter's Marketplace income on the first day of each quarter."
		prefs.PayoutBusinessDays = 5
		prefs.ProcessingFeePercent = 0.5
	}
	return &prefs, nil
}

func normalizeSettlementCycle(cycle string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(cycle))
	switch normalized {
	case settlementCycleWeekly, settlementCycleMonthly, settlementCycleQuarterly:
		return normalized, nil
	default:
		return "", fmt.Errorf("cycle must be one of: weekly, monthly, quarterly")
	}
}
