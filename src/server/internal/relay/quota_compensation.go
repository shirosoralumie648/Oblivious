package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	QuotaCompensationStageLateModelReadiness    = "late-model-readiness"
	QuotaCompensationStageLateProviderReadiness = "late-provider-readiness"

	QuotaCompensationScopeOrganization = "organization-quota"
	QuotaCompensationScopeAPIToken     = "api-token-quota"

	QuotaCompensationJobStatusPending    = "pending"
	QuotaCompensationJobStatusProcessing = "processing"
	QuotaCompensationJobStatusSucceeded  = "succeeded"
	QuotaCompensationJobStatusFailed     = "failed"

	QuotaCompensationErrorCodeRefundFailed      = "refund_failed"
	QuotaCompensationErrorCodeMarkFailed        = "mark_failed"
	QuotaCompensationErrorCodeRetryFailed       = "retry_failed"
	QuotaCompensationErrorCodePersistenceFailed = "persistence_failed"
	QuotaCompensationErrorCodeUnknown           = "unknown"

	defaultQuotaCompensationClaimLease = 5 * time.Minute
)

var (
	ErrRouteAttemptIdentityRequired      = errors.New("route_attempt_identity_required")
	ErrQuotaCompensationInvalidRequest   = errors.New("quota_compensation_request_invalid")
	ErrQuotaCompensationReceiptMismatch  = errors.New("quota_compensation_receipt_mismatch")
	ErrQuotaCompensationStoreUnavailable = errors.New("quota_compensation_store_unavailable")
)

// QuotaCompensationRequest holds the minimum opaque coordinates required to
// reverse quota reservations after a late readiness denial. RouteAttemptID and
// CallerIdempotencyKey are used only to derive digests and are never persisted.
type QuotaCompensationRequest struct {
	RouteAttemptID       string
	Stage                string
	OrganizationID       string
	BillingSessionID     string
	APITokenID           string
	Amount               float64
	CallerIdempotencyKey string
}

// QuotaCompensationKeys are safe-to-persist SHA-256 digests. The raw source
// values used to derive them never appear in a QuotaCompensationJob.
type QuotaCompensationKeys struct {
	JobKeyDigest               string
	OrganizationScopeKeyDigest string
	APITokenScopeKeyDigest     string
}

// QuotaCompensationJob is the durable lifecycle view consumed by immediate
// compensation and the reconciliation worker. All identifiers are opaque
// server-side records; no caller token, route attempt, idempotency key, or
// readiness diagnostic is retained here.
type QuotaCompensationJob struct {
	JobKeyDigest               string
	OrganizationScopeKeyDigest string
	APITokenScopeKeyDigest     string
	OrganizationID             string
	BillingSessionID           string
	APITokenID                 string
	Amount                     float64
	OrganizationRequired       bool
	OrganizationCompleted      bool
	APITokenRequired           bool
	APITokenCompleted          bool
	OrganizationErrorCode      string
	APITokenErrorCode          string
	Status                     string
	Attempts                   int
	LockedAt                   *time.Time
	LockedBy                   string
	AvailableAt                time.Time
	CompletedAt                *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// QuotaCompensationStore owns the durable intent, per-scope completion, lease,
// and API-token receipt contracts. Effect execution itself remains with the
// quota stores so each reversal can use its own transaction/idempotency receipt.
type QuotaCompensationStore interface {
	ArmQuotaCompensation(ctx context.Context, req QuotaCompensationRequest) (QuotaCompensationJob, error)
	ClaimQuotaCompensationJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]QuotaCompensationJob, error)
	MarkQuotaCompensationScopeSucceeded(ctx context.Context, jobKeyDigest, scope, lockedBy string, completedAt time.Time) error
	MarkQuotaCompensationScopeFailed(ctx context.Context, jobKeyDigest, scope, lockedBy, errorCode string, availableAt time.Time) error
	RecordAPITokenQuotaRefundReceipt(ctx context.Context, scopeKeyDigest, apiTokenID string, amount float64, createdAt time.Time) (bool, error)
}

// SQLQuotaCompensationStore persists durable compensation state in PostgreSQL.
type SQLQuotaCompensationStore struct {
	db *sql.DB
}

func NewSQLQuotaCompensationStore(db *sql.DB) *SQLQuotaCompensationStore {
	return &SQLQuotaCompensationStore{db: db}
}

