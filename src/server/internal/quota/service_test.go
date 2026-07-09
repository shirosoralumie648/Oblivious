package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/relay/ratelimit"
)

// fakeStore implements Store for testing quota.Service.
type fakeStore struct {
	quotas          map[string]*Quota
	billingSessions map[string]*BillingSession
	idempotencyKeys map[string]string
	usageLimits     map[string]UsageLimitSettings
	packages        map[string]*Package
	subscriptions   []*Subscription
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		quotas:          make(map[string]*Quota),
		billingSessions: make(map[string]*BillingSession),
		idempotencyKeys: make(map[string]string),
		usageLimits:     make(map[string]UsageLimitSettings),
		packages:        make(map[string]*Package),
	}
}

func (s *fakeStore) GetOrCreateQuota(ctx context.Context, userID, organizationID string) (*Quota, error) {
	key := s.quotaBalanceKey(userID, organizationID)
	if s.quotas[key] == nil {
		s.quotas[key] = &Quota{
			ID:             "quota_" + organizationID + "_" + userID,
			OrganizationID: organizationID,
			UserID:         userID,
			Balance:        100.0,
			Used:           0,
		}
	}
	return s.quotas[key], nil
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

func (s *fakeStore) SettleBillingSession(ctx context.Context, id, organizationID string, settledAmt float64) error {
	if settledAmt < 0 {
		return fmt.Errorf("settled amount must be non-negative")
	}
	session, ok := s.billingSessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.OrganizationID != organizationID {
		return fmt.Errorf("session not found")
	}
	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}
	if settledAmt > session.PreAuthorizedAmt {
		return fmt.Errorf("settled amount %.6f exceeds preauthorized amount %.6f", settledAmt, session.PreAuthorizedAmt)
	}
	session.Status = "settled"
	session.SettledAmt = settledAmt

	// Refund difference.
	refund := session.PreAuthorizedAmt - settledAmt
	if refund > 0 {
		q, _ := s.GetOrCreateQuota(ctx, session.UserID, session.OrganizationID)
		q.Balance += refund
		q.Used -= refund
		if q.Used < 0 {
			q.Used = 0
		}
	}
	return nil
}

func (s *fakeStore) RefundBillingSession(ctx context.Context, id, organizationID string) error {
	session, ok := s.billingSessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.OrganizationID != organizationID {
		return fmt.Errorf("session not found")
	}
	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}
	session.Status = "refunded"
	q, _ := s.GetOrCreateQuota(ctx, session.UserID, session.OrganizationID)
	q.Balance += session.PreAuthorizedAmt
	q.Used -= session.PreAuthorizedAmt
	if q.Used < 0 {
		q.Used = 0
	}
	return nil
}

// Unused Store methods.
func (s *fakeStore) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	packages := make([]*Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		if activeOnly && (!pkg.IsActive || !pkg.IsPublic) {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}
func (s *fakeStore) GetPackage(ctx context.Context, id string) (*Package, error) {
	return s.packages[id], nil
}
func (s *fakeStore) CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error) {
	s.subscriptions = append(s.subscriptions, sub)
	return sub, nil
}
func (s *fakeStore) ListActiveSubscriptions(ctx context.Context, userID, organizationID string) ([]*Subscription, error) {
	subs := []*Subscription{}
	for _, sub := range s.subscriptions {
		if sub.UserID == userID && sub.OrganizationID == organizationID && sub.Status == "active" {
			subs = append(subs, sub)
		}
	}
	return subs, nil
}
func (s *fakeStore) CreateTopupOrder(ctx context.Context, order *TopupOrder) (*TopupOrder, error) {
	return nil, nil
}
func (s *fakeStore) UpdateTopupOrderCheckoutSession(ctx context.Context, organizationID, paymentIntentID string, providerCheckoutSessionID string) error {
	return nil
}
func (s *fakeStore) MarkTopupOrderFailedByPaymentIntent(ctx context.Context, organizationID, paymentIntentID string) error {
	return nil
}
func (s *fakeStore) UpdateTopupOrderStatus(ctx context.Context, id, organizationID string, status string, tradeNo string) error {
	return nil
}
func (s *fakeStore) SaveUsageLimitSettings(ctx context.Context, settings UsageLimitSettings) (*UsageLimitSettings, error) {
	normalized, err := normalizeUsageLimitSettings(settings)
	if err != nil {
		return nil, err
	}
	normalized.UpdatedAt = time.Now().UTC()
	s.usageLimits[usageLimitSettingsKey(normalized.OrganizationID, normalized.UserID)] = normalized
	return &normalized, nil
}
func (s *fakeStore) ResolveUsageLimitSettings(ctx context.Context, organizationID, userID string) (UsageLimitSettings, error) {
	if userID != "" {
		if settings, ok := s.usageLimits[usageLimitSettingsKey(organizationID, userID)]; ok {
			return settings, nil
		}
	}
	if settings, ok := s.usageLimits[usageLimitSettingsKey(organizationID, "")]; ok {
		return settings, nil
	}
	return UsageLimitSettings{OrganizationID: organizationID, UserID: userID, QuotaMode: usageLimitQuotaMode(userID)}, nil
}
func (s *fakeStore) ListUsageLimitSettings(ctx context.Context, organizationID string) ([]UsageLimitSettings, error) {
	settings := make([]UsageLimitSettings, 0, len(s.usageLimits))
	if orgSettings, ok := s.usageLimits[usageLimitSettingsKey(organizationID, "")]; ok {
		settings = append(settings, orgSettings)
	}
	for _, item := range s.usageLimits {
		if item.OrganizationID == organizationID && item.UserID != "" {
			settings = append(settings, item)
		}
	}
	return settings, nil
}

