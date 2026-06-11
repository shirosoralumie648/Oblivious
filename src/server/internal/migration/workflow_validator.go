package migration

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type WorkflowValidator struct {
	db *gorm.DB
}

func NewWorkflowValidator(db *gorm.DB) *WorkflowValidator {
	return &WorkflowValidator{db: db}
}

func (v *WorkflowValidator) Validate(ctx context.Context) error {
	tables := []string{
		"workflows",
		"workflow_runs",
	}

	for _, table := range tables {
		if err := v.validateTable(ctx, table); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}

func (v *WorkflowValidator) validateTable(ctx context.Context, tableName string) error {
	var exists bool
	err := v.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?)",
		tableName,
	).Scan(&exists).Error

	if err != nil {
		return fmt.Errorf("check existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("does not exist")
	}

	return nil
}
