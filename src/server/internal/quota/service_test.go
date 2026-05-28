package quota

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"oblivious/server/internal/metrics"
)

// fakeStore implements Store for testing quota.Service.
type fakeStore struct {
	quotas          map[string]*Quota
	billingSessions map[string]*BillingSession
	idempotencyKeys map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		quotas:          make(map[string]*Quota),
		billingSessions: make(map[string]*BillingSession),
		idempotencyKeys: make(map[string]string),
	}
}

func (s *fakeStore) GetOrCreateQuota(ctx context.Context, userID, organizationID string) (*Quota, error) {
	if s.quotas[organizationID] == nil {
		s.quotas[organizationID] = &Quota{
			ID:             "quota_" + organizationID,
			OrganizationID: organizationID,
			UserID:         userID,
			Balance:        100.0,
			Used:           0,
		}
	}
	return s.quotas[organizationID], nil
}

func (s *fakeStore) UpdateQuotaBalance(ctx context.Context, userID, organizationID string, delta float64) error {
	q, _ := s.GetOrCreateQuota(ctx, userID, organizationID)
	q.Balance += delta
	if delta < 0 {
		q.Used -= delta
	}
	return nil
}

func (s *fakeStore) CreateBillingSession(ctx context.Context, session *BillingSession) (*BillingSession, error) {
	if session.IdempotencyKey != "" {
		s.idempotencyKeys[session.OrganizationID+"|"+session.IdempotencyKey] = session.ID
	}
	s.billingSessions[session.ID] = session
	return session, nil
}

func (s *fakeStore) GetBillingSessionByIdempotencyKey(ctx context.Context, key, organizationID string) (*BillingSession, error) {
	if id, ok := s.idempotencyKeys[organizationID+"|"+key]; ok {
		if session, exists := s.billingSessions[id]; exists {
			return session, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) SettleBillingSession(ctx context.Context, id string, settledAmt float64) error {
	session, ok := s.billingSessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}
	session.Status = "settled"
	session.SettledAmt = settledAmt

	// Refund difference.
	refund := session.PreAuthorizedAmt - settledAmt
	if refund > 0 {
		s.UpdateQuotaBalance(ctx, session.UserID, session.OrganizationID, refund)
	}
	return nil
}

func (s *fakeStore) RefundBillingSession(ctx context.Context, id string) error {
	session, ok := s.billingSessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}
	session.Status = "refunded"
	s.UpdateQuotaBalance(ctx, session.UserID, session.OrganizationID, session.PreAuthorizedAmt)
	return nil
}

// Unused Store methods.
func (s *fakeStore) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	return nil, nil
}
func (s *fakeStore) GetPackage(ctx context.Context, id string) (*Package, error) { return nil, nil }
func (s *fakeStore) CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error) {
	return nil, nil
}
func (s *fakeStore) ListActiveSubscriptions(ctx context.Context, userID, organizationID string) ([]*Subscription, error) {
	return nil, nil
}
func (s *fakeStore) CreateTopupOrder(ctx context.Context, order *TopupOrder) (*TopupOrder, error) {
	return nil, nil
}
func (s *fakeStore) UpdateTopupOrderCheckoutSession(ctx context.Context, paymentIntentID string, providerCheckoutSessionID string) error {
	return nil
}
func (s *fakeStore) UpdateTopupOrderStatus(ctx context.Context, id string, status string, tradeNo string) error {
	return nil
}

func TestPreConsume_Success(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, err := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_1", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("PreConsume failed: %v", err)
	}
	if session.ID == "" {
		t.Fatal("session ID should be set")
	}
	if session.Status != "preauthorized" {
		t.Fatalf("expected preauthorized status, got %s", session.Status)
	}
	if session.PreAuthorizedAmt != 10.0 {
		t.Fatalf("expected 10.0 preauthorized, got %f", session.PreAuthorizedAmt)
	}
	if session.OrganizationID != "org_1" {
		t.Fatalf("expected billing session organization org_1, got %q", session.OrganizationID)
	}

	// Balance should have decreased.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 90.0 {
		t.Fatalf("expected balance 90.0, got %f", q.Balance)
	}
}

func TestPreConsume_InsufficientBalance(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	_, err := svc.PreConsume(context.Background(), "user_1", "org_1", 200.0, "idem_2", "ch_1", "gpt-4o", "chat")
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Fatalf("expected insufficient balance message, got: %v", err)
	}

	// Balance should remain unchanged.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 100.0 {
		t.Fatalf("expected balance unchanged at 100.0, got %f", q.Balance)
	}
}

func TestPreConsume_Idempotency(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session1, err := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_dup", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("first PreConsume failed: %v", err)
	}

	// Second call with same idempotency key returns existing session without double-charging.
	session2, err := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_dup", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("idempotent PreConsume failed: %v", err)
	}
	if session1.ID != session2.ID {
		t.Fatalf("expected same session ID for idempotent call. got %s vs %s", session1.ID, session2.ID)
	}

	// Balance should only be deducted once.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 90.0 {
		t.Fatalf("expected balance 90.0 (single deduction), got %f", q.Balance)
	}
}

func TestPreConsume_IdempotencyScopedByOrganization(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session1, err := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_shared", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("first PreConsume failed: %v", err)
	}
	session2, err := svc.PreConsume(context.Background(), "user_1", "org_2", 10.0, "idem_shared", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("second organization PreConsume failed: %v", err)
	}
	if session1.ID == session2.ID {
		t.Fatal("same idempotency key in different organizations must create distinct billing sessions")
	}
	if store.quotas["org_1"].Balance != 90.0 {
		t.Fatalf("expected org_1 balance 90.0, got %f", store.quotas["org_1"].Balance)
	}
	if store.quotas["org_2"].Balance != 90.0 {
		t.Fatalf("expected org_2 balance 90.0, got %f", store.quotas["org_2"].Balance)
	}
}

