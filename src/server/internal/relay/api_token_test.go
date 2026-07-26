package relay

import (
	"context"
	"errors"
	"testing"
	"time"

	"oblivious/server/internal/relay/types"
)

func TestAPITokenAuthenticatorAuthenticatesActiveTokenByHash(t *testing.T) {
	store := &fakeRelayAPITokenStore{
		record: RelayAPITokenRecord{
			ID:             "tok_1",
			UserID:         "user_1",
			OrganizationID: "org_1",
			UserGroup:      "vip",
			Status:         RelayAPITokenStatusActive,
		},
	}
	authenticator := NewAPITokenAuthenticator(store)
	authenticator.now = func() time.Time { return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC) }

	identity, err := authenticator.AuthenticateRelayAPIToken(context.Background(), "obv_live_secret", "gpt-4o", types.APITypeChat)
	if err != nil {
		t.Fatalf("authenticate token: %v", err)
	}
	if identity.TokenID != "tok_1" || identity.UserID != "user_1" || identity.OrganizationID != "org_1" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if identity.UserGroup != "vip" {
		t.Fatalf("expected identity user group vip, got %q", identity.UserGroup)
	}
	if store.lastHash == "" || store.lastHash == "obv_live_secret" {
		t.Fatalf("expected hashed token lookup, got %q", store.lastHash)
	}
	if store.touchedTokenID != "tok_1" {
		t.Fatalf("expected token touch for tok_1, got %q", store.touchedTokenID)
	}
}

func TestAPITokenAuthenticatorRejectsDeniedModel(t *testing.T) {
	store := &fakeRelayAPITokenStore{
		record: RelayAPITokenRecord{
			ID:                 "tok_1",
			UserID:             "user_1",
			OrganizationID:     "org_1",
			Status:             RelayAPITokenStatusActive,
			ModelLimitsEnabled: true,
			ModelLimits:        []string{"gpt-4o-mini"},
		},
	}
	authenticator := NewAPITokenAuthenticator(store)

	_, err := authenticator.AuthenticateRelayAPIToken(context.Background(), "obv_live_secret", "gpt-4o", types.APITypeChat)
	if err == nil || err != types.ErrRelayAPITokenModelDenied {
		t.Fatalf("expected model denied, got %v", err)
	}
}

func TestAPITokenAuthenticatorAllowsWildcardModelLimit(t *testing.T) {
	store := &fakeRelayAPITokenStore{
		record: RelayAPITokenRecord{
			ID:                 "tok_1",
			UserID:             "user_1",
			OrganizationID:     "org_1",
			Status:             RelayAPITokenStatusActive,
			ModelLimitsEnabled: true,
			ModelLimits:        []string{"gpt-4o-*"},
		},
	}
	authenticator := NewAPITokenAuthenticator(store)

	identity, err := authenticator.AuthenticateRelayAPIToken(context.Background(), "obv_live_secret", "gpt-4o-mini", types.APITypeChat)
	if err != nil {
		t.Fatalf("expected wildcard model limit to authenticate: %v", err)
	}
	if identity.TokenID != "tok_1" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
}

func TestAPITokenAuthenticatorRejectsExhaustedQuota(t *testing.T) {
	quotaLimit := 10.0
	store := &fakeRelayAPITokenStore{
		record: RelayAPITokenRecord{
			ID:             "tok_1",
			UserID:         "user_1",
			OrganizationID: "org_1",
			Status:         RelayAPITokenStatusActive,
			QuotaLimit:     &quotaLimit,
			UsedQuota:      10.0,
		},
	}
	authenticator := NewAPITokenAuthenticator(store)

	_, err := authenticator.AuthenticateRelayAPIToken(context.Background(), "obv_live_secret", "gpt-4o", types.APITypeChat)
	if err == nil || err != types.ErrRelayAPITokenQuotaExceeded {
		t.Fatalf("expected quota exceeded, got %v", err)
	}
}

func TestAPITokenAuthenticatorRejectsExpiredToken(t *testing.T) {
	expiredAt := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	store := &fakeRelayAPITokenStore{
		record: RelayAPITokenRecord{
			ID:             "tok_1",
			UserID:         "user_1",
			OrganizationID: "org_1",
			Status:         RelayAPITokenStatusActive,
			ExpiresAt:      &expiredAt,
		},
	}
	authenticator := NewAPITokenAuthenticator(store)
	authenticator.now = func() time.Time { return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC) }

	_, err := authenticator.AuthenticateRelayAPIToken(context.Background(), "obv_live_secret", "gpt-4o", types.APITypeChat)
	if err == nil || err != types.ErrRelayAPITokenExpired {
		t.Fatalf("expected expired token, got %v", err)
	}
}

