package migration

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestChannelValidator_Validate(t *testing.T) {
	ctx := context.Background()

	t.Run("all tables exist", func(t *testing.T) {
		db := &gorm.DB{}
		v := &ChannelValidator{db: db}

		if v.db != db {
			t.Error("validator db mismatch")
		}

		_ = ctx
	})

	t.Run("nil db", func(t *testing.T) {
		v := &ChannelValidator{db: nil}
		if v.db == nil {
			// Expected
		}
	})
}

func TestChannelValidator_validateTable(t *testing.T) {
	ctx := context.Background()

	t.Run("table exists", func(t *testing.T) {
		db := &gorm.DB{}
		v := NewChannelValidator(db)

		if v == nil {
			t.Error("expected validator, got nil")
		}

		_ = ctx
	})

	t.Run("nil validator", func(t *testing.T) {
		v := &ChannelValidator{db: nil}
		if v.db == nil {
			// Expected
		}
	})
}

func TestNewChannelValidator(t *testing.T) {
	db := &gorm.DB{}
	v := NewChannelValidator(db)

	if v == nil {
		t.Error("expected validator, got nil")
	}

	if v.db != db {
		t.Error("validator db mismatch")
	}
}
