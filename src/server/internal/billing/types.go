package billing

import (
	"context"
	"time"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
)

// PaymentStatus 支付状态
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// UsageEvent 使用事件
type UsageEvent struct {
	ID              string
	UserID          string
	WorkspaceID     string
	ConversationID  string
	ModelID         string
	APIType         string
	InputTokens     int
	OutputTokens    int
	ImageCount      int
	AudioSeconds    float64
	RequestCount    int
	EstimatedCost   float64
	SettledCost     float64
	IdempotencyKey  string
	CreatedAt       time.Time
}

// Subscription 订阅
type Subscription struct {
	ID          string
	UserID      string
	WorkspaceID string
	PlanID      string
	Status      SubscriptionStatus
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	CancelAtPeriodEnd  bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Payment 支付记录
type Payment struct {
	ID          string
	UserID      string
	WorkspaceID string
	Amount      float64
	Currency    string
	Status      PaymentStatus
	Description string
	StripePaymentID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Store 持久化接口
type Store interface {
	CreateUsageEvent(ctx context.Context, event UsageEvent) (UsageEvent, error)
	GetUsageEventsByWorkspace(ctx context.Context, workspaceID string, since time.Time) ([]UsageEvent, error)
	GetSubscriptionByWorkspace(ctx context.Context, workspaceID string) (Subscription, error)
	CreateSubscription(ctx context.Context, sub Subscription) (Subscription, error)
	UpdateSubscription(ctx context.Context, sub Subscription) error
	CreatePayment(ctx context.Context, payment Payment) (Payment, error)
	GetPayment(ctx context.Context, paymentID string) (Payment, error)
	UpdatePaymentStatus(ctx context.Context, paymentID string, status PaymentStatus) error
}
