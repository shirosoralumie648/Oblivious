package relay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

func TestBatchPollingReadinessGuardContract(t *testing.T) {
	now := time.Date(2026, time.July, 18, 3, 0, 0, 0, time.UTC)
	denialCodes := []releasecontract.ReadinessCode{
		releasecontract.CodeCapabilityDisabled,
		releasecontract.CodeCapabilityBlocked,
		releasecontract.CodeReadinessStale,
		releasecontract.CodeCapabilityUnknown,
		releasecontract.CodeReadinessUnavailable,
		releasecontract.CodeBuildIdentityMismatch,
	}
	for _, code := range denialCodes {
		t.Run("claim denial "+string(code), func(t *testing.T) {
			store, client := batchReadinessFixture("completed")
			completion := &recordingBatchCompletionFinalizer{}
			failure := &recordingBatchFailureFinalizer{}
			guard := &batchGuardSpy{denyAtCall: 1, denial: &releasecontract.ReadinessError{Code: code}}
			worker := newBatchReadinessWorker(t, store, client, completion, failure, guard, now)

			worker.runOnce(context.Background())

			if store.claimCalls != 0 || len(client.seen) != 0 || len(completion.calls) != 0 || len(failure.calls) != 0 || store.transitionCalls() != 0 {
				t.Fatalf("denied claim reached effects: claim=%d retrieve=%d completeFinalize=%d failureFinalize=%d transitions=%d", store.claimCalls, len(client.seen), len(completion.calls), len(failure.calls), store.transitionCalls())
			}
		})
	}

	t.Run("nil guard fails before worker construction", func(t *testing.T) {
		contract, profile := loadBatchReadinessAuthority(t)
		authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, &batchGuardSpy{})
		if err != nil {
			t.Fatalf("compile runtime authorities: %v", err)
		}
		_, err = NewReadinessBatchPollingWorker(&recordingBatchPollingWorkerStore{}, &recordingBatchStatusClient{}, BatchPollingWorkerConfig{
			Guard: nil, Authorities: authorities, Effects: &batchEffectRegistrar{},
		})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("nil guard error = %v", err)
		}
	})

	t.Run("expiry after claim blocks provider retrieval and records retry only", func(t *testing.T) {
		store, client := batchReadinessFixture("completed")
		completion := &recordingBatchCompletionFinalizer{}
		failure := &recordingBatchFailureFinalizer{}
		guard := &batchGuardSpy{denyAtCall: 2, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "manager"}}
		worker := newBatchReadinessWorker(t, store, client, completion, failure, guard, now)

		worker.runOnce(context.Background())

		if store.claimCalls != 1 || len(client.seen) != 0 || len(completion.calls) != 0 || len(failure.calls) != 0 {
			t.Fatalf("provider denial reached effects: claim=%d retrieve=%d completion=%d failure=%d", store.claimCalls, len(client.seen), len(completion.calls), len(failure.calls))
		}
		if store.failedCalls != 1 || store.succeededCalls != 0 || store.deadLetterCalls != 0 || store.failedReason != string(releasecontract.CodeReadinessStale) {
			t.Fatalf("expected stable retry only: failed=%d succeeded=%d dead=%d reason=%q", store.failedCalls, store.succeededCalls, store.deadLetterCalls, store.failedReason)
		}
	})

	t.Run("expiry after retrieval blocks completion finalizer", func(t *testing.T) {
		store, client := batchReadinessFixture("completed")
		completion := &recordingBatchCompletionFinalizer{}
		guard := &batchGuardSpy{denyAtCall: 3, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newBatchReadinessWorker(t, store, client, completion, &recordingBatchFailureFinalizer{}, guard, now)

		worker.runOnce(context.Background())

		if len(client.seen) != 1 || len(completion.calls) != 0 || store.failedCalls != 1 || store.succeededCalls != 0 || store.deadLetterCalls != 0 {
			t.Fatalf("completion finalizer denial counts: retrieve=%d finalize=%d failed=%d succeeded=%d dead=%d", len(client.seen), len(completion.calls), store.failedCalls, store.succeededCalls, store.deadLetterCalls)
		}
	})

	t.Run("expiry after completion finalizer blocks succeeded transition", func(t *testing.T) {
		store, client := batchReadinessFixture("completed")
		completion := &recordingBatchCompletionFinalizer{}
		guard := &batchGuardSpy{denyAtCall: 4, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newBatchReadinessWorker(t, store, client, completion, &recordingBatchFailureFinalizer{}, guard, now)

		worker.runOnce(context.Background())

		if len(completion.calls) != 1 || store.failedCalls != 1 || store.succeededCalls != 0 || store.deadLetterCalls != 0 {
			t.Fatalf("succeeded transition reused finalizer authorization: finalize=%d failed=%d succeeded=%d dead=%d", len(completion.calls), store.failedCalls, store.succeededCalls, store.deadLetterCalls)
		}
	})

	t.Run("expiry after retrieval blocks failure finalizer and refund", func(t *testing.T) {
		store, client := batchReadinessFixture("failed")
		failure := &recordingBatchFailureFinalizer{}
		guard := &batchGuardSpy{denyAtCall: 3, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newBatchReadinessWorker(t, store, client, &recordingBatchCompletionFinalizer{}, failure, guard, now)

		worker.runOnce(context.Background())

		if len(client.seen) != 1 || len(failure.calls) != 0 || store.failedCalls != 1 || store.deadLetterCalls != 0 || store.succeededCalls != 0 {
			t.Fatalf("failure finalizer denial counts: retrieve=%d finalize=%d failed=%d dead=%d succeeded=%d", len(client.seen), len(failure.calls), store.failedCalls, store.deadLetterCalls, store.succeededCalls)
		}
	})

	t.Run("expiry after failure finalizer blocks dead letter transition", func(t *testing.T) {
		store, client := batchReadinessFixture("failed")
		failure := &recordingBatchFailureFinalizer{}
		guard := &batchGuardSpy{denyAtCall: 4, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newBatchReadinessWorker(t, store, client, &recordingBatchCompletionFinalizer{}, failure, guard, now)

		worker.runOnce(context.Background())

		if len(failure.calls) != 1 || store.failedCalls != 1 || store.deadLetterCalls != 0 || store.succeededCalls != 0 {
			t.Fatalf("dead-letter transition reused finalizer authorization: finalize=%d failed=%d dead=%d succeeded=%d", len(failure.calls), store.failedCalls, store.deadLetterCalls, store.succeededCalls)
		}
	})

	t.Run("authorizing guard preserves provider finalizer and succeeded flow", func(t *testing.T) {
		store, client := batchReadinessFixture("completed")
		completion := &recordingBatchCompletionFinalizer{}
		guard := &batchGuardSpy{}
		registrar := &batchEffectRegistrar{}
		worker := newBatchReadinessWorkerWithRegistrar(t, store, client, completion, &recordingBatchFailureFinalizer{}, guard, registrar, now)

		worker.runOnce(context.Background())

		if store.claimCalls != 1 || len(client.seen) != 1 || len(completion.calls) != 1 || store.succeededCalls != 1 || store.failedCalls != 0 || store.deadLetterCalls != 0 {
			t.Fatalf("authorized flow changed: claim=%d retrieve=%d finalize=%d succeeded=%d failed=%d dead=%d", store.claimCalls, len(client.seen), len(completion.calls), store.succeededCalls, store.failedCalls, store.deadLetterCalls)
		}
		if len(registrar.descriptors) != 6 || len(guard.calls) != 4 {
			t.Fatalf("descriptor or guard coverage missing: descriptors=%#v calls=%#v", registrar.descriptors, guard.calls)
		}
	})
}

func TestBatchPollingWorkerMarksCompletedBatchSucceeded(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:     "batch_completed",
			RequestID:   "req_completed",
			Model:       "gpt-4o-mini",
			APIType:     "batch",
			Attempts:    1,
			MaxAttempts: 5,
			LockedBy:    "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_completed": {ID: "batch_completed", Status: "completed"},
		},
	}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:      func() time.Time { return now },
		WorkerID: "worker-1",
		Limit:    10,
	})

	worker.runOnce(context.Background())

	if store.claimWorkerID != "worker-1" {
		t.Fatalf("claim worker id = %q, want worker-1", store.claimWorkerID)
	}
	if len(client.seen) != 1 || client.seen[0].BatchID != "batch_completed" {
		t.Fatalf("client saw jobs %+v, want batch_completed", client.seen)
	}
	if store.succeededBatchID != "batch_completed" || store.succeededLockedBy != "worker-1" || !store.succeededAt.Equal(now) {
		t.Fatalf("succeeded batch=%q lockedBy=%q at=%s", store.succeededBatchID, store.succeededLockedBy, store.succeededAt)
	}
	if store.failedBatchID != "" || store.deadLetterBatchID != "" {
		t.Fatalf("completed batch should not fail/dead-letter, failed=%q dead=%q", store.failedBatchID, store.deadLetterBatchID)
	}
}