// BuildQuotaCompensationKeys derives the only route-attempt material that can
// cross the process-to-database boundary. A caller idempotency key may join the
// canonical tuple as correlation, but the mandatory server route attempt leads
// every key and therefore isolates every new reservation.
func BuildQuotaCompensationKeys(req QuotaCompensationRequest) (QuotaCompensationKeys, error) {
	normalized, err := normalizeQuotaCompensationRequest(req)
	if err != nil {
		return QuotaCompensationKeys{}, err
	}
	jobDigest := quotaCompensationDigest(
		"relay-quota-compensation-job-v1",
		normalized.RouteAttemptID,
		normalized.Stage,
		normalized.BillingSessionID,
		normalized.APITokenID,
		quotaCompensationAmount(normalized.Amount),
		normalized.CallerIdempotencyKey,
	)
	keys := QuotaCompensationKeys{JobKeyDigest: jobDigest}
	if normalized.OrganizationID != "" {
		keys.OrganizationScopeKeyDigest = quotaCompensationDigest("relay-quota-compensation-scope-v1", jobDigest, QuotaCompensationScopeOrganization)
	}
	if normalized.APITokenID != "" {
		keys.APITokenScopeKeyDigest = quotaCompensationDigest("relay-quota-compensation-scope-v1", jobDigest, QuotaCompensationScopeAPIToken)
	}
	return keys, nil
}

func normalizeQuotaCompensationRequest(req QuotaCompensationRequest) (QuotaCompensationRequest, error) {
	req.RouteAttemptID = strings.TrimSpace(req.RouteAttemptID)
	if req.RouteAttemptID == "" {
		return QuotaCompensationRequest{}, ErrRouteAttemptIdentityRequired
	}
	req.Stage = strings.TrimSpace(req.Stage)
	if req.Stage != QuotaCompensationStageLateModelReadiness && req.Stage != QuotaCompensationStageLateProviderReadiness {
		return QuotaCompensationRequest{}, ErrQuotaCompensationInvalidRequest
	}
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	req.BillingSessionID = strings.TrimSpace(req.BillingSessionID)
	req.APITokenID = strings.TrimSpace(req.APITokenID)
	req.CallerIdempotencyKey = strings.TrimSpace(req.CallerIdempotencyKey)
	if (req.OrganizationID == "") != (req.BillingSessionID == "") {
		return QuotaCompensationRequest{}, ErrQuotaCompensationInvalidRequest
	}
	if req.OrganizationID == "" && req.APITokenID == "" {
		return QuotaCompensationRequest{}, ErrQuotaCompensationInvalidRequest
	}
	if req.APITokenID != "" && (req.Amount <= 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0)) {
		return QuotaCompensationRequest{}, ErrQuotaCompensationInvalidRequest
	}
	if req.Amount < 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) {
		return QuotaCompensationRequest{}, ErrQuotaCompensationInvalidRequest
	}
	return req, nil
}

func quotaCompensationAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', 6, 64)
}

func quotaCompensationDigest(parts ...string) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = hasher.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hasher.Write([]byte{':'})
		_, _ = hasher.Write([]byte(part))
		_, _ = hasher.Write([]byte{'|'})
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func isQuotaCompensationDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}

func normalizeQuotaCompensationErrorCode(code string) string {
	switch strings.TrimSpace(code) {
	case QuotaCompensationErrorCodeRefundFailed:
		return QuotaCompensationErrorCodeRefundFailed
	case QuotaCompensationErrorCodeMarkFailed:
		return QuotaCompensationErrorCodeMarkFailed
	case QuotaCompensationErrorCodeRetryFailed:
		return QuotaCompensationErrorCodeRetryFailed
	case QuotaCompensationErrorCodePersistenceFailed:
		return QuotaCompensationErrorCodePersistenceFailed
	default:
		return QuotaCompensationErrorCodeUnknown
	}
}

const quotaCompensationJobColumns = `
	job_key_digest,
	organization_scope_key_digest,
	api_token_scope_key_digest,
	organization_id,
	billing_session_id,
	api_token_id,
	amount,
	organization_required,
	organization_completed,
	api_token_required,
	api_token_completed,
	organization_error_code,
	api_token_error_code,
	status,
	attempts,
	locked_at,
	locked_by,
	available_at,
	completed_at,
	created_at,
	updated_at`