func TestPreConsume_EmptyIdempotencyKeyCreatesNewSession(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session1, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 5.0, "", "ch_1", "gpt-4o", "chat")
	session2, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 5.0, "", "ch_1", "gpt-4o", "chat")

	if session1.ID == session2.ID {
		t.Fatal("empty idempotency keys should create separate billing sessions")
	}

	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 90.0 {
		t.Fatalf("expected balance 90.0 (two 5.0 deductions), got %f", q.Balance)
	}
}

func TestSettle_FullAmount(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_settle", "ch_1", "gpt-4o", "chat")

	err := svc.Settle(context.Background(), session.ID, 10.0)
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}

	updated := store.billingSessions[session.ID]
	if updated.Status != "settled" {
		t.Fatalf("expected settled status, got %s", updated.Status)
	}
	if updated.SettledAmt != 10.0 {
		t.Fatalf("expected settled amount 10.0, got %f", updated.SettledAmt)
	}

	// No refund: balance should be exactly 90.0 (100 - 10).
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 90.0 {
		t.Fatalf("expected balance 90.0, got %f", q.Balance)
	}
}

func TestSettle_PartialAmountRefundsDifference(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_partial", "ch_1", "gpt-4o", "chat")

	err := svc.Settle(context.Background(), session.ID, 3.0)
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}

	updated := store.billingSessions[session.ID]
	if updated.Status != "settled" {
		t.Fatalf("expected settled status, got %s", updated.Status)
	}
	if updated.SettledAmt != 3.0 {
		t.Fatalf("expected settled amount 3.0, got %f", updated.SettledAmt)
	}

	// 7.0 should be refunded: balance = 100 - 10 + 7 = 97.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 97.0 {
		t.Fatalf("expected balance 97.0 after partial settle refund, got %f", q.Balance)
	}
}

func TestRefund_FullRefund(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_refund", "ch_1", "gpt-4o", "chat")

	err := svc.Refund(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}

	updated := store.billingSessions[session.ID]
	if updated.Status != "refunded" {
		t.Fatalf("expected refunded status, got %s", updated.Status)
	}

	// Full refund: balance should be back to 100.0.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 100.0 {
		t.Fatalf("expected balance 100.0 after full refund, got %f", q.Balance)
	}
}

func TestRefund_AlreadySettledSession(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_refund_settled", "ch_1", "gpt-4o", "chat")
	svc.Settle(context.Background(), session.ID, 10.0)

	err := svc.Refund(context.Background(), session.ID)
	if err == nil {
		t.Fatal("expected error when refunding already settled session, got nil")
	}
	if !strings.Contains(err.Error(), "already settled or refunded") {
		t.Fatalf("expected already settled error, got: %v", err)
	}
}

func TestQuotaObservabilityRecordsSettlementFailure(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	before := testutil.ToFloat64(metrics.QuotaSettlementFailuresTotal.WithLabelValues("settlement"))
	err := svc.Settle(context.Background(), "missing_session", 10)
	if err == nil {
		t.Fatal("expected missing session settlement error")
	}
	after := testutil.ToFloat64(metrics.QuotaSettlementFailuresTotal.WithLabelValues("settlement"))
	if after != before+1 {
		t.Fatalf("expected settlement failure metric increment, before=%v after=%v", before, after)
	}
}

func TestPreConsume_Settle_Refund_Lifecycle(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	// Step 1: PreConsume.
	session, err := svc.PreConsume(context.Background(), "user_1", "org_1", 20.0, "idem_lifecycle", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("PreConsume failed: %v", err)
	}
	if session.Status != "preauthorized" {
		t.Fatalf("expected preauthorized, got %s", session.Status)
	}

	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 80.0 {
		t.Fatalf("expected balance 80.0 after preconsume, got %f", q.Balance)
	}

	// Step 2: Refund (simulating a failure before settle).
	err = svc.Refund(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}

	q, _ = store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 100.0 {
		t.Fatalf("expected balance 100.0 after refund, got %f", q.Balance)
	}

	// Step 3: New PreConsume and Settle.
	session2, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 15.0, "idem_lifecycle_2", "ch_1", "gpt-4o", "chat")
	q, _ = store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 85.0 {
		t.Fatalf("expected balance 85.0 after second preconsume, got %f", q.Balance)
	}

	svc.Settle(context.Background(), session2.ID, 12.0)
	q, _ = store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 88.0 {
		t.Fatalf("expected balance 88.0 after partial settle (85 + 3 refund), got %f", q.Balance)
	}
}

func TestGetBalance_NewUser(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	q, err := svc.GetBalance(context.Background(), "new_user", "org_new")
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if q.Balance != 100.0 {
		t.Fatalf("expected default balance 100.0, got %f", q.Balance)
	}
	if q.UserID != "new_user" {
		t.Fatalf("expected userID new_user, got %s", q.UserID)
	}
}

func TestTopup(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	err := svc.Topup(context.Background(), "user_1", "org_1", 50.0)
	if err != nil {
		t.Fatalf("Topup failed: %v", err)
	}

	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 150.0 {
		t.Fatalf("expected balance 150.0 after topup, got %f", q.Balance)
	}
}
