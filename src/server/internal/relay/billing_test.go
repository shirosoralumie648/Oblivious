package relay

import (
	"context"
	"testing"

	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay/types"
)

type stubQuotaManager struct {
	preconsumeCalls  int
	settleCalls      int
	refundCalls      int
	lastUserID       string
	lastSessionID    string
	lastSettledAmt   float64
}

func (s *stubQuotaManager) PreConsume(ctx context.Context, userID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error) {
	s.preconsumeCalls++
	s.lastUserID = userID
	return &quota.BillingSession{
		ID:               "quota_session_1",
		UserID:           userID,
		PreAuthorizedAmt: amount,
	}, nil
}

func (s *stubQuotaManager) Settle(ctx context.Context, sessionID string, actualAmount float64) error {
	s.settleCalls++
	s.lastSessionID = sessionID
	s.lastSettledAmt = actualAmount
	return nil
}

func (s *stubQuotaManager) Refund(ctx context.Context, sessionID string) error {
	s.refundCalls++
	return nil
}

func TestBillingHook_PreBill(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:             "sess_123",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_123",
	}

	preAuth, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preAuth <= 0 {
		t.Fatal("PreBill should return positive pre_auth_amount")
	}
	if session.PreAuthorizedAmt <= 0 {
		t.Fatal("session.PreAuthorizedAmt should be set")
	}
}

func TestBillingHook_PostBill_Settles(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:               "sess_123",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		IdempotencyKey:  "idem_123",
		PreAuthorizedAmt: 10.0,
	}

	settled, err := hook.PostBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settled <= 0 {
		t.Fatal("PostBill should settle positive amount")
	}
}

func TestBillingHook_Refund(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:               "sess_123",
		ChannelID:        "ch_1",
		PreAuthorizedAmt: 10.0,
		SettledAmt:       5.0,
	}

	refunded, err := hook.Refund(session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refunded != 5.0 {
		t.Fatalf("expected refund of 5.0, got %f", refunded)
	}
	if session.Status != BillingStatusRefunded {
		t.Fatalf("expected status Refunded, got %s", session.Status)
	}
}

func TestBillingHook_DuplicateIdempotency(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)

	session := &BillingSession{
		ID:             "sess_123",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_123",
	}

	_, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second PreBill with same idempotency key should return cached
	_, err = hook.PreBill(session, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("duplicate PreBill should not error: %v", err)
	}
}

func TestBillingHook_BuildBillingSession(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := hook.BuildBillingSession("ch_1", "gpt-4o", types.APITypeChat, "idem_123", "")
	if session.ChannelID != "ch_1" {
		t.Fatalf("expected ch_1, got %s", session.ChannelID)
	}
	if session.Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", session.Model)
	}
	if session.Status != BillingStatusAuthorized {
		t.Fatalf("expected Authorized, got %s", session.Status)
	}
	if session.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

func TestBillingHook_PreBillAndPostBill_UseQuotaLifecycle(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	session := &BillingSession{
		ID:             "sess_123",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_123",
	}

	preAuth, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("prebill error: %v", err)
	}
	if preAuth <= 0 {
		t.Fatalf("expected positive preauth, got %f", preAuth)
	}
	if quotaManager.preconsumeCalls != 1 {
		t.Fatalf("expected 1 preconsume call, got %d", quotaManager.preconsumeCalls)
	}
	if session.QuotaSessionID != "quota_session_1" {
		t.Fatalf("expected quota session to be stored, got %q", session.QuotaSessionID)
	}

	settled, err := hook.PostBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("postbill error: %v", err)
	}
	if settled <= 0 {
		t.Fatalf("expected positive settled amount, got %f", settled)
	}
	if quotaManager.settleCalls != 1 {
		t.Fatalf("expected settle to be called once, got %d", quotaManager.settleCalls)
	}
	if quotaManager.lastSessionID != "quota_session_1" {
		t.Fatalf("expected settle to use quota session id, got %q", quotaManager.lastSessionID)
	}
}

func TestBillingHook_PostBill_NoQuotaManagerFallsBack(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:               "sess_noquota",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		IdempotencyKey:  "idem_noquota",
		PreAuthorizedAmt: 10.0,
	}

	settled, err := hook.PostBill(session, &types.Usage{PromptTokens: 500, CompletionTokens: 250})
	if err != nil {
		t.Fatalf("PostBill without QuotaManager should not error: %v", err)
	}
	if settled <= 0 {
		t.Fatal("PostBill should compute actual cost even without QuotaManager")
	}
	if session.Status != BillingStatusSettled {
		t.Fatalf("expected Settled status, got %s", session.Status)
	}
	if session.SettledAmt <= 0 {
		t.Fatal("settled amount should be positive")
	}
}

func TestBillingHook_Refund_NoQuotaManagerStillMarksRefunded(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:               "sess_noquota_refund",
		ChannelID:        "ch_1",
		PreAuthorizedAmt: 10.0,
		SettledAmt:       5.0,
	}

	refunded, err := hook.Refund(session)
	if err != nil {
		t.Fatalf("Refund without QuotaManager should not error: %v", err)
	}
	if refunded != 5.0 {
		t.Fatalf("expected refund 5.0, got %f", refunded)
	}
	if session.Status != BillingStatusRefunded {
		t.Fatalf("expected Refunded status, got %s", session.Status)
	}
}

func TestBillingHook_PreBillAndRefund_UseQuotaLifecycle(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	// Use the same session across PreBill and Refund to verify
	// that the Refund call reaches the correct quota session.
	session := &BillingSession{
		ID:             "sess_123",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_refund_test",
	}

	_, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("prebill error: %v", err)
	}
	if quotaManager.preconsumeCalls != 1 {
		t.Fatalf("expected 1 preconsume call, got %d", quotaManager.preconsumeCalls)
	}
	if session.QuotaSessionID != "quota_session_1" {
		t.Fatalf("expected quota session to be stored, got %q", session.QuotaSessionID)
	}

	_, err = hook.Refund(session)
	if err != nil {
		t.Fatalf("refund error: %v", err)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("expected 1 refund call after PreBill, got %d", quotaManager.refundCalls)
	}
	if session.Status != BillingStatusRefunded {
		t.Fatalf("expected status Refunded after refund, got %s", session.Status)
	}
}