func (s *SQLQuotaCompensationStore) ArmQuotaCompensation(ctx context.Context, req QuotaCompensationRequest) (QuotaCompensationJob, error) {
	if s == nil || s.db == nil {
		return QuotaCompensationJob{}, ErrQuotaCompensationStoreUnavailable
	}
	normalized, err := normalizeQuotaCompensationRequest(req)
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	keys, err := BuildQuotaCompensationKeys(normalized)
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	job, err := scanQuotaCompensationJob(tx.QueryRowContext(ctx, `
		INSERT INTO relay_quota_compensation_jobs (
			job_key_digest,
			organization_scope_key_digest,
			api_token_scope_key_digest,
			organization_id,
			billing_session_id,
			api_token_id,
			amount,
			organization_required,
			organization_completed,
			api_token_required,
			api_token_completed,
			organization_error_code,
			api_token_error_code,
			status,
			attempts,
			locked_at,
			locked_by,
			available_at,
			completed_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9, FALSE, '', '', $10, 0, NULL, '', $11, NULL, $11, $11)
		ON CONFLICT (job_key_digest) DO NOTHING
		RETURNING `+quotaCompensationJobColumns,
		keys.JobKeyDigest,
		keys.OrganizationScopeKeyDigest,
		keys.APITokenScopeKeyDigest,
		normalized.OrganizationID,
		normalized.BillingSessionID,
		normalized.APITokenID,
		normalized.Amount,
		normalized.OrganizationID != "",
		normalized.APITokenID != "",
		QuotaCompensationJobStatusPending,
		now,
	))
	if errors.Is(err, sql.ErrNoRows) {
		job, err = scanQuotaCompensationJob(tx.QueryRowContext(ctx, `
			SELECT `+quotaCompensationJobColumns+`
			FROM relay_quota_compensation_jobs
			WHERE job_key_digest = $1
		`, keys.JobKeyDigest))
	}
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return QuotaCompensationJob{}, err
	}
	committed = true
	return job, nil
}

func (s *SQLQuotaCompensationStore) ClaimQuotaCompensationJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]QuotaCompensationJob, error) {
	if s == nil || s.db == nil {
		return nil, ErrQuotaCompensationStoreUnavailable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 10
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, ErrQuotaCompensationInvalidRequest
	}
	leaseUntil := now.Add(defaultQuotaCompensationClaimLease)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	rows, err := tx.QueryContext(ctx, `
		WITH due AS (
			SELECT job_key_digest
			FROM relay_quota_compensation_jobs
			WHERE (
				(status IN ($2, $3) AND available_at <= $1)
				OR (status = $4 AND available_at <= $1)
			)
			ORDER BY available_at ASC, created_at ASC, job_key_digest ASC
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		), updated AS (
			UPDATE relay_quota_compensation_jobs job
			SET status = $4,
				attempts = attempts + 1,
				locked_at = $1,
				locked_by = $6,
				available_at = $7,
				completed_at = NULL,
				updated_at = $1
			FROM due
			WHERE job.job_key_digest = due.job_key_digest
			RETURNING `+quotaCompensationJobColumns+`
		)
		SELECT `+quotaCompensationJobColumns+`
		FROM updated
		ORDER BY available_at ASC, created_at ASC, job_key_digest ASC
	`, now, QuotaCompensationJobStatusPending, QuotaCompensationJobStatusFailed, QuotaCompensationJobStatusProcessing, limit, workerID, leaseUntil)
	if err != nil {
		return nil, err
	}
	jobs, err := scanQuotaCompensationJobs(rows)
	if closeErr := rows.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return jobs, nil
}

