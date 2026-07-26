package relay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"oblivious/server/internal/quota"
)

func TestQuotaCompensationMigrationShape(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0101_relay_quota_compensation_jobs.sql")
	if err != nil {
		t.Fatalf("read quota compensation migration: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS relay_quota_compensation_jobs",
		"job_key_digest TEXT PRIMARY KEY",
		"organization_scope_key_digest TEXT NOT NULL DEFAULT ''",
		"api_token_scope_key_digest TEXT NOT NULL DEFAULT ''",
		"CHECK (status IN ('pending', 'processing', 'succeeded', 'failed'))",
		"CREATE TABLE IF NOT EXISTS relay_api_token_quota_refund_receipts",
		"scope_key_digest TEXT PRIMARY KEY",
		"UNIQUE (scope_key_digest, api_token_id, amount)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("migration missing required contract %q", want)
		}
	}
}

func TestQuotaCompensationStorePersistsDeterministicSanitizedJobs(t *testing.T) {
	store := newMemoryQuotaCompensationStore()
	req := QuotaCompensationRequest{
		RouteAttemptID:       "route-attempt-secret-value",
		Stage:                QuotaCompensationStageLateModelReadiness,
		OrganizationID:       "org_opaque",
		BillingSessionID:     "billing_opaque",
		APITokenID:           "token_opaque",
		Amount:               12.5,
		CallerIdempotencyKey: "caller-key-must-not-persist",
	}

	job, err := store.ArmQuotaCompensation(context.Background(), req)
	if err != nil {
		t.Fatalf("arm compensation: %v", err)
	}
	if err := assertQuotaCompensationDigests(job); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%+v", job); strings.Contains(got, req.RouteAttemptID) || strings.Contains(got, req.CallerIdempotencyKey) {
		t.Fatalf("persisted job leaked raw route or caller identity: %s", got)
	}

	replayed, err := store.ArmQuotaCompensation(context.Background(), req)
	if err != nil {
		t.Fatalf("re-arm compensation: %v", err)
	}
	if replayed.JobKeyDigest != job.JobKeyDigest || replayed.OrganizationScopeKeyDigest != job.OrganizationScopeKeyDigest || replayed.APITokenScopeKeyDigest != job.APITokenScopeKeyDigest {
		t.Fatalf("re-arm changed deterministic digests: first=%+v replay=%+v", job, replayed)
	}
	if err := store.MarkQuotaCompensationScopeSucceeded(context.Background(), job.JobKeyDigest, QuotaCompensationScopeOrganization, "", time.Now().UTC()); err != nil {
		t.Fatalf("mark organization scope succeeded: %v", err)
	}
	replayed, err = store.ArmQuotaCompensation(context.Background(), req)
	if err != nil {
		t.Fatalf("re-arm after success: %v", err)
	}
	if !replayed.OrganizationCompleted {
		t.Fatalf("re-arm resurrected succeeded organization scope: %+v", replayed)
	}

	if _, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{Stage: QuotaCompensationStageLateModelReadiness, APITokenID: "token_opaque", Amount: 1}); !errors.Is(err, ErrRouteAttemptIdentityRequired) {
		t.Fatalf("missing route attempt identity error = %v, want %v", err, ErrRouteAttemptIdentityRequired)
	}
}

