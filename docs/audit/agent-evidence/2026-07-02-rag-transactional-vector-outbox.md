# RAG Transactional Vector Outbox - 2026-07-02

This slice closes the crash window where Knowledge SQL mutations could commit before the durable vector index job was created. It started with document create/update and was extended on 2026-07-02 to chunk edit/split/merge plus document delete cleanup intent.

## What Changed

- `SQLStore.CreateKnowledgeDocumentWithOptions` and `SQLStore.UpdateKnowledgeDocumentWithOptions` can now insert the `knowledge_index_jobs` outbox row in the same SQL transaction as the document, chunk, and version writes.
- `KnowledgeDocumentOptions` has an internal `createIndexJob` flag used only by the service/store boundary, so API payloads do not expose outbox control.
- `Service.CreateDocumentWithOptions` and `Service.UpdateDocumentWithOptions` detect stores that implement `UsesTransactionalKnowledgeIndexOutbox`; when a vector store is configured, they ask the SQL store to enqueue the outbox job transactionally and skip request-path Qdrant writes.
- `SQLStore.UpdateKnowledgeDocumentChunkWithOptions`, `SplitKnowledgeDocumentChunkWithOptions`, and `MergeKnowledgeDocumentChunksWithOptions` now enqueue an upsert vector job in the same transaction as the chunk edit.
- `SQLStore.DeleteKnowledgeDocumentWithOptions` enqueues a `delete_document` vector job before deleting the SQL document row; `knowledge_index_jobs.document_id` no longer has a document FK, so cleanup jobs survive the document deletion.
- `Service.DeleteDocument` and `Service.DeleteDocumentByID` use the transactional store path when a vector store is configured, so request-path Qdrant cleanup is not required for SQL-backed stores.
- The index worker now marks documents `indexing` when a pending job is claimed, then `ready` with `indexed_at` on success or `failed` on retry/dead-letter failure.
- `delete_document` jobs delete stale Qdrant points without writing `index_status` back to the already deleted SQL document.
- Non-transactional stores keep the prior request-path vector write behavior, preserving tests and non-SQL fallback semantics.

## Files

- `src/server/internal/knowledge/types_enhanced.go`
- `src/server/internal/knowledge/service.go`
- `src/server/internal/knowledge/store.go`
- `src/server/internal/knowledge/index_jobs.go`
- `src/server/internal/knowledge/index_job_store.go`
- `src/server/internal/knowledge/service_test.go`
- `src/server/internal/knowledge/store_test.go`
- `src/server/migrations/0092_knowledge_index_jobs_delete_operation.sql`

## Verification

```text
command: go test ./internal/knowledge -run "Test(SQLStoreCreateKnowledgeDocumentWithOptions|SQLStoreUpdateKnowledgeDocument|CreateDocumentWithTransactionalOutbox|CreateDocumentUpserts|UpdateDocumentIncremental|UpdateDocumentFullReplace|IndexWorker|KnowledgeIndexJobsMigrations)" -count=1 -v
result: pass

command: go test ./internal/knowledge -count=1
result: pass

command: go test ./internal/knowledge -run "Test(IndexWorkerProcessesDeleteDocumentJobWithoutDocumentStatusUpdates|ProcessKnowledgeIndexJobDeleteDocumentDeletesVectorsWithoutEmbedder|SQLStoreDeleteKnowledgeDocumentWithOptionsCreatesTransactionalDeleteIndexOutbox|KnowledgeIndexJobsMigrationsDeclareDeleteDocumentJobsSurviveDeletedDocuments|UpdateDocumentChunkUsesTransactionalOutbox|SplitDocumentChunkUsesTransactionalOutbox|DeleteDocumentUsesTransactionalOutbox|DeleteDocumentByIDUsesTransactionalOutbox)" -count=1 -v
result: pass
```

Covered tests include:

- `TestSQLStoreCreateKnowledgeDocumentWithOptionsCreatesTransactionalIndexOutbox`
- `TestSQLStoreUpdateKnowledgeDocumentWithOptionsCreatesTransactionalIndexOutbox`
- `TestCreateDocumentWithTransactionalOutboxSkipsRequestPathVectorWrite`
- `TestIndexWorkerRebuildsClaimedDocumentVectors`
- `TestIndexWorkerMarksDocumentFailedWhenVectorUpsertFails`
- `TestIndexWorkerDeadLettersDocumentAfterMaxAttempts`
- `TestUpdateDocumentChunkUsesTransactionalOutboxWhenSQLStoreSupportsIt`
- `TestSplitDocumentChunkUsesTransactionalOutboxWhenSQLStoreSupportsIt`
- `TestDeleteDocumentUsesTransactionalOutboxWhenSQLStoreSupportsIt`
- `TestDeleteDocumentByIDUsesTransactionalOutboxWhenSQLStoreSupportsIt`
- `TestIndexWorkerProcessesDeleteDocumentJobWithoutDocumentStatusUpdates`
- `TestProcessKnowledgeIndexJobDeleteDocumentDeletesVectorsWithoutEmbedder`
- `TestSQLStoreDeleteKnowledgeDocumentWithOptionsCreatesTransactionalDeleteIndexOutbox`
- `TestKnowledgeIndexJobsMigrationsDeclareDeleteDocumentJobsSurviveDeletedDocuments`

## Remaining Boundary

This closes transactional vector intent for current SQL document/chunk mutation paths. A later 2026-07-02 slice added parsed-content ingestion jobs and a worker. Final commercial RAG readiness still needs upload enqueue/status responses, raw upload parser replay, retrieval-debug evidence, and target Postgres/Qdrant runtime proof.