func usageLimitSettingsKey(organizationID, userID string) string {
	return organizationID + "|" + userID
}

func (s *fakeStore) quotaBalanceKey(userID, organizationID string) string {
	if userID != "" {
		if settings, ok := s.usageLimits[usageLimitSettingsKey(organizationID, userID)]; ok && settings.QuotaMode == "user" {
			return organizationID + "|" + userID
		}
	}
	return organizationID + "|"
}

func quotaBalanceKey(userID, organizationID string) string {
	if userID == "" {
		return organizationID + "|"
	}
	return organizationID + "|" + userID
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

func TestPreConsumeRejectsNonPositiveAmount(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	if _, err := svc.PreConsume(context.Background(), "user_1", "org_1", 0, "idem_zero", "ch_1", "gpt-4o", "chat"); err == nil {
		t.Fatal("expected zero preauthorization amount to fail")
	} else if !strings.Contains(err.Error(), "preauthorized amount must be positive") {
		t.Fatalf("expected positive amount validation error, got %v", err)
	}

	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 100.0 || q.Used != 0 {
		t.Fatalf("expected balance to remain unchanged after invalid amount, got %+v", q)
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
	if store.quotas[quotaBalanceKey("", "org_1")].Balance != 90.0 {
		t.Fatalf("expected org_1 balance 90.0, got %f", store.quotas[quotaBalanceKey("", "org_1")].Balance)
	}
	if store.quotas[quotaBalanceKey("", "org_2")].Balance != 90.0 {
		t.Fatalf("expected org_2 balance 90.0, got %f", store.quotas[quotaBalanceKey("", "org_2")].Balance)
	}
}

func TestPreConsumeUserQuotaModeUsesUserScopedBalance(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	ctx := context.Background()

	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		MaxTokensPerWindow:  1000,
		MaxTokensPerRequest: 100,
	}); err != nil {
		t.Fatalf("save organization quota mode: %v", err)
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		UserID:              "user_2",
		QuotaMode:           "user",
		MaxTokensPerWindow:  500,
		MaxTokensPerRequest: 50,
	}); err != nil {
		t.Fatalf("save user quota mode: %v", err)
	}

	if _, err := svc.PreConsume(ctx, "user_2", "org_1", 25.0, "idem_user_mode", "ch_1", "gpt-4o", "chat"); err != nil {
		t.Fatalf("user-mode PreConsume failed: %v", err)
	}

	userQuota, _ := store.GetOrCreateQuota(ctx, "user_2", "org_1")
	orgQuota, _ := store.GetOrCreateQuota(ctx, "user_1", "org_1")
	if userQuota.Balance != 75.0 || userQuota.UserID != "user_2" {
		t.Fatalf("expected user_2 scoped balance 75, got %+v", userQuota)
	}
	if orgQuota.Balance != 100.0 || orgQuota.UserID != "user_1" {
		t.Fatalf("expected organization/default balance to remain 100 for user_1, got %+v", orgQuota)
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

	err := svc.Settle(context.Background(), "org_1", session.ID, 10.0)
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

	err := svc.Settle(context.Background(), "org_1", session.ID, 3.0)
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
	if q.Balance != 97.0 || q.Used != 3.0 {
		t.Fatalf("expected balance 97.0 and used 3.0 after partial settle refund, got %+v", q)
	}
}

func TestSettleRejectsAmountAbovePreauthorization(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_over_settle", "ch_1", "gpt-4o", "chat")

	err := svc.Settle(context.Background(), "org_1", session.ID, 12.0)
	if err == nil {
		t.Fatal("expected over-preauthorization settlement to fail")
	}
	if !strings.Contains(err.Error(), "exceeds preauthorized amount") {
		t.Fatalf("expected over-preauthorization error, got %v", err)
	}
	updated := store.billingSessions[session.ID]
	if updated.Status != "preauthorized" || updated.SettledAmt != 0 {
		t.Fatalf("expected failed over-settle to preserve session, got %+v", updated)
	}
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 90.0 || q.Used != 10.0 {
		t.Fatalf("expected failed over-settle to preserve quota reservation, got %+v", q)
	}
}

func TestRefund_FullRefund(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_refund", "ch_1", "gpt-4o", "chat")

	err := svc.Refund(context.Background(), "org_1", session.ID)
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}

	updated := store.billingSessions[session.ID]
	if updated.Status != "refunded" {
		t.Fatalf("expected refunded status, got %s", updated.Status)
	}

	// Full refund: balance should be back to 100.0.
	q, _ := store.GetOrCreateQuota(context.Background(), "user_1", "org_1")
	if q.Balance != 100.0 || q.Used != 0 {
		t.Fatalf("expected balance 100.0 and used 0 after full refund, got %+v", q)
	}
}

func TestRefund_AlreadySettledSession(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)

	session, _ := svc.PreConsume(context.Background(), "user_1", "org_1", 10.0, "idem_refund_settled", "ch_1", "gpt-4o", "chat")
	svc.Settle(context.Background(), "org_1", session.ID, 10.0)

	err := svc.Refund(context.Background(), "org_1", session.ID)
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
	err := svc.Settle(context.Background(), "org_1", "missing_session", 10)
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
	err = svc.Refund(context.Background(), "org_1", session.ID)
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

	svc.Settle(context.Background(), "org_1", session2.ID, 12.0)
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

