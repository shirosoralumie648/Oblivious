# Relay Batch Failure Usage Audit

Date: 2026-07-05

## Runtime Claim

Relay Batch terminal failures now write an error usage/audit record keyed by the original `request_id` before refunding reserved quota and API-token quota. This closes a concrete commercial lifecycle gap where failed async Batch jobs could refund/dead-letter without leaving a request-linked usage record for billing and operations review.

This is repository-local lifecycle evidence. It does not by itself prove target provider Batch execution, target request-log joins, or final target release readiness.

## Changed Files

- `src/server/internal/relay/batch_polling_worker.go`
- `src/server/internal/relay/batch_polling_worker_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`
- `docs/audit/agent-evidence/2026-07-05-relay-batch-failure-usage-audit.md`
- `docs/release/fusion-spec-evidence-pack.md`
- `docs/release/commercial-completion-audit.md`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/implementation-roadmap.md`

## Contract Changes

- `BatchUsageFinalizer.FinalizeFailedBatch` now replaces the Relay usage record with `status=error`, `status_code=502`, and a `batch_<status>` error code before refunding durable quota reservations.
- The no-skip `relay-runtime-channel-isolation` profile now includes:
  - `TestRelayStoreRegisterBatchPollingPersistsDurableTask`
  - `TestBatchUsageFinalizerRecordsFailureUsageAuditBeforeRefund`
  - `TestBatchUsageFinalizerSettlesDurableQuotaContext`
  - `TestBatchPollingWorkerRefundsTerminalFailureBeforeDeadLetter`
  - `TestNewRelayProductionBatchCommercialLifecycleRequiresPollingRegistrar`

## RED Evidence

```bash
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/relay -run '^TestBatchUsageFinalizerRecordsFailureUsageAuditBeforeRefund$' -count=1 -v
```

Failed before implementation because `FinalizeFailedBatch` refunded without writing a failure usage record:

```text
expected one failure usage audit record, got []
```

## GREEN Evidence

```bash
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/relay -run '^TestBatchUsageFinalizerRecordsFailureUsageAuditBeforeRefund$' -count=1 -v
```

Passed after implementation.

Additional focused Batch lifecycle checks:

```bash
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/relay -run '^(TestBatchPollingWorker|TestOpenAIBatchStatusClient|TestBatchUsageFinalizer|TestNewRelayBatch|TestNewRelayProductionBatch|TestRelayStoreRegisterBatchPolling|TestRelayStoreClaimBatchPolling|TestRelayStoreMarkBatchPolling)' -count=1 -v

GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./cmd/relay -run '^TestStartStandaloneRelayBatchPollingWorker' -count=1 -v
```

## Deferred

- Target provider Batch execution.
- Target request-log to usage/billing join proof.
- Provider/result reconciliation.
- Final target release proof.
