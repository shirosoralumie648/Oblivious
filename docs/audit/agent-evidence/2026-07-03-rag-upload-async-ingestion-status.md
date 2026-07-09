# RAG Upload Async Ingestion Status

Date: 2026-07-03

## Scope

This slice closes the request-path portion of the RAG upload P0 gap by making multipart uploads enqueue durable ingestion jobs and expose status metadata.

## Code Evidence

- `src/server/internal/http/knowledge_handler.go`: upload now parses the file, calls `EnqueueDocumentIngestion`, returns `202 Accepted`, and does not synchronously create document chunks.
- `src/server/internal/http/routes_knowledge.go`: canonical `GET /api/v1/app/knowledge-bases/:knowledgeBaseId/documents/ingestion-jobs` route.
- `src/server/internal/http/routes_knowledge_alias.go`: compatibility `GET /api/v1/knowledge-bases/:knowledgeBaseId/documents/ingestion-jobs` route.
- `src/server/internal/knowledge/ingestion_jobs.go`: service-level `ListDocumentIngestionJobs`.
- `src/server/internal/knowledge/ingestion_job_store.go`: tenant-scoped SQL listing ordered by latest update.
- `docs/API.md`: public API contract for `202 Accepted` upload and ingestion status fields.

## Verification

- `git diff --check`
- `cd src/server && go test ./internal/knowledge ./internal/http -run 'TestEnqueueDocumentIngestion|TestIngestionWorker|TestSQLStoreListKnowledgeIngestionJobs|TestKnowledgeHandlerUploadDocument|TestKnowledgeHandlerListsDocumentIngestionJobs|TestRouteSurfaceKnowledgeRoutes|TestRouteSurfaceKnowledgeAlias' -count=1 -timeout 300s -v`

The first run hit transient Go proxy `unexpected EOF` errors while downloading ClickHouse and `golang.org/x/*` dependencies. After pre-downloading the missing modules through `GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct`, the focused verification passed for `internal/knowledge` and `internal/http`.

## Remaining Boundary

Raw uploaded file bytes are still not persisted for parser-worker replay. Target Postgres/Qdrant worker recovery and retrieval-debug evidence also remain open.
