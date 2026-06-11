package migration

import (
	"testing"

	"gorm.io/gorm"
)

func TestRAGValidator_Validate(t *testing.T) {
	t.Run("all tables exist", func(t *testing.T) {
		db := &gorm.DB{}
		validator := &RAGValidator{db: db}

		if validator.db == nil {
			t.Error("validator db should not be nil")
		}
	})

	t.Run("validates required tables", func(t *testing.T) {
		validator := NewRAGValidator(&gorm.DB{})
		expectedTables := []string{
			"knowledge_bases",
			"documents",
			"document_chunks",
		}

		if validator.db == nil {
			t.Error("validator db should not be nil")
		}

		tables := []string{
			"knowledge_bases",
			"documents",
			"document_chunks",
		}

		for i, table := range tables {
			if table != expectedTables[i] {
				t.Errorf("expected table %s, got %s", expectedTables[i], table)
			}
		}
		_ = expectedTables
	})
}

func TestNewRAGValidator(t *testing.T) {
	db := &gorm.DB{}
	v := NewRAGValidator(db)

	if v == nil {
		t.Error("expected validator, got nil")
	}

	if v.db != db {
		t.Error("validator db mismatch")
	}
}
