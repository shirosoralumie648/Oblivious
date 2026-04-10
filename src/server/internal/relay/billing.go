package relay

import (
	"fmt"
	"sync"
	"time"

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
	ChannelID        string
	APIType         types.APIType
	Model           string
	IdempotencyKey  string
	RequestID       string
	AttemptNo       int
	PreAuthorizedAmt float64
	SettledAmt      float64
	Status          BillingStatus
	CreatedAt       time.Time
}

type BillingHook struct {
	pricing  *PricingStore
	seenIdem *map[string]bool
	mu       sync.Mutex
}

func NewBillingHook(pricing *PricingStore, seenIdem *map[string]bool) *BillingHook {
	return &BillingHook{
		pricing:  pricing,
		seenIdem: seenIdem,
	}
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

	session.PreAuthorizedAmt = preAuth
	session.Status = BillingStatusAuthorized
	session.CreatedAt = time.Now()

	return preAuth, nil
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

	// Refund excess authorization
	excess := session.PreAuthorizedAmt - actualCost
	if excess > 0 {
		h.refund(session, excess)
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

func (h *BillingHook) BuildBillingSession(channelID, model string, apiType types.APIType, idempotencyKey string) *BillingSession {
	now := time.Now()
	return &BillingSession{
		ID:              fmt.Sprintf("sess_%d", now.UnixNano()),
		ChannelID:       channelID,
		APIType:         apiType,
		Model:           model,
		IdempotencyKey:  idempotencyKey,
		PreAuthorizedAmt: 0,
		SettledAmt:      0,
		Status:          BillingStatusAuthorized,
		CreatedAt:       now,
	}
}