func TestBatchPollingWorkerFinalizesCompletedBatchBeforeMarkingSucceeded(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:        "batch_completed",
			RequestID:      "req_completed",
			UserID:         "user_batch",
			OrganizationID: "org_batch",
			APITokenID:     "tok_batch",
			FeatureType:    "workflow",
			Model:          "gpt-4o-mini",
			APIType:        "batch",
			Attempts:       1,
			MaxAttempts:    5,
			LockedBy:       "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_completed": {ID: "batch_completed", Status: "completed"},
		},
	}
	finalizer := &recordingBatchCompletionFinalizer{}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:                 func() time.Time { return now },
		WorkerID:            "worker-1",
		CompletionFinalizer: finalizer,
	})

	worker.runOnce(context.Background())

	if len(finalizer.calls) != 1 {
		t.Fatalf("expected one finalizer call, got %+v", finalizer.calls)
	}
	if finalizer.calls[0].Job.BatchID != "batch_completed" ||
		finalizer.calls[0].Job.UserID != "user_batch" ||
		finalizer.calls[0].Job.OrganizationID != "org_batch" ||
		finalizer.calls[0].Job.APITokenID != "tok_batch" ||
		finalizer.calls[0].Job.FeatureType != "workflow" {
		t.Fatalf("finalizer job context not preserved: %+v", finalizer.calls[0].Job)
	}
	if store.succeededBatchID != "batch_completed" || store.failedBatchID != "" || store.deadLetterBatchID != "" {
		t.Fatalf("expected completed batch to succeed after finalizer, succeeded=%q failed=%q dead=%q", store.succeededBatchID, store.failedBatchID, store.deadLetterBatchID)
	}
	if finalizer.calls[0].SucceededBeforeFinalizer {
		t.Fatal("batch should not be marked succeeded before finalizer runs")
	}
}

