package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

const (
	KnowledgeIndexJobOperationUpsertDocument = "upsert_document"
	KnowledgeIndexJobOperationDeleteDocument = "delete_document"

	KnowledgeIndexJobStatusPending    = "pending"
	KnowledgeIndexJobStatusProcessing = "processing"
	KnowledgeIndexJobStatusSucceeded  = "succeeded"
	KnowledgeIndexJobStatusFailed     = "failed"
	KnowledgeIndexJobStatusDeadLetter = "dead_letter"
)

const (
	defaultKnowledgeIndexWorkerInterval    = time.Minute
	defaultKnowledgeIndexJobMaxAttempts    = 5
	defaultKnowledgeIndexJobClaimLease     = 5 * time.Minute
	defaultKnowledgeIndexJobWorkerIDPrefix = "rag-index-worker"
)

type CreateKnowledgeIndexJobRequest struct {
	OrganizationID  string
	KnowledgeBaseID string
	DocumentID      string
	Operation       string
	MaxAttempts     int
}

type KnowledgeIndexJob struct {
	ID              string
	OrganizationID  string
	KnowledgeBaseID string
	DocumentID      string
	Operation       string
	Status          string
	Error           string
	Attempts        int
	MaxAttempts     int
	LockedAt        *time.Time
	LockedBy        string
	AvailableAt     time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type knowledgeIndexJobStore interface {
	CreateKnowledgeIndexJob(ctx context.Context, req CreateKnowledgeIndexJobRequest) (KnowledgeIndexJob, error)
	MarkKnowledgeIndexJobFailed(ctx context.Context, organizationID, jobID, lockedBy, reason string, availableAt time.Time) error
	MarkKnowledgeIndexJobSucceeded(ctx context.Context, organizationID, jobID, lockedBy string, completedAt time.Time) error
	SetKnowledgeDocumentIndexStatus(ctx context.Context, organizationID, documentID, status, errorMessage string, indexedAt *time.Time) error
}

type KnowledgeIndexWorkerStore interface {
	ClaimKnowledgeIndexJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]KnowledgeIndexJob, error)
	MarkKnowledgeIndexJobDeadLetter(ctx context.Context, organizationID, jobID, lockedBy, reason string, completedAt time.Time) error
	MarkKnowledgeIndexJobFailed(ctx context.Context, organizationID, jobID, lockedBy, reason string, availableAt time.Time) error
	MarkKnowledgeIndexJobSucceeded(ctx context.Context, organizationID, jobID, lockedBy string, completedAt time.Time) error
	SetKnowledgeDocumentIndexStatus(ctx context.Context, organizationID, documentID, status, errorMessage string, indexedAt *time.Time) error
}

type IndexWorkerConfig struct {
	Interval time.Duration
	Limit    int
	Now      func() time.Time
	Ticks    <-chan time.Time
	WorkerID string
	OnError  func(error)
}

type IndexWorker struct {
	service  *Service
	store    KnowledgeIndexWorkerStore
	interval time.Duration
	limit    int
	now      func() time.Time
	ticks    <-chan time.Time
	workerID string
	onError  func(error)
}

func NewIndexWorker(service *Service, store KnowledgeIndexWorkerStore, config IndexWorkerConfig) *IndexWorker {
	interval := config.Interval
	if interval <= 0 {
		interval = defaultKnowledgeIndexWorkerInterval
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	limit := config.Limit
	if limit <= 0 {
		limit = 10
	}
	workerID := strings.TrimSpace(config.WorkerID)
	if workerID == "" {
		workerID = defaultKnowledgeIndexWorkerID()
	}
	return &IndexWorker{
		service:  service,
		store:    store,
		interval: interval,
		limit:    limit,
		now:      now,
		ticks:    config.Ticks,
		workerID: workerID,
		onError:  config.OnError,
	}
}

func (w *IndexWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil || w.store == nil {
		return
	}
	ticks := w.ticks
	var ticker *time.Ticker
	if ticks == nil {
		ticker = time.NewTicker(w.interval)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			w.runOnce(ctx)
		}
	}
}

