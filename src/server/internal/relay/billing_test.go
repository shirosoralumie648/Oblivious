package relay

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestBillingHook_PreBill(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:             "sess_123",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		UserGroup:      "vip",
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

func TestBillingHook_PreBillAppliesGroupMultiplier(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0)
	store.ApplyMultipliers(nil, map[string]float64{"vip": 0.5})
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:             "sess_123",
		ChannelID:      "ch_1",
		APIType:        types.APITypeChat,
		Model:          "gpt-4o",
		UserGroup:      "vip",
		IdempotencyKey: "idem_123",
	}

	preAuth, err := hook.PreBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preAuth != 3600 {
		t.Fatalf("expected vip pre-auth amount 3600, got %f", preAuth)
	}
	if session.PreAuthorizedAmt != 3600 {
		t.Fatalf("expected session pre-auth amount 3600, got %f", session.PreAuthorizedAmt)
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
		UserGroup:        "vip",
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

func TestBillingHook_PostBillAppliesGroupMultiplier(t *testing.T) {
	store := NewPricingStore()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0)
	store.ApplyMultipliers(nil, map[string]float64{"vip": 0.5})
	hook := NewBillingHook(store, nil)

	session := &BillingSession{
		ID:               "sess_123",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		UserGroup:        "vip",
		IdempotencyKey:   "idem_123",
		PreAuthorizedAmt: 4000.0,
	}

	settled, err := hook.PostBill(session, &types.Usage{PromptTokens: 1000, CompletionTokens: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settled != 3000 {
		t.Fatalf("expected vip settled amount 3000, got %f", settled)
	}
	if session.SettledAmt != 3000 {
		t.Fatalf("expected session settled amount 3000, got %f", session.SettledAmt)
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
		UserGroup:        "vip",
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

	session := hook.BuildBillingSession("ch_1", "gpt-4o", types.APITypeChat, "idem_123")
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
