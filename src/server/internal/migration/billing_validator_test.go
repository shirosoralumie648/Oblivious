package migration

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupBillingTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	schema := `
		CREATE TABLE subscriptions (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			plan_id INTEGER,
			status TEXT
		);
		CREATE TABLE invoices (
			id INTEGER PRIMARY KEY,
			subscription_id INTEGER,
			amount REAL,
			status TEXT
		);
		CREATE TABLE payments (
			id INTEGER PRIMARY KEY,
			invoice_id INTEGER,
			amount REAL,
			status TEXT
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	return db
}

func TestBillingValidator_ValidateSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*sql.DB)
		wantErr bool
	}{
		{
			name: "valid subscriptions",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO subscriptions (user_id, plan_id, status) VALUES (1, 1, 'active')")
				db.Exec("INSERT INTO subscriptions (user_id, plan_id, status) VALUES (2, 2, 'cancelled')")
			},
			wantErr: false,
		},
		{
			name: "null user_id",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO subscriptions (plan_id, status) VALUES (1, 'active')")
			},
			wantErr: true,
		},
		{
			name: "null plan_id",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO subscriptions (user_id, status) VALUES (1, 'active')")
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO subscriptions (user_id, plan_id, status) VALUES (1, 1, 'invalid')")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupBillingTestDB(t)
			defer db.Close()

			tt.setup(db)
			v := NewBillingValidator(db)
			err := v.validateSubscriptions(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestBillingValidator_ValidateInvoices(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*sql.DB)
		wantErr bool
	}{
		{
			name: "valid invoices",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO invoices (subscription_id, amount, status) VALUES (1, 99.99, 'paid')")
			},
			wantErr: false,
		},
		{
			name: "null subscription_id",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO invoices (amount, status) VALUES (99.99, 'paid')")
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO invoices (subscription_id, amount, status) VALUES (1, -10.00, 'paid')")
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO invoices (subscription_id, amount, status) VALUES (1, 99.99, 'unknown')")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupBillingTestDB(t)
			defer db.Close()

			tt.setup(db)
			v := NewBillingValidator(db)
			err := v.validateInvoices(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestBillingValidator_ValidatePayments(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*sql.DB)
		wantErr bool
	}{
		{
			name: "valid payments",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO payments (invoice_id, amount, status) VALUES (1, 99.99, 'succeeded')")
			},
			wantErr: false,
		},
		{
			name: "null invoice_id",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO payments (amount, status) VALUES (99.99, 'succeeded')")
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO payments (invoice_id, amount, status) VALUES (1, 0, 'succeeded')")
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			setup: func(db *sql.DB) {
				db.Exec("INSERT INTO payments (invoice_id, amount, status) VALUES (1, 99.99, 'unknown')")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupBillingTestDB(t)
			defer db.Close()

			tt.setup(db)
			v := NewBillingValidator(db)
			err := v.validatePayments(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestBillingValidator_Validate(t *testing.T) {
	db := setupBillingTestDB(t)
	defer db.Close()

	db.Exec("INSERT INTO subscriptions (user_id, plan_id, status) VALUES (1, 1, 'active')")
	db.Exec("INSERT INTO invoices (subscription_id, amount, status) VALUES (1, 99.99, 'paid')")
	db.Exec("INSERT INTO payments (invoice_id, amount, status) VALUES (1, 99.99, 'succeeded')")

	v := NewBillingValidator(db)
	if err := v.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}
