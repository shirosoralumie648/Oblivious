# RAG Index Job Dead-Letter And Lease Reclaim - 2026-07-02

## Scope

This slice hardens the durable Knowledge/RAG vector index job lifecycle for commercial operation:

- Added `max_attempts`, `locked_at`, `locked_by`, and `completed_at` fields to `knowledge_index_jobs`.
- Added terminal `dead_letter` job status through the 0083 baseline migration and the 0086 upgrade migration.
- Changed worker claims to cover pending jobs, retryable failed jobs, and lease-expired processing jobs.
- Added owner tokens to claimed jobs so stale workers cannot overwrite another worker's later claim.
- Added a 5 minute processing lease and a default retry cap of 5 attempts.
- Marked exhausted vector repair jobs as `dead_letter` and kept the Knowledge document in failed state with a diagnosable `dead_letter:` error.

## Files Changed

```text
src/server/internal/knowledge/index_jobs.go
src/server/internal/knowledge/index_job_store.go
src/server/internal/knowledge/service.go
src/server/internal/knowledge/service_test.go
src/server/internal/knowledge/store_test.go
src/server/migrations/0083_knowledge_index_jobs.sql
src/server/migrations/0086_knowledge_index_jobs_dead_letter.sql
docs/architecture/current-system-contracts.md
docs/audit/current-implementation-depth.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/vertical-slice-gap-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/implementation-roadmap.md
docs/audit/reference-capability-map.md
docs/audit/agent-evidence/2026-07-02-rag-index-job-dead-letter-reclaim.md
```

## Verification

```text
command: go test ./internal/knowledge -run "Test(IndexWorker|SQLStore.*KnowledgeIndex|KnowledgeIndexJobsMigrations|CreateDocumentUpserts|UpdateDocumentFullReplace|UpdateDocumentIncremental)" -count=1 -v
result: passed
```

Covered tests:

```text
TestIndexWorkerRebuildsClaimedDocumentVectors
TestIndexWorkerDeletesStaleVectorsForEmptyDocument
TestIndexWorkerMarksDocumentFailedWhenVectorUpsertFails
TestIndexWorkerDeadLettersDocumentAfterMaxAttempts
TestSQLStoreClaimKnowledgeIndexJobsRecoversExpiredLeasesWithOwnerAndMaxAttempts
TestSQLStoreMarksKnowledgeIndexJobDeadLetterWithOwnerGuard
TestKnowledgeIndexJobsMigrationsDeclareDeadLetterLeaseAndAttemptFields
```

## Residual Risk

This closes vector index repair retry ownership and terminal failure state. Later 2026-07-02 slices extended transactional vector intent to current SQL document/chunk mutation paths and added parsed-content ingestion jobs with a worker. Final commercial RAG readiness still needs upload enqueue/status, raw parser replay, target Qdrant proof, and retrieval-debug evidence.
