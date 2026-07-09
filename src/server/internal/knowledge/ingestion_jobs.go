package knowledge

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

const (
	KnowledgeIngestionJobStatusPending    = KnowledgeIndexJobStatusPending
	KnowledgeIngestionJobStatusProcessing = KnowledgeIndexJobStatusProcessing
	KnowledgeIngestionJobStatusSucceeded  = KnowledgeIndexJobStatusSucceeded
	KnowledgeIngestionJobStatusFailed     = KnowledgeIndexJobStatusFailed
	KnowledgeIngestionJobStatusDeadLetter = KnowledgeIndexJobStatusDeadLetter
)

const (
	defaultKnowledgeIngestionWorkerInterval    = time.Minute
	defaultKnowledgeIngestionJobMaxAttempts    = 5
	defaultKnowledgeIngestionJobClaimLease     = 5 * time.Minute
	defaultKnowledgeIngestionJobWorkerIDPrefix = "rag-ingestion-worker"
	defaultKnowledgeIngestionRawParseMaxBytes  = 10 * 1024 * 1024
)

type KnowledgeIngestionRawPayload struct {
	Content     []byte
	Filename    string
	ContentType string
	SizeBytes   int64
}

type DocumentParser interface {
	Parse(ctx context.Context, reader io.Reader, filename, contentType string, maxBytes int64) (ParsedDocumentWithPages, error)
}

type CreateKnowledgeIngestionJobRequest struct {
	OrganizationID  string
	KnowledgeBaseID string
	Title           string
	Content         string
	RawContent      []byte
	RawFilename     string
	RawContentType  string
	RawSizeBytes    int64
	Options         KnowledgeDocumentOptions
	MaxAttempts     int
}

type KnowledgeIngestionJob struct {
	ID              string
	OrganizationID  string
	KnowledgeBaseID string
	DocumentID      string
	Title           string
	Content         string
	RawContent      []byte
	RawFilename     string
	RawContentType  string
	RawSizeBytes    int64
	Options         KnowledgeDocumentOptions
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

type knowledgeIngestionJobCreator interface {
	CreateKnowledgeIngestionJob(ctx context.Context, req CreateKnowledgeIngestionJobRequest) (KnowledgeIngestionJob, error)
}

type knowledgeIngestionJobLister interface {
	ListKnowledgeIngestionJobs(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeIngestionJob, error)
}

type KnowledgeIngestionWorkerStore interface {
	ClaimKnowledgeIngestionJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]KnowledgeIngestionJob, error)
	MarkKnowledgeIngestionJobDeadLetter(ctx context.Context, organizationID, jobID, lockedBy, reason string, completedAt time.Time) error
	MarkKnowledgeIngestionJobFailed(ctx context.Context, organizationID, jobID, lockedBy, reason string, availableAt time.Time) error
	MarkKnowledgeIngestionJobSucceeded(ctx context.Context, organizationID, jobID, lockedBy, documentID string, completedAt time.Time) error
}

type IngestionWorkerConfig struct {
	Interval time.Duration
	Limit    int
	Now      func() time.Time
	Ticks    <-chan time.Time
	WorkerID string
	OnError  func(error)
}

type IngestionWorker struct {
	service  *Service
	store    KnowledgeIngestionWorkerStore
	interval time.Duration
	limit    int
	now      func() time.Time
	ticks    <-chan time.Time
	workerID string
	onError  func(error)
}