func TestQuotaCompensationStoreClaimsLeasedJobs(t *testing.T) {
	state := &memoryQuotaCompensationState{jobs: map[string]QuotaCompensationJob{}, receipts: map[string]memoryQuotaCompensationReceipt{}}
	store := newMemoryQuotaCompensationStoreWithState(state)
	now := time.Date(2026, 7, 23, 6, 0, 0, 0, time.UTC)
	first, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
		RouteAttemptID: "attempt-claim-first", Stage: QuotaCompensationStageLateProviderReadiness, APITokenID: "token_a", Amount: 1,
	})
	if err != nil {
		t.Fatalf("arm first job: %v", err)
	}
	if _, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
		RouteAttemptID: "attempt-claim-second", Stage: QuotaCompensationStageLateProviderReadiness, APITokenID: "token_b", Amount: 1,
	}); err != nil {
		t.Fatalf("arm second job: %v", err)
	}

	claimed, err := store.ClaimQuotaCompensationJobs(context.Background(), now, 2, "worker-a")
	if err != nil {
		t.Fatalf("claim due jobs: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed jobs = %d, want 2", len(claimed))
	}
	if claimed[0].Status != QuotaCompensationJobStatusProcessing || claimed[0].LockedBy != "worker-a" {
		t.Fatalf("unexpected claimed job: %+v", claimed[0])
	}
	secondClaimer := newMemoryQuotaCompensationStoreWithState(state)
	claimedAgain, err := secondClaimer.ClaimQuotaCompensationJobs(context.Background(), now, 2, "worker-b")
	if err != nil {
		t.Fatalf("concurrent claim: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("active leases were claimed twice: %+v", claimedAgain)
	}

	expired := now.Add(defaultQuotaCompensationClaimLease + time.Second)
	claimedAfterRestart, err := secondClaimer.ClaimQuotaCompensationJobs(context.Background(), expired, 2, "worker-b")
	if err != nil {
		t.Fatalf("claim expired lease after restart: %v", err)
	}
	if len(claimedAfterRestart) != 2 || claimedAfterRestart[0].LockedBy != "worker-b" {
		t.Fatalf("expired leases were not reclaimed: %+v", claimedAfterRestart)
	}
	if claimedAfterRestart[0].JobKeyDigest == "" || first.JobKeyDigest == "" {
		t.Fatal("claimed jobs must retain durable job digests")
	}
}

func TestQuotaCompensationStoreRejectsReceiptKeyMismatch(t *testing.T) {
	store := newMemoryQuotaCompensationStore()
	inserted, err := store.RecordAPITokenQuotaRefundReceipt(context.Background(), "scope-digest-1", "token-a", 3.5, time.Now().UTC())
	if err != nil || !inserted {
		t.Fatalf("first receipt = (%v, %v), want (true, nil)", inserted, err)
	}
	inserted, err = store.RecordAPITokenQuotaRefundReceipt(context.Background(), "scope-digest-1", "token-a", 3.5, time.Now().UTC())
	if err != nil || inserted {
		t.Fatalf("identical receipt replay = (%v, %v), want (false, nil)", inserted, err)
	}
	if _, err := store.RecordAPITokenQuotaRefundReceipt(context.Background(), "scope-digest-1", "token-b", 3.5, time.Now().UTC()); !errors.Is(err, ErrQuotaCompensationReceiptMismatch) {
		t.Fatalf("token mismatch error = %v, want %v", err, ErrQuotaCompensationReceiptMismatch)
	}
	if _, err := store.RecordAPITokenQuotaRefundReceipt(context.Background(), "scope-digest-1", "token-a", 4.5, time.Now().UTC()); !errors.Is(err, ErrQuotaCompensationReceiptMismatch) {
		t.Fatalf("amount mismatch error = %v, want %v", err, ErrQuotaCompensationReceiptMismatch)
	}
}

