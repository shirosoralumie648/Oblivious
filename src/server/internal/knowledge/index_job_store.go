package knowledge

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

type knowledgeIndexJobQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *SQLStore) CreateKnowledgeIndexJob(ctx context.Context, req CreateKnowledgeIndexJobRequest) (KnowledgeIndexJob, error) {
	return insertKnowledgeIndexJob(ctx, s.db, req, time.Now().UTC())
}

func insertKnowledgeIndexJob(ctx context.Context, queryer knowledgeIndexJobQueryer, req CreateKnowledgeIndexJobRequest, now time.Time) (KnowledgeIndexJob, error) {
	organizationID := strings.TrimSpace(req.OrganizationID)
	knowledgeBaseID := strings.TrimSpace(req.KnowledgeBaseID)
	documentID := strings.TrimSpace(req.DocumentID)
	if organizationID == "" || knowledgeBaseID == "" || documentID == "" {
		return KnowledgeIndexJob{}, sql.ErrNoRows
	}
	jobID, err := auth.NewID("kij")
	if err != nil {
		return KnowledgeIndexJob{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultKnowledgeIndexJobMaxAttempts
	}
	return scanKnowledgeIndexJob(queryer.QueryRowContext(ctx, `
		INSERT INTO knowledge_index_jobs (
			id,
			organization_id,
			knowledge_base_id,
			document_id,
			operation,
			status,
			error,
			attempts,
			max_attempts,
			locked_at,
			locked_by,
			available_at,
			completed_at,
			created_at,
			updated_at
		)
		SELECT $1, $2, kb.id, d.id, $5, $6, '', 0, $7, NULL, '', $8, NULL, $8, $8
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		WHERE d.organization_id = $2
		  AND kb.organization_id = $2
		  AND kb.id = $3
		  AND d.id = $4
		RETURNING id, organization_id, knowledge_base_id, document_id, operation, status, COALESCE(error, ''), attempts, max_attempts, locked_at, COALESCE(locked_by, ''), available_at, completed_at, created_at, updated_at
	`, jobID, organizationID, knowledgeBaseID, documentID, normalizeKnowledgeIndexOperation(req.Operation), KnowledgeIndexJobStatusPending, maxAttempts, now))
}

func (s *SQLStore) ClaimKnowledgeIndexJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]KnowledgeIndexJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = defaultKnowledgeIndexWorkerID()
	}
	leaseUntil := now.Add(defaultKnowledgeIndexJobClaimLease)
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
			SELECT id
			FROM knowledge_index_jobs
			WHERE (
			    (status IN ($2, $3) AND available_at <= $1)
			 OR (status = $4 AND available_at <= $1)
			)
			  AND attempts <= max_attempts
			ORDER BY available_at ASC, created_at ASC, id ASC
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE knowledge_index_jobs job
			SET status = $4,
			    attempts = attempts + 1,
			    locked_at = $1,
			    locked_by = $6,
			    completed_by = '',
			    available_at = $7,
		    completed_at = NULL,
		    updated_at = $1
			FROM due
			WHERE job.id = due.id
			RETURNING job.id, job.organization_id, job.knowledge_base_id, job.document_id, job.operation, job.status, COALESCE(job.error, ''), job.attempts, job.max_attempts, job.locked_at, COALESCE(job.locked_by, ''), job.available_at, job.completed_at, job.created_at, job.updated_at
		)
		SELECT id, organization_id, knowledge_base_id, document_id, operation, status, error, attempts, max_attempts, locked_at, locked_by, available_at, completed_at, created_at, updated_at
		FROM updated
		ORDER BY available_at ASC, created_at ASC, id ASC
	`, now, KnowledgeIndexJobStatusPending, KnowledgeIndexJobStatusFailed, KnowledgeIndexJobStatusProcessing, limit, workerID, leaseUntil)
	if err != nil {
		return nil, err
	}
	jobs, err := scanKnowledgeIndexJobs(rows)
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

func (s *SQLStore) MarkKnowledgeIndexJobSucceeded(ctx context.Context, organizationID, jobID, lockedBy string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	lockedBy = strings.TrimSpace(lockedBy)
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_index_jobs
		SET status = $4,
		    error = '',
		    locked_at = NULL,
		    completed_by = CASE
		        WHEN $3 = '' THEN COALESCE(NULLIF(locked_by, ''), completed_by, '')
		        ELSE $3
		    END,
		    locked_by = '',
		    available_at = $5,
		    completed_at = $5,
		    updated_at = $5
		WHERE organization_id = $1
		  AND id = $2
		  AND ($3 = '' OR locked_by = $3)
	`, organizationID, jobID, lockedBy, KnowledgeIndexJobStatusSucceeded, completedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLStore) MarkKnowledgeIndexJobFailed(ctx context.Context, organizationID, jobID, lockedBy, reason string, availableAt time.Time) error {
	if availableAt.IsZero() {
		availableAt = time.Now().UTC().Add(time.Minute)
	}
	lockedBy = strings.TrimSpace(lockedBy)
	updatedAt := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_index_jobs
		SET status = $4,
		    error = $5,
		    locked_at = NULL,
		    locked_by = '',
		    available_at = $6,
		    completed_at = NULL,
		    updated_at = $7
		WHERE organization_id = $1
		  AND id = $2
		  AND ($3 = '' OR locked_by = $3)
	`, organizationID, jobID, lockedBy, KnowledgeIndexJobStatusFailed, strings.TrimSpace(reason), availableAt, updatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLStore) MarkKnowledgeIndexJobDeadLetter(ctx context.Context, organizationID, jobID, lockedBy, reason string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	lockedBy = strings.TrimSpace(lockedBy)
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_index_jobs
		SET status = $4,
		    error = $5,
		    locked_at = NULL,
		    locked_by = '',
		    available_at = $6,
		    completed_at = $6,
		    updated_at = $6
		WHERE organization_id = $1
		  AND id = $2
		  AND ($3 = '' OR locked_by = $3)
	`, organizationID, jobID, lockedBy, KnowledgeIndexJobStatusDeadLetter, strings.TrimSpace(reason), completedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLStore) SetKnowledgeDocumentIndexStatus(ctx context.Context, organizationID, documentID, status, errorMessage string, indexedAt *time.Time) error {
	var indexedAtValue any
	if indexedAt != nil {
		indexedAtValue = *indexedAt
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_documents
		SET index_status = $3,
		    index_error = $4,
		    indexed_at = $5,
		    updated_at = $6
		WHERE organization_id = $1
		  AND id = $2
	`, organizationID, documentID, status, strings.TrimSpace(errorMessage), indexedAtValue, time.Now().UTC())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func ensureRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
