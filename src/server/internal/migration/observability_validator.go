package migration

import (
	"context"
	"database/sql"
	"fmt"
)

type ObservabilityValidator struct {
	db *sql.DB
}

func NewObservabilityValidator(db *sql.DB) *ObservabilityValidator {
	return &ObservabilityValidator{db: db}
}

func (v *ObservabilityValidator) Validate(ctx context.Context) error {
	if err := v.validateAlertConfigsTable(ctx); err != nil {
		return fmt.Errorf("alert_configs validation failed: %w", err)
	}
	return nil
}

func (v *ObservabilityValidator) validateAlertConfigsTable(ctx context.Context) error {
	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = 'alert_configs'
		ORDER BY ordinal_position
	`

	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	expected := map[string]struct {
		dataType   string
		isNullable string
	}{
		"id":          {"bigint", "NO"},
		"name":        {"character varying", "NO"},
		"type":        {"character varying", "NO"},
		"condition":   {"jsonb", "NO"},
		"severity":    {"character varying", "NO"},
		"enabled":     {"boolean", "NO"},
		"created_at":  {"timestamp with time zone", "NO"},
		"updated_at":  {"timestamp with time zone", "NO"},
	}

	found := make(map[string]bool)
	for rows.Next() {
		var colName, dataType, isNullable string
		var colDefault sql.NullString
		if err := rows.Scan(&colName, &dataType, &isNullable, &colDefault); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}

		exp, ok := expected[colName]
		if !ok {
			return fmt.Errorf("unexpected column: %s", colName)
		}

		if exp.dataType != dataType {
			return fmt.Errorf("column %s: expected type %s, got %s", colName, exp.dataType, dataType)
		}

		if exp.isNullable != isNullable {
			return fmt.Errorf("column %s: expected nullable %s, got %s", colName, exp.isNullable, isNullable)
		}

		found[colName] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	for col := range expected {
		if !found[col] {
			return fmt.Errorf("missing column: %s", col)
		}
	}

	if err := v.validateIndexes(ctx); err != nil {
		return fmt.Errorf("index validation: %w", err)
	}

	return nil
}

func (v *ObservabilityValidator) validateIndexes(ctx context.Context) error {
	query := `
		SELECT indexname
		FROM pg_indexes
		WHERE tablename = 'alert_configs'
	`

	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		indexes[name] = true
	}

	if !indexes["alert_configs_pkey"] {
		return fmt.Errorf("missing primary key index")
	}

	return nil
}