func TestQuotaConcurrencyLimitPreventsSecondLeaseUntilReleased(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, WithRateLimiter(ratelimit.NewInMemoryRateLimiter(ratelimit.InMemoryOptions{})))
	ctx := context.Background()
	limit := UsageLimit{OrganizationID: "org_1", UserID: "user_1", MaxConcurrentRequests: 1}

	lease, err := svc.BeginUsage(ctx, limit)
	if err != nil {
		t.Fatalf("BeginUsage first lease returned error: %v", err)
	}
	if lease == nil || lease.OrganizationID != "org_1" || lease.UserID != "user_1" {
		t.Fatalf("unexpected usage lease: %+v", lease)
	}

	_, err = svc.BeginUsage(ctx, limit)
	if !errors.Is(err, ErrUsageLimited) {
		t.Fatalf("BeginUsage second lease err = %v, want ErrUsageLimited", err)
	}
	var usageErr *UsageLimitError
	if !errors.As(err, &usageErr) || usageErr.Dimension != UsageLimitDimensionConcurrent {
		t.Fatalf("expected concurrent UsageLimitError, got %#v", err)
	}

	if err := svc.EndUsage(ctx, lease); err != nil {
		t.Fatalf("EndUsage returned error: %v", err)
	}
	if _, err := svc.BeginUsage(ctx, limit); err != nil {
		t.Fatalf("BeginUsage after release returned error: %v", err)
	}
}

func TestQuotaTokenRateLimitUsesOrganizationAndUserWindow(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := newFakeStore()
	svc := NewService(store, WithRateLimiter(ratelimit.NewInMemoryRateLimiter(ratelimit.InMemoryOptions{
		Clock: func() time.Time { return now },
	})))
	ctx := context.Background()
	limit := UsageLimit{OrganizationID: "org_1", UserID: "user_1", MaxTokensPerWindow: 100}

	if err := svc.ReserveUsageTokens(ctx, limit, 60); err != nil {
		t.Fatalf("ReserveUsageTokens first reservation returned error: %v", err)
	}
	err := svc.ReserveUsageTokens(ctx, limit, 50)
	if !errors.Is(err, ErrUsageLimited) {
		t.Fatalf("ReserveUsageTokens over limit err = %v, want ErrUsageLimited", err)
	}
	var usageErr *UsageLimitError
	if !errors.As(err, &usageErr) || usageErr.Dimension != UsageLimitDimensionTokens || usageErr.Remaining != 40 {
		t.Fatalf("expected token UsageLimitError remaining=40, got %#v", err)
	}

	otherUser := limit
	otherUser.UserID = "user_2"
	if err := svc.ReserveUsageTokens(ctx, otherUser, 50); err != nil {
		t.Fatalf("other user should have an independent token window: %v", err)
	}
}

func TestQuotaRequestTokenLimitRejectsOversizedSingleRequestWithoutConsumingWindow(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, WithRateLimiter(ratelimit.NewInMemoryRateLimiter(ratelimit.InMemoryOptions{})))
	ctx := context.Background()
	limit := UsageLimit{
		OrganizationID:      "org_1",
		UserID:              "user_1",
		MaxTokensPerWindow:  100,
		MaxTokensPerRequest: 20,
	}

	err := svc.ReserveUsageTokens(ctx, limit, 25)
	if !errors.Is(err, ErrUsageLimited) {
		t.Fatalf("oversized request err = %v, want ErrUsageLimited", err)
	}
	var usageErr *UsageLimitError
	if !errors.As(err, &usageErr) || usageErr.Dimension != UsageLimitDimensionRequestTokens {
		t.Fatalf("expected request-token UsageLimitError, got %#v", err)
	}

	if err := svc.ReserveUsageTokens(ctx, limit, 10); err != nil {
		t.Fatalf("valid request should pass after oversized request was rejected without consuming token window: %v", err)
	}
}

func TestQuotaUsageLimitSettingsResolveUserOverrideWithOrganizationFallback(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	ctx := context.Background()

	organizationSettings, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		MaxConcurrentRequests: 10,
		WindowSeconds:         60,
		MaxTokensPerWindow:    1000,
		MaxTokensPerRequest:   250,
	})
	if err != nil {
		t.Fatalf("save organization usage limit settings: %v", err)
	}
	if organizationSettings.QuotaMode != "organization" {
		t.Fatalf("expected organization quota mode, got %+v", organizationSettings)
	}
	userSettings, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		UserID:                "user_1",
		QuotaMode:             "user",
		MaxConcurrentRequests: 2,
		WindowSeconds:         30,
		MaxTokensPerWindow:    200,
		MaxTokensPerRequest:   75,
	})
	if err != nil {
		t.Fatalf("save user usage limit settings: %v", err)
	}
	if userSettings.QuotaMode != "user" {
		t.Fatalf("expected user quota mode, got %+v", userSettings)
	}

	userLimit, err := svc.ResolveUsageLimit(ctx, "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve user usage limit: %v", err)
	}
	if userLimit.MaxConcurrentRequests != 2 || userLimit.MaxTokensPerWindow != 200 || userLimit.MaxTokensPerRequest != 75 {
		t.Fatalf("expected user override limits, got %+v", userLimit)
	}

	orgLimit, err := svc.ResolveUsageLimit(ctx, "org_1", "user_2")
	if err != nil {
		t.Fatalf("resolve organization fallback usage limit: %v", err)
	}
	if orgLimit.UserID != "" || orgLimit.MaxConcurrentRequests != 10 || orgLimit.MaxTokensPerWindow != 1000 || orgLimit.MaxTokensPerRequest != 250 {
		t.Fatalf("expected organization-scoped fallback limits, got %+v", orgLimit)
	}

	settings, err := svc.ListUsageLimitSettings(ctx, "org_1")
	if err != nil {
		t.Fatalf("list usage limit settings: %v", err)
	}
	if len(settings) != 2 {
		t.Fatalf("expected two persisted settings, got %+v", settings)
	}
	if settings[0].UserID != "" || settings[1].UserID != "user_1" {
		t.Fatalf("expected organization scope before user scope, got %+v", settings)
	}
	if settings[0].QuotaMode != "organization" || settings[1].QuotaMode != "user" {
		t.Fatalf("expected explicit quota modes, got %+v", settings)
	}
}

