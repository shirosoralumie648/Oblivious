package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

const (
	defaultBatchPollingWorkerInterval = time.Minute
	defaultBatchPollingWorkerLimit    = 10
	defaultBatchPollingWorkerIDPrefix = "relay-batch-polling-worker"
	defaultBatchPollingJobMaxAttempts = 5
)

type BatchPollingWorkerStore interface {
	ClaimBatchPollingJobs(ctx context.Context, now time.Time, limit int, workerID string) ([]RelayBatchPollingJob, error)
	MarkBatchPollingJobDeadLetter(ctx context.Context, batchID, lockedBy, reason string, completedAt time.Time) error
	MarkBatchPollingJobFailed(ctx context.Context, batchID, lockedBy, reason string, availableAt time.Time) error
	MarkBatchPollingJobSucceeded(ctx context.Context, batchID, lockedBy string, completedAt time.Time) error
}

type BatchStatusClient interface {
	RetrieveBatch(ctx context.Context, job RelayBatchPollingJob) (BatchStatusResult, error)
}

type BatchCompletionFinalizer interface {
	FinalizeCompletedBatch(ctx context.Context, job RelayBatchPollingJob, result BatchStatusResult) error
}

type BatchFailureFinalizer interface {
	FinalizeFailedBatch(ctx context.Context, job RelayBatchPollingJob, result BatchStatusResult) error
}

type BatchStatusResult struct {
	ID     string
	Status string
	Error  string
	Usage  *types.Usage
	Quote  *PricingQuote
}

type BatchUsageFinalizerConfig struct {
	PricingStore         *PricingStore
	QuotaManager         QuotaManager
	APITokenQuotaManager APITokenQuotaManager
	Now                  func() time.Time
}

type BatchUsageFinalizer struct {
	logger               RelayUsageReplacer
	pricing              *PricingStore
	quotaManager         QuotaManager
	apiTokenQuotaManager APITokenQuotaManager
	now                  func() time.Time
}

func NewBatchUsageFinalizer(logger RelayUsageReplacer, config BatchUsageFinalizerConfig) *BatchUsageFinalizer {
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &BatchUsageFinalizer{
		logger:               logger,
		pricing:              config.PricingStore,
		quotaManager:         config.QuotaManager,
		apiTokenQuotaManager: config.APITokenQuotaManager,
		now:                  now,
	}
}

func (f *BatchUsageFinalizer) FinalizeCompletedBatch(ctx context.Context, job RelayBatchPollingJob, result BatchStatusResult) error {
	if f == nil || f.logger == nil {
		return nil
	}
	requestID := strings.TrimSpace(job.RequestID)
	if requestID == "" {
		return fmt.Errorf("batch usage finalization missing request id")
	}
	apiType := strings.TrimSpace(job.APIType)
	if apiType == "" {
		apiType = types.APITypeBatch.String()
	}
	createdAt := f.now()
	record := RelayUsageLogRecord{
		UserID:         strings.TrimSpace(job.UserID),
		OrganizationID: strings.TrimSpace(job.OrganizationID),
		APITokenID:     strings.TrimSpace(job.APITokenID),
		RequestID:      requestID,
		APIType:        apiType,
		FeatureType:    strings.TrimSpace(job.FeatureType),
		Model:          strings.TrimSpace(job.Model),
		Status:         RelayUsageStatusSuccess,
		StatusCode:     http.StatusOK,
		CreatedAt:      createdAt,
	}
	if result.Usage != nil {
		record.PromptTokens = result.Usage.PromptTokens
		record.CompletionTokens = result.Usage.CompletionTokens
		record.TotalTokens = result.Usage.TotalTokens
	}
	quote := result.Quote
	if quote == nil && f.pricing != nil && result.Usage != nil {
		var err error
		quote, err = f.pricing.QuoteUsageForGroupStrict(record.Model, batchAPIType(apiType), result.Usage, "")
		if err != nil {
			return err
		}
	}
	if quote != nil {
		record.Cost = quote.TotalCost
		record.ChannelCost = batchChannelCost(quote)
		record.PriceSnapshot = quote
		record.PriceCurrency = quote.Currency
		record.PriceSource = quote.Source
		record.PriceEffectiveFrom = cloneTime(quote.EffectiveFrom)
	}
	if err := f.logger.ReplaceRelayUsage(ctx, record); err != nil {
		return err
	}
	actualCost := record.Cost
	if f.quotaManager != nil && strings.TrimSpace(job.BillingSessionID) != "" {
		if err := f.quotaManager.Settle(ctx, strings.TrimSpace(job.OrganizationID), strings.TrimSpace(job.BillingSessionID), actualCost); err != nil {
			return err
		}
	}
	if f.apiTokenQuotaManager != nil && strings.TrimSpace(job.APITokenID) != "" && job.TokenPreauthorizedAmount > 0 {
		if err := f.apiTokenQuotaManager.SettleRelayAPITokenQuota(ctx, strings.TrimSpace(job.APITokenID), job.TokenPreauthorizedAmount, actualCost); err != nil {
			return err
		}
	}
	return nil
}

