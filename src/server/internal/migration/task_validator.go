package migration

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type TaskValidator struct {
	db *gorm.DB
}

func NewTaskValidator(db *gorm.DB) *TaskValidator {
	return &TaskValidator{db: db}
}

func (v *TaskValidator) Validate(ctx context.Context) error {
	tables := []string{
		"scheduled_tasks",
		"task_executions",
	}

	for _, table := range tables {
		if err := v.validateTable(ctx, table); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}

func (v *TaskValidator) validateTable(ctx context.Context, tableName string) error {
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
