package relay

import (
	"context"
	"fmt"
	"testing"

	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay/types"
)

type stubQuotaManager struct {
	preconsumeCalls    int
	settleCalls        int
	refundCalls        int
	lastUserID         string
	lastOrganizationID string
	lastChannelID      string
	lastModel          string
	lastAPIType        string
	lastIdempotencyKey string
	lastSessionID      string
	lastSettledAmt     float64
	settleErr          error
	refundErr          error
}

func (s *stubQuotaManager) PreConsume(ctx context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error) {
	s.preconsumeCalls++
	s.lastUserID = userID
	s.lastOrganizationID = organizationID
	s.lastChannelID = channelID
	s.lastModel = model
	s.lastAPIType = apiType
	s.lastIdempotencyKey = idempotencyKey
	sessionID := fmt.Sprintf("quota_session_%d", s.preconsumeCalls)
	return &quota.BillingSession{
		ID:               sessionID,
		OrganizationID:   organizationID,
		UserID:           userID,
		ChannelID:        channelID,
		Model:            model,
		APIType:          apiType,
		IdempotencyKey:   idempotencyKey,
		PreAuthorizedAmt: amount,
	}, nil
}

func (s *stubQuotaManager) Settle(ctx context.Context, sessionID string, actualAmount float64) error {
	s.settleCalls++
	s.lastSessionID = sessionID
	s.lastSettledAmt = actualAmount
	if s.settleErr != nil {
		return s.settleErr
	}
	return nil
}

func (s *stubQuotaManager) Refund(ctx context.Context, sessionID string) error {
	s.refundCalls++
	s.lastSessionID = sessionID
	if s.refundErr != nil {
		return s.refundErr
	}
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
		IdempotencyKey:   "idem_123",
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

func TestBillingHook_DuplicateIdempotencyDoesNotPreConsumeTwice(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	session := &BillingSession{
		ID:             "sess_quota_dup",
		OrganizationID: "org_1",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_quota_dup",
	}

	firstPreAuth, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("first PreBill failed: %v", err)
	}
	secondPreAuth, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("duplicate PreBill failed: %v", err)
	}
	if firstPreAuth != secondPreAuth {
		t.Fatalf("expected duplicate PreBill to return cached amount, got %f vs %f", firstPreAuth, secondPreAuth)
	}
	if quotaManager.preconsumeCalls != 1 {
		t.Fatalf("expected duplicate idempotency to call PreConsume once, got %d", quotaManager.preconsumeCalls)
	}
}

func TestBillingHook_DuplicateIdempotencyIsScopedByOrganization(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	session1 := &BillingSession{
		ID:             "sess_org_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_shared",
	}
	session2 := &BillingSession{
		ID:             "sess_org_2",
		OrganizationID: "org_2",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_shared",
	}

	if _, err := hook.PreBill(session1, &types.Usage{PromptTokens: 1000}); err != nil {
		t.Fatalf("first PreBill failed: %v", err)
	}
	if _, err := hook.PreBill(session2, &types.Usage{PromptTokens: 1000}); err != nil {
		t.Fatalf("second organization PreBill failed: %v", err)
	}
	if quotaManager.preconsumeCalls != 2 {
		t.Fatalf("expected idempotency to be scoped per organization with 2 PreConsume calls, got %d", quotaManager.preconsumeCalls)
	}
}

