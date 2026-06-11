package migration

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRelayValidator_Validate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	validator := NewRelayValidator(gormDB)

	tables := []string{
		"channels",
		"model_routes",
		"relay_api_tokens",
		"relay_semantic_cache",
		"relay_usage_records",
	}

	for _, table := range tables {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(table).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	}

	err = validator.Validate(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRelayValidator_Validate_MissingTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	validator := NewRelayValidator(gormDB)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("channels").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err = validator.Validate(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channels")
	assert.Contains(t, err.Error(), "does not exist")
}