func TestBatchPollingWorkerRetriesCompletedBatchWhenFinalizerFails(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:     "batch_completed",
			RequestID:   "req_completed",
			Model:       "gpt-4o-mini",
			APIType:     "batch",
			Attempts:    2,
			MaxAttempts: 5,
			LockedBy:    "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_completed": {ID: "batch_completed", Status: "completed"},
		},
	}
	finalizer := &recordingBatchCompletionFinalizer{err: errors.New("usage finalization unavailable")}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:                 func() time.Time { return now },
		WorkerID:            "worker-1",
		CompletionFinalizer: finalizer,
	})

	worker.runOnce(context.Background())

	if store.succeededBatchID != "" {
		t.Fatalf("batch should not be marked succeeded when finalizer fails, succeeded=%q", store.succeededBatchID)
	}
	if store.failedBatchID != "batch_completed" || !strings.Contains(store.failedReason, "usage finalization unavailable") {
		t.Fatalf("expected retry failure from finalizer, failed=%q reason=%q", store.failedBatchID, store.failedReason)
	}
	if want := now.Add(2 * time.Minute); !store.failedAvailableAt.Equal(want) {
		t.Fatalf("next retry at %s, want %s", store.failedAvailableAt, want)
	}
}

func TestBatchPollingWorkerRetriesInProgressBatch(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:     "batch_in_progress",
			RequestID:   "req_in_progress",
			Model:       "gpt-4o-mini",
			APIType:     "batch",
			Attempts:    2,
			MaxAttempts: 5,
			LockedBy:    "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_in_progress": {ID: "batch_in_progress", Status: "in_progress"},
		},
	}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:      func() time.Time { return now },
		WorkerID: "worker-1",
	})

	worker.runOnce(context.Background())

	if store.failedBatchID != "batch_in_progress" || store.failedLockedBy != "worker-1" {
		t.Fatalf("failed batch=%q lockedBy=%q", store.failedBatchID, store.failedLockedBy)
	}
	if !strings.Contains(store.failedReason, "upstream batch status in_progress") {
		t.Fatalf("failed reason = %q, want upstream status", store.failedReason)
	}
	if want := now.Add(2 * time.Minute); !store.failedAvailableAt.Equal(want) {
		t.Fatalf("next retry at %s, want %s", store.failedAvailableAt, want)
	}
	if store.succeededBatchID != "" || store.deadLetterBatchID != "" {
		t.Fatalf("in-progress batch should only retry, succeeded=%q dead=%q", store.succeededBatchID, store.deadLetterBatchID)
	}
}

func TestBatchPollingWorkerDeadLettersTerminalFailureAfterMaxAttempts(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:     "batch_failed",
			RequestID:   "req_failed",
			Model:       "gpt-4o-mini",
			APIType:     "batch",
			Attempts:    5,
			MaxAttempts: 5,
			LockedBy:    "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_failed": {ID: "batch_failed", Status: "failed", Error: "validation failed"},
		},
	}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:      func() time.Time { return now },
		WorkerID: "worker-1",
	})

	worker.runOnce(context.Background())

	if store.deadLetterBatchID != "batch_failed" || store.deadLetterLockedBy != "worker-1" || !store.deadLetterAt.Equal(now) {
		t.Fatalf("dead-letter batch=%q lockedBy=%q at=%s", store.deadLetterBatchID, store.deadLetterLockedBy, store.deadLetterAt)
	}
	if !strings.Contains(store.deadLetterReason, "dead_letter: upstream batch status failed") ||
		!strings.Contains(store.deadLetterReason, "validation failed") {
		t.Fatalf("dead-letter reason = %q", store.deadLetterReason)
	}
	if store.succeededBatchID != "" || store.failedBatchID != "" {
		t.Fatalf("terminal failure after max attempts should only dead-letter, succeeded=%q failed=%q", store.succeededBatchID, store.failedBatchID)
	}
}

func TestBatchPollingWorkerDeadLettersTerminalFailureImmediately(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:     "batch_cancelled",
			RequestID:   "req_cancelled",
			Model:       "gpt-4o-mini",
			APIType:     "batch",
			Attempts:    1,
			MaxAttempts: 5,
			LockedBy:    "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_cancelled": {ID: "batch_cancelled", Status: "cancelled", Error: "user cancelled batch"},
		},
	}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:      func() time.Time { return now },
		WorkerID: "worker-1",
	})

	worker.runOnce(context.Background())

	if store.deadLetterBatchID != "batch_cancelled" || store.deadLetterLockedBy != "worker-1" || !store.deadLetterAt.Equal(now) {
		t.Fatalf("dead-letter batch=%q lockedBy=%q at=%s", store.deadLetterBatchID, store.deadLetterLockedBy, store.deadLetterAt)
	}
	if !strings.Contains(store.deadLetterReason, "dead_letter: upstream batch status cancelled") ||
		!strings.Contains(store.deadLetterReason, "user cancelled batch") {
		t.Fatalf("dead-letter reason = %q", store.deadLetterReason)
	}
	if store.failedBatchID != "" || store.succeededBatchID != "" {
		t.Fatalf("terminal failure should not retry, failed=%q succeeded=%q", store.failedBatchID, store.succeededBatchID)
	}
}