func (s *SQLQuotaCompensationStore) MarkQuotaCompensationScopeSucceeded(ctx context.Context, jobKeyDigest, scope, lockedBy string, completedAt time.Time) error {
	if s == nil || s.db == nil {
		return ErrQuotaCompensationStoreUnavailable
	}
	if !isQuotaCompensationDigest(jobKeyDigest) {
		return ErrQuotaCompensationInvalidRequest
	}
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	lockedBy = strings.TrimSpace(lockedBy)
	var query string
	switch scope {
	case QuotaCompensationScopeOrganization:
		query = `
			UPDATE relay_quota_compensation_jobs
			SET organization_completed = TRUE,
				organization_error_code = '',
				status = CASE WHEN (NOT api_token_required OR api_token_completed) THEN $4 ELSE $5 END,
				locked_at = CASE WHEN (NOT api_token_required OR api_token_completed) THEN NULL ELSE locked_at END,
				locked_by = CASE WHEN (NOT api_token_required OR api_token_completed) THEN '' ELSE locked_by END,
				available_at = $3,
				completed_at = CASE WHEN (NOT api_token_required OR api_token_completed) THEN $3 ELSE NULL END,
				updated_at = $3
			WHERE job_key_digest = $1
				AND organization_required = TRUE
				AND ($2 = '' OR locked_by = $2)`
	case QuotaCompensationScopeAPIToken:
		query = `
			UPDATE relay_quota_compensation_jobs
			SET api_token_completed = TRUE,
				api_token_error_code = '',
				status = CASE WHEN (NOT organization_required OR organization_completed) THEN $4 ELSE $5 END,
				locked_at = CASE WHEN (NOT organization_required OR organization_completed) THEN NULL ELSE locked_at END,
				locked_by = CASE WHEN (NOT organization_required OR organization_completed) THEN '' ELSE locked_by END,
				available_at = $3,
				completed_at = CASE WHEN (NOT organization_required OR organization_completed) THEN $3 ELSE NULL END,
				updated_at = $3
			WHERE job_key_digest = $1
				AND api_token_required = TRUE
				AND ($2 = '' OR locked_by = $2)`
	default:
		return ErrQuotaCompensationInvalidRequest
	}
	result, err := s.db.ExecContext(ctx, query, jobKeyDigest, lockedBy, completedAt, QuotaCompensationJobStatusSucceeded, QuotaCompensationJobStatusPending)
	if err != nil {
		return err
	}
	return ensureRelayRowsAffected(result)
}

func (s *SQLQuotaCompensationStore) MarkQuotaCompensationScopeFailed(ctx context.Context, jobKeyDigest, scope, lockedBy, errorCode string, availableAt time.Time) error {
	if s == nil || s.db == nil {
		return ErrQuotaCompensationStoreUnavailable
	}
	if !isQuotaCompensationDigest(jobKeyDigest) {
		return ErrQuotaCompensationInvalidRequest
	}
	if availableAt.IsZero() {
		availableAt = time.Now().UTC().Add(time.Minute)
	}
	lockedBy = strings.TrimSpace(lockedBy)
	errorCode = normalizeQuotaCompensationErrorCode(errorCode)
	var query string
	switch scope {
	case QuotaCompensationScopeOrganization:
		query = `
			UPDATE relay_quota_compensation_jobs
			SET organization_error_code = $3,
				status = $4,
				locked_at = NULL,
				locked_by = '',
				available_at = $5,
				completed_at = NULL,
				updated_at = $6
			WHERE job_key_digest = $1
				AND organization_required = TRUE
				AND organization_completed = FALSE
				AND ($2 = '' OR locked_by = $2)`
	case QuotaCompensationScopeAPIToken:
		query = `
			UPDATE relay_quota_compensation_jobs
			SET api_token_error_code = $3,
				status = $4,
				locked_at = NULL,
				locked_by = '',
				available_at = $5,
				completed_at = NULL,
				updated_at = $6
			WHERE job_key_digest = $1
				AND api_token_required = TRUE
				AND api_token_completed = FALSE
				AND ($2 = '' OR locked_by = $2)`
	default:
		return ErrQuotaCompensationInvalidRequest
	}
	result, err := s.db.ExecContext(ctx, query, jobKeyDigest, lockedBy, errorCode, QuotaCompensationJobStatusFailed, availableAt, time.Now().UTC())
	if err != nil {
		return err
	}
	return ensureRelayRowsAffected(result)
}