func (w *IndexWorker) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	now := w.now()
	jobs, err := w.store.ClaimKnowledgeIndexJobs(ctx, now, w.limit, w.workerID)
	if err != nil {
		w.reportError(err)
		return
	}
	for _, job := range jobs {
		if knowledgeIndexJobUpdatesDocumentStatus(job) {
			if err := w.store.SetKnowledgeDocumentIndexStatus(ctx, job.OrganizationID, job.DocumentID, KnowledgeDocumentIndexStatusIndexing, "", nil); err != nil {
				w.reportError(err)
				continue
			}
		}
		if err := w.service.ProcessKnowledgeIndexJob(ctx, job); err != nil {
			w.recordFailedJob(ctx, job, err, now)
			continue
		}
		completedAt := w.now()
		if err := w.store.MarkKnowledgeIndexJobSucceeded(ctx, job.OrganizationID, job.ID, job.LockedBy, completedAt); err != nil {
			w.reportError(err)
			continue
		}
		if knowledgeIndexJobUpdatesDocumentStatus(job) {
			if err := w.store.SetKnowledgeDocumentIndexStatus(ctx, job.OrganizationID, job.DocumentID, KnowledgeDocumentIndexStatusReady, "", &completedAt); err != nil {
				w.reportError(err)
			}
		}
	}
}

func (w *IndexWorker) recordFailedJob(ctx context.Context, job KnowledgeIndexJob, indexErr error, now time.Time) {
	reason := strings.TrimSpace(indexErr.Error())
	if knowledgeIndexJobAttemptsExhausted(job) {
		deadLetterReason := "dead_letter: " + reason
		if markErr := w.store.MarkKnowledgeIndexJobDeadLetter(ctx, job.OrganizationID, job.ID, job.LockedBy, deadLetterReason, now); markErr != nil {
			w.reportError(markErr)
		}
		if knowledgeIndexJobUpdatesDocumentStatus(job) {
			if statusErr := w.store.SetKnowledgeDocumentIndexStatus(ctx, job.OrganizationID, job.DocumentID, KnowledgeDocumentIndexStatusFailed, deadLetterReason, nil); statusErr != nil {
				w.reportError(statusErr)
			}
		}
		return
	}
	nextAttemptAt := now.Add(knowledgeIndexRetryDelay(job.Attempts))
	if markErr := w.store.MarkKnowledgeIndexJobFailed(ctx, job.OrganizationID, job.ID, job.LockedBy, reason, nextAttemptAt); markErr != nil {
		w.reportError(markErr)
	}
	if knowledgeIndexJobUpdatesDocumentStatus(job) {
		if statusErr := w.store.SetKnowledgeDocumentIndexStatus(ctx, job.OrganizationID, job.DocumentID, KnowledgeDocumentIndexStatusFailed, reason, nil); statusErr != nil {
			w.reportError(statusErr)
		}
	}
}

func (w *IndexWorker) reportError(err error) {
	if err != nil && w.onError != nil {
		w.onError(err)
	}
}

func (s *Service) ProcessKnowledgeIndexJob(ctx context.Context, job KnowledgeIndexJob) error {
	if s == nil {
		return sql.ErrNoRows
	}
	if s.vectorStore == nil {
		return fmt.Errorf("knowledge vector store is not configured")
	}
	organizationID := strings.TrimSpace(job.OrganizationID)
	knowledgeBaseID := strings.TrimSpace(job.KnowledgeBaseID)
	documentID := strings.TrimSpace(job.DocumentID)
	if organizationID == "" || knowledgeBaseID == "" || documentID == "" {
		return fmt.Errorf("knowledge index job missing scope")
	}
	switch normalizeKnowledgeIndexOperation(job.Operation) {
	case KnowledgeIndexJobOperationDeleteDocument:
		return s.deleteDocumentVectors(ctx, organizationID, knowledgeBaseID, documentID)
	case KnowledgeIndexJobOperationUpsertDocument:
	default:
		return fmt.Errorf("unsupported knowledge index job operation: %s", job.Operation)
	}
	store, ok := s.store.(documentChunkLister)
	if !ok {
		return sql.ErrNoRows
	}
	views, err := store.ListKnowledgeDocumentChunks(ctx, organizationID, knowledgeBaseID, documentID)
	if err != nil {
		return err
	}
	return s.reindexDocumentChunkViewsDirect(ctx, auth.Session{OrganizationID: organizationID}, organizationID, knowledgeBaseID, documentID, views)
}