func TestBatchPollingWorkerRefundsTerminalFailureBeforeDeadLetter(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:                  "batch_failed_refund",
			RequestID:                "req_failed_refund",
			OrganizationID:           "org_batch",
			APITokenID:               "tok_batch",
			Model:                    "gpt-4o-mini",
			APIType:                  "batch",
			BillingSessionID:         "bill_batch_refund",
			TokenPreauthorizedAmount: 1.5,
			Attempts:                 1,
			MaxAttempts:              5,
			LockedBy:                 "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_failed_refund": {ID: "batch_failed_refund", Status: "failed", Error: "validation failed"},
		},
	}
	refunder := &recordingBatchFailureFinalizer{}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:              func() time.Time { return now },
		WorkerID:         "worker-1",
		FailureFinalizer: refunder,
	})

	worker.runOnce(context.Background())

	if len(refunder.calls) != 1 {
		t.Fatalf("expected one refund finalizer call, got %+v", refunder.calls)
	}
	call := refunder.calls[0]
	if call.Job.BatchID != "batch_failed_refund" ||
		call.Job.BillingSessionID != "bill_batch_refund" ||
		call.Job.TokenPreauthorizedAmount != 1.5 ||
		call.Result.Status != "failed" {
		t.Fatalf("refund finalizer context mismatch: %+v", call)
	}
	if store.deadLetterBatchID != "batch_failed_refund" || store.failedBatchID != "" {
		t.Fatalf("expected terminal failure to dead-letter after refund, dead=%q failed=%q", store.deadLetterBatchID, store.failedBatchID)
	}
}

func TestBatchPollingWorkerRetriesTerminalFailureWhenRefundFails(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	store := &recordingBatchPollingWorkerStore{
		jobs: []RelayBatchPollingJob{{
			BatchID:          "batch_failed_refund_retry",
			RequestID:        "req_failed_refund_retry",
			OrganizationID:   "org_batch",
			BillingSessionID: "bill_batch_refund_retry",
			Model:            "gpt-4o-mini",
			APIType:          "batch",
			Attempts:         2,
			MaxAttempts:      5,
			LockedBy:         "worker-1",
		}},
	}
	client := &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{
			"batch_failed_refund_retry": {ID: "batch_failed_refund_retry", Status: "failed", Error: "validation failed"},
		},
	}
	refunder := &recordingBatchFailureFinalizer{err: errors.New("refund unavailable")}
	worker := NewBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now:              func() time.Time { return now },
		WorkerID:         "worker-1",
		FailureFinalizer: refunder,
	})

	worker.runOnce(context.Background())

	if store.deadLetterBatchID != "" {
		t.Fatalf("terminal failure should not dead-letter when refund fails, dead=%q", store.deadLetterBatchID)
	}
	if store.failedBatchID != "batch_failed_refund_retry" || !strings.Contains(store.failedReason, "refund unavailable") {
		t.Fatalf("expected retry after refund failure, failed=%q reason=%q", store.failedBatchID, store.failedReason)
	}
	if want := now.Add(2 * time.Minute); !store.failedAvailableAt.Equal(want) {
		t.Fatalf("next retry at %s, want %s", store.failedAvailableAt, want)
	}
}

