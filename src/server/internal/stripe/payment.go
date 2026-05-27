package stripe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PaymentIntent struct {
	ID                        string
	Provider                  string
	ProviderCheckoutSessionID string
	OrganizationID            string
	UserID                    string
	PackageID                 string
	Kind                      string
	Amount                    float64
	Currency                  string
	Status                    string
	Metadata                  map[string]string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type PaymentIntentStore interface {
	CreatePaymentIntent(ctx context.Context, intent PaymentIntent) (PaymentIntent, error)
	SetCheckoutSession(ctx context.Context, id string, providerCheckoutSessionID string, metadata map[string]string) error
}

type SQLPaymentIntentStore struct {
	db *sql.DB
}

func NewSQLPaymentIntentStore(db *sql.DB) *SQLPaymentIntentStore {
	return &SQLPaymentIntentStore{db: db}
}

func (s *SQLPaymentIntentStore) CreatePaymentIntent(ctx context.Context, intent PaymentIntent) (PaymentIntent, error) {
	if intent.ID == "" {
		intent.ID = uuid.New().String()
	}
	if intent.Provider == "" {
		intent.Provider = "stripe"
	}
	if intent.Currency == "" {
		intent.Currency = "usd"
	}
	if intent.Status == "" {
		intent.Status = "pending"
	}
	if intent.Metadata == nil {
		intent.Metadata = map[string]string{}
	}
	now := time.Now().UTC()
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = now
	}
	if intent.UpdatedAt.IsZero() {
		intent.UpdatedAt = now
	}
	metadata, err := json.Marshal(intent.Metadata)
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("marshal payment intent metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO payment_intents (
			id, provider, provider_checkout_session_id, organization_id, user_id,
			package_id, kind, amount, currency, status, metadata, created_at, updated_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13)
	`, intent.ID, intent.Provider, intent.ProviderCheckoutSessionID, intent.OrganizationID, intent.UserID, intent.PackageID, intent.Kind, intent.Amount, intent.Currency, intent.Status, metadata, intent.CreatedAt, intent.UpdatedAt)
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("insert payment intent: %w", err)
	}

	return intent, nil
}

func (s *SQLPaymentIntentStore) SetCheckoutSession(ctx context.Context, id string, providerCheckoutSessionID string, metadata map[string]string) error {
	if metadata == nil {
		metadata = map[string]string{}
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal payment intent metadata: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE payment_intents
		SET provider_checkout_session_id = $2, metadata = $3, updated_at = $4
		WHERE id = $1
	`, id, providerCheckoutSessionID, encodedMetadata, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update payment intent checkout session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("payment intent checkout rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