func (s *SQLQuotaCompensationStore) RecordAPITokenQuotaRefundReceipt(ctx context.Context, scopeKeyDigest, apiTokenID string, amount float64, createdAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, ErrQuotaCompensationStoreUnavailable
	}
	scopeKeyDigest = strings.TrimSpace(scopeKeyDigest)
	apiTokenID = strings.TrimSpace(apiTokenID)
	if !isQuotaCompensationDigest(scopeKeyDigest) || apiTokenID == "" || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return false, ErrQuotaCompensationInvalidRequest
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var storedTokenID string
	var storedAmount float64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO relay_api_token_quota_refund_receipts (scope_key_digest, api_token_id, amount, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (scope_key_digest) DO NOTHING
		RETURNING api_token_id, amount
	`, scopeKeyDigest, apiTokenID, amount, createdAt).Scan(&storedTokenID, &storedAmount)
	inserted := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT api_token_id, amount
			FROM relay_api_token_quota_refund_receipts
			WHERE scope_key_digest = $1
		`, scopeKeyDigest).Scan(&storedTokenID, &storedAmount)
	}
	if err != nil {
		return false, err
	}
	if storedTokenID != apiTokenID || storedAmount != amount {
		return false, ErrQuotaCompensationReceiptMismatch
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	committed = true
	return inserted, nil
}

