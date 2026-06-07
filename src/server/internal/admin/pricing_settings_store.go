package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const relayPricingSettingsKey = "global"

type RelayPricingSettingsStore interface {
	GetRelayPricingSettings(ctx context.Context) (*RelayPricingSettings, error)
	UpdateRelayPricingSettings(ctx context.Context, settings RelayPricingSettings) (*RelayPricingSettings, error)
}

func (s *SQLStore) GetRelayPricingSettings(ctx context.Context) (*RelayPricingSettings, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT value
		FROM relay_pricing_settings
		WHERE key = $1
	`, relayPricingSettingsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		settings := normalizeRelayPricingSettings(RelayPricingSettings{})
		return &settings, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relay pricing settings: %w", err)
	}

	var settings RelayPricingSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decode relay pricing settings: %w", err)
	}
	normalized := normalizeRelayPricingSettings(settings)
	return &normalized, nil
}

func (s *SQLStore) UpdateRelayPricingSettings(ctx context.Context, settings RelayPricingSettings) (*RelayPricingSettings, error) {
	normalized := normalizeRelayPricingSettings(settings)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode relay pricing settings: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO relay_pricing_settings (key, value, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`, relayPricingSettingsKey, string(raw))
	if err != nil {
		return nil, fmt.Errorf("update relay pricing settings: %w", err)
	}

	return &normalized, nil
}