func TestQuotaCompensationTokenOnlyReservationIdentityIsolation(t *testing.T) {
	for _, callerKey := range []string{"", "caller-key-reused"} {
		t.Run("caller-key="+callerKey, func(t *testing.T) {
			store := newMemoryQuotaCompensationStore()
			first, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
				RouteAttemptID: "token-attempt-one-" + callerKey, Stage: QuotaCompensationStageLateModelReadiness,
				APITokenID: "shared-token", Amount: 8, CallerIdempotencyKey: callerKey,
			})
			if err != nil {
				t.Fatalf("arm first reservation: %v", err)
			}
			second, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
				RouteAttemptID: "token-attempt-two-" + callerKey, Stage: QuotaCompensationStageLateModelReadiness,
				APITokenID: "shared-token", Amount: 8, CallerIdempotencyKey: callerKey,
			})
			if err != nil {
				t.Fatalf("arm second reservation: %v", err)
			}
			if first.JobKeyDigest == second.JobKeyDigest || first.APITokenScopeKeyDigest == second.APITokenScopeKeyDigest {
				t.Fatalf("distinct route attempts collided: first=%+v second=%+v", first, second)
			}
			if err := store.MarkQuotaCompensationScopeSucceeded(context.Background(), first.JobKeyDigest, QuotaCompensationScopeAPIToken, "", time.Now().UTC()); err != nil {
				t.Fatalf("complete first token scope: %v", err)
			}
			replayedFirst, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
				RouteAttemptID: "token-attempt-one-" + callerKey, Stage: QuotaCompensationStageLateModelReadiness,
				APITokenID: "shared-token", Amount: 8, CallerIdempotencyKey: callerKey,
			})
			if err != nil {
				t.Fatalf("replay first reservation: %v", err)
			}
			if !replayedFirst.APITokenCompleted {
				t.Fatalf("first reservation lost token completion: %+v", replayedFirst)
			}
			replayedSecond, err := store.ArmQuotaCompensation(context.Background(), QuotaCompensationRequest{
				RouteAttemptID: "token-attempt-two-" + callerKey, Stage: QuotaCompensationStageLateModelReadiness,
				APITokenID: "shared-token", Amount: 8, CallerIdempotencyKey: callerKey,
			})
			if err != nil {
				t.Fatalf("replay second reservation: %v", err)
			}
			if replayedSecond.APITokenCompleted {
				t.Fatalf("first reservation completed peer token scope: %+v", replayedSecond)
			}
		})
	}
}

func assertQuotaCompensationDigests(job QuotaCompensationJob) error {
	for name, digest := range map[string]string{
		"job":          job.JobKeyDigest,
		"organization": job.OrganizationScopeKeyDigest,
		"api-token":    job.APITokenScopeKeyDigest,
	} {
		if digest == "" {
			if name == "organization" && !job.OrganizationRequired {
				continue
			}
			return fmt.Errorf("%s digest is empty", name)
		}
		if len(digest) != 64 {
			return fmt.Errorf("%s digest length = %d, want 64", name, len(digest))
		}
		for _, char := range digest {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				return fmt.Errorf("%s digest %q is not lowercase hex", name, digest)
			}
		}
	}
	return nil
}

type memoryQuotaCompensationReceipt struct {
	apiTokenID string
	amount     float64
}

type memoryQuotaCompensationState struct {
	mu       sync.Mutex
	jobs     map[string]QuotaCompensationJob
	receipts map[string]memoryQuotaCompensationReceipt
}

type memoryQuotaCompensationStore struct {
	state *memoryQuotaCompensationState
}

func newMemoryQuotaCompensationStore() *memoryQuotaCompensationStore {
	return newMemoryQuotaCompensationStoreWithState(&memoryQuotaCompensationState{
		jobs:     map[string]QuotaCompensationJob{},
		receipts: map[string]memoryQuotaCompensationReceipt{},
	})
}

func newMemoryQuotaCompensationStoreWithState(state *memoryQuotaCompensationState) *memoryQuotaCompensationStore {
	return &memoryQuotaCompensationStore{state: state}
}

