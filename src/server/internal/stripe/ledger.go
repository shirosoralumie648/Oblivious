package stripe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent is the durable provider event record used for Stripe webhook
// idempotency and later billing/admin inspection.
type WebhookEvent struct {
	ID              string
	Provider        string
	EventID         string
	EventType       string
	Status          string
	OrganizationID  string
	UserID          string
	PaymentIntentID string
	Payload         json.RawMessage
	Error           string
	ReceivedAt      time.Time
	ProcessedAt     *time.Time
}

// WebhookLedger records provider webhook events exactly once per provider event ID.
type WebhookLedger interface {
	RecordWebhookEvent(ctx context.Context, event WebhookEvent) (bool, error)
}

// SQLWebhookLedger stores Stripe webhook events in PostgreSQL.
type SQLWebhookLedger struct {
	db *sql.DB
}

func NewSQLWebhookLedger(db *sql.DB) *SQLWebhookLedger {
	return &SQLWebhookLedger{db: db}
}

func (s *SQLWebhookLedger) RecordWebhookEvent(ctx context.Context, event WebhookEvent) (bool, error) {
	id := event.ID
	if id == "" {
		id = uuid.New().String()
	}
	provider := event.Provider
	if provider == "" {
		provider = "stripe"
	}
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO stripe_webhook_events (
			id, provider, event_id, event_type, status, organization_id, user_id,
			payment_intent_id, payload, error, received_at, processed_at
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, NULLIF($10, ''), $11, $12)
		ON CONFLICT (event_id) DO NOTHING
	`, id, provider, event.EventID, event.EventType, event.Status, event.OrganizationID, event.UserID, event.PaymentIntentID, event.Payload, event.Error, receivedAt, event.ProcessedAt)
	if err != nil {
		return false, fmt.Errorf("insert stripe webhook event: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("stripe webhook rows affected: %w", err)
	}
	return rows == 1, nil
}
