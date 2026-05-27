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
	BillingStatusSettled    BillingStatus = "settled"
	BillingStatusRefunded   BillingStatus = "refunded"
	BillingStatusFailed     BillingStatus = "failed"
)

type BillingSession struct {
	ID               string
	OrganizationID   string
	UserID           string
	ChannelID        string
	APIType          types.APIType
	Model            string
	IdempotencyKey   string
	RequestID        string
	AttemptNo        int
	PreAuthorizedAmt float64
	SettledAmt       float64
	QuotaSessionID   string
	Status           BillingStatus
	CreatedAt        time.Time
}

// QuotaManager is the exportable billing adapter interface that matches
// quota.Service.PreConsume/Settle/Refund signatures.  It is wired into
// BillingHook so that successful Relay calls create preauthorized quota
// sessions and settle them, while failed calls refund correctly.
type QuotaManager interface {
	PreConsume(ctx context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error)
	Settle(ctx context.Context, sessionID string, actualAmount float64) error
	Refund(ctx context.Context, sessionID string) error
}

type BillingHook struct {
	pricing           *PricingStore
	seenIdem          *map[string]bool
	QuotaManager      QuotaManager
	mu                sync.Mutex
	preauthSnapshots  map[string]billingSnapshot
	settledSnapshots  map[string]billingSnapshot
	refundedSnapshots map[string]float64
}

func NewBillingHook(pricing *PricingStore, seenIdem *map[string]bool) *BillingHook {
	return &BillingHook{
		pricing:           pricing,
		seenIdem:          seenIdem,
		preauthSnapshots:  make(map[string]billingSnapshot),
		settledSnapshots:  make(map[string]billingSnapshot),
		refundedSnapshots: make(map[string]float64),
	}
}

func (h *BillingHook) SetQuotaManager(manager QuotaManager) {
	h.QuotaManager = manager
}

func (h *BillingHook) PreBill(session *BillingSession, usage *types.Usage) (float64, error) {
	// Check idempotency
	idempotencyKey := scopedBillingIdempotencyKey(session, "")
	if idempotencyKey != "" {
		h.mu.Lock()
		if snapshot, ok := h.preauthSnapshots[idempotencyKey]; ok {
			applyBillingSnapshot(session, snapshot)
			h.mu.Unlock()
			return session.PreAuthorizedAmt, nil
		}
		h.mu.Unlock()
	}

	// Estimate cost
	cost := h.pricing.CalculateCost(session.Model, session.APIType, usage)
	// Add 20% buffer for safety
	preAuth := cost * 1.2

	if h.QuotaManager != nil && session.UserID != "" && session.OrganizationID != "" {
		quotaSession, err := h.QuotaManager.PreConsume(
			context.Background(),
			session.UserID,
			session.OrganizationID,
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

	if idempotencyKey != "" {
		h.mu.Lock()
		if h.seenIdem != nil {
			(*h.seenIdem)[idempotencyKey] = true
		}
		h.preauthSnapshots[idempotencyKey] = snapshotFromBillingSession(session)
		h.mu.Unlock()
	}

	return session.PreAuthorizedAmt, nil
}

func (h *BillingHook) PostBill(session *BillingSession, usage *types.Usage) (float64, error) {
	// Check idempotency
	key := scopedBillingIdempotencyKey(session, "settled")
	if key != "" {
		h.mu.Lock()
		if snapshot, ok := h.settledSnapshots[key]; ok {
			applyBillingSnapshot(session, snapshot)
			h.mu.Unlock()
			return session.SettledAmt, nil
		}
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

	if key != "" {
		h.mu.Lock()
		if h.seenIdem != nil {
			(*h.seenIdem)[key] = true
		}
		h.settledSnapshots[key] = snapshotFromBillingSession(session)
		h.mu.Unlock()
	}

	return actualCost, nil
}

func (h *BillingHook) Refund(session *BillingSession) (float64, error) {
	key := scopedBillingIdempotencyKey(session, "refunded")
	if key != "" {
		h.mu.Lock()
		if refund, ok := h.refundedSnapshots[key]; ok {
			session.Status = BillingStatusRefunded
			h.mu.Unlock()
			return refund, nil
		}
		h.mu.Unlock()
	}

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
	if key != "" {
		h.mu.Lock()
		if h.seenIdem != nil {
			(*h.seenIdem)[key] = true
		}
		h.refundedSnapshots[key] = refund
		h.mu.Unlock()
	}
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

func (h *BillingHook) BuildBillingSession(channelID, model string, apiType types.APIType, idempotencyKey, userID, organizationID string) *BillingSession {
	now := time.Now()
	return &BillingSession{
		ID:               fmt.Sprintf("sess_%d", now.UnixNano()),
		OrganizationID:   organizationID,
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

func scopedBillingIdempotencyKey(session *BillingSession, suffix string) string {
	if session == nil || session.IdempotencyKey == "" {
		return ""
	}
	scope := session.OrganizationID
	if scope == "" {
		scope = "global"
	}
	if suffix != "" {
		return scope + "|" + session.IdempotencyKey + ":" + suffix
	}
	return scope + "|" + session.IdempotencyKey
}

type billingSnapshot struct {
	ChannelID        string
	APIType          types.APIType
	Model            string
	IdempotencyKey   string
	RequestID        string
	AttemptNo        int
	PreAuthorizedAmt float64
	SettledAmt       float64
	QuotaSessionID   string
	Status           BillingStatus
	CreatedAt        time.Time
}

func snapshotFromBillingSession(session *BillingSession) billingSnapshot {
	return billingSnapshot{
		ChannelID:        session.ChannelID,
		APIType:          session.APIType,
		Model:            session.Model,
		IdempotencyKey:   session.IdempotencyKey,
		RequestID:        session.RequestID,
		AttemptNo:        session.AttemptNo,
		PreAuthorizedAmt: session.PreAuthorizedAmt,
		SettledAmt:       session.SettledAmt,
		QuotaSessionID:   session.QuotaSessionID,
		Status:           session.Status,
		CreatedAt:        session.CreatedAt,
	}
}

func applyBillingSnapshot(session *BillingSession, snapshot billingSnapshot) {
	session.ChannelID = snapshot.ChannelID
	session.APIType = snapshot.APIType
	session.Model = snapshot.Model
	session.IdempotencyKey = snapshot.IdempotencyKey
	session.RequestID = snapshot.RequestID
	session.AttemptNo = snapshot.AttemptNo
	session.PreAuthorizedAmt = snapshot.PreAuthorizedAmt
	session.SettledAmt = snapshot.SettledAmt
	session.QuotaSessionID = snapshot.QuotaSessionID
	session.Status = snapshot.Status
	session.CreatedAt = snapshot.CreatedAt
}