func TestOpenAIBatchStatusClientRetrievesBatchStatus(t *testing.T) {
	var seenPath string
	var seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"batch_http","status":"completed"}`))
	}))
	t.Cleanup(upstream.Close)

	client := NewOpenAIBatchStatusClient(channel.NewOpenAIAdapter(upstream.URL, "sk-status"))
	result, err := client.RetrieveBatch(context.Background(), RelayBatchPollingJob{
		BatchID: "batch_http",
		Model:   "gpt-4o-mini",
		APIType: "batch",
	})
	if err != nil {
		t.Fatalf("RetrieveBatch returned error: %v", err)
	}

	if seenPath != "/v1/batches/batch_http" {
		t.Fatalf("upstream path = %q, want /v1/batches/batch_http", seenPath)
	}
	if seenAuth != "Bearer sk-status" {
		t.Fatalf("authorization = %q, want Bearer sk-status", seenAuth)
	}
	if result.ID != "batch_http" || result.Status != "completed" {
		t.Fatalf("result = %+v, want completed batch_http", result)
	}
}

func TestBatchUsageFinalizerReplacesUsageRecordFromJobContext(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		Now: func() time.Time { return time.Date(2026, 7, 5, 10, 15, 0, 0, time.UTC) },
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:        "batch_usage",
		RequestID:      "req_batch_usage",
		UserID:         "user_batch",
		OrganizationID: "org_batch",
		APITokenID:     "tok_batch",
		FeatureType:    "workflow",
		Model:          "gpt-4o-mini",
		APIType:        "batch",
	}, BatchStatusResult{
		ID:     "batch_usage",
		Status: "completed",
		Usage:  &types.Usage{PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100},
	})
	if err != nil {
		t.Fatalf("FinalizeCompletedBatch returned error: %v", err)
	}
	if len(logger.replaced) != 1 {
		t.Fatalf("expected one replaced usage record, got %+v", logger.replaced)
	}
	record := logger.replaced[0]
	if record.RequestID != "req_batch_usage" ||
		record.UserID != "user_batch" ||
		record.OrganizationID != "org_batch" ||
		record.APITokenID != "tok_batch" ||
		record.FeatureType != "workflow" ||
		record.APIType != "batch" ||
		record.Model != "gpt-4o-mini" ||
		record.Status != RelayUsageStatusSuccess ||
		record.StatusCode != http.StatusOK {
		t.Fatalf("unexpected finalized usage record: %+v", record)
	}
	if record.PromptTokens != 40 || record.CompletionTokens != 60 || record.TotalTokens != 100 {
		t.Fatalf("usage tokens not preserved: %+v", record)
	}
}

func TestBatchUsageFinalizerRecordsFailureUsageAuditBeforeRefund(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	quotaManager := &recordingBatchQuotaManager{}
	apiTokenQuota := &recordingBatchAPITokenQuotaManager{}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		QuotaManager:         quotaManager,
		APITokenQuotaManager: apiTokenQuota,
		Now:                  func() time.Time { return time.Date(2026, 7, 5, 10, 20, 0, 0, time.UTC) },
	})

	err := finalizer.FinalizeFailedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:                  "batch_failed_usage",
		RequestID:                "req_batch_failed_usage",
		UserID:                   "user_batch",
		OrganizationID:           "org_batch",
		APITokenID:               "tok_batch",
		FeatureType:              "workflow",
		Model:                    "gpt-4o-mini",
		APIType:                  "batch",
		BillingSessionID:         "bill_batch_failed_usage",
		TokenPreauthorizedAmount: 1.5,
	}, BatchStatusResult{
		ID:     "batch_failed_usage",
		Status: "failed",
		Error:  "validation failed",
	})
	if err != nil {
		t.Fatalf("FinalizeFailedBatch returned error: %v", err)
	}
	if len(logger.replaced) != 1 {
		t.Fatalf("expected one failure usage audit record, got %+v", logger.replaced)
	}
	record := logger.replaced[0]
	if record.RequestID != "req_batch_failed_usage" ||
		record.UserID != "user_batch" ||
		record.OrganizationID != "org_batch" ||
		record.APITokenID != "tok_batch" ||
		record.FeatureType != "workflow" ||
		record.APIType != "batch" ||
		record.Model != "gpt-4o-mini" ||
		record.Status != RelayUsageStatusError ||
		record.StatusCode != http.StatusBadGateway ||
		record.ErrorCode != "batch_failed" {
		t.Fatalf("unexpected failure usage audit record: %+v", record)
	}
	if quotaManager.refundCalls != 1 ||
		quotaManager.refundedOrganizationID != "org_batch" ||
		quotaManager.refundedSessionID != "bill_batch_failed_usage" {
		t.Fatalf("expected durable quota refund after audit record, got %+v", quotaManager)
	}
	if apiTokenQuota.refundCalls != 1 ||
		apiTokenQuota.refundedTokenID != "tok_batch" ||
		apiTokenQuota.refundedAmount != 1.5 {
		t.Fatalf("expected API token quota refund after audit record, got %+v", apiTokenQuota)
	}
}

func TestBatchUsageFinalizerRecordsPriceSnapshotAndCost(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	effectiveFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	quote := &PricingQuote{
		Model:             "gpt-4o-mini",
		APIType:           "batch",
		Currency:          "quota",
		Source:            "catalog_import_1",
		EffectiveFrom:     &effectiveFrom,
		Subtotal:          0.25,
		GroupMultiplier:   1.2,
		ChannelMultiplier: 1.1,
		TotalCost:         0.33,
	}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		Now: func() time.Time { return time.Date(2026, 7, 5, 10, 15, 0, 0, time.UTC) },
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:        "batch_priced",
		RequestID:      "req_batch_priced",
		UserID:         "user_batch",
		OrganizationID: "org_batch",
		APITokenID:     "tok_batch",
		Model:          "gpt-4o-mini",
		APIType:        "batch",
	}, BatchStatusResult{
		ID:     "batch_priced",
		Status: "completed",
		Usage:  &types.Usage{PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100},
		Quote:  quote,
	})
	if err != nil {
		t.Fatalf("FinalizeCompletedBatch returned error: %v", err)
	}

	if len(logger.replaced) != 1 {
		t.Fatalf("expected one replaced usage record, got %+v", logger.replaced)
	}
	record := logger.replaced[0]
	if record.Cost != 0.33 || record.ChannelCost != 0.3 {
		t.Fatalf("expected finalized cost/channel cost from quote, got cost=%f channel=%f", record.Cost, record.ChannelCost)
	}
	if record.PriceSnapshot != quote {
		t.Fatalf("expected price snapshot to be preserved")
	}
	if record.PriceCurrency != "quota" || record.PriceSource != "catalog_import_1" {
		t.Fatalf("expected price metadata from quote, got currency=%q source=%q", record.PriceCurrency, record.PriceSource)
	}
	if record.PriceEffectiveFrom == nil || !record.PriceEffectiveFrom.Equal(effectiveFrom) {
		t.Fatalf("expected price effective_from %s, got %v", effectiveFrom, record.PriceEffectiveFrom)
	}
}

func TestBatchUsageFinalizerQuotesCompletedBatchFromPricingStore(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	pricing := NewPricingStore()
	pricing.SetPrice("gpt-4o-mini", types.APITypeBatch, types.DimTotalTokens, 0.002)
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		PricingStore: pricing,
		Now:          func() time.Time { return time.Date(2026, 7, 5, 10, 15, 0, 0, time.UTC) },
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:        "batch_priced_from_store",
		RequestID:      "req_batch_priced_from_store",
		UserID:         "user_batch",
		OrganizationID: "org_batch",
		APITokenID:     "tok_batch",
		Model:          "gpt-4o-mini",
		APIType:        "batch",
	}, BatchStatusResult{
		ID:     "batch_priced_from_store",
		Status: "completed",
		Usage:  &types.Usage{TotalTokens: 100},
	})
	if err != nil {
		t.Fatalf("FinalizeCompletedBatch returned error: %v", err)
	}

	if len(logger.replaced) != 1 {
		t.Fatalf("expected one replaced usage record, got %+v", logger.replaced)
	}
	record := logger.replaced[0]
	if record.Cost != 0.2 || record.ChannelCost != 0.2 {
		t.Fatalf("expected finalized cost from pricing store, got cost=%f channel=%f", record.Cost, record.ChannelCost)
	}
	if record.PriceSnapshot == nil || record.PriceSnapshot.TotalCost != 0.2 {
		t.Fatalf("expected computed price snapshot, got %+v", record.PriceSnapshot)
	}
	if record.PriceCurrency != "quota" || record.PriceSource != "runtime" {
		t.Fatalf("expected pricing metadata from store, got currency=%q source=%q", record.PriceCurrency, record.PriceSource)
	}
}

func TestBatchUsageFinalizerSettlesDurableQuotaContext(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	quotaManager := &recordingBatchQuotaManager{}
	apiTokenQuota := &recordingBatchAPITokenQuotaManager{}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		QuotaManager:         quotaManager,
		APITokenQuotaManager: apiTokenQuota,
		Now:                  func() time.Time { return time.Date(2026, 7, 5, 10, 15, 0, 0, time.UTC) },
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:                  "batch_settle",
		RequestID:                "req_batch_settle",
		UserID:                   "user_batch",
		OrganizationID:           "org_batch",
		APITokenID:               "tok_batch",
		Model:                    "gpt-4o-mini",
		APIType:                  "batch",
		BillingSessionID:         "bill_batch_settle",
		PreauthorizedAmount:      1.25,
		TokenPreauthorizedAmount: 1.5,
	}, BatchStatusResult{
		ID:     "batch_settle",
		Status: "completed",
		Usage:  &types.Usage{TotalTokens: 100},
		Quote:  &PricingQuote{Model: "gpt-4o-mini", APIType: "batch", TotalCost: 0.75, ChannelMultiplier: 1},
	})
	if err != nil {
		t.Fatalf("FinalizeCompletedBatch returned error: %v", err)
	}
	if quotaManager.settleCalls != 1 ||
		quotaManager.settledOrganizationID != "org_batch" ||
		quotaManager.settledSessionID != "bill_batch_settle" ||
		quotaManager.settledAmount != 0.75 {
		t.Fatalf("expected durable quota settlement, got %+v", quotaManager)
	}
	if apiTokenQuota.settleCalls != 1 ||
		apiTokenQuota.settledTokenID != "tok_batch" ||
		apiTokenQuota.preauthorizedAmount != 1.5 ||
		apiTokenQuota.settledAmount != 0.75 {
		t.Fatalf("expected API token settlement, got %+v", apiTokenQuota)
	}
}

func TestBatchUsageFinalizerFailsWhenQuotaSettlementFails(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	quotaManager := &recordingBatchQuotaManager{settleErr: errors.New("quota settlement unavailable")}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		QuotaManager: quotaManager,
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:             "batch_settle_fail",
		RequestID:           "req_batch_settle_fail",
		UserID:              "user_batch",
		OrganizationID:      "org_batch",
		Model:               "gpt-4o-mini",
		APIType:             "batch",
		BillingSessionID:    "bill_batch_settle_fail",
		PreauthorizedAmount: 1.25,
	}, BatchStatusResult{
		ID:     "batch_settle_fail",
		Status: "completed",
		Usage:  &types.Usage{TotalTokens: 100},
		Quote:  &PricingQuote{Model: "gpt-4o-mini", APIType: "batch", TotalCost: 0.75, ChannelMultiplier: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "quota settlement unavailable") {
		t.Fatalf("expected quota settlement error, got %v", err)
	}
}

func TestBatchUsageFinalizerFailsClosedWhenPricingMissing(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{
		PricingStore: NewPricingStore(),
	})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:        "batch_missing_price",
		RequestID:      "req_batch_missing_price",
		UserID:         "user_batch",
		OrganizationID: "org_batch",
		Model:          "gpt-4o-mini",
		APIType:        "batch",
	}, BatchStatusResult{
		ID:     "batch_missing_price",
		Status: "completed",
		Usage:  &types.Usage{TotalTokens: 100},
	})
	if err == nil || !strings.Contains(err.Error(), "relay price not configured") {
		t.Fatalf("expected missing pricing error, got %v", err)
	}
	if len(logger.replaced) != 0 {
		t.Fatalf("usage should not be written when pricing is missing, got %+v", logger.replaced)
	}
}

func TestBatchUsageFinalizerFailsClosedWithoutRequestID(t *testing.T) {
	logger := &recordingBatchUsageReplacer{}
	finalizer := NewBatchUsageFinalizer(logger, BatchUsageFinalizerConfig{})

	err := finalizer.FinalizeCompletedBatch(context.Background(), RelayBatchPollingJob{
		BatchID:        "batch_missing_request",
		UserID:         "user_batch",
		OrganizationID: "org_batch",
		Model:          "gpt-4o-mini",
		APIType:        "batch",
	}, BatchStatusResult{ID: "batch_missing_request", Status: "completed"})
	if err == nil || !strings.Contains(err.Error(), "missing request id") {
		t.Fatalf("expected missing request id error, got %v", err)
	}
	if len(logger.replaced) != 0 {
		t.Fatalf("usage should not be written without request id, got %+v", logger.replaced)
	}
}

type recordingBatchPollingWorkerStore struct {
	jobs       []RelayBatchPollingJob
	err        error
	claimCalls int

	claimNow      time.Time
	claimLimit    int
	claimWorkerID string

	succeededBatchID  string
	succeededLockedBy string
	succeededAt       time.Time
	succeededCalls    int

	failedBatchID     string
	failedLockedBy    string
	failedReason      string
	failedAvailableAt time.Time
	failedCalls       int

	deadLetterBatchID  string
	deadLetterLockedBy string
	deadLetterReason   string
	deadLetterAt       time.Time
	deadLetterCalls    int
}

func (s *recordingBatchPollingWorkerStore) ClaimBatchPollingJobs(_ context.Context, now time.Time, limit int, workerID string) ([]RelayBatchPollingJob, error) {
	s.claimCalls++
	s.claimNow = now
	s.claimLimit = limit
	s.claimWorkerID = workerID
	if s.err != nil {
		return nil, s.err
	}
	return append([]RelayBatchPollingJob(nil), s.jobs...), nil
}

func (s *recordingBatchPollingWorkerStore) MarkBatchPollingJobSucceeded(_ context.Context, batchID, lockedBy string, completedAt time.Time) error {
	s.succeededCalls++
	s.succeededBatchID = batchID
	s.succeededLockedBy = lockedBy
	s.succeededAt = completedAt
	return nil
}

func (s *recordingBatchPollingWorkerStore) MarkBatchPollingJobFailed(_ context.Context, batchID, lockedBy, reason string, availableAt time.Time) error {
	s.failedCalls++
	s.failedBatchID = batchID
	s.failedLockedBy = lockedBy
	s.failedReason = reason
	s.failedAvailableAt = availableAt
	return nil
}

func (s *recordingBatchPollingWorkerStore) MarkBatchPollingJobDeadLetter(_ context.Context, batchID, lockedBy, reason string, completedAt time.Time) error {
	s.deadLetterCalls++
	s.deadLetterBatchID = batchID
	s.deadLetterLockedBy = lockedBy
	s.deadLetterReason = reason
	s.deadLetterAt = completedAt
	return nil
}

func (s *recordingBatchPollingWorkerStore) transitionCalls() int {
	return s.succeededCalls + s.failedCalls + s.deadLetterCalls
}

type recordingBatchCompletionFinalizer struct {
	err   error
	calls []recordingBatchCompletionFinalizerCall
}

type recordingBatchCompletionFinalizerCall struct {
	Job                      RelayBatchPollingJob
	Result                   BatchStatusResult
	SucceededBeforeFinalizer bool
}

func (f *recordingBatchCompletionFinalizer) FinalizeCompletedBatch(_ context.Context, job RelayBatchPollingJob, result BatchStatusResult) error {
	f.calls = append(f.calls, recordingBatchCompletionFinalizerCall{
		Job:                      job,
		Result:                   result,
		SucceededBeforeFinalizer: false,
	})
	return f.err
}

type recordingBatchFailureFinalizer struct {
	err   error
	calls []recordingBatchFailureFinalizerCall
}

type recordingBatchFailureFinalizerCall struct {
	Job    RelayBatchPollingJob
	Result BatchStatusResult
}

func (f *recordingBatchFailureFinalizer) FinalizeFailedBatch(_ context.Context, job RelayBatchPollingJob, result BatchStatusResult) error {
	f.calls = append(f.calls, recordingBatchFailureFinalizerCall{Job: job, Result: result})
	return f.err
}

type recordingBatchStatusClient struct {
	results map[string]BatchStatusResult
	err     error
	seen    []RelayBatchPollingJob
}

type recordingBatchUsageReplacer struct {
	replaced []RelayUsageLogRecord
	err      error
}

func (r *recordingBatchUsageReplacer) RecordRelayUsage(_ context.Context, record RelayUsageLogRecord) error {
	r.replaced = append(r.replaced, record)
	return r.err
}

func (r *recordingBatchUsageReplacer) ReplaceRelayUsage(_ context.Context, record RelayUsageLogRecord) error {
	r.replaced = append(r.replaced, record)
	return r.err
}

type recordingBatchQuotaManager struct {
	settleCalls            int
	settledOrganizationID  string
	settledSessionID       string
	settledAmount          float64
	settleErr              error
	refundCalls            int
	refundedOrganizationID string
	refundedSessionID      string
	refundErr              error
}

func (m *recordingBatchQuotaManager) PreConsume(context.Context, string, string, float64, string, string, string, string) (*quota.BillingSession, error) {
	return nil, nil
}

func (m *recordingBatchQuotaManager) Settle(_ context.Context, organizationID, sessionID string, actualAmount float64) error {
	m.settleCalls++
	m.settledOrganizationID = organizationID
	m.settledSessionID = sessionID
	m.settledAmount = actualAmount
	return m.settleErr
}

func (m *recordingBatchQuotaManager) Refund(_ context.Context, organizationID, sessionID string) error {
	m.refundCalls++
	m.refundedOrganizationID = organizationID
	m.refundedSessionID = sessionID
	return m.refundErr
}

type recordingBatchAPITokenQuotaManager struct {
	settleCalls         int
	settledTokenID      string
	preauthorizedAmount float64
	settledAmount       float64
	settleErr           error
	refundCalls         int
	refundedTokenID     string
	refundedAmount      float64
	refundErr           error
}

func (m *recordingBatchAPITokenQuotaManager) PreAuthorizeRelayAPITokenQuota(context.Context, string, float64) error {
	return nil
}

func (m *recordingBatchAPITokenQuotaManager) SettleRelayAPITokenQuota(_ context.Context, tokenID string, preauthorizedAmount, actualAmount float64) error {
	m.settleCalls++
	m.settledTokenID = tokenID
	m.preauthorizedAmount = preauthorizedAmount
	m.settledAmount = actualAmount
	return m.settleErr
}

func (m *recordingBatchAPITokenQuotaManager) RefundRelayAPITokenQuota(_ context.Context, tokenID string, amount float64) error {
	m.refundCalls++
	m.refundedTokenID = tokenID
	m.refundedAmount = amount
	return m.refundErr
}

func (c *recordingBatchStatusClient) RetrieveBatch(_ context.Context, job RelayBatchPollingJob) (BatchStatusResult, error) {
	c.seen = append(c.seen, job)
	if c.err != nil {
		return BatchStatusResult{}, c.err
	}
	result, ok := c.results[job.BatchID]
	if !ok {
		return BatchStatusResult{}, errors.New("missing batch status result")
	}
	return result, nil
}

type batchGuardCall struct {
	capabilityID string
	boundary     releasecontract.Boundary
}

type batchGuardSpy struct {
	denyAtCall int
	denial     error
	calls      []batchGuardCall
}

func (g *batchGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.calls = append(g.calls, batchGuardCall{capabilityID: capabilityID, boundary: boundary})
	if g.denyAtCall > 0 && len(g.calls) >= g.denyAtCall {
		return g.denial
	}
	return nil
}

type batchEffectRegistrar struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *batchEffectRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

func batchReadinessFixture(status string) (*recordingBatchPollingWorkerStore, *recordingBatchStatusClient) {
	job := RelayBatchPollingJob{
		BatchID: "batch_guarded", RequestID: "request_guarded", UserID: "user_guarded",
		OrganizationID: "org_guarded", APITokenID: "token_guarded", FeatureType: "workflow",
		Model: "gpt-4o-mini", APIType: "batch", BillingSessionID: "billing_guarded",
		TokenPreauthorizedAmount: 1.5, Attempts: 1, MaxAttempts: 5, LockedBy: "worker-guarded",
	}
	return &recordingBatchPollingWorkerStore{jobs: []RelayBatchPollingJob{job}}, &recordingBatchStatusClient{
		results: map[string]BatchStatusResult{job.BatchID: {ID: job.BatchID, Status: status, Error: "provider status"}},
	}
}

func newBatchReadinessWorker(t *testing.T, store *recordingBatchPollingWorkerStore, client *recordingBatchStatusClient, completion BatchCompletionFinalizer, failure BatchFailureFinalizer, guard *batchGuardSpy, now time.Time) *BatchPollingWorker {
	t.Helper()
	return newBatchReadinessWorkerWithRegistrar(t, store, client, completion, failure, guard, &batchEffectRegistrar{}, now)
}

func newBatchReadinessWorkerWithRegistrar(t *testing.T, store *recordingBatchPollingWorkerStore, client *recordingBatchStatusClient, completion BatchCompletionFinalizer, failure BatchFailureFinalizer, guard *batchGuardSpy, registrar *batchEffectRegistrar, now time.Time) *BatchPollingWorker {
	t.Helper()
	contract, profile := loadBatchReadinessAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile runtime authorities: %v", err)
	}
	worker, err := NewReadinessBatchPollingWorker(store, client, BatchPollingWorkerConfig{
		Now: func() time.Time { return now }, WorkerID: "worker-guarded", Limit: 5,
		CompletionFinalizer: completion, FailureFinalizer: failure,
		Guard: guard, Authorities: authorities, Effects: registrar,
	})
	if err != nil {
		t.Fatalf("construct readiness batch worker: %v", err)
	}
	return worker
}

func loadBatchReadinessAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve batch readiness test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}
