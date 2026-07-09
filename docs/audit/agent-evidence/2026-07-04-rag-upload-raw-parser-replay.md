# RAG Upload Raw Parser Replay

Date: 2026-07-04

## Scope

This slice closes the raw-payload portion of the RAG upload P0 gap. Multipart upload still validates/parses in the request path for immediate format errors and title fallback, but it now persists the raw upload bytes and source metadata on the durable ingestion job so a worker can replay parsing after restart.

## Code Evidence

- `src/server/internal/http/knowledge_handler.go`: upload reads the multipart file into a size-limited raw byte buffer, parses from that buffer, enqueues `KnowledgeIngestionRawPayload`, and returns `202 Accepted` job metadata without echoing parsed or raw content.
- `src/server/internal/knowledge/ingestion_jobs.go`: ingestion job requests/jobs now carry `RawContent`, `RawFilename`, `RawContentType`, and `RawSizeBytes`; `ProcessKnowledgeIngestionJob` prefers persisted raw bytes and replays parsing through an injected parser before chunk/embed/create.
- `src/server/internal/knowledge/ingestion_job_store.go`: SQL insert/list/claim/scan paths persist and return the raw upload payload fields under tenant scope.
- `src/server/migrations/0093_knowledge_ingestion_jobs.sql`: `knowledge_ingestion_jobs` declares `raw_content`, `raw_filename`, `raw_content_type`, and `raw_size_bytes`.
- `src/server/internal/http/router.go`: production knowledge service wiring injects the document parser used by the ingestion worker.

## Verification

- RED: `cd src/server && go test ./internal/knowledge ./internal/http -run 'TestEnqueueDocumentIngestionCreatesDurableJobWithNormalizedOptions|TestIngestionWorkerReplaysDurableRawUploadPayload|TestSQLStoreCreateKnowledgeIngestionJobPersistsDurablePayload|TestKnowledgeIngestionJobsMigrationDeclaresDurableWorkerFields|TestKnowledgeHandlerUploadDocumentEnqueuesParsedKnowledgeIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesDurableIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesParsedCSVKnowledgeIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesParsedHTMLKnowledgeIngestionJob' -count=1 -timeout 300s`
  - Failed as expected because `KnowledgeIngestionRawPayload`, raw job fields, and parser injection did not exist yet.
- GREEN: `cd src/server && go test ./internal/knowledge ./internal/http -run 'TestEnqueueDocumentIngestionCreatesDurableJobWithNormalizedOptions|TestIngestionWorkerProcessesClaimedJobIntoKnowledgeDocument|TestIngestionWorkerReplaysDurableRawUploadPayload|TestIngestionWorkerDeadLettersExhaustedMalformedJob|TestSQLStoreCreateKnowledgeIngestionJobPersistsDurablePayload|TestSQLStoreClaimKnowledgeIngestionJobsRecoversExpiredLeasesWithOwnerAndMaxAttempts|TestSQLStoreListKnowledgeIngestionJobsScopesByOrganizationAndKnowledgeBase|TestKnowledgeIngestionJobsMigrationDeclaresDurableWorkerFields|TestKnowledgeHandlerUploadDocumentEnqueuesParsedKnowledgeIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesDurableIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesParsedCSVKnowledgeIngestionJob|TestKnowledgeHandlerUploadDocumentEnqueuesParsedHTMLKnowledgeIngestionJob' -count=1 -timeout 300s`
  - Passed for `internal/knowledge` and `internal/http`.
- Focused route/upload regression: `cd src/server && go test ./internal/knowledge ./internal/http -run 'TestEnqueueDocumentIngestion|TestIngestionWorker|TestSQLStore.*Ingestion|TestKnowledgeIngestionJobsMigration|TestKnowledgeHandlerUploadDocument|TestKnowledgeHandlerListsDocumentIngestionJobs|TestRouteSurfaceKnowledgeRoutes|TestRouteSurfaceKnowledgeAlias' -count=1 -timeout 300s`
  - Passed for `internal/knowledge` and `internal/http`.
- Package regression: `cd src/server && go test ./internal/knowledge ./internal/http -count=1 -timeout 300s`
  - Passed for `internal/knowledge` and `internal/http`.
- `git diff --check`
  - Passed.

## Remaining Boundary

Target Postgres/Qdrant runtime proof is still open: the next evidence slice should run the worker against real migrated storage, show restart recovery from persisted raw payload, and prove retrieval cannot return stale vectors after update/delete worker processing.
