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
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		quotas:          make(map[string]*Quota),
		billingSessions: make(map[string]*BillingSession),
		idempotencyKeys: make(map[string]string),
		usageLimits:     make(map[string]UsageLimitSettings),
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

func TestQuotaUsageLimitSettingsResolveUserOverrideWithOrganizationFallback(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	ctx := context.Background()

	organizationSettings, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		MaxConcurrentRequests: 10,
		WindowSeconds:         60,
		MaxTokensPerWindow:    1000,
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
	if userLimit.MaxConcurrentRequests != 2 || userLimit.MaxTokensPerWindow != 200 {
		t.Fatalf("expected user override limits, got %+v", userLimit)
	}

	orgLimit, err := svc.ResolveUsageLimit(ctx, "org_1", "user_2")
	if err != nil {
		t.Fatalf("resolve organization fallback usage limit: %v", err)
	}
	if orgLimit.UserID != "" || orgLimit.MaxConcurrentRequests != 10 || orgLimit.MaxTokensPerWindow != 1000 {
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

	saved, err := svc.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:     "org_1",
		MaxTokensPerWindow: 500,
	})
	if err != nil {
		t.Fatalf("save token usage limit with default window: %v", err)
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
	}); err != nil {
		t.Fatalf("save organization SQL usage limit settings: %v", err)
	}
	if _, err := store.SaveUsageLimitSettings(ctx, UsageLimitSettings{
		OrganizationID:        "org_1",
		UserID:                "user_1",
		MaxConcurrentRequests: 3,
		WindowSeconds:         45,
		MaxTokensPerWindow:    300,
	}); err != nil {
		t.Fatalf("save user SQL usage limit settings: %v", err)
	}

	userLimit, err := store.ResolveUsageLimitSettings(ctx, "org_1", "user_1")
	if err != nil {
		t.Fatalf("resolve SQL user settings: %v", err)
	}
	if userLimit.MaxConcurrentRequests != 3 || userLimit.WindowSeconds != 45 || userLimit.MaxTokensPerWindow != 300 {
		t.Fatalf("expected SQL user override settings, got %+v", userLimit)
	}
	if userLimit.QuotaMode != "user" {
		t.Fatalf("expected SQL user quota mode, got %+v", userLimit)
	}

	orgLimit, err := store.ResolveUsageLimitSettings(ctx, "org_1", "user_2")
	if err != nil {
		t.Fatalf("resolve SQL organization fallback settings: %v", err)
	}
	if orgLimit.MaxConcurrentRequests != 8 || orgLimit.MaxTokensPerWindow != 900 {
		t.Fatalf("expected SQL organization fallback settings, got %+v", orgLimit)
	}
	if orgLimit.QuotaMode != "organization" {
		t.Fatalf("expected SQL organization quota mode, got %+v", orgLimit)
	}
}

func TestSQLStoreListPackagesReturnsOnlyActivePublicHybridPlans(t *testing.T) {
	store, ctx := testSQLQuotaStore(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO packages (
			id, name, description, quota_amount, token_quota, price, model_access,
			agent_limit, duration_days, is_active, is_public, sort_order, created_at, updated_at
		)
		VALUES
			('pkg_public', 'Public Pro', 'public plan', 100, 2000000, 29, ARRAY['gpt-4o','gpt-4o-mini'], 25, 30, true, true, 1, $1, $1),
			('pkg_private', 'Private Enterprise', 'hidden plan', 1000, 9000000, 299, ARRAY['gpt-4o'], 100, 30, true, false, 0, $1, $1),
			('pkg_inactive', 'Inactive Starter', 'inactive plan', 10, 100000, 9, ARRAY['gpt-4o-mini'], 5, 30, false, true, 2, $1, $1)
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
	if len(pkg.ModelAccess) != 2 || pkg.ModelAccess[0] != "gpt-4o" || pkg.ModelAccess[1] != "gpt-4o-mini" {
		t.Fatalf("expected model access list, got %+v", pkg.ModelAccess)
	}
	if !pkg.UpdatedAt.Equal(now) {
		t.Fatalf("expected updatedAt %s, got %s", now, pkg.UpdatedAt)
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
		`DROP TABLE IF EXISTS token_rate_limits CASCADE`,
		`DROP TABLE IF EXISTS concurrency_limits CASCADE`,
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
			duration_days INT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			is_public BOOLEAN NOT NULL DEFAULT true,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
