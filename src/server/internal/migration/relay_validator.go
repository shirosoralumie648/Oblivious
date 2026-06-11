package migration

import (
	"context"
	"fmt"
)

type RelayValidator struct{}

func (v *RelayValidator) Validate(ctx context.Context, legacyDB, newDB DB) error {
	tables := []string{
		"channels",
		"model_routes",
		"relay_api_tokens",
		"relay_semantic_cache",
		"relay_usage_records",
	}

	for _, table := range tables {
		if err := ValidateTableRowCount(ctx, legacyDB, newDB, table); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}