func NewIngestionWorker(service *Service, store KnowledgeIngestionWorkerStore, config IngestionWorkerConfig) *IngestionWorker {
	interval := config.Interval
	if interval <= 0 {
		interval = defaultKnowledgeIngestionWorkerInterval
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
		workerID = defaultKnowledgeIngestionWorkerID()
	}
	return &IngestionWorker{
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

func (w *IngestionWorker) Run(ctx context.Context) {
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

func (w *IngestionWorker) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	now := w.now()
	jobs, err := w.store.ClaimKnowledgeIngestionJobs(ctx, now, w.limit, w.workerID)
	if err != nil {
		w.reportError(err)
		return
	}
	for _, job := range jobs {
		document, err := w.service.ProcessKnowledgeIngestionJob(ctx, job)
		if err != nil {
			failedAt := w.now()
			w.recordFailedJob(ctx, job, err, failedAt)
			continue
		}
		completedAt := w.now()
		if err := w.store.MarkKnowledgeIngestionJobSucceeded(ctx, job.OrganizationID, job.ID, job.LockedBy, document.ID, completedAt); err != nil {
			w.reportError(err)
		}
	}
}

func (w *IngestionWorker) recordFailedJob(ctx context.Context, job KnowledgeIngestionJob, ingestionErr error, now time.Time) {
	reason := strings.TrimSpace(ingestionErr.Error())
	if knowledgeIngestionJobAttemptsExhausted(job) {
		deadLetterReason := "dead_letter: " + reason
		if markErr := w.store.MarkKnowledgeIngestionJobDeadLetter(ctx, job.OrganizationID, job.ID, job.LockedBy, deadLetterReason, now); markErr != nil {
			w.reportError(markErr)
		}
		return
	}
	nextAttemptAt := now.Add(knowledgeIndexRetryDelay(job.Attempts))
	if markErr := w.store.MarkKnowledgeIngestionJobFailed(ctx, job.OrganizationID, job.ID, job.LockedBy, reason, nextAttemptAt); markErr != nil {
		w.reportError(markErr)
	}
}

func (w *IngestionWorker) reportError(err error) {
	if err != nil && w.onError != nil {
		w.onError(err)
	}
}

func (s *Service) WithDocumentParser(parser DocumentParser) *Service {
	if s == nil {
		return nil
	}
	s.documentParser = parser
	return s
}

func (s *Service) EnqueueDocumentIngestion(ctx context.Context, session auth.Session, knowledgeBaseID, title, content string, options KnowledgeDocumentOptions, rawPayload KnowledgeIngestionRawPayload) (KnowledgeIngestionJob, error) {
	if s == nil || s.store == nil {
		return KnowledgeIngestionJob{}, sql.ErrNoRows
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	rawContent := append([]byte(nil), rawPayload.Content...)
	rawFilename := strings.TrimSpace(rawPayload.Filename)
	rawContentType := strings.TrimSpace(rawPayload.ContentType)
	rawSizeBytes := rawPayload.SizeBytes
	if rawSizeBytes <= 0 {
		rawSizeBytes = int64(len(rawContent))
	}
	if title == "" || (content == "" && len(rawContent) == 0) {
		return KnowledgeIngestionJob{}, ErrEmptyKnowledgeDocument
	}
	store, ok := s.store.(knowledgeIngestionJobCreator)
	if !ok {
		return KnowledgeIngestionJob{}, sql.ErrNoRows
	}
	return store.CreateKnowledgeIngestionJob(ctx, CreateKnowledgeIngestionJobRequest{
		OrganizationID:  knowledgeSessionScope(session),
		KnowledgeBaseID: strings.TrimSpace(knowledgeBaseID),
		Title:           title,
		Content:         content,
		RawContent:      rawContent,
		RawFilename:     rawFilename,
		RawContentType:  rawContentType,
		RawSizeBytes:    rawSizeBytes,
		Options:         normalizeKnowledgeDocumentOptions(options),
	})
}

func (s *Service) ListDocumentIngestionJobs(ctx context.Context, session auth.Session, knowledgeBaseID string) ([]KnowledgeIngestionJob, error) {
	if s == nil || s.store == nil {
		return nil, sql.ErrNoRows
	}
	store, ok := s.store.(knowledgeIngestionJobLister)
	if !ok {
		return nil, sql.ErrNoRows
	}
	return store.ListKnowledgeIngestionJobs(ctx, knowledgeSessionScope(session), strings.TrimSpace(knowledgeBaseID))
}

func (s *Service) ProcessKnowledgeIngestionJob(ctx context.Context, job KnowledgeIngestionJob) (KnowledgeDocument, error) {
	if s == nil {
		return KnowledgeDocument{}, sql.ErrNoRows
	}
	organizationID := strings.TrimSpace(job.OrganizationID)
	knowledgeBaseID := strings.TrimSpace(job.KnowledgeBaseID)
	title := strings.TrimSpace(job.Title)
	content := strings.TrimSpace(job.Content)
	if len(job.RawContent) > 0 {
		if s.documentParser == nil {
			return KnowledgeDocument{}, fmt.Errorf("knowledge ingestion job raw payload requires document parser")
		}
		parsed, err := s.documentParser.Parse(ctx, bytes.NewReader(job.RawContent), strings.TrimSpace(job.RawFilename), strings.TrimSpace(job.RawContentType), defaultKnowledgeIngestionRawParseMaxBytes)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		if title == "" {
			title = strings.TrimSpace(parsed.Title)
		}
		content = strings.TrimSpace(parsed.Content)
	}
	if organizationID == "" || knowledgeBaseID == "" || title == "" || content == "" {
		return KnowledgeDocument{}, fmt.Errorf("knowledge ingestion job missing document payload")
	}
	return s.CreateDocumentWithOptions(ctx, auth.Session{OrganizationID: organizationID}, knowledgeBaseID, title, content, job.Options)
}

func knowledgeIngestionJobMaxAttempts(job KnowledgeIngestionJob) int {
	if job.MaxAttempts <= 0 {
		return defaultKnowledgeIngestionJobMaxAttempts
	}
	return job.MaxAttempts
}

func knowledgeIngestionJobAttemptsExhausted(job KnowledgeIngestionJob) bool {
	return job.Attempts >= knowledgeIngestionJobMaxAttempts(job)
}

func defaultKnowledgeIngestionWorkerID() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%s:%d", defaultKnowledgeIngestionJobWorkerIDPrefix, hostname, os.Getpid())
}
