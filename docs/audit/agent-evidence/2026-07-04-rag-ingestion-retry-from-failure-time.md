# RAG Ingestion Retry From Failure Time

Date: 2026-07-04

## Runtime Claim

Durable RAG ingestion worker retries now schedule from the actual failure completion time instead of the original job-claim time. This prevents long-running parser/embedder failures from becoming immediately retryable as soon as they are marked failed.

The worker still persists parser failure reason, retry availability, worker ownership, and raw upload replay evidence through `knowledge_ingestion_jobs`.

## Reference Inputs

- `reference/ragflow/README_zh.md` - RAG worker and parser lifecycle reference.
- `reference/MaxKB/README_CN.md` - knowledge-base document task lifecycle reference.
- `docs/audit/implementation-roadmap.md` - current Oblivious RAG target-runtime gap list.

## Oblivious Files Changed

```text
src/server/internal/knowledge/ingestion_jobs.go
src/server/internal/knowledge/service_test.go
docs/audit/agent-evidence/2026-07-04-rag-ingestion-retry-from-failure-time.md
```

## Contract Changes

None. The public API, database schema, and configuration contracts are unchanged. This is a worker retry-scheduling behavior fix.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/knowledge -run 'TestIngestionWorkerSchedulesRetryFromFailureTimeAfterLongParse' -count=1 -v
result: RED before fix; retry was scheduled at 2026-07-03 15:03:00 UTC instead of 2026-07-03 15:15:00 UTC.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/knowledge -run 'TestIngestionWorker(RecordsRetryableParserFailureDiagnostics|SchedulesRetryFromFailureTimeAfterLongParse|ReplaysDurableRawUploadPayload|DeadLettersExhaustedMalformedJob|ProcessesClaimedJobIntoKnowledgeDocument)' -count=1 -v
result: PASS; ingestion worker success, raw replay, retry diagnostics, long-parse retry, and dead-letter paths pass.
```

## Failure Evidence

Before the fix:

```text
expected retry from failure time at 2026-07-03 15:15:00 +0000 UTC, got 2026-07-03 15:03:00 +0000 UTC
```

This showed the worker used the claim timestamp for retry backoff even when parsing failed 12 minutes later.

## Unsupported / Deferred Surfaces

- Target Postgres/Qdrant runtime recovery proof is still required.
- Retrieval-debug and target Qdrant citation evidence are still required.
- Object-store-backed raw payload references for large uploads remain deferred.

## Known Residual Risk

This is repository-local worker behavior evidence, not a target-environment RAG indexing proof. Final commercial readiness still requires deployed worker evidence, raw parser replay, Qdrant upsert/delete/retrieval proof, and artifact-linked target evidence.