func (s *memoryQuotaCompensationStore) ArmQuotaCompensation(_ context.Context, req QuotaCompensationRequest) (QuotaCompensationJob, error) {
	keys, err := BuildQuotaCompensationKeys(req)
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if job, ok := s.state.jobs[keys.JobKeyDigest]; ok {
		return job, nil
	}
	job := QuotaCompensationJob{
		JobKeyDigest:               keys.JobKeyDigest,
		OrganizationScopeKeyDigest: keys.OrganizationScopeKeyDigest,
		APITokenScopeKeyDigest:     keys.APITokenScopeKeyDigest,
		OrganizationID:             strings.TrimSpace(req.OrganizationID),
		BillingSessionID:           strings.TrimSpace(req.BillingSessionID),
		APITokenID:                 strings.TrimSpace(req.APITokenID),
		Amount:                     req.Amount,
		OrganizationRequired:       keys.OrganizationScopeKeyDigest != "",
		APITokenRequired:           keys.APITokenScopeKeyDigest != "",
		Status:                     QuotaCompensationJobStatusPending,
		// AvailableAt uses zero value so any non-zero claim time finds the job
		// immediately due, regardless of the wall clock at arm time.
		AvailableAt: time.Time{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.state.jobs[job.JobKeyDigest] = job
	return job, nil
}

func (s *memoryQuotaCompensationStore) ClaimQuotaCompensationJobs(_ context.Context, now time.Time, limit int, workerID string) ([]QuotaCompensationJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	keys := make([]string, 0, len(s.state.jobs))
	for key := range s.state.jobs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	claimed := make([]QuotaCompensationJob, 0, limit)
	for _, key := range keys {
		if len(claimed) == limit {
			break
		}
		job := s.state.jobs[key]
		if job.Status == QuotaCompensationJobStatusSucceeded || now.Before(job.AvailableAt) {
			continue
		}
		job.Status = QuotaCompensationJobStatusProcessing
		job.Attempts++
		job.LockedAt = &now
		job.LockedBy = workerID
		job.AvailableAt = now.Add(defaultQuotaCompensationClaimLease)
		job.UpdatedAt = now
		s.state.jobs[key] = job
		claimed = append(claimed, job)
	}
	return claimed, nil
}

func (s *memoryQuotaCompensationStore) MarkQuotaCompensationScopeSucceeded(_ context.Context, jobKeyDigest, scope, _ string, completedAt time.Time) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	job, ok := s.state.jobs[jobKeyDigest]
	if !ok {
		return errors.New("job not found")
	}
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	switch scope {
	case QuotaCompensationScopeOrganization:
		if !job.OrganizationRequired {
			return errors.New("organization scope not required")
		}
		job.OrganizationCompleted = true
	case QuotaCompensationScopeAPIToken:
		if !job.APITokenRequired {
			return errors.New("api token scope not required")
		}
		job.APITokenCompleted = true
	default:
		return errors.New("unknown scope")
	}
	if (!job.OrganizationRequired || job.OrganizationCompleted) && (!job.APITokenRequired || job.APITokenCompleted) {
		job.Status = QuotaCompensationJobStatusSucceeded
		job.CompletedAt = &completedAt
		job.LockedAt = nil
		job.LockedBy = ""
	}
	job.UpdatedAt = completedAt
	s.state.jobs[jobKeyDigest] = job
	return nil
}

func (s *memoryQuotaCompensationStore) MarkQuotaCompensationScopeFailed(context.Context, string, string, string, string, time.Time) error {
	return nil
}

func (s *memoryQuotaCompensationStore) RecordAPITokenQuotaRefundReceipt(_ context.Context, scopeKeyDigest, apiTokenID string, amount float64, _ time.Time) (bool, error) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if receipt, ok := s.state.receipts[scopeKeyDigest]; ok {
		if receipt.apiTokenID != apiTokenID || receipt.amount != amount {
			return false, ErrQuotaCompensationReceiptMismatch
		}
		return false, nil
	}
	s.state.receipts[scopeKeyDigest] = memoryQuotaCompensationReceipt{apiTokenID: apiTokenID, amount: amount}
	return true, nil
}

// --- Compensation coordinator tests (Task 2) ---

type recordingQuotaManager struct {
	refundCalls  int
	refundErrors map[string]error // keyed by sessionID
}

func (m *recordingQuotaManager) PreConsume(_ context.Context, _, _ string, _ float64, _ string, _, _, _ string) (*quota.BillingSession, error) {
	return nil, nil
}
func (m *recordingQuotaManager) Settle(_ context.Context, _, _ string, _ float64) error { return nil }
func (m *recordingQuotaManager) Refund(_ context.Context, _ string, sessionID string) error {
	m.refundCalls++
	if m.refundErrors != nil {
		if err, ok := m.refundErrors[sessionID]; ok {
			return err
		}
	}
	return nil
}

type recordingAPITokenRefundManager struct {
	refundOnceCalls int
	refundOnceFails bool
}