func (f *BatchUsageFinalizer) FinalizeFailedBatch(ctx context.Context, job RelayBatchPollingJob, result BatchStatusResult) error {
	if f == nil {
		return nil
	}
	if f.logger != nil {
		requestID := strings.TrimSpace(job.RequestID)
		if requestID == "" {
			return fmt.Errorf("batch failure usage finalization missing request id")
		}
		apiType := strings.TrimSpace(job.APIType)
		if apiType == "" {
			apiType = types.APITypeBatch.String()
		}
		record := RelayUsageLogRecord{
			UserID:         strings.TrimSpace(job.UserID),
			OrganizationID: strings.TrimSpace(job.OrganizationID),
			APITokenID:     strings.TrimSpace(job.APITokenID),
			RequestID:      requestID,
			APIType:        apiType,
			FeatureType:    strings.TrimSpace(job.FeatureType),
			Model:          strings.TrimSpace(job.Model),
			Status:         RelayUsageStatusError,
			StatusCode:     http.StatusBadGateway,
			ErrorCode:      batchFailureUsageErrorCode(result.Status),
			CreatedAt:      f.now(),
		}
		if result.Usage != nil {
			record.PromptTokens = result.Usage.PromptTokens
			record.CompletionTokens = result.Usage.CompletionTokens
			record.TotalTokens = result.Usage.TotalTokens
		}
		if err := f.logger.ReplaceRelayUsage(ctx, record); err != nil {
			return err
		}
	}
	if f.quotaManager != nil && strings.TrimSpace(job.BillingSessionID) != "" {
		if err := f.quotaManager.Refund(ctx, strings.TrimSpace(job.OrganizationID), strings.TrimSpace(job.BillingSessionID)); err != nil {
			return err
		}
	}
	if f.apiTokenQuotaManager != nil && strings.TrimSpace(job.APITokenID) != "" && job.TokenPreauthorizedAmount > 0 {
		if err := f.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, strings.TrimSpace(job.APITokenID), job.TokenPreauthorizedAmount); err != nil {
			return err
		}
	}
	return nil
}

func batchFailureUsageErrorCode(status string) string {
	status = normalizeBatchPollingStatus(status)
	if status == "" {
		status = "unknown"
	}
	switch status {
	case "cancelled":
		status = "canceled"
	}
	return "batch_" + status
}

