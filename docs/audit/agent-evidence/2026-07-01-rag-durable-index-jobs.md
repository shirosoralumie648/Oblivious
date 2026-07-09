# RAG Durable Index Jobs Evidence - 2026-07-01

## Scope

This slice hardens Knowledge/RAG vector indexing from best-effort Qdrant writes into a durable job-backed lifecycle.

Implemented changes:

- Added `knowledge_index_jobs` plus `knowledge_documents.index_status`, `index_error`, and `indexed_at` in migration `0083_knowledge_index_jobs.sql`.
- Added SQL store methods to create, claim, succeed, fail, and retry index jobs with `FOR UPDATE SKIP LOCKED`.
- Added `IndexWorker` and `ProcessKnowledgeIndexJob` to rebuild Qdrant document vectors from SQL chunks.
- Wired the durable index worker into `NewServer` startup behind `RAG_INDEX_WORKER_ENABLED`, with shutdown cancellation through `RegisterOnShutdown`.
- Added worker deployment config: `RAG_INDEX_WORKER_ENABLED`, `RAG_INDEX_WORKER_INTERVAL_MS`, and `RAG_INDEX_WORKER_CLAIM_LIMIT`.
- Added production fail-fast: `APP_ENV=production` with `QDRANT_URL` set cannot explicitly disable `RAG_INDEX_WORKER_ENABLED`.
- Updated `current-system-contracts.md` and `.env.example` so Qdrant and durable index worker runtime configuration is deployable and auditable.
- Wrapped synchronous create/update vector writes in index job state transitions: document `indexing -> ready` on success, `failed` with error text on vector failure.
- Full document replacement now deletes stale Qdrant document points before upserting new chunks.
- Incremental SQL document updates still use SQL chunk diffs, but Qdrant indexing now performs document-scoped replacement so removed or merged chunks cannot leave stale vector points.
- Empty document reindex jobs delete stale Qdrant document points and do not write empty vector payloads.

## Verification

- `git diff --check -- config/.env.example docs/architecture/current-system-contracts.md docs/audit/agent-evidence/2026-07-01-rag-durable-index-jobs.md src/server/internal/config/config.go src/server/internal/config/config_test.go src/server/internal/http/server.go src/server/internal/knowledge/service.go src/server/internal/knowledge/index_jobs.go src/server/internal/knowledge/index_job_store.go src/server/internal/knowledge/store.go src/server/internal/knowledge/service_test.go src/server/internal/knowledge/store_test.go src/server/migrations/0083_knowledge_index_jobs.sql src/server/migrations/microservices/table-ownership.json`
  - Passed. Git reported only LF-to-CRLF working-copy warnings.
- 2026-07-02 follow-up: `gofmt` and focused Knowledge tests were rerun with Go on PATH. `go test ./internal/knowledge -run "Test(UpdateDocumentIncremental|UpdateDocumentFullReplace|CreateDocumentUpserts|IndexWorkerRebuilds|UpdateDocumentChunk|SplitDocumentChunk|SQLStoreListKnowledgeDocumentChunks|SQLStoreUpdateKnowledgeDocumentChunk|QdrantVectorStoreUpserts|QdrantVectorStoreSearches|DeleteDocument)" -count=1 -v` passed.
- 2026-07-02 follow-up: vector index jobs now include `max_attempts`, owner-guarded processing leases, expired-lease reclaim, `completed_at`, and terminal `dead_letter` state through `0086_knowledge_index_jobs_dead_letter.sql`. `go test ./internal/knowledge -run "Test(IndexWorker|SQLStore.*KnowledgeIndex|KnowledgeIndexJobsMigrations|CreateDocumentUpserts|UpdateDocumentFullReplace|UpdateDocumentIncremental)" -count=1 -v` passed.

## Residual Release Risk

- This is not yet a full async ingestion pipeline. Create/update still performs chunking and embedding inline before writing vectors.
- Worker startup requires both `QDRANT_URL` and `RELAY_ENABLED=true`; non-production deployments without those dependencies will not run background vector repair.
- Qdrant behavior is covered by HTTP contract tests in this repository, but still needs live target Qdrant evidence before final release signoff.
