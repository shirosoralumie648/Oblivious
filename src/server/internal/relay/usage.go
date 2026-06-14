package relay

import (
	"context"
	"time"

	"oblivious/server/internal/quota"
)

type RelayUsageStatus string

const (
	RelayUsageStatusSuccess RelayUsageStatus = "success"
	RelayUsageStatusError   RelayUsageStatus = "error"
)

type RelayUsageLogRecord struct {
	UserID           string
	OrganizationID   string
	APITokenID       string
	RequestID        string
	APIType          string
	FeatureType      string
	QuotaMode        string
	Model            string
	ChannelID        string
	Provider         string
	Status           RelayUsageStatus
	StatusCode       int
	ErrorCode        string
	LatencyMS        int64
	Cost             float64
	ChannelCost      float64
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CreatedAt        time.Time
}

type UsageLogger interface {
	RecordRelayUsage(ctx context.Context, record RelayUsageLogRecord) error
}

type APITokenQuotaManager interface {
	PreAuthorizeRelayAPITokenQuota(ctx context.Context, tokenID string, amount float64) error
	SettleRelayAPITokenQuota(ctx context.Context, tokenID string, preauthorizedAmount, actualAmount float64) error
	RefundRelayAPITokenQuota(ctx context.Context, tokenID string, amount float64) error
}

type QuotaManager interface {
	PreConsume(ctx context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error)
	Settle(ctx context.Context, organizationID, sessionID string, actualAmount float64) error
	Refund(ctx context.Context, organizationID, sessionID string) error
}
