package knowledge

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

type knowledgeIngestionJobQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *SQLStore) CreateKnowledgeIngestionJob(ctx context.Context, req CreateKnowledgeIngestionJobRequest) (KnowledgeIngestionJob, error) {
	return insertKnowledgeIngestionJob(ctx, s.db, req, time.Now().UTC())
}

func (s *SQLStore) ListKnowledgeIngestionJobs(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeIngestionJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, knowledge_base_id, COALESCE(document_id, ''), title, content, raw_content, raw_filename, raw_content_type, raw_size_bytes, document_version, update_strategy, source_url, page_number, status, COALESCE(error, ''), attempts, max_attempts, locked_at, COALESCE(locked_by, ''), available_at, completed_at, created_at, updated_at
		FROM knowledge_ingestion_jobs
		WHERE organization_id = $1
		  AND knowledge_base_id = $2
		ORDER BY updated_at DESC, created_at DESC, id DESC
	`, strings.TrimSpace(organizationID), strings.TrimSpace(knowledgeBaseID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKnowledgeIngestionJobs(rows)
}

func insertKnowledgeIngestionJob(ctx context.Context, queryer knowledgeIngestionJobQueryer, req CreateKnowledgeIngestionJobRequest, now time.Time) (KnowledgeIngestionJob, error) {
	organizationID := strings.TrimSpace(req.OrganizationID)
	knowledgeBaseID := strings.TrimSpace(req.KnowledgeBaseID)
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	rawContent := append([]byte(nil), req.RawContent...)
	rawFilename := strings.TrimSpace(req.RawFilename)
	rawContentType := strings.TrimSpace(req.RawContentType)
	rawSizeBytes := req.RawSizeBytes
	if rawSizeBytes <= 0 {
		rawSizeBytes = int64(len(rawContent))
	}
	if organizationID == "" || knowledgeBaseID == "" || title == "" || (content == "" && len(rawContent) == 0) {
		return KnowledgeIngestionJob{}, sql.ErrNoRows
	}
	options := normalizeKnowledgeDocumentOptions(req.Options)
	jobID, err := auth.NewID("kig")
	if err != nil {
		return KnowledgeIngestionJob{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultKnowledgeIngestionJobMaxAttempts
	}
	return scanKnowledgeIngestionJob(queryer.QueryRowContext(ctx, `
		INSERT INTO knowledge_ingestion_jobs (
			id,
			organization_id,
			knowledge_base_id,
			document_id,
			title,
			content,
			raw_content,
			raw_filename,
			raw_content_type,
			raw_size_bytes,
			document_version,
			update_strategy,
			source_url,
			page_number,
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
		SELECT $1, $2, kb.id, '', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, '', 0, $15, NULL, '', $16, NULL, $16, $16
		FROM knowledge_bases kb
		WHERE kb.organization_id = $2
		  AND kb.id = $3
		RETURNING id, organization_id, knowledge_base_id, COALESCE(document_id, ''), title, content, raw_content, raw_filename, raw_content_type, raw_size_bytes, document_version, update_strategy, source_url, page_number, status, COALESCE(error, ''), attempts, max_attempts, locked_at, COALESCE(locked_by, ''), available_at, completed_at, created_at, updated_at
	`, jobID, organizationID, knowledgeBaseID, title, content, rawContent, rawFilename, rawContentType, rawSizeBytes, options.DocumentVersion, options.UpdateStrategy, options.SourceURL, options.PageNumber, KnowledgeIngestionJobStatusPending, maxAttempts, now))
}

func (s *SQLStore) ClaimKnowledgeIngestionJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]KnowledgeIngestionJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = defaultKnowledgeIngestionWorkerID()
	}
	leaseUntil := now.Add(defaultKnowledgeIngestionJobClaimLease)
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
			FROM knowledge_ingestion_jobs
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
			UPDATE knowledge_ingestion_jobs job
			SET status = $4,
			    attempts = attempts + 1,
			    locked_at = $1,
			    locked_by = $6,
			    available_at = $7,
			    completed_at = NULL,
			    updated_at = $1
			FROM due
			WHERE job.id = due.id
			RETURNING job.id, job.organization_id, job.knowledge_base_id, COALESCE(job.document_id, ''), job.title, job.content, job.raw_content, job.raw_filename, job.raw_content_type, job.raw_size_bytes, job.document_version, job.update_strategy, job.source_url, job.page_number, job.status, COALESCE(job.error, ''), job.attempts, job.max_attempts, job.locked_at, COALESCE(job.locked_by, ''), job.available_at, job.completed_at, job.created_at, job.updated_at
		)
		SELECT id, organization_id, knowledge_base_id, document_id, title, content, raw_content, raw_filename, raw_content_type, raw_size_bytes, document_version, update_strategy, source_url, page_number, status, error, attempts, max_attempts, locked_at, locked_by, available_at, completed_at, created_at, updated_at
		FROM updated
		ORDER BY available_at ASC, created_at ASC, id ASC
	`, now, KnowledgeIngestionJobStatusPending, KnowledgeIngestionJobStatusFailed, KnowledgeIngestionJobStatusProcessing, limit, workerID, leaseUntil)
	if err != nil {
		return nil, err
	}
	jobs, err := scanKnowledgeIngestionJobs(rows)
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

func (s *SQLStore) MarkKnowledgeIngestionJobSucceeded(ctx context.Context, organizationID, jobID, lockedBy, documentID string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	lockedBy = strings.TrimSpace(lockedBy)
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_ingestion_jobs
		SET status = $4,
		    document_id = $5,
		    error = '',
		    locked_at = NULL,
		    locked_by = '',
		    available_at = $6,
		    completed_at = $6,
		    updated_at = $6
		WHERE organization_id = $1
		  AND id = $2
		  AND ($3 = '' OR locked_by = $3)
	`, organizationID, jobID, lockedBy, KnowledgeIngestionJobStatusSucceeded, strings.TrimSpace(documentID), completedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLStore) MarkKnowledgeIngestionJobFailed(ctx context.Context, organizationID, jobID, lockedBy, reason string, availableAt time.Time) error {
	if availableAt.IsZero() {
		availableAt = time.Now().UTC().Add(time.Minute)
	}
	lockedBy = strings.TrimSpace(lockedBy)
	updatedAt := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_ingestion_jobs
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
	`, organizationID, jobID, lockedBy, KnowledgeIngestionJobStatusFailed, strings.TrimSpace(reason), availableAt, updatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLStore) MarkKnowledgeIngestionJobDeadLetter(ctx context.Context, organizationID, jobID, lockedBy, reason string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	lockedBy = strings.TrimSpace(lockedBy)
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_ingestion_jobs
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
	`, organizationID, jobID, lockedBy, KnowledgeIngestionJobStatusDeadLetter, strings.TrimSpace(reason), completedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func scanKnowledgeIngestionJobs(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]KnowledgeIngestionJob, error) {
	var jobs []KnowledgeIngestionJob
	for rows.Next() {
		job, err := scanKnowledgeIngestionJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func scanKnowledgeIngestionJob(scanner interface{ Scan(dest ...any) error }) (KnowledgeIngestionJob, error) {
	var job KnowledgeIngestionJob
	var lockedAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&job.ID,
		&job.OrganizationID,
		&job.KnowledgeBaseID,
		&job.DocumentID,
		&job.Title,
		&job.Content,
		&job.RawContent,
		&job.RawFilename,
		&job.RawContentType,
		&job.RawSizeBytes,
		&job.Options.DocumentVersion,
		&job.Options.UpdateStrategy,
		&job.Options.SourceURL,
		&job.Options.PageNumber,
		&job.Status,
		&job.Error,
		&job.Attempts,
		&job.MaxAttempts,
		&lockedAt,
		&job.LockedBy,
		&job.AvailableAt,
		&completedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return KnowledgeIngestionJob{}, err
	}
	job.Options = normalizeKnowledgeDocumentOptions(job.Options)
	job.RawFilename = strings.TrimSpace(job.RawFilename)
	job.RawContentType = strings.TrimSpace(job.RawContentType)
	if job.RawSizeBytes <= 0 && len(job.RawContent) > 0 {
		job.RawSizeBytes = int64(len(job.RawContent))
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = defaultKnowledgeIngestionJobMaxAttempts
	}
	if lockedAt.Valid {
		lockedAtValue := lockedAt.Time
		job.LockedAt = &lockedAtValue
	}
	if completedAt.Valid {
		completedAtValue := completedAt.Time
		job.CompletedAt = &completedAtValue
	}
	return job, nil
}