func TestQuotaResolveUsageLimitFallsBackToActiveSubscriptionRequestCap(t *testing.T) {
	store := newFakeStore()
	store.packages["pkg_pro"] = &Package{
		ID:                  "pkg_pro",
		Name:                "Pro",
		MaxTokensPerRequest: 32000,
		IsActive:            true,
		IsPublic:            true,
	}
	store.subscriptions = []*Subscription{{
		ID:             "sub_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		PackageID:      "pkg_pro",
		Status:         "active",
		StartedAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
	}}
	svc := NewService(store)

	limit, err := svc.ResolveUsageLimit(context.Background(), "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve usage limit: %v", err)
	}
	if limit.MaxTokensPerRequest != 32000 {
		t.Fatalf("expected request cap from active subscription package, got %+v", limit)
	}

	if _, err := svc.SaveUsageLimitSettings(context.Background(), UsageLimitSettings{
		OrganizationID:      "org_1",
		UserID:              "user_1",
		QuotaMode:           "user",
		MaxTokensPerRequest: 4000,
	}); err != nil {
		t.Fatalf("save explicit user request cap: %v", err)
	}
	limit, err = svc.ResolveUsageLimit(context.Background(), "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve explicit usage limit: %v", err)
	}
	if limit.MaxTokensPerRequest != 4000 {
		t.Fatalf("expected explicit usage-limit setting to override subscription cap, got %+v", limit)
	}
}

func TestQuotaUsageLimitSettingsValidationAndDefaults(t *testing.T) {
	svc := NewService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{MaxConcurrentRequests: 1}); err == nil {
		t.Fatal("expected organization_id validation error")
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{OrganizationID: "org_1"}); err == nil {
		t.Fatal("expected positive limit validation error")
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:     "org_1",
		QuotaMode:          "user",
		MaxTokensPerWindow: 500,
	}); err == nil || !strings.Contains(err.Error(), "user_id is required for user quota mode") {
		t.Fatalf("expected user quota mode to require user_id, got %v", err)
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:     "org_1",
		UserID:             "user_1",
		QuotaMode:          "organization",
		MaxTokensPerWindow: 500,
	}); err == nil || !strings.Contains(err.Error(), "user_id must be empty for organization quota mode") {
		t.Fatalf("expected organization quota mode to reject user_id, got %v", err)
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:     "org_1",
		QuotaMode:          "team",
		MaxTokensPerWindow: 500,
	}); err == nil || !strings.Contains(err.Error(), "quota_mode must be organization or user") {
		t.Fatalf("expected quota mode validation error, got %v", err)
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		MaxTokensPerRequest: -1,
	}); err == nil || !strings.Contains(err.Error(), "max_tokens_per_request must be non-negative") {
		t.Fatalf("expected request token limit validation error, got %v", err)
	}

	saved, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		MaxTokensPerRequest: 500,
	})
	if err != nil {
		t.Fatalf("save request token usage limit with default window: %v", err)
	}
	if saved.WindowSeconds != defaultUsageLimitWindowSeconds {
		t.Fatalf("expected default token window %d seconds, got %d", defaultUsageLimitWindowSeconds, saved.WindowSeconds)
	}
	if saved.QuotaMode != "organization" {
		t.Fatalf("expected organization quota mode default, got %+v", saved)
	}
}

func TestSQLStoreUsageLimitSettingsRoundTrip(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)

	if _, err := store.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		MaxConcurrentRequests: 8,
		WindowSeconds:         60,
		MaxTokensPerWindow:    900,
		MaxTokensPerRequest:   250,
	}); err != nil {
		t.Fatalf("save organization SQL usage limit settings: %v", err)
	}
	if _, err := store.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		UserID:                "user_1",
		MaxConcurrentRequests: 3,
		WindowSeconds:         45,
		MaxTokensPerWindow:    300,
		MaxTokensPerRequest:   75,
	}); err != nil {
		t.Fatalf("save user SQL usage limit settings: %v", err)
	}

	userLimit, err := store.ResolveUsageLimitSettings(ctx, "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve SQL user settings: %v", err)
	}
	if userLimit.MaxConcurrentRequests != 3 || userLimit.WindowSeconds != 45 || userLimit.MaxTokensPerWindow != 300 || userLimit.MaxTokensPerRequest != 75 {
		t.Fatalf("expected SQL user override settings, got %+v", userLimit)
	}
	if userLimit.QuotaMode != "user" {
		t.Fatalf("expected SQL user quota mode, got %+v", userLimit)
	}

	orgLimit, err := store.ResolveUsageLimitSettings(ctx, "org_1", "user_2")
	if err != nil {
		t.Fatalf("resolve SQL organization fallback settings: %v", err)
	}
	if orgLimit.MaxConcurrentRequests != 8 || orgLimit.MaxTokensPerWindow != 900 || orgLimit.MaxTokensPerRequest != 250 {
		t.Fatalf("expected SQL organization fallback settings, got %+v", orgLimit)
	}
	if orgLimit.QuotaMode != "organization" {
		t.Fatalf("expected SQL organization quota mode, got %+v", orgLimit)
	}
}

