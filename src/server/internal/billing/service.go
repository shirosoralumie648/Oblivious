package billing

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"oblivious/server/internal/auth"
)

var ErrSubscriptionNotFound = errors.New("billing: subscription not found")
var ErrPaymentNotFound = errors.New("billing: payment not found")

// Service Billing 服务
type Service struct {
	store Store
	now   func() time.Time
}

// NewService 创建 Billing 服务
func NewService(store Store) *Service {
	return &Service{
		store: store,
		now:   time.Now,
	}
}

// HandleUsageEvent 处理使用事件，记录用量并返回事件
func (s *Service) HandleUsageEvent(ctx context.Context, userID, workspaceID, conversationID, modelID, apiType string, inputTokens, outputTokens, imageCount int, audioSeconds float64, idempotencyKey string) (UsageEvent, error) {
	event := UsageEvent{
		UserID:         userID,
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		ModelID:        modelID,
		APIType:        apiType,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		ImageCount:     imageCount,
		AudioSeconds:   audioSeconds,
		RequestCount:   1,
		IdempotencyKey: idempotencyKey,
	}

	stored, err := s.store.CreateUsageEvent(ctx, event)
	if err != nil {
		return UsageEvent{}, err
	}

	return stored, nil
}

// GetSubscription 获取工作区订阅
func (s *Service) GetSubscription(ctx context.Context, workspaceID string) (Subscription, error) {
	return s.store.GetSubscriptionByWorkspace(ctx, workspaceID)
}

// CreatePayment 创建支付记录
func (s *Service) CreatePayment(ctx context.Context, userID, workspaceID string, amount float64, currency, description string) (Payment, error) {
	paymentID, err := auth.NewID("pay")
	if err != nil {
		return Payment{}, err
	}

	payment := Payment{
		ID:          paymentID,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Amount:      amount,
		Currency:    currency,
		Status:      PaymentStatusPending,
		Description: description,
		CreatedAt:   s.now().UTC(),
		UpdatedAt:   s.now().UTC(),
	}

	return s.store.CreatePayment(ctx, payment)
}

// SQLStore 基于 database/sql 的 Store 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CreateUsageEvent(ctx context.Context, event UsageEvent) (UsageEvent, error) {
	eventID, err := auth.NewID("usage")
	if err != nil {
		return UsageEvent{}, err
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO billing_usage_events (
			id, user_id, workspace_id, conversation_id, model_id, api_type,
			input_tokens, output_tokens, image_count, audio_seconds,
			request_count, estimated_cost, settled_cost, idempotency_key, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, eventID, event.UserID, event.WorkspaceID, event.ConversationID,
		event.ModelID, event.APIType, event.InputTokens, event.OutputTokens,
		event.ImageCount, event.AudioSeconds, event.RequestCount,
		event.EstimatedCost, event.SettledCost, event.IdempotencyKey, now)
	if err != nil {
		return UsageEvent{}, err
	}

	event.ID = eventID
	event.CreatedAt = now
	return event, nil
}

func (s *SQLStore) GetUsageEventsByWorkspace(ctx context.Context, workspaceID string, since time.Time) ([]UsageEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, workspace_id, conversation_id, model_id, api_type,
			input_tokens, output_tokens, image_count, audio_seconds,
			request_count, estimated_cost, settled_cost, idempotency_key, created_at
		FROM billing_usage_events
		WHERE workspace_id = $1 AND created_at >= $2
		ORDER BY created_at ASC
	`, workspaceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []UsageEvent{}
	for rows.Next() {
		var event UsageEvent
		if err := rows.Scan(
			&event.ID, &event.UserID, &event.WorkspaceID, &event.ConversationID,
			&event.ModelID, &event.APIType, &event.InputTokens, &event.OutputTokens,
			&event.ImageCount, &event.AudioSeconds, &event.RequestCount,
			&event.EstimatedCost, &event.SettledCost, &event.IdempotencyKey, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

func (s *SQLStore) GetSubscriptionByWorkspace(ctx context.Context, workspaceID string) (Subscription, error) {
	var sub Subscription
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, workspace_id, plan_id, status,
			current_period_start, current_period_end, cancel_at_period_end,
			created_at, updated_at
		FROM billing_subscriptions
		WHERE workspace_id = $1
	`, workspaceID).Scan(
		&sub.ID, &sub.UserID, &sub.WorkspaceID, &sub.PlanID, &sub.Status,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAtPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Subscription{}, ErrSubscriptionNotFound
		}
		return Subscription{}, err
	}
	return sub, nil
}

func (s *SQLStore) CreateSubscription(ctx context.Context, sub Subscription) (Subscription, error) {
	subID, err := auth.NewID("sub")
	if err != nil {
		return Subscription{}, err
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO billing_subscriptions (
			id, user_id, workspace_id, plan_id, status,
			current_period_start, current_period_end, cancel_at_period_end,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, subID, sub.UserID, sub.WorkspaceID, sub.PlanID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd,
		now, now)
	if err != nil {
		return Subscription{}, err
	}

	sub.ID = subID
	sub.CreatedAt = now
	sub.UpdatedAt = now
	return sub, nil
}

func (s *SQLStore) UpdateSubscription(ctx context.Context, sub Subscription) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE billing_subscriptions
		SET plan_id = $2, status = $3, current_period_start = $4,
			current_period_end = $5, cancel_at_period_end = $6, updated_at = $7
		WHERE id = $1
	`, sub.ID, sub.PlanID, sub.Status, sub.CurrentPeriodStart,
		sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd, time.Now().UTC())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

func (s *SQLStore) CreatePayment(ctx context.Context, payment Payment) (Payment, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_payments (
			id, user_id, workspace_id, amount, currency, status,
			description, stripe_payment_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, payment.ID, payment.UserID, payment.WorkspaceID, payment.Amount,
		payment.Currency, payment.Status, payment.Description,
		payment.StripePaymentID, payment.CreatedAt, payment.UpdatedAt)
	if err != nil {
		return Payment{}, err
	}
	return payment, nil
}

func (s *SQLStore) GetPayment(ctx context.Context, paymentID string) (Payment, error) {
	var payment Payment
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, workspace_id, amount, currency, status,
			description, stripe_payment_id, created_at, updated_at
		FROM billing_payments
		WHERE id = $1
	`, paymentID).Scan(
		&payment.ID, &payment.UserID, &payment.WorkspaceID, &payment.Amount,
		&payment.Currency, &payment.Status, &payment.Description,
		&payment.StripePaymentID, &payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	return payment, nil
}

func (s *SQLStore) UpdatePaymentStatus(ctx context.Context, paymentID string, status PaymentStatus) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE billing_payments SET status = $2, updated_at = $3 WHERE id = $1
	`, paymentID, status, time.Now().UTC())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrPaymentNotFound
	}
	return nil
}