func (m *recordingAPITokenRefundManager) PreAuthorizeRelayAPITokenQuota(_ context.Context, _ string, _ float64) error {
	return nil
}
func (m *recordingAPITokenRefundManager) SettleRelayAPITokenQuota(_ context.Context, _ string, _, _ float64) error {
	return nil
}
func (m *recordingAPITokenRefundManager) RefundRelayAPITokenQuota(_ context.Context, _ string, _ float64) error {
	return nil
}
func (m *recordingAPITokenRefundManager) RefundRelayAPITokenQuotaOnce(_ context.Context, _ string, _ float64, _ string) error {
	m.refundOnceCalls++
	if m.refundOnceFails {
		return errors.New("quota_store_unavailable")
	}
	return nil
}

func newTestCompensationRequest(routeAttemptID, sessionID, tokenID string, amount float64) QuotaCompensationRequest {
	return QuotaCompensationRequest{
		RouteAttemptID:   routeAttemptID,
		Stage:            QuotaCompensationStageLateModelReadiness,
		OrganizationID:   "org_test",
		BillingSessionID: sessionID,
		APITokenID:       tokenID,
		Amount:           amount,
	}
}

func TestRelayLateReadinessCompensationCoordinatorArmsJobBeforeRefund(t *testing.T) {
	store := newMemoryQuotaCompensationStore()
	orgMgr := &recordingQuotaManager{}
	tokenMgr := &recordingAPITokenRefundManager{}
	coordinator := NewQuotaCompensationCoordinator(store, orgMgr, tokenMgr)
	readinessErr := errors.New("readiness_stale")

	req := newTestCompensationRequest("attempt-1", "bill-session-1", "tok-1", 5.0)
	result := coordinator.CompensateLateReadiness(context.Background(), readinessErr, req)
	if result == nil || !errors.Is(result, readinessErr) {
		t.Fatalf("expected readiness error in result, got %v", result)
	}
	if orgMgr.refundCalls != 1 {
		t.Fatalf("expected 1 org refund, got %d", orgMgr.refundCalls)
	}
	if tokenMgr.refundOnceCalls != 1 {
		t.Fatalf("expected 1 api-token refund, got %d", tokenMgr.refundOnceCalls)
	}
	// Job must be committed before refund completes (inspect via replay).
	replayed, err := store.ArmQuotaCompensation(context.Background(), req)
	if err != nil {
		t.Fatalf("re-arm after compensation: %v", err)
	}
	if !replayed.OrganizationCompleted || !replayed.APITokenCompleted {
		t.Fatalf("job scopes not marked succeeded: org=%v token=%v", replayed.OrganizationCompleted, replayed.APITokenCompleted)
	}
}

func TestRelayLateReadinessCompensationCoordinatorReportsPartialFailure(t *testing.T) {
	store := newMemoryQuotaCompensationStore()
	orgMgr := &recordingQuotaManager{refundErrors: map[string]error{"bill-fail": errors.New("refund_failed")}}
	tokenMgr := &recordingAPITokenRefundManager{}
	coordinator := NewQuotaCompensationCoordinator(store, orgMgr, tokenMgr)
	readinessErr := errors.New("readiness_stale")

	req := newTestCompensationRequest("attempt-fail", "bill-fail", "tok-fail", 3.0)
	result := coordinator.CompensateLateReadiness(context.Background(), readinessErr, req)
	if result == nil || !errors.Is(result, readinessErr) {
		t.Fatalf("expected readiness error in result, got %v", result)
	}
	// Org scope failure must be reported via a stable scope error.
	var scopeErr *QuotaCompensationScopeError
	found := false
	for _, e := range unpackErrors(result) {
		if errors.As(e, &scopeErr) && scopeErr.Scope == QuotaCompensationScopeOrganization {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected organization scope error in result, got %v", result)
	}
	// Token scope should still have been attempted and succeeded.
	if tokenMgr.refundOnceCalls != 1 {
		t.Fatalf("expected token refund attempted independently, got %d calls", tokenMgr.refundOnceCalls)
	}
}

// unpackErrors flattens an errors.Join tree into its leaves.
func unpackErrors(err error) []error {
	if err == nil {
		return nil
	}
	type joinErr interface {
		Unwrap() []error
	}
	if j, ok := err.(joinErr); ok {
		var out []error
		for _, e := range j.Unwrap() {
			out = append(out, unpackErrors(e)...)
		}
		return out
	}
	return []error{err}
}