func scanQuotaCompensationJobs(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]QuotaCompensationJob, error) {
	jobs := []QuotaCompensationJob{}
	for rows.Next() {
		job, err := scanQuotaCompensationJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func scanQuotaCompensationJob(scanner interface{ Scan(dest ...any) error }) (QuotaCompensationJob, error) {
	var job QuotaCompensationJob
	var lockedAt sql.NullTime
	var completedAt sql.NullTime
	err := scanner.Scan(
		&job.JobKeyDigest,
		&job.OrganizationScopeKeyDigest,
		&job.APITokenScopeKeyDigest,
		&job.OrganizationID,
		&job.BillingSessionID,
		&job.APITokenID,
		&job.Amount,
		&job.OrganizationRequired,
		&job.OrganizationCompleted,
		&job.APITokenRequired,
		&job.APITokenCompleted,
		&job.OrganizationErrorCode,
		&job.APITokenErrorCode,
		&job.Status,
		&job.Attempts,
		&lockedAt,
		&job.LockedBy,
		&job.AvailableAt,
		&completedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return QuotaCompensationJob{}, err
	}
	if lockedAt.Valid {
		value := lockedAt.Time
		job.LockedAt = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		job.CompletedAt = &value
	}
	return job, nil
}

// GenerateRouteAttemptID mints a cryptographically random route-attempt identity
// that is mandatory before either quota preauthorization can begin.
func GenerateRouteAttemptID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate route attempt identity: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// QuotaCompensationScopeError is a stable, sanitized error that identifies
// a specific compensation scope and operation code. It never carries IDs or
// raw storage details.
type QuotaCompensationScopeError struct {
	Scope string
	Code  string
}

func (e *QuotaCompensationScopeError) Error() string {
	return fmt.Sprintf("quota_compensation_%s_%s", e.Scope, e.Code)
}

// quotaCompensationScopeErr produces a stable scope error from any cause.
// It sanitizes the cause to an allowlisted code without leaking raw details.
func quotaCompensationScopeErr(scope string, cause error) *QuotaCompensationScopeError {
	code := QuotaCompensationErrorCodeRefundFailed
	if cause != nil {
		switch {
		case errors.Is(cause, ErrQuotaCompensationReceiptMismatch):
			code = QuotaCompensationErrorCodeRefundFailed
		case errors.Is(cause, ErrQuotaCompensationStoreUnavailable):
			code = QuotaCompensationErrorCodePersistenceFailed
		}
	}
	return &QuotaCompensationScopeError{Scope: scope, Code: code}
}

// QuotaCompensationCoordinator arms the durable job before any refund attempt
// and then drives each required scope independently.
type QuotaCompensationCoordinator struct {
	store            QuotaCompensationStore
	quotaManager     QuotaManager
	apiTokenManager  APITokenQuotaManager
}

// NewQuotaCompensationCoordinator constructs a coordinator with the given
// durable store and quota/API-token managers. A nil store means the coordinator
// falls back to direct in-place refunds without durability.
func NewQuotaCompensationCoordinator(
	store QuotaCompensationStore,
	quotaManager QuotaManager,
	apiTokenManager APITokenQuotaManager,
) *QuotaCompensationCoordinator {
	return &QuotaCompensationCoordinator{
		store:           store,
		quotaManager:    quotaManager,
		apiTokenManager: apiTokenManager,
	}
}

// CompensateLateReadiness is the single entry point for late-readiness denial
// compensation. It arms the durable job (if a store is configured), attempts
// every required scope independently, and returns the original readiness error
// joined with any stable compensation/persistence failures so callers can still
// classify the root readiness code.
func (c *QuotaCompensationCoordinator) CompensateLateReadiness(
	ctx context.Context,
	readinessErr error,
	req QuotaCompensationRequest,
) error {
	if c == nil || c.store == nil {
		// No durable store — fall back to direct best-effort refunds.
		return c.directFallback(ctx, readinessErr, req)
	}

	job, armErr := c.store.ArmQuotaCompensation(ctx, req)
	if armErr != nil {
		// Arm failure: still attempt direct refunds and report persistence failure.
		persistErr := &QuotaCompensationScopeError{Scope: "compensation-persistence", Code: QuotaCompensationErrorCodePersistenceFailed}
		directErr := c.directFallback(ctx, nil, req)
		if directErr != nil {
			return errors.Join(readinessErr, persistErr, directErr)
		}
		return errors.Join(readinessErr, persistErr)
	}

	var scopeErrs []error
	now := time.Now().UTC()

	if job.OrganizationRequired && !job.OrganizationCompleted && c.quotaManager != nil {
		if refundErr := c.quotaManager.Refund(ctx, req.OrganizationID, req.BillingSessionID); refundErr != nil {
			if markErr := c.store.MarkQuotaCompensationScopeFailed(
				ctx, job.JobKeyDigest, QuotaCompensationScopeOrganization, "",
				QuotaCompensationErrorCodeRefundFailed, now.Add(time.Minute),
			); markErr != nil {
				scopeErrs = append(scopeErrs, &QuotaCompensationScopeError{Scope: QuotaCompensationScopeOrganization, Code: QuotaCompensationErrorCodeMarkFailed})
			} else {
				scopeErrs = append(scopeErrs, quotaCompensationScopeErr(QuotaCompensationScopeOrganization, refundErr))
			}
		} else {
			_ = c.store.MarkQuotaCompensationScopeSucceeded(ctx, job.JobKeyDigest, QuotaCompensationScopeOrganization, "", now)
		}
	}

	if job.APITokenRequired && !job.APITokenCompleted && c.apiTokenManager != nil && req.APITokenID != "" {
		var refundErr error
		if req.Amount > 0 {
			refundErr = c.apiTokenManager.RefundRelayAPITokenQuotaOnce(ctx, req.APITokenID, req.Amount, job.APITokenScopeKeyDigest)
		}
		if refundErr != nil {
			if markErr := c.store.MarkQuotaCompensationScopeFailed(
				ctx, job.JobKeyDigest, QuotaCompensationScopeAPIToken, "",
				QuotaCompensationErrorCodeRefundFailed, now.Add(time.Minute),
			); markErr != nil {
				scopeErrs = append(scopeErrs, &QuotaCompensationScopeError{Scope: QuotaCompensationScopeAPIToken, Code: QuotaCompensationErrorCodeMarkFailed})
			} else {
				scopeErrs = append(scopeErrs, quotaCompensationScopeErr(QuotaCompensationScopeAPIToken, refundErr))
			}
		} else if req.Amount > 0 {
			_ = c.store.MarkQuotaCompensationScopeSucceeded(ctx, job.JobKeyDigest, QuotaCompensationScopeAPIToken, "", now)
		}
	}

	if len(scopeErrs) == 0 {
		return readinessErr
	}
	return errors.Join(append([]error{readinessErr}, scopeErrs...)...)
}

// directFallback performs best-effort in-place refunds when no durable store
// is available.
func (c *QuotaCompensationCoordinator) directFallback(ctx context.Context, readinessErr error, req QuotaCompensationRequest) error {
	var scopeErrs []error
	// Match original router behavior: refund whenever a billing session ID exists,
	// regardless of whether organizationID is empty (some callers set only one).
	if req.BillingSessionID != "" && c != nil && c.quotaManager != nil {
		if err := c.quotaManager.Refund(ctx, req.OrganizationID, req.BillingSessionID); err != nil {
			scopeErrs = append(scopeErrs, quotaCompensationScopeErr(QuotaCompensationScopeOrganization, err))
		}
	}
	if req.APITokenID != "" && req.Amount > 0 && c != nil && c.apiTokenManager != nil {
		if err := c.apiTokenManager.RefundRelayAPITokenQuota(ctx, req.APITokenID, req.Amount); err != nil {
			scopeErrs = append(scopeErrs, quotaCompensationScopeErr(QuotaCompensationScopeAPIToken, err))
		}
	}
	if readinessErr == nil && len(scopeErrs) == 0 {
		return nil
	}
	if len(scopeErrs) == 0 {
		return readinessErr
	}
	return errors.Join(append([]error{readinessErr}, scopeErrs...)...)
}

// --- Reconciliation worker ---

const (
	defaultQuotaCompensationWorkerInterval = 30 * time.Second
	defaultQuotaCompensationWorkerLimit    = 10
	defaultQuotaCompensationWorkerIDPrefix = "relay-quota-compensation-worker"
)

// QuotaCompensationWorkerConfig holds tunable worker parameters.
type QuotaCompensationWorkerConfig struct {
	Interval time.Duration
	Limit    int
	WorkerID string
	Now      func() time.Time
	OnError  func(error)
}

// QuotaCompensationWorker claims persisted compensation jobs and retries any
// still-pending scopes through the same idempotent coordinator.
type QuotaCompensationWorker struct {
	coordinator *QuotaCompensationCoordinator
	store       QuotaCompensationStore
	config      QuotaCompensationWorkerConfig
}

// NewQuotaCompensationWorker constructs a worker that drives the given store
// and coordinator. OnError receives any non-fatal per-batch errors.
func NewQuotaCompensationWorker(store QuotaCompensationStore, coordinator *QuotaCompensationCoordinator, config QuotaCompensationWorkerConfig) *QuotaCompensationWorker {
	if config.Interval <= 0 {
		config.Interval = defaultQuotaCompensationWorkerInterval
	}
	if config.Limit <= 0 {
		config.Limit = defaultQuotaCompensationWorkerLimit
	}
	if config.WorkerID == "" {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err == nil {
			config.WorkerID = defaultQuotaCompensationWorkerIDPrefix + "-" + hex.EncodeToString(b)
		} else {
			config.WorkerID = defaultQuotaCompensationWorkerIDPrefix
		}
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &QuotaCompensationWorker{coordinator: coordinator, store: store, config: config}
}

// Run processes claimed jobs at every tick until ctx is cancelled.
func (w *QuotaCompensationWorker) Run(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processOnce(ctx); err != nil && w.config.OnError != nil {
				w.config.OnError(err)
			}
		}
	}
}

func (w *QuotaCompensationWorker) processOnce(ctx context.Context) error {
	now := w.config.Now()
	jobs, err := w.store.ClaimQuotaCompensationJobs(ctx, now, w.config.Limit, w.config.WorkerID)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		w.processJob(ctx, job, now)
	}
	return nil
}