func TestBillingHook_BuildBillingSession(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := hook.BuildBillingSession("ch_1", "gpt-4o", types.APITypeChat, "idem_123", "user_1", "org_1")
	if session.ChannelID != "ch_1" {
		t.Fatalf("expected ch_1, got %s", session.ChannelID)
	}
	if session.OrganizationID != "org_1" {
		t.Fatalf("expected org_1, got %s", session.OrganizationID)
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
		OrganizationID: "org_1",
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
	if quotaManager.lastOrganizationID != "org_1" {
		t.Fatalf("expected quota preconsume organization org_1, got %q", quotaManager.lastOrganizationID)
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
		IdempotencyKey:   "idem_noquota",
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
		OrganizationID: "org_1",
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

func TestBillingHook_DuplicatePreBillFreshSessionCopiesQuotaContext(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	first := &BillingSession{
		ID:             "sess_first",
		OrganizationID: "org_1",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_fresh_dup",
	}
	firstAmount, err := hook.PreBill(first, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("first PreBill failed: %v", err)
	}

	duplicate := &BillingSession{
		ID:             "sess_duplicate",
		OrganizationID: "org_1",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_fresh_dup",
	}
	duplicateAmount, err := hook.PreBill(duplicate, &types.Usage{PromptTokens: 1000})
	if err != nil {
		t.Fatalf("duplicate PreBill failed: %v", err)
	}

	if quotaManager.preconsumeCalls != 1 {
		t.Fatalf("expected duplicate PreBill to preconsume once, got %d", quotaManager.preconsumeCalls)
	}
	if duplicateAmount != firstAmount || duplicate.PreAuthorizedAmt != first.PreAuthorizedAmt {
		t.Fatalf("duplicate preauth amount = %f/%f, want %f", duplicateAmount, duplicate.PreAuthorizedAmt, firstAmount)
	}
	if duplicate.QuotaSessionID != first.QuotaSessionID || duplicate.QuotaSessionID == "" {
		t.Fatalf("duplicate quota session = %q, want original %q", duplicate.QuotaSessionID, first.QuotaSessionID)
	}
	if duplicate.Status != BillingStatusAuthorized {
		t.Fatalf("duplicate status = %s, want %s", duplicate.Status, BillingStatusAuthorized)
	}
}

func TestBillingHook_DuplicatePostBillFreshSessionReturnsPriorSettlement(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	first := &BillingSession{
		ID:               "sess_first",
		OrganizationID:   "org_1",
		UserID:           "user_1",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		IdempotencyKey:   "idem_settle_dup",
		QuotaSessionID:   "quota_session_1",
		PreAuthorizedAmt: 10,
		Status:           BillingStatusAuthorized,
	}
	firstSettled, err := hook.PostBill(first, &types.Usage{PromptTokens: 1000, CompletionTokens: 100})
	if err != nil {
		t.Fatalf("first PostBill failed: %v", err)
	}

	duplicate := &BillingSession{
		ID:             "sess_duplicate",
		OrganizationID: "org_1",
		UserID:         "user_1",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		IdempotencyKey: "idem_settle_dup",
		QuotaSessionID: "quota_session_1",
	}
	duplicateSettled, err := hook.PostBill(duplicate, &types.Usage{PromptTokens: 1000, CompletionTokens: 100})
	if err != nil {
		t.Fatalf("duplicate PostBill failed: %v", err)
	}

	if quotaManager.settleCalls != 1 {
		t.Fatalf("expected duplicate PostBill to settle once, got %d", quotaManager.settleCalls)
	}
	if duplicateSettled != firstSettled || duplicate.SettledAmt != firstSettled {
		t.Fatalf("duplicate settled amount = %f/%f, want %f", duplicateSettled, duplicate.SettledAmt, firstSettled)
	}
	if duplicate.Status != BillingStatusSettled {
		t.Fatalf("duplicate status = %s, want %s", duplicate.Status, BillingStatusSettled)
	}
}

func TestBillingHook_DuplicateRefundFreshSessionReturnsPriorRefund(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	seen := make(map[string]bool)
	hook := NewBillingHook(store, &seen)
	quotaManager := &stubQuotaManager{}
	hook.SetQuotaManager(quotaManager)

	first := &BillingSession{
		ID:               "sess_first",
		OrganizationID:   "org_1",
		UserID:           "user_1",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		IdempotencyKey:   "idem_refund_dup",
		QuotaSessionID:   "quota_session_1",
		PreAuthorizedAmt: 10,
		Status:           BillingStatusAuthorized,
	}
	firstRefund, err := hook.Refund(first)
	if err != nil {
		t.Fatalf("first Refund failed: %v", err)
	}

	duplicate := &BillingSession{
		ID:               "sess_duplicate",
		OrganizationID:   "org_1",
		UserID:           "user_1",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		IdempotencyKey:   "idem_refund_dup",
		QuotaSessionID:   "quota_session_1",
		PreAuthorizedAmt: 10,
	}
	duplicateRefund, err := hook.Refund(duplicate)
	if err != nil {
		t.Fatalf("duplicate Refund failed: %v", err)
	}

	if quotaManager.refundCalls != 1 {
		t.Fatalf("expected duplicate Refund to refund once, got %d", quotaManager.refundCalls)
	}
	if duplicateRefund != firstRefund {
		t.Fatalf("duplicate refund = %f, want %f", duplicateRefund, firstRefund)
	}
	if duplicate.Status != BillingStatusRefunded {
		t.Fatalf("duplicate status = %s, want %s", duplicate.Status, BillingStatusRefunded)
	}
}
