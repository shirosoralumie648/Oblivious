package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type SQLAlertProviderConfigStore struct {
	db *sql.DB
}

func NewSQLAlertProviderConfigStore(db *sql.DB) *SQLAlertProviderConfigStore {
	return &SQLAlertProviderConfigStore{db: db}
}

func (s *SQLAlertProviderConfigStore) GetAlertProviderConfig(ctx context.Context, id string) (AlertProviderConfig, bool, error) {
	if s == nil || s.db == nil {
		return AlertProviderConfig{}, false, errors.New("alert provider config store database is required")
	}
	config, err := scanAlertProviderConfig(s.db.QueryRowContext(ctx, `
SELECT id, kind, channel, name, status, config, created_at, updated_at
FROM observability_alert_provider_configs
WHERE id = $1
`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AlertProviderConfig{}, false, nil
		}
		return AlertProviderConfig{}, false, err
	}
	return config, true, nil
}

func (s *SQLAlertProviderConfigStore) ListAlertProviderConfigs(ctx context.Context) ([]AlertProviderConfig, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert provider config store database is required")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, kind, channel, name, status, config, created_at, updated_at
FROM observability_alert_provider_configs
ORDER BY name ASC, id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list alert provider configs: %w", err)
	}
	defer rows.Close()

	configs := []AlertProviderConfig{}
	for rows.Next() {
		config, err := scanAlertProviderConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert provider configs: %w", err)
	}
	return configs, nil
}

func (s *SQLAlertProviderConfigStore) SaveAlertProviderConfig(ctx context.Context, config AlertProviderConfig) (AlertProviderConfig, error) {
	if s == nil || s.db == nil {
		return AlertProviderConfig{}, errors.New("alert provider config store database is required")
	}
	normalized, err := NormalizeAlertProviderConfig(config)
	if err != nil {
		return AlertProviderConfig{}, err
	}
	configJSON, err := json.Marshal(normalized.Config)
	if err != nil {
		return AlertProviderConfig{}, fmt.Errorf("encode alert provider config: %w", err)
	}
	return scanAlertProviderConfig(s.db.QueryRowContext(ctx, `
INSERT INTO observability_alert_provider_configs (id, kind, channel, name, status, config, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
	kind = EXCLUDED.kind,
	channel = EXCLUDED.channel,
	name = EXCLUDED.name,
	status = EXCLUDED.status,
	config = EXCLUDED.config,
	updated_at = NOW()
RETURNING id, kind, channel, name, status, config, created_at, updated_at
`, normalized.ID, normalized.Kind, normalized.Channel, normalized.Name, normalized.Status, string(configJSON)))
}

type alertProviderConfigScanner interface {
	Scan(dest ...any) error
}

func scanAlertProviderConfig(scanner alertProviderConfigScanner) (AlertProviderConfig, error) {
	var config AlertProviderConfig
	var rawConfig []byte
	if err := scanner.Scan(
		&config.ID,
		&config.Kind,
		&config.Channel,
		&config.Name,
		&config.Status,
		&rawConfig,
		&config.CreatedAt,
		&config.UpdatedAt,
	); err != nil {
		return AlertProviderConfig{}, err
	}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &config.Config); err != nil {
			return AlertProviderConfig{}, fmt.Errorf("decode alert provider config %s: %w", config.ID, err)
		}
	}
	normalized, err := NormalizeAlertProviderConfig(config)
	if err != nil {
		return AlertProviderConfig{}, err
	}
	normalized.CreatedAt = config.CreatedAt
	normalized.UpdatedAt = config.UpdatedAt
	return normalized, nil
}