func (w *QuotaCompensationWorker) processJob(ctx context.Context, job QuotaCompensationJob, now time.Time) {
	c := w.coordinator

	if job.OrganizationRequired && !job.OrganizationCompleted && c != nil && c.quotaManager != nil {
		if refundErr := c.quotaManager.Refund(ctx, job.OrganizationID, job.BillingSessionID); refundErr != nil {
			_ = w.store.MarkQuotaCompensationScopeFailed(
				ctx, job.JobKeyDigest, QuotaCompensationScopeOrganization, w.config.WorkerID,
				QuotaCompensationErrorCodeRetryFailed, now.Add(time.Minute),
			)
		} else {
			_ = w.store.MarkQuotaCompensationScopeSucceeded(ctx, job.JobKeyDigest, QuotaCompensationScopeOrganization, w.config.WorkerID, now)
		}
	}

	if job.APITokenRequired && !job.APITokenCompleted && c != nil && c.apiTokenManager != nil && job.Amount > 0 {
		if refundErr := c.apiTokenManager.RefundRelayAPITokenQuotaOnce(ctx, job.APITokenID, job.Amount, job.APITokenScopeKeyDigest); refundErr != nil {
			_ = w.store.MarkQuotaCompensationScopeFailed(
				ctx, job.JobKeyDigest, QuotaCompensationScopeAPIToken, w.config.WorkerID,
				QuotaCompensationErrorCodeRetryFailed, now.Add(time.Minute),
			)
		} else {
			_ = w.store.MarkQuotaCompensationScopeSucceeded(ctx, job.JobKeyDigest, QuotaCompensationScopeAPIToken, w.config.WorkerID, now)
		}
	}
}
