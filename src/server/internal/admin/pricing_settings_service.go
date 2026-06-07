package admin

import (
	"context"
	"fmt"

	"oblivious/server/internal/auth"
)

func (s *Service) GetRelayPricingSettings(ctx context.Context) (*RelayPricingSettings, error) {
	settings, err := s.store.GetRelayPricingSettings(ctx)
	if err != nil {
		return nil, err
	}
	normalized := normalizeRelayPricingSettings(derefRelayPricingSettings(settings))
	return &normalized, nil
}

func (s *Service) UpdateRelayPricingSettings(ctx context.Context, actor auth.Session, settings RelayPricingSettings, ipAddress string) (*RelayPricingSettings, error) {
	normalized := normalizeRelayPricingSettings(settings)
	if err := validateRelayPricingSettings(normalized); err != nil {
		return nil, err
	}
	updated, err := s.store.UpdateRelayPricingSettings(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		updated = &normalized
	}
	applied := normalizeRelayPricingSettings(*updated)
	if s.relayPricingSettingsApplier != nil {
		s.relayPricingSettingsApplier(applied)
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "settings.relay_pricing.update", "settings", "relay_pricing", toJSON(applied), ipAddress)
	return &applied, nil
}

func validateRelayPricingSettings(settings RelayPricingSettings) error {
	for model, multiplier := range settings.ModelMultipliers {
		if multiplier < 0 {
			return fmt.Errorf("model multiplier must be not less than 0: %s", model)
		}
	}
	for group, multiplier := range settings.GroupMultipliers {
		if multiplier < 0 {
			return fmt.Errorf("group multiplier must be not less than 0: %s", group)
		}
	}
	return nil
}

func derefRelayPricingSettings(settings *RelayPricingSettings) RelayPricingSettings {
	if settings == nil {
		return RelayPricingSettings{}
	}
	return *settings
}

func normalizeRelayPricingSettings(settings RelayPricingSettings) RelayPricingSettings {
	return RelayPricingSettings{
		ModelMultipliers: copyFloatMap(settings.ModelMultipliers),
		GroupMultipliers: copyFloatMap(settings.GroupMultipliers),
	}
}

func copyFloatMap(input map[string]float64) map[string]float64 {
	output := make(map[string]float64)
	for key, value := range input {
		output[key] = value
	}
	return output
}
