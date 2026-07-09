# RAG Durable Ingestion Worker Skeleton - 2026-07-02

This slice adds repository-local durable ingestion jobs for parsed Knowledge content and wires a background worker into server startup.

## What Changed

- Added `knowledge_ingestion_jobs` with status, retry, owner lease, `dead_letter`, document id backfill, and claim indexes.
- Added `Service.EnqueueDocumentIngestion` and `Service.ProcessKnowledgeIngestionJob`.
- Added `IngestionWorker` with claim, retry, success, and dead-letter behavior.
- Added SQL store methods for create, claim, success, failed retry, and dead-letter.
- Added `RAG_INGESTION_WORKER_ENABLED`, `RAG_INGESTION_WORKER_INTERVAL_MS`, and `RAG_INGESTION_WORKER_CLAIM_LIMIT`.
- Wired the ingestion worker into server startup when Relay is enabled.

## Files

- `src/server/internal/knowledge/ingestion_jobs.go`
- `src/server/internal/knowledge/ingestion_job_store.go`
- `src/server/internal/knowledge/service_test.go`
- `src/server/internal/knowledge/store_test.go`
- `src/server/internal/config/config.go`
- `src/server/internal/http/server.go`
- `src/server/migrations/0093_knowledge_ingestion_jobs.sql`
- `src/server/migrations/microservices/table-ownership.json`
- `config/.env.example`

## Verification

```text
command: go test ./internal/knowledge -run "Test(EnqueueDocumentIngestion|IngestionWorker|SQLStoreCreateKnowledgeIngestionJob|SQLStoreClaimKnowledgeIngestionJobs|KnowledgeIngestionJobsMigration)" -count=1 -v
result: pass

command: go test ./internal/knowledge -count=1
result: pass

command: go test ./internal/config -run "TestLoad(Default|QdrantConfig|RAGIndexWorkerConfig|RAGIngestionWorkerConfig|RejectsInvalidRAGIndexWorkerConfig)" -count=1 -v
result: pass

command: go test ./internal/http -run "Test(KnowledgeHandlerUploadDocumentCreatesParsedKnowledgeDocument|KnowledgeHandlerUploadDocumentPersistsSourceMetadataOnChunks|RegisterKnowledgeAliasRoutesDispatchesDocumentUpload)" -count=1 -v
result: pass
```

## Remaining Boundary

The public upload handler still parses and creates documents synchronously for compatibility. Commercial completion still needs upload enqueue/status responses, raw upload payload or object-store reference persistence for parser replay, retrieval-debug evidence, and target Postgres/Qdrant runtime proof.