func batchAPIType(value string) types.APIType {
	switch strings.TrimSpace(value) {
	case types.APITypeChat.String():
		return types.APITypeChat
	case types.APITypeResponses.String():
		return types.APITypeResponses
	case types.APITypeRealtime.String():
		return types.APITypeRealtime
	case types.APITypeAssistants.String():
		return types.APITypeAssistants
	case types.APITypeThreads.String():
		return types.APITypeThreads
	case types.APITypeRuns.String():
		return types.APITypeRuns
	case types.APITypeBatch.String(), "":
		return types.APITypeBatch
	case types.APITypeBatchFiles.String():
		return types.APITypeBatchFiles
	case types.APITypeFineTuning.String():
		return types.APITypeFineTuning
	case types.APITypeFiles.String():
		return types.APITypeFiles
	case types.APITypeEmbeddings.String():
		return types.APITypeEmbeddings
	case types.APITypeImageGen.String():
		return types.APITypeImageGen
	case types.APITypeImageEdit.String():
		return types.APITypeImageEdit
	case types.APITypeImageVar.String():
		return types.APITypeImageVar
	case types.APITypeVideos.String():
		return types.APITypeVideos
	case types.APITypeAudioSpeech.String():
		return types.APITypeAudioSpeech
	case types.APITypeAudioSTT.String():
		return types.APITypeAudioSTT
	case types.APITypeAudioTranslate.String():
		return types.APITypeAudioTranslate
	case types.APITypeModeration.String():
		return types.APITypeModeration
	case types.APITypeCompletions.String():
		return types.APITypeCompletions
	case types.APITypeModels.String():
		return types.APITypeModels
	default:
		return types.APITypeBatch
	}
}

func batchChannelCost(quote *PricingQuote) float64 {
	if quote == nil || quote.TotalCost <= 0 {
		return 0
	}
	if quote.ChannelMultiplier <= 0 {
		return quote.TotalCost
	}
	return quote.TotalCost / quote.ChannelMultiplier
}

type OpenAIBatchStatusClient struct {
	adapter *channel.OpenAIAdapter
	client  *http.Client
}

