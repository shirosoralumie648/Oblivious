package migration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupBillingValidationRow(t *testing.T, queryPattern string, columns []string, values ...driver.Value) (*BillingValidator, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	mock.ExpectQuery(queryPattern).WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
	return NewBillingValidator(db), mock, db
}

func expectBillingValidationRow(mock sqlmock.Sqlmock, queryPattern string, columns []string, values ...driver.Value) {
	mock.ExpectQuery(queryPattern).WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
}

func TestBillingValidator_ValidateSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		counts  []driver.Value
		wantErr bool
	}{
		{name: "valid subscriptions", counts: []driver.Value{2, 0, 0, 0}},
		{name: "null user_id", counts: []driver.Value{1, 1, 0, 0}, wantErr: true},
		{name: "null plan_id", counts: []driver.Value{1, 0, 1, 0}, wantErr: true},
		{name: "invalid status", counts: []driver.Value{1, 0, 0, 1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, mock, db := setupBillingValidationRow(
				t,
				"FROM subscriptions",
				[]string{"total", "null_user", "null_plan", "invalid_status"},
				tt.counts...,
			)
			defer db.Close()

			err := v.validateSubscriptions(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBillingValidator_ValidateInvoices(t *testing.T) {
	tests := []struct {
		name    string
		counts  []driver.Value
		wantErr bool
	}{
		{name: "valid invoices", counts: []driver.Value{1, 0, 0, 0}},
		{name: "null subscription_id", counts: []driver.Value{1, 1, 0, 0}, wantErr: true},
		{name: "negative amount", counts: []driver.Value{1, 0, 1, 0}, wantErr: true},
		{name: "invalid status", counts: []driver.Value{1, 0, 0, 1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, mock, db := setupBillingValidationRow(
				t,
				"FROM invoices",
				[]string{"total", "null_subscription", "negative_amount", "invalid_status"},
				tt.counts...,
			)
			defer db.Close()

			err := v.validateInvoices(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBillingValidator_ValidatePayments(t *testing.T) {
	tests := []struct {
		name    string
		counts  []driver.Value
		wantErr bool
	}{
		{name: "valid payments", counts: []driver.Value{1, 0, 0, 0}},
		{name: "null invoice_id", counts: []driver.Value{1, 1, 0, 0}, wantErr: true},
		{name: "zero amount", counts: []driver.Value{1, 0, 1, 0}, wantErr: true},
		{name: "invalid status", counts: []driver.Value{1, 0, 0, 1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, mock, db := setupBillingValidationRow(
				t,
				"FROM payments",
				[]string{"total", "null_invoice", "invalid_amount", "invalid_status"},
				tt.counts...,
			)
			defer db.Close()

			err := v.validatePayments(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBillingValidator_Validate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	expectBillingValidationRow(mock, "FROM subscriptions", []string{"total", "null_user", "null_plan", "invalid_status"}, 1, 0, 0, 0)
	expectBillingValidationRow(mock, "FROM invoices", []string{"total", "null_subscription", "negative_amount", "invalid_status"}, 1, 0, 0, 0)
	expectBillingValidationRow(mock, "FROM payments", []string{"total", "null_invoice", "invalid_amount", "invalid_status"}, 1, 0, 0, 0)

	v := NewBillingValidator(db)
	if err := v.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