func TestSQLStoreUserQuotaModeUsesUserScopedBalance(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	svc := NewService(store)

	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		MaxTokensPerWindow:  1000,
		MaxTokensPerRequest: 100,
	}); err != nil {
		t.Fatalf("save organization SQL quota mode: %v", err)
	}
	if _, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:      "org_1",
		UserID:              "user_2",
		QuotaMode:           "user",
		MaxTokensPerWindow:  500,
		MaxTokensPerRequest: 50,
	}); err != nil {
		t.Fatalf("save user SQL quota mode: %v", err)
	}
	if err := svc.Topup(ctx, "user_1", "org_1", 100.0); err != nil {
		t.Fatalf("top up organization SQL quota: %v", err)
	}
	if err := svc.Topup(ctx, "user_2", "org_1", 100.0); err != nil {
		t.Fatalf("top up user SQL quota: %v", err)
	}

	if _, err := svc.PreConsume(ctx, "user_2", "org_1", 25.0, "idem_sql_user_mode", "ch_1", "gpt-4o", "chat"); err != nil {
		t.Fatalf("user-mode SQL PreConsume failed: %v", err)
	}

	var userBalance, orgBalance float64
	if err := store.db.QueryRowContext(ctx, `
		SELECT balance FROM quotas
		WHERE organization_id = 'org_1' AND scope = 'user' AND user_id = 'user_2'
	`).Scan(&userBalance); err != nil {
		t.Fatalf("query user-scoped SQL quota: %v", err)
	}
	if err := store.db.QueryRowContext(ctx, `
		SELECT balance FROM quotas
		WHERE organization_id = 'org_1' AND scope = 'organization'
	`).Scan(&orgBalance); err != nil {
		t.Fatalf("query organization-scoped SQL quota: %v", err)
	}
	if userBalance != 75.0 || orgBalance != 100.0 {
		t.Fatalf("expected user balance 75 and organization balance 100, got user=%.2f org=%.2f", userBalance, orgBalance)
	}
}

func TestSQLStorePreConsumeIdempotencyDoesNotDoubleReserve(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	svc := NewService(store)

	if err := svc.Topup(ctx, "user_1", "org_1", 100); err != nil {
		t.Fatalf("top up quota: %v", err)
	}

	session1, err := svc.PreConsume(ctx, "user_1", "org_1", 30, "sql_idem_once", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("first preconsume: %v", err)
	}
	session2, err := svc.PreConsume(ctx, "user_1", "org_1", 30, "sql_idem_once", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("idempotent preconsume: %v", err)
	}
	if session1.ID != session2.ID {
		t.Fatalf("expected idempotent preconsume to return session %s, got %s", session1.ID, session2.ID)
	}

	assertSQLQuotaBalance(t, store, ctx, "org_1", 70)

	var count int
	if err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM billing_sessions
		WHERE organization_id = 'org_1' AND idempotency_key = 'sql_idem_once'
	`).Scan(&count); err != nil {
		t.Fatalf("count idempotent billing sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one billing session for idempotency key, got %d", count)
	}
}

func TestSQLStorePreConsumeAtomicallyReservesQuota(t *testing.T) {
	store, baseCtx := testSQLQuotaStore(t)
	store.db.SetMaxOpenConns(4)
	store.db.SetMaxIdleConns(4)
	ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	defer cancel()
	svc := NewService(store)

	if err := svc.Topup(ctx, "user_1", "org_1", 100); err != nil {
		t.Fatalf("top up quota: %v", err)
	}

	type result struct {
		session *BillingSession
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		idempotencyKey := fmt.Sprintf("sql_atomic_%d", i)
		go func() {
			<-start
			session, err := svc.PreConsume(ctx, "user_1", "org_1", 80, idempotencyKey, "ch_1", "gpt-4o", "chat")
			results <- result{session: session, err: err}
		}()
	}
	close(start)

	successes := 0
	insufficient := 0
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err == nil {
			successes++
			if got.session == nil || got.session.Status != "preauthorized" {
				t.Fatalf("expected preauthorized session, got %+v", got.session)
			}
			continue
		}
		if strings.Contains(got.err.Error(), "insufficient balance") {
			insufficient++
			continue
		}
		t.Fatalf("unexpected preconsume error: %v", got.err)
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("expected one success and one insufficient balance failure, got successes=%d insufficient=%d", successes, insufficient)
	}

	assertSQLQuotaBalance(t, store, ctx, "org_1", 20)

	var used float64
	if err := store.db.QueryRowContext(ctx, `
		SELECT used FROM quotas
		WHERE organization_id = 'org_1' AND scope = 'organization'
	`).Scan(&used); err != nil {
		t.Fatalf("query quota used: %v", err)
	}
	if used != 80 {
		t.Fatalf("expected used amount 80 after one reservation, got %.2f", used)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM billing_sessions
		WHERE organization_id = 'org_1' AND status = 'preauthorized'
	`).Scan(&count); err != nil {
		t.Fatalf("count preauthorized billing sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one committed preauthorized billing session, got %d", count)
	}
}

