package migration

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type RAGValidator struct {
	db *gorm.DB
}

func NewRAGValidator(db *gorm.DB) *RAGValidator {
	return &RAGValidator{db: db}
}

func (v *RAGValidator) Validate(ctx context.Context) error {
	tables := []string{
		"knowledge_bases",
		"documents",
		"document_chunks",
	}

	for _, table := range tables {
		if err := v.validateTable(ctx, table); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}

func (v *RAGValidator) validateTable(ctx context.Context, tableName string) error {
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
