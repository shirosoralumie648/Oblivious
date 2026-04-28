package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay/types"
)

type BillingStatus string

const (
	BillingStatusAuthorized BillingStatus = "authorized"
	BillingStatusSettled   BillingStatus = "settled"
	BillingStatusRefunded  BillingStatus = "refunded"
	BillingStatusFailed    BillingStatus = "failed"
)

type BillingSession struct {
	ID               string
	UserID           string
	ChannelID        string
	APIType         types.APIType
	Model           string
	IdempotencyKey  string
	RequestID       string
	AttemptNo       int
	PreAuthorizedAmt float64
	SettledAmt      float64
	QuotaSessionID  string
	Status          BillingStatus
	CreatedAt       time.Time
}

// QuotaManager is the exportable billing adapter interface that matches
// quota.Service.PreConsume/Settle/Refund signatures.  It is wired into
// BillingHook so that successful Relay calls create preauthorized quota
// sessions and settle them, while failed calls refund correctly.
type QuotaManager interface {
	PreConsume(ctx context.Context, userID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error)
	Settle(ctx context.Context, sessionID string, actualAmount float64) error
	Refund(ctx context.Context, sessionID string) error
}

type BillingHook struct {
	pricing      *PricingStore
	seenIdem     *map[string]bool
	QuotaManager QuotaManager
	mu           sync.Mutex
}

func NewBillingHook(pricing *PricingStore, seenIdem *map[string]bool) *BillingHook {
	return &BillingHook{
		pricing:  pricing,
		seenIdem: seenIdem,
	}
}

func (h *BillingHook) SetQuotaManager(manager QuotaManager) {
	h.QuotaManager = manager
}

func (h *BillingHook) PreBill(session *BillingSession, usage *types.Usage) (float64, error) {
	// Check idempotency
	if h.seenIdem != nil {
		h.mu.Lock()
		if (*h.seenIdem)[session.IdempotencyKey] {
			h.mu.Unlock()
			return session.PreAuthorizedAmt, nil
		}
		(*h.seenIdem)[session.IdempotencyKey] = true
		h.mu.Unlock()
	}

	// Estimate cost
	cost := h.pricing.CalculateCost(session.Model, session.APIType, usage)
	// Add 20% buffer for safety
	preAuth := cost * 1.2

	if h.QuotaManager != nil && session.UserID != "" {
		quotaSession, err := h.QuotaManager.PreConsume(
			context.Background(),
			session.UserID,
			preAuth,
			session.IdempotencyKey,
			session.ChannelID,
			session.Model,
			session.APIType.String(),
		)
		if err != nil {
			return 0, err
		}
		session.QuotaSessionID = quotaSession.ID
		session.PreAuthorizedAmt = quotaSession.PreAuthorizedAmt
	} else {
		session.PreAuthorizedAmt = preAuth
	}
	session.Status = BillingStatusAuthorized
	session.CreatedAt = time.Now()

	return session.PreAuthorizedAmt, nil
}

func (h *BillingHook) PostBill(session *BillingSession, usage *types.Usage) (float64, error) {
	// Check idempotency
	if h.seenIdem != nil {
		h.mu.Lock()
		key := session.IdempotencyKey + ":settled"
		if (*h.seenIdem)[key] {
			h.mu.Unlock()
			return session.SettledAmt, nil
		}
		(*h.seenIdem)[key] = true
		h.mu.Unlock()
	}

	actualCost := h.pricing.CalculateCost(session.Model, session.APIType, usage)

	if h.QuotaManager != nil && session.QuotaSessionID != "" {
		if err := h.QuotaManager.Settle(context.Background(), session.QuotaSessionID, actualCost); err != nil {
			return 0, err
		}
	} else {
		// Refund excess authorization
		excess := session.PreAuthorizedAmt - actualCost
		if excess > 0 {
			h.refund(session, excess)
		}
	}

	session.SettledAmt = actualCost
	session.Status = BillingStatusSettled

	return actualCost, nil
}

func (h *BillingHook) Refund(session *BillingSession) (float64, error) {
	refund := session.PreAuthorizedAmt - session.SettledAmt
	if refund < 0 {
		refund = 0
	}
	if h.QuotaManager != nil && session.QuotaSessionID != "" {
		if err := h.QuotaManager.Refund(context.Background(), session.QuotaSessionID); err != nil {
			return 0, err
		}
	}
	session.Status = BillingStatusRefunded
	return refund, nil
}

func (h *BillingHook) refund(session *BillingSession, amount float64) {
	// In production: call channel's refund endpoint
	_ = amount
	_ = session
}

func (h *BillingHook) SetRequestID(session *BillingSession, requestID string) {
	session.RequestID = requestID
}

func (h *BillingHook) IncrementAttempt(session *BillingSession) {
	session.AttemptNo++
}

func (h *BillingHook) BuildBillingSession(channelID, model string, apiType types.APIType, idempotencyKey, userID string) *BillingSession {
	now := time.Now()
	return &BillingSession{
		ID:               fmt.Sprintf("sess_%d", now.UnixNano()),
		ChannelID:        channelID,
		APIType:          apiType,
		Model:            model,
		IdempotencyKey:   idempotencyKey,
		UserID:           userID,
		PreAuthorizedAmt: 0,
		SettledAmt:       0,
		Status:           BillingStatusAuthorized,
		CreatedAt:        now,
	}
}