func TestSQLStoreBillingSessionsAreOrganizationScoped(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	svc := NewService(store)

	if _, err := store.db.ExecContext(ctx, `INSERT INTO organizations (id, name) VALUES ('org_2', 'Org Two')`); err != nil {
		t.Fatalf("insert second organization: %v", err)
	}
	if err := svc.Topup(ctx, "user_1", "org_1", 100); err != nil {
		t.Fatalf("top up org_1 quota: %v", err)
	}
	if err := svc.Topup(ctx, "user_1", "org_2", 80); err != nil {
		t.Fatalf("top up org_2 quota: %v", err)
	}

	org1Session, err := svc.PreConsume(ctx, "user_1", "org_1", 30, "shared_idempotency", "ch_1", "gpt-4o", "chat")
	if err != nil {
		t.Fatalf("preconsume org_1: %v", err)
	}
	org2Session, err := svc.PreConsume(ctx, "user_1", "org_2", 40, "shared_idempotency", "ch_2", "gpt-4o-mini", "chat")
	if err != nil {
		t.Fatalf("preconsume org_2: %v", err)
	}
	if org1Session.ID == org2Session.ID {
		t.Fatal("same idempotency key in different organizations must create isolated billing sessions")
	}

	if got, err := store.GetBillingSessionByIdempotencyKey(ctx, "shared_idempotency", "org_1"); err != nil || got == nil || got.ID != org1Session.ID {
		t.Fatalf("expected org_1 idempotency lookup to return org_1 session, got %+v err=%v", got, err)
	}
	if got, err := store.GetBillingSessionByIdempotencyKey(ctx, "shared_idempotency", "org_2"); err != nil || got == nil || got.ID != org2Session.ID {
		t.Fatalf("expected org_2 idempotency lookup to return org_2 session, got %+v err=%v", got, err)
	}

	if err := svc.Settle(ctx, "org_2", org1Session.ID, 10); err == nil {
		t.Fatal("expected wrong-organization settlement to fail")
	}
	assertSQLQuotaBalance(t, store, ctx, "org_1", 70)
	assertSQLBillingSession(t, store, ctx, org1Session.ID, "org_1", "preauthorized", 0)

	if err := svc.Settle(ctx, "org_1", org1Session.ID, 20); err != nil {
		t.Fatalf("settle org_1 session: %v", err)
	}
	assertSQLQuotaBalance(t, store, ctx, "org_1", 80)
	assertSQLQuotaLedger(t, store, ctx, "org_1", 80, 20)
	assertSQLBillingSession(t, store, ctx, org1Session.ID, "org_1", "settled", 20)

	if err := svc.Refund(ctx, "org_1", org2Session.ID); err == nil {
		t.Fatal("expected wrong-organization refund to fail")
	}
	assertSQLQuotaBalance(t, store, ctx, "org_2", 40)
	assertSQLBillingSession(t, store, ctx, org2Session.ID, "org_2", "preauthorized", 0)

	if err := svc.Refund(ctx, "org_2", org2Session.ID); err != nil {
		t.Fatalf("refund org_2 session: %v", err)
	}
	assertSQLQuotaBalance(t, store, ctx, "org_2", 80)
	assertSQLQuotaLedger(t, store, ctx, "org_2", 80, 0)
	assertSQLBillingSession(t, store, ctx, org2Session.ID, "org_2", "refunded", 0)
}

