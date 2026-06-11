package migration

import (
	"context"
	"fmt"
)

type ChatValidator struct{}

func (v *ChatValidator) Validate(ctx context.Context, legacyDB, newDB DB) error {
	tables := []string{
		"conversations",
		"messages",
		"personas",
	}

	for _, table := range tables {
		if err := ValidateTableRowCount(ctx, legacyDB, newDB, table); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}