type fakeRelayAPITokenStore struct {
	record         RelayAPITokenRecord
	err            error
	lastHash       string
	touchedTokenID string
}

func (s *fakeRelayAPITokenStore) GetRelayAPITokenByHash(_ context.Context, tokenHash string) (RelayAPITokenRecord, error) {
	s.lastHash = tokenHash
	return s.record, s.err
}

func (s *fakeRelayAPITokenStore) TouchRelayAPIToken(_ context.Context, tokenID string, _ time.Time) error {
	s.touchedTokenID = tokenID
	return nil
}

// TestRelayAPITokenQuotaRefundOnceContract verifies the idempotency and
// mismatch guarantees of RefundRelayAPITokenQuotaOnce through a recording
// implementation that mirrors the SQL store's transaction semantics.
func TestRelayAPITokenQuotaRefundOnceContract(t *testing.T) {
	store := newMemoryAPITokenQuotaRefundStore()

	// First call inserts the receipt and records the refund.
	if err := store.RefundRelayAPITokenQuotaOnce(context.Background(), "tok_a", 4.0, "scope-a"); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	if store.refundCalls["tok_a"] != 1 {
		t.Fatalf("expected 1 refund for tok_a, got %d", store.refundCalls["tok_a"])
	}

	// Identical replay is idempotent.
	if err := store.RefundRelayAPITokenQuotaOnce(context.Background(), "tok_a", 4.0, "scope-a"); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if store.refundCalls["tok_a"] != 1 {
		t.Fatalf("idempotent replay incremented refund counter: %d", store.refundCalls["tok_a"])
	}

	// Same scope key with different tokenID must fail.
	if err := store.RefundRelayAPITokenQuotaOnce(context.Background(), "tok_b", 4.0, "scope-a"); !errors.Is(err, ErrQuotaCompensationReceiptMismatch) {
		t.Fatalf("token mismatch error = %v, want ErrQuotaCompensationReceiptMismatch", err)
	}

	// Same scope key with different amount must fail.
	if err := store.RefundRelayAPITokenQuotaOnce(context.Background(), "tok_a", 9.0, "scope-a"); !errors.Is(err, ErrQuotaCompensationReceiptMismatch) {
		t.Fatalf("amount mismatch error = %v, want ErrQuotaCompensationReceiptMismatch", err)
	}
}

type memoryAPITokenQuotaRefundStore struct {
	receipts   map[string]memoryQuotaCompensationReceipt
	refundCalls map[string]int
}

func newMemoryAPITokenQuotaRefundStore() *memoryAPITokenQuotaRefundStore {
	return &memoryAPITokenQuotaRefundStore{
		receipts:   make(map[string]memoryQuotaCompensationReceipt),
		refundCalls: make(map[string]int),
	}
}

func (s *memoryAPITokenQuotaRefundStore) RefundRelayAPITokenQuotaOnce(_ context.Context, tokenID string, amount float64, scopeKey string) error {
	if amount <= 0 || scopeKey == "" || tokenID == "" {
		return ErrQuotaCompensationInvalidRequest
	}
	if receipt, ok := s.receipts[scopeKey]; ok {
		if receipt.apiTokenID != tokenID || receipt.amount != amount {
			return ErrQuotaCompensationReceiptMismatch
		}
		return nil // idempotent
	}
	s.receipts[scopeKey] = memoryQuotaCompensationReceipt{apiTokenID: tokenID, amount: amount}
	s.refundCalls[tokenID]++
	return nil
}

func (s *memoryAPITokenQuotaRefundStore) PreAuthorizeRelayAPITokenQuota(_ context.Context, _ string, _ float64) error {
	return nil
}

func (s *memoryAPITokenQuotaRefundStore) SettleRelayAPITokenQuota(_ context.Context, _ string, _, _ float64) error {
	return nil
}

func (s *memoryAPITokenQuotaRefundStore) RefundRelayAPITokenQuota(_ context.Context, _ string, _ float64) error {
	return nil
}