func (s *Service) reindexDocumentChunkViewsDirect(ctx context.Context, session auth.Session, organizationID, knowledgeBaseID, documentID string, views []KnowledgeDocumentChunkView) error {
	if s == nil || s.vectorStore == nil {
		return nil
	}
	if len(views) == 0 {
		return s.deleteDocumentVectors(ctx, organizationID, knowledgeBaseID, documentID)
	}
	if s.embedder == nil {
		return fmt.Errorf("knowledge embedder is not configured")
	}
	chunks, err := s.embeddedDocumentChunksFromViews(ctx, session, views)
	if err != nil {
		return err
	}
	if err := s.deleteDocumentVectors(ctx, organizationID, knowledgeBaseID, documentID); err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}
	return s.upsertDocumentChunksDirect(ctx, organizationID, knowledgeBaseID, documentID, chunks)
}

func (s *Service) embeddedDocumentChunksFromViews(ctx context.Context, session auth.Session, views []KnowledgeDocumentChunkView) ([]KnowledgeDocumentChunk, error) {
	chunks := make([]KnowledgeDocumentChunk, 0, len(views))
	texts := make([]string, 0, len(views))
	for _, view := range views {
		content := strings.TrimSpace(view.Content)
		if content == "" {
			continue
		}
		chunk := knowledgeDocumentChunkFromView(view)
		chunk.Content = content
		if chunk.EstimatedTokenCount == 0 {
			chunk.EstimatedTokenCount = estimateKnowledgeTokens(content)
		}
		chunks = append(chunks, chunk)
		texts = append(texts, content)
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	embeddings, err := s.embedder.EmbedBatch(withKnowledgeRelayIdentity(ctx, session), texts)
	if err != nil {
		return nil, err
	}
	for index := range chunks {
		if index < len(embeddings) {
			chunks[index].Embedding = append([]float32(nil), embeddings[index]...)
		}
	}
	return chunks, nil
}

func knowledgeIndexRetryDelay(attempts int) time.Duration {
	if attempts <= 0 {
		return time.Minute
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Duration(attempts) * time.Minute
}

func knowledgeIndexJobMaxAttempts(job KnowledgeIndexJob) int {
	if job.MaxAttempts <= 0 {
		return defaultKnowledgeIndexJobMaxAttempts
	}
	return job.MaxAttempts
}

func knowledgeIndexJobAttemptsExhausted(job KnowledgeIndexJob) bool {
	return job.Attempts >= knowledgeIndexJobMaxAttempts(job)
}

func defaultKnowledgeIndexWorkerID() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%s:%d", defaultKnowledgeIndexJobWorkerIDPrefix, hostname, os.Getpid())
}

func normalizeKnowledgeIndexOperation(operation string) string {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return KnowledgeIndexJobOperationUpsertDocument
	}
	if operation == KnowledgeIndexJobOperationDeleteDocument {
		return KnowledgeIndexJobOperationDeleteDocument
	}
	return operation
}

func knowledgeIndexJobUpdatesDocumentStatus(job KnowledgeIndexJob) bool {
	return normalizeKnowledgeIndexOperation(job.Operation) == KnowledgeIndexJobOperationUpsertDocument
}

func scanKnowledgeIndexJob(scanner interface{ Scan(dest ...any) error }) (KnowledgeIndexJob, error) {
	var job KnowledgeIndexJob
	var lockedAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&job.ID,
		&job.OrganizationID,
		&job.KnowledgeBaseID,
		&job.DocumentID,
		&job.Operation,
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
		return KnowledgeIndexJob{}, err
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = defaultKnowledgeIndexJobMaxAttempts
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

func scanKnowledgeIndexJobs(rows *sql.Rows) ([]KnowledgeIndexJob, error) {
	jobs := []KnowledgeIndexJob{}
	for rows.Next() {
		job, err := scanKnowledgeIndexJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
