# RAG Transactional Vector Intents - 2026-07-02

This follow-up extends the durable Knowledge/RAG vector outbox beyond document create/update.

## What Changed

- Chunk edit, split, and merge store methods now have `WithOptions` variants that update SQL chunks and enqueue an `upsert_document` vector index job in the same SQL transaction.
- Document delete now enqueues a `delete_document` vector index job before the SQL document row is removed.
- `knowledge_index_jobs.document_id` no longer has a document foreign key, so delete cleanup jobs survive document deletion.
- The index worker processes `delete_document` by deleting Qdrant points and intentionally skips document `index_status` writes for already deleted documents.
- Service delete/chunk mutation paths use transactional SQL outbox writes when `UsesTransactionalKnowledgeIndexOutbox` is available.

## Files

- `src/server/internal/knowledge/index_jobs.go`
- `src/server/internal/knowledge/service.go`
- `src/server/internal/knowledge/store.go`
- `src/server/internal/knowledge/service_test.go`
- `src/server/internal/knowledge/store_test.go`
- `src/server/migrations/0083_knowledge_index_jobs.sql`
- `src/server/migrations/0092_knowledge_index_jobs_delete_operation.sql`

## Verification

```text
command: go test ./internal/knowledge -run "Test(IndexWorkerProcessesDeleteDocumentJobWithoutDocumentStatusUpdates|ProcessKnowledgeIndexJobDeleteDocumentDeletesVectorsWithoutEmbedder|SQLStoreDeleteKnowledgeDocumentWithOptionsCreatesTransactionalDeleteIndexOutbox|KnowledgeIndexJobsMigrationsDeclareDeleteDocumentJobsSurviveDeletedDocuments|UpdateDocumentChunkUsesTransactionalOutbox|SplitDocumentChunkUsesTransactionalOutbox|DeleteDocumentUsesTransactionalOutbox|DeleteDocumentByIDUsesTransactionalOutbox)" -count=1 -v
result: pass

command: go test ./internal/knowledge -count=1
result: pass
```

## Remaining Boundary

Repository-local vector intent durability is now covered for current SQL document/chunk mutation paths. Commercial RAG readiness still needs durable upload/parser/embedding workers, retrieval-debug evidence, and target Postgres/Qdrant runtime proof.
