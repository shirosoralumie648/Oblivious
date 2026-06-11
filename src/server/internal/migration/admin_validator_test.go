package migration

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminValidator_Validate(t *testing.T) {
	ctx := context.Background()

	t.Run("all tables exist", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("plans").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mock.ExpectQuery("SELECT EXISTS").WithArgs("audit_logs").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewAdminValidator(db)
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

		mock.ExpectQuery("SELECT EXISTS").WithArgs("plans").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(false),
		)

		validator := NewAdminValidator(db)
		err := validator.Validate(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestAdminValidator_validateTable(t *testing.T) {
	ctx := context.Background()

	t.Run("table exists", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("plans").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewAdminValidator(db)
		err := validator.validateTable(ctx, "plans")
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

		validator := NewAdminValidator(db)
		err := validator.validateTable(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}
