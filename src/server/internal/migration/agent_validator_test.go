package migration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm: %v", err)
	}

	return gormDB, mock, sqlDB
}

func TestAgentValidator_Validate(t *testing.T) {
	ctx := context.Background()

	t.Run("all tables exist", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mock.ExpectQuery("SELECT EXISTS").WithArgs("agent_runs").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mock.ExpectQuery("SELECT EXISTS").WithArgs("agent_memories").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewAgentValidator(db)
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

		mock.ExpectQuery("SELECT EXISTS").WithArgs("agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(false),
		)

		validator := NewAgentValidator(db)
		err := validator.Validate(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestAgentValidator_validateTable(t *testing.T) {
	ctx := context.Background()

	t.Run("table exists", func(t *testing.T) {
		db, mock, sqlDB := setupMockDB(t)
		defer sqlDB.Close()

		mock.ExpectQuery("SELECT EXISTS").WithArgs("agents").WillReturnRows(
			sqlmock.NewRows([]string{"exists"}).AddRow(true),
		)

		validator := NewAgentValidator(db)
		err := validator.validateTable(ctx, "agents")
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

		validator := NewAgentValidator(db)
		err := validator.validateTable(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}
