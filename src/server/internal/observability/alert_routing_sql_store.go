package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type SQLAlertRoutingRuleStore struct {
	db *sql.DB
}

func NewSQLAlertRoutingRuleStore(db *sql.DB) *SQLAlertRoutingRuleStore {
	return &SQLAlertRoutingRuleStore{db: db}
}

func (s *SQLAlertRoutingRuleStore) GetRoutingRules(ctx context.Context) (AlertRoutingRules, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert routing rule store database is required")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT severity, channels
FROM observability_alert_routing_rules
`)
	if err != nil {
		return nil, fmt.Errorf("list alert routing rules: %w", err)
	}
	defer rows.Close()

	rawRules := AlertRoutingRules{}
	for rows.Next() {
		var severity AlertSeverity
		var channelsJSON []byte
		if err := rows.Scan(&severity, &channelsJSON); err != nil {
			return nil, fmt.Errorf("scan alert routing rule: %w", err)
		}
		var channels []AlertDeliveryChannel
		if len(channelsJSON) > 0 {
			if err := json.Unmarshal(channelsJSON, &channels); err != nil {
				return nil, fmt.Errorf("decode alert routing channels for %s: %w", severity, err)
			}
		}
		rawRules[severity] = channels
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert routing rules: %w", err)
	}
	if len(rawRules) == 0 {
		return s.UpdateRoutingRules(ctx, DefaultAlertRoutingRules())
	}
	return NormalizeAlertRoutingRules(rawRules)
}

func (s *SQLAlertRoutingRuleStore) UpdateRoutingRules(ctx context.Context, rules AlertRoutingRules) (AlertRoutingRules, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert routing rule store database is required")
	}
	normalized, err := NormalizeAlertRoutingRules(rules)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin alert routing rule update: %w", err)
	}
	defer tx.Rollback()

	for severity, channels := range normalized {
		channelsJSON, err := json.Marshal(channels)
		if err != nil {
			return nil, fmt.Errorf("encode alert routing channels for %s: %w", severity, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO observability_alert_routing_rules (severity, channels, updated_at)
VALUES ($1, $2::jsonb, NOW())
ON CONFLICT (severity) DO UPDATE SET
	channels = EXCLUDED.channels,
	updated_at = NOW()
`, severity, string(channelsJSON)); err != nil {
			return nil, fmt.Errorf("upsert alert routing rule for %s: %w", severity, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit alert routing rule update: %w", err)
	}
	return copyAlertRoutingRules(normalized), nil
}
