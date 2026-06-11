package migration

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMarketplaceValidator_Validate(t *testing.T) {
	ctx := context.Background()

	t.Run("all tables exist", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("marketplace_agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mock.ExpectQuery("SELECT EXISTS").WithArgs("installs").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewMarketplaceValidator(db)
		err := validator.Validate(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("table does not exist", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("marketplace_agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(false),
		)

		validator := NewMarketplaceValidator(db)
		err := validator.Validate(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestMarketplaceValidator_validateTable(t *testing.T) {
	ctx := context.Background()

	t.Run("table exists", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("marketplace_agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewMarketplaceValidator(db)
		err := validator.validateTable(ctx, "marketplace_agents")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("table does not exist", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("nonexistent").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(false),
		)

		validator := NewMarketplaceValidator(db)
		err := validator.validateTable(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}
