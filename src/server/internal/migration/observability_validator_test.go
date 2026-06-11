package migration

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestObservabilityValidator_Validate(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT column_name").WillReturnRows(
		sqlmock.NewRows([]string{"column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("id", "bigint", "NO", nil).
			AddRow("name", "character varying", "NO", nil).
			AddRow("type", "character varying", "NO", nil).
			AddRow("condition", "jsonb", "NO", nil).
			AddRow("severity", "character varying", "NO", nil).
			AddRow("enabled", "boolean", "NO", "true").
			AddRow("created_at", "timestamp with time zone", "NO", "CURRENT_TIMESTAMP").
			AddRow("updated_at", "timestamp with time zone", "NO", "CURRENT_TIMESTAMP"),
	)
	mock.ExpectQuery("SELECT indexname").WillReturnRows(
		sqlmock.NewRows([]string{"indexname"}).AddRow("alert_configs_pkey"),
	)

	validator := NewObservabilityValidator(sqlDB)
	if err := validator.Validate(context.Background()); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestObservabilityValidator_MissingColumn(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT column_name").WillReturnRows(
		sqlmock.NewRows([]string{"column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("id", "bigint", "NO", nil).
			AddRow("name", "character varying", "NO", nil),
	)

	validator := NewObservabilityValidator(sqlDB)
	err = validator.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for missing columns")
	}
}

func TestObservabilityValidator_WrongType(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT column_name").WillReturnRows(
		sqlmock.NewRows([]string{"column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("id", "bigint", "NO", nil).
			AddRow("name", "text", "NO", nil).
			AddRow("type", "character varying", "NO", nil).
			AddRow("condition", "jsonb", "NO", nil).
			AddRow("severity", "character varying", "NO", nil).
			AddRow("enabled", "boolean", "NO", "true").
			AddRow("created_at", "timestamp with time zone", "NO", "CURRENT_TIMESTAMP").
			AddRow("updated_at", "timestamp with time zone", "NO", "CURRENT_TIMESTAMP"),
	)

	validator := NewObservabilityValidator(sqlDB)
	err = validator.Validate(context.Background())
	if err == nil {
		t.Fatal("expected validation error for wrong type")
	}
}