func NewOpenAIBatchStatusClient(adapter *channel.OpenAIAdapter) *OpenAIBatchStatusClient {
	return &OpenAIBatchStatusClient{
		adapter: adapter,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *OpenAIBatchStatusClient) RetrieveBatch(ctx context.Context, job RelayBatchPollingJob) (BatchStatusResult, error) {
	if c == nil || c.adapter == nil {
		return BatchStatusResult{}, fmt.Errorf("batch status client is not configured")
	}
	batchID := strings.TrimSpace(job.BatchID)
	if batchID == "" {
		return BatchStatusResult{}, fmt.Errorf("batch status job missing batch id")
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		model = "gpt-4o"
	}
	upstreamURL, err := batchStatusURL(c.adapter, model, batchID)
	if err != nil {
		return BatchStatusResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return BatchStatusResult{}, err
	}
	headers, err := c.adapter.BuildHeaders(ctx, model, types.APITypeBatch)
	if err != nil {
		return BatchStatusResult{}, err
	}
	req.Header = headers

	httpClient := c.client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return BatchStatusResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BatchStatusResult{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return BatchStatusResult{}, fmt.Errorf("upstream batch status returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		ID     string       `json:"id"`
		Status string       `json:"status"`
		Error  any          `json:"error"`
		Usage  *types.Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return BatchStatusResult{}, err
	}
	return BatchStatusResult{
		ID:     strings.TrimSpace(payload.ID),
		Status: strings.TrimSpace(payload.Status),
		Error:  batchStatusErrorMessage(payload.Error),
		Usage:  payload.Usage,
	}, nil
}

type BatchPollingWorkerConfig struct {
	Interval            time.Duration
	Limit               int
	Now                 func() time.Time
	Ticks               <-chan time.Time
	WorkerID            string
	CompletionFinalizer BatchCompletionFinalizer
	FailureFinalizer    BatchFailureFinalizer
	OnError             func(error)
	Guard               releasecontract.Guard
	Authorities         releasecontract.RuntimeAuthorities
	Effects             releasecontract.EffectRegistrar
}

func batchStatusURL(adapter *channel.OpenAIAdapter, model, batchID string) (string, error) {
	batchURL, err := adapter.BuildURL(model, types.APITypeBatch)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(batchURL)
	if err != nil {
		return "", err
	}
	basePath := strings.TrimRight(strings.TrimSuffix(parsed.Path, "/batch"), "/")
	parsed.Path = basePath + "/batches/" + url.PathEscape(batchID)
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func batchStatusErrorMessage(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if message, _ := typed["message"].(string); strings.TrimSpace(message) != "" {
			return strings.TrimSpace(message)
		}
		if code, _ := typed["code"].(string); strings.TrimSpace(code) != "" {
			return strings.TrimSpace(code)
		}
	}
	marshaled, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(marshaled)
}

type BatchPollingWorker struct {
	store               BatchPollingWorkerStore
	client              BatchStatusClient
	completionFinalizer BatchCompletionFinalizer
	failureFinalizer    BatchFailureFinalizer
	interval            time.Duration
	limit               int
	now                 func() time.Time
	ticks               <-chan time.Time
	workerID            string
	onError             func(error)
	readiness           *batchPollingReadiness
}

func NewBatchPollingWorker(store BatchPollingWorkerStore, client BatchStatusClient, config BatchPollingWorkerConfig) *BatchPollingWorker {
	worker, _ := newBatchPollingWorker(store, client, config, false)
	return worker
}

func NewReadinessBatchPollingWorker(store BatchPollingWorkerStore, client BatchStatusClient, config BatchPollingWorkerConfig) (*BatchPollingWorker, error) {
	return newBatchPollingWorker(store, client, config, true)
}

func newBatchPollingWorker(store BatchPollingWorkerStore, client BatchStatusClient, config BatchPollingWorkerConfig, requireReadiness bool) (*BatchPollingWorker, error) {
	interval := config.Interval
	if interval <= 0 {
		interval = defaultBatchPollingWorkerInterval
	}
	limit := config.Limit
	if limit <= 0 {
		limit = defaultBatchPollingWorkerLimit
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	workerID := strings.TrimSpace(config.WorkerID)
	if workerID == "" {
		workerID = defaultBatchPollingWorkerID()
	}
	var readiness *batchPollingReadiness
	if requireReadiness {
		var err error
		readiness, err = newBatchPollingReadiness(config.Guard, config.Authorities, config.Effects)
		if err != nil {
			return nil, err
		}
	}
	return &BatchPollingWorker{
		store:               store,
		client:              client,
		completionFinalizer: config.CompletionFinalizer,
		failureFinalizer:    config.FailureFinalizer,
		interval:            interval,
		limit:               limit,
		now:                 now,
		ticks:               config.Ticks,
		workerID:            workerID,
		onError:             config.OnError,
		readiness:           readiness,
	}, nil
}

type batchPollingReadiness struct {
	guard              releasecontract.Guard
	claimCapability    releasecontract.CapabilityID
	providerCapability releasecontract.CapabilityID
	finalizeCapability releasecontract.CapabilityID
}

func newBatchPollingReadiness(guard releasecontract.Guard, authorities releasecontract.RuntimeAuthorities, effects releasecontract.EffectRegistrar) (*batchPollingReadiness, error) {
	if guard == nil || effects == nil || !authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "relay.batch.readiness"}
	}
	claim, err := authorities.CapabilityBindings.Resolve(releasecontract.EffectRelayBatchClaim)
	if err != nil {
		return nil, err
	}
	provider, err := authorities.CapabilityBindings.Resolve(releasecontract.EffectRelayBatchProvider)
	if err != nil {
		return nil, err
	}
	finalize, err := authorities.CapabilityBindings.Resolve(releasecontract.EffectRelayBatchFinalize)
	if err != nil {
		return nil, err
	}
	for _, descriptor := range []releasecontract.EffectDescriptor{
		{ID: "worker.relay_batch.claim", CapabilityID: string(claim), Boundary: releasecontract.BoundaryWorkerClaim, Owner: "relay.BatchPollingWorker"},
		{ID: "worker.relay_batch.retrieve", CapabilityID: string(provider), Boundary: releasecontract.BoundaryOutbound, Owner: "relay.BatchPollingWorker"},
		{ID: "worker.relay_batch.complete.finalize", CapabilityID: string(finalize), Boundary: releasecontract.BoundaryWorkerEffect, Owner: "relay.BatchPollingWorker"},
		{ID: "worker.relay_batch.failure.finalize", CapabilityID: string(finalize), Boundary: releasecontract.BoundaryWorkerEffect, Owner: "relay.BatchPollingWorker"},
		{ID: "worker.relay_batch.succeeded", CapabilityID: string(finalize), Boundary: releasecontract.BoundaryWorkerEffect, Owner: "relay.BatchPollingWorker"},
		{ID: "worker.relay_batch.dead_letter", CapabilityID: string(finalize), Boundary: releasecontract.BoundaryWorkerEffect, Owner: "relay.BatchPollingWorker"},
	} {
		if err := effects.Register(descriptor); err != nil {
			return nil, err
		}
	}
	return &batchPollingReadiness{guard: guard, claimCapability: claim, providerCapability: provider, finalizeCapability: finalize}, nil
}

func (r *batchPollingReadiness) requireClaim(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.guard.Require(ctx, string(r.claimCapability), releasecontract.BoundaryWorkerClaim)
}

func (r *batchPollingReadiness) requireProvider(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.guard.Require(ctx, string(r.providerCapability), releasecontract.BoundaryOutbound)
}

func (r *batchPollingReadiness) requireFinalizer(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.guard.Require(ctx, string(r.finalizeCapability), releasecontract.BoundaryWorkerEffect)
}

func (r *batchPollingReadiness) requireTerminal(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.guard.Require(ctx, string(r.finalizeCapability), releasecontract.BoundaryWorkerEffect)
}

func (w *BatchPollingWorker) Run(ctx context.Context) {
	if w == nil || w.store == nil || w.client == nil {
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

func (w *BatchPollingWorker) runOnce(ctx context.Context) {
	if w == nil || w.store == nil || w.client == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	now := w.now()
	if err := w.readiness.requireClaim(ctx); err != nil {
		w.reportError(err)
		return
	}
	jobs, err := w.store.ClaimBatchPollingJobs(ctx, now, w.limit, w.workerID)
	if err != nil {
		w.reportError(err)
		return
	}
	for _, job := range jobs {
		if err := w.readiness.requireProvider(ctx); err != nil {
			w.recordReadinessDeniedJob(ctx, job, err, w.now())
			continue
		}
		result, err := w.client.RetrieveBatch(ctx, job)
		if err != nil {
			w.recordFailedJob(ctx, job, err, w.now())
			continue
		}
		w.recordBatchStatus(ctx, job, result, w.now())
	}
}

func (w *BatchPollingWorker) recordBatchStatus(ctx context.Context, job RelayBatchPollingJob, result BatchStatusResult, now time.Time) {
	status := normalizeBatchPollingStatus(result.Status)
	switch {
	case batchPollingStatusSucceeded(status):
		if w.completionFinalizer != nil {
			if err := w.readiness.requireFinalizer(ctx); err != nil {
				w.recordReadinessDeniedJob(ctx, job, err, now)
				return
			}
			if err := w.completionFinalizer.FinalizeCompletedBatch(ctx, job, result); err != nil {
				w.recordFailedJob(ctx, job, err, now)
				return
			}
		}
		if err := w.readiness.requireTerminal(ctx); err != nil {
			w.recordReadinessDeniedJob(ctx, job, err, now)
			return
		}
		if err := w.store.MarkBatchPollingJobSucceeded(ctx, job.BatchID, job.LockedBy, now); err != nil {
			w.reportError(err)
		}
	case batchPollingStatusTerminalFailure(status):
		if w.failureFinalizer != nil {
			if err := w.readiness.requireFinalizer(ctx); err != nil {
				w.recordReadinessDeniedJob(ctx, job, err, now)
				return
			}
			if err := w.failureFinalizer.FinalizeFailedBatch(ctx, job, result); err != nil {
				w.recordFailedJob(ctx, job, err, now)
				return
			}
		}
		if err := w.readiness.requireTerminal(ctx); err != nil {
			w.recordReadinessDeniedJob(ctx, job, err, now)
			return
		}
		w.recordTerminalFailedJob(ctx, job, batchPollingStatusReason(status, result.Error), now)
	default:
		w.recordFailedJob(ctx, job, fmt.Errorf("%s", batchPollingStatusReason(status, result.Error)), now)
	}
}

func (w *BatchPollingWorker) recordReadinessDeniedJob(ctx context.Context, job RelayBatchPollingJob, denial error, now time.Time) {
	reason := batchReadinessDenialReason(denial)
	if reason == "" {
		reason = string(releasecontract.CodeReadinessUnavailable)
	}
	nextAttemptAt := now.Add(batchPollingRetryDelay(job.Attempts))
	if err := w.store.MarkBatchPollingJobFailed(ctx, job.BatchID, job.LockedBy, reason, nextAttemptAt); err != nil {
		w.reportError(err)
	}
}

func batchReadinessDenialReason(denial error) string {
	var readinessErr *releasecontract.ReadinessError
	if errors.As(denial, &readinessErr) && readinessErr != nil && readinessErr.Code != "" {
		return string(readinessErr.Code)
	}
	if denial == nil {
		return ""
	}
	return strings.TrimSpace(denial.Error())
}

func (w *BatchPollingWorker) recordTerminalFailedJob(ctx context.Context, job RelayBatchPollingJob, reason string, now time.Time) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "upstream batch status terminal failure"
	}
	if err := w.store.MarkBatchPollingJobDeadLetter(ctx, job.BatchID, job.LockedBy, "dead_letter: "+reason, now); err != nil {
		w.reportError(err)
	}
}

func (w *BatchPollingWorker) recordFailedJob(ctx context.Context, job RelayBatchPollingJob, pollingErr error, now time.Time) {
	reason := strings.TrimSpace(pollingErr.Error())
	if reason == "" {
		reason = "upstream batch status unknown"
	}
	if batchPollingJobAttemptsExhausted(job) {
		if err := w.readiness.requireTerminal(ctx); err != nil {
			w.recordReadinessDeniedJob(ctx, job, err, now)
			return
		}
		deadLetterReason := "dead_letter: " + reason
		if err := w.store.MarkBatchPollingJobDeadLetter(ctx, job.BatchID, job.LockedBy, deadLetterReason, now); err != nil {
			w.reportError(err)
		}
		return
	}
	nextAttemptAt := now.Add(batchPollingRetryDelay(job.Attempts))
	if err := w.store.MarkBatchPollingJobFailed(ctx, job.BatchID, job.LockedBy, reason, nextAttemptAt); err != nil {
		w.reportError(err)
	}
}

func (w *BatchPollingWorker) reportError(err error) {
	if err != nil && w.onError != nil {
		w.onError(err)
	}
}

func normalizeBatchPollingStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func batchPollingStatusSucceeded(status string) bool {
	return status == "completed"
}

func batchPollingStatusTerminalFailure(status string) bool {
	switch status {
	case "failed", "expired", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func batchPollingStatusReason(status, detail string) string {
	status = normalizeBatchPollingStatus(status)
	if status == "" {
		status = "unknown"
	}
	reason := "upstream batch status " + status
	detail = strings.TrimSpace(detail)
	if detail != "" {
		reason += ": " + detail
	}
	return reason
}

func batchPollingRetryDelay(attempts int) time.Duration {
	if attempts <= 0 {
		return time.Minute
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Duration(attempts) * time.Minute
}

func batchPollingJobMaxAttempts(job RelayBatchPollingJob) int {
	if job.MaxAttempts <= 0 {
		return defaultBatchPollingJobMaxAttempts
	}
	return job.MaxAttempts
}

func batchPollingJobAttemptsExhausted(job RelayBatchPollingJob) bool {
	return job.Attempts >= batchPollingJobMaxAttempts(job)
}

func defaultBatchPollingWorkerID() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%s:%d", defaultBatchPollingWorkerIDPrefix, hostname, os.Getpid())
}