func TestSQLStoreTopupOrderMutationsRequireOrganizationScope(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	svc := NewService(store)

	if _, err := store.db.ExecContext(ctx, `INSERT INTO organizations (id, name) VALUES ('org_2', 'Org Two')`); err != nil {
		t.Fatalf("insert second organization: %v", err)
	}

	org1Order, err := svc.CreatePendingTopup(ctx, "user_1", "org_1", "pi_quota_org1", 25, 25)
	if err != nil {
		t.Fatalf("create org_1 topup: %v", err)
	}
	org2Order, err := svc.CreatePendingTopup(ctx, "user_1", "org_2", "pi_quota_org2", 30, 30)
	if err != nil {
		t.Fatalf("create org_2 topup: %v", err)
	}

	if err := svc.SetTopupCheckoutSession(ctx, "org_2", "pi_quota_org1", "cs_wrong_org"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected wrong-organization checkout session update to fail closed, got %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org1Order.ID, "org_1", "pending", "", "")

	if err := svc.SetTopupCheckoutSession(ctx, "org_1", "pi_quota_org1", "cs_org1"); err != nil {
		t.Fatalf("set org_1 checkout session: %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org1Order.ID, "org_1", "pending", "cs_org1", "")

	if err := svc.MarkTopupCheckoutFailed(ctx, "org_1", "pi_quota_org2"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected wrong-organization checkout failure update to fail closed, got %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org2Order.ID, "org_2", "pending", "", "")

	if err := svc.MarkTopupCheckoutFailed(ctx, "org_2", "pi_quota_org2"); err != nil {
		t.Fatalf("mark org_2 checkout failed: %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org2Order.ID, "org_2", "failed", "", "")

	if err := store.UpdateTopupOrderStatus(ctx, org1Order.ID, "org_2", "paid", "trade_wrong_org"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected wrong-organization status update to fail closed, got %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org1Order.ID, "org_1", "pending", "cs_org1", "")

	if err := store.UpdateTopupOrderStatus(ctx, org1Order.ID, "org_1", "paid", "trade_org1"); err != nil {
		t.Fatalf("mark org_1 topup paid: %v", err)
	}
	assertSQLTopupOrder(t, store, ctx, org1Order.ID, "org_1", "paid", "cs_org1", "trade_org1")
}

func TestSQLStoreResolveUsageLimitFallsBackToActiveSubscriptionRequestCap(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO packages (
			id, name, description, quota_amount, token_quota, price, model_access,
			agent_limit, max_tokens_per_request, duration_days, is_active, is_public, sort_order, created_at, updated_at
		)
		VALUES ('pkg_pro', 'Pro', 'request cap plan', 100, 2000000, 29, ARRAY['gpt-4o'], 25, 32000, 30, true, true, 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert package: %v", err)
	}
	if _, err := store.CreateSubscription(ctx, &Subscription{
		ID:             "sub_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		PackageID:      "pkg_pro",
		Status:         "active",
		StartedAt:      now,
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	svc := NewService(store)
	limit, err := svc.ResolveUsageLimit(ctx, "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve usage limit: %v", err)
	}
	if limit.MaxTokensPerRequest != 32000 {
		t.Fatalf("expected SQL subscription request cap, got %+v", limit)
	}
}

func TestSQLStoreListPackagesReturnsOnlyActivePublicHybridPlans(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO packages (
			id, name, description, quota_amount, token_quota, price, model_access,
			agent_limit, max_tokens_per_request, duration_days, is_active, is_public, sort_order, created_at, updated_at
		)
		VALUES
			('pkg_public', 'Public Pro', 'public plan', 100, 2000000, 29, ARRAY['gpt-4o','gpt-4o-mini'], 25, 32000, 30, true, true, 1, $1, $1),
			('pkg_private', 'Private Enterprise', 'hidden plan', 1000, 9000000, 299, ARRAY['gpt-4o'], 100, 64000, 30, true, false, 0, $1, $1),
			('pkg_inactive', 'Inactive Starter', 'inactive plan', 10, 100000, 9, ARRAY['gpt-4o-mini'], 5, 4000, 30, false, true, 2, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert packages: %v", err)
	}

	packages, err := store.ListPackages(ctx, true)
	if err != nil {
		t.Fatalf("ListPackages returned error: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("expected one active public package, got %d: %+v", len(packages), packages)
	}
	pkg := packages[0]
	if pkg.ID != "pkg_public" || !pkg.IsActive || !pkg.IsPublic {
		t.Fatalf("expected active public package only, got %+v", pkg)
	}
	if pkg.TokenQuota != 2000000 || pkg.AgentLimit != 25 {
		t.Fatalf("expected hybrid quota fields, got %+v", pkg)
	}
	if pkg.MaxTokensPerRequest != 32000 {
		t.Fatalf("expected request token cap from plan, got %+v", pkg)
	}
	if len(pkg.ModelAccess) != 2 || pkg.ModelAccess[0] != "gpt-4o" || pkg.ModelAccess[1] != "gpt-4o-mini" {
		t.Fatalf("expected model access list, got %+v", pkg.ModelAccess)
	}
	if !pkg.UpdatedAt.Equal(now) {
		t.Fatalf("expected updatedAt %s, got %s", now, pkg.UpdatedAt)
	}
}

func assertSQLQuotaBalance(t *testing.T, store *SQLStore, ctx context.Context, organizationID string, want float64) {
	t.Helper()

	var got float64
	if err := store.db.QueryRowContext(ctx, `
		SELECT balance FROM quotas
		WHERE organization_id = $1 AND scope = 'organization'
	`, organizationID).Scan(&got); err != nil {
		t.Fatalf("query %s quota balance: %v", organizationID, err)
	}
	if got != want {
		t.Fatalf("expected %s quota balance %.2f, got %.2f", organizationID, want, got)
	}
}

func assertSQLQuotaLedger(t *testing.T, store *SQLStore, ctx context.Context, organizationID string, wantBalance, wantUsed float64) {
	t.Helper()

	var gotBalance, gotUsed float64
	if err := store.db.QueryRowContext(ctx, `
                SELECT balance, used FROM quotas
                WHERE organization_id = $1 AND scope = 'organization'
        `, organizationID).Scan(&gotBalance, &gotUsed); err != nil {
		t.Fatalf("query %s quota ledger: %v", organizationID, err)
	}
	if gotBalance != wantBalance || gotUsed != wantUsed {
		t.Fatalf("expected %s quota ledger balance=%.2f used=%.2f, got balance=%.2f used=%.2f", organizationID, wantBalance, wantUsed, gotBalance, gotUsed)
	}
}

func assertSQLBillingSession(t *testing.T, store *SQLStore, ctx context.Context, sessionID, organizationID, wantStatus string, wantSettled float64) {
	t.Helper()

	var gotOrganizationID, gotStatus string
	var gotSettled float64
	if err := store.db.QueryRowContext(ctx, `
		SELECT organization_id, status, settled_amt FROM billing_sessions WHERE id = $1
	`, sessionID).Scan(&gotOrganizationID, &gotStatus, &gotSettled); err != nil {
		t.Fatalf("query billing session %s: %v", sessionID, err)
	}
	if gotOrganizationID != organizationID || gotStatus != wantStatus || gotSettled != wantSettled {
		t.Fatalf("expected billing session %s org=%s status=%s settled=%.2f, got org=%s status=%s settled=%.2f", sessionID, organizationID, wantStatus, wantSettled, gotOrganizationID, gotStatus, gotSettled)
	}
}

func assertSQLTopupOrder(t *testing.T, store *SQLStore, ctx context.Context, orderID, organizationID, wantStatus, wantCheckoutSessionID, wantTradeNo string) {
	t.Helper()

	var gotOrganizationID, gotStatus, gotCheckoutSessionID, gotTradeNo string
	if err := store.db.QueryRowContext(ctx, `
		SELECT organization_id, status, COALESCE(provider_checkout_session_id, ''), COALESCE(trade_no, '')
		FROM topup_orders WHERE id = $1
	`, orderID).Scan(&gotOrganizationID, &gotStatus, &gotCheckoutSessionID, &gotTradeNo); err != nil {
		t.Fatalf("query topup order %s: %v", orderID, err)
	}
	if gotOrganizationID != organizationID || gotStatus != wantStatus || gotCheckoutSessionID != wantCheckoutSessionID || gotTradeNo != wantTradeNo {
		t.Fatalf("expected topup order %s org=%s status=%s checkout=%q trade=%q, got org=%s status=%s checkout=%q trade=%q", orderID, organizationID, wantStatus, wantCheckoutSessionID, wantTradeNo, gotOrganizationID, gotStatus, gotCheckoutSessionID, gotTradeNo)
	}
}

func testSQLQuotaStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for quota SQL store tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	// Pin to a single connection so the advisory lock is held for the
	// lifetime of the test and cannot be bypassed by the connection pool.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104254)`); err != nil {
		t.Fatalf("lock quota test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104254)`); err != nil {
			t.Fatalf("unlock quota test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS packages CASCADE`,
		`DROP TABLE IF EXISTS subscriptions CASCADE`,
		`DROP TABLE IF EXISTS token_rate_limits CASCADE`,
		`DROP TABLE IF EXISTS concurrency_limits CASCADE`,
		`DROP TABLE IF EXISTS billing_sessions CASCADE`,
		`DROP TABLE IF EXISTS topup_orders CASCADE`,
		`DROP TABLE IF EXISTS quotas CASCADE`,
		`DROP TABLE IF EXISTS organization_memberships CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE packages (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			quota_amount DECIMAL(15,6) NOT NULL,
			token_quota INTEGER NOT NULL DEFAULT 1000000,
			price DECIMAL(10,2) NOT NULL,
			model_access TEXT[] NOT NULL DEFAULT '{}',
			agent_limit INTEGER NOT NULL DEFAULT 10,
			max_tokens_per_request INTEGER NOT NULL DEFAULT 0,
			duration_days INT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			is_public BOOLEAN NOT NULL DEFAULT true,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE quotas (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			scope TEXT NOT NULL DEFAULT 'organization',
			balance DECIMAL(15,6) NOT NULL DEFAULT 0,
			used DECIMAL(15,6) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (scope IN ('organization', 'user'))
		)`,
		`CREATE UNIQUE INDEX idx_test_quotas_unique_organization_scope ON quotas(organization_id) WHERE scope = 'organization'`,
		`CREATE UNIQUE INDEX idx_test_quotas_unique_user_scope ON quotas(organization_id, user_id) WHERE scope = 'user'`,
		`CREATE TABLE billing_sessions (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			channel_id TEXT,
			model TEXT,
			api_type TEXT,
			idempotency_key TEXT NOT NULL,
			pre_authorized_amt DECIMAL(15,6) NOT NULL DEFAULT 0,
			settled_amt DECIMAL(15,6) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'preauthorized',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			settled_at TIMESTAMPTZ
		)`,
		`CREATE UNIQUE INDEX idx_test_billing_sessions_unique_org_idempotency ON billing_sessions(organization_id, idempotency_key) WHERE idempotency_key <> ''`,
		`CREATE TABLE topup_orders (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(15,6) NOT NULL,
			money DECIMAL(10,2) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			trade_no TEXT,
			paid_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			payment_intent_id TEXT,
			provider_checkout_session_id TEXT,
			refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX idx_test_topup_orders_payment_intent ON topup_orders(payment_intent_id) WHERE payment_intent_id IS NOT NULL`,
		`CREATE TABLE subscriptions (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			package_id TEXT NOT NULL REFERENCES packages(id),
			status TEXT NOT NULL DEFAULT 'active',
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`INSERT INTO users (id, email, name) VALUES ('user_1', 'user@example.test', 'User One'), ('user_2', 'user2@example.test', 'User Two')`,
		`INSERT INTO organizations (id, name) VALUES ('org_1', 'Org One')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare quota test database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0054_usage_limits.sql")
	if err != nil {
		t.Fatalf("read usage limits migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply usage limits migration: %v", err)
	}

	return NewSQLStore(database), context.Background()
}
