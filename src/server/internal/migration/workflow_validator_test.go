package migration

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestWorkflowValidator_Validate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	validator := NewWorkflowValidator(gormDB)

	tables := []string{
		"workflows",
		"workflow_runs",
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

func TestWorkflowValidator_Validate_MissingTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	validator := NewWorkflowValidator(gormDB)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("workflows").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err = validator.Validate(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflows")
	assert.Contains(t, err.Error(), "does not exist")
}
