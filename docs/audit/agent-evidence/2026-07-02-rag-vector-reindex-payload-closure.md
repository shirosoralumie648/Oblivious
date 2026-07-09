# RAG Vector Reindex And Payload Closure - 2026-07-02

## Scope

This slice closes the remaining repository-local RAG vector consistency issues that could produce stale or incomplete Qdrant retrieval results:

- Qdrant upsert payloads now include `document_title`, matching the search path that reads `document_title` into citations and retrieval results.
- Knowledge document chunks and chunk views now carry `DocumentTitle`, so create/update, worker rebuild, chunk edit, split, and merge-driven reindex paths preserve title metadata.
- Incremental SQL document updates no longer use diff-only chunks for Qdrant. The SQL write path can remain incremental, but Qdrant indexing now deletes document-scoped points and upserts the current document chunks to avoid stale vectors after removed, merged, or shortened chunks.
- Direct document deletion and document-id-only deletion continue to fail closed on Qdrant cleanup errors before SQL deletion.
- The legacy `Processor` path now writes a best-effort document title into vector payloads.

## Files Changed

```text
src/server/internal/knowledge/types_enhanced.go
src/server/internal/knowledge/service.go
src/server/internal/knowledge/store.go
src/server/internal/knowledge/qdrant_store.go
src/server/internal/knowledge/document_processor.go
src/server/internal/knowledge/service_test.go
src/server/internal/knowledge/store_test.go
src/server/internal/knowledge/qdrant_store_test.go
docs/audit/current-implementation-depth.md
docs/audit/vertical-slice-gap-report.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/reference-capability-map.md
docs/audit/agent-evidence/2026-07-01-rag-durable-index-jobs.md
docs/audit/agent-evidence/2026-07-02-rag-vector-reindex-payload-closure.md
```

## Verification

```text
command: go test ./internal/knowledge -run "Test(UpdateDocumentIncremental|UpdateDocumentFullReplace|CreateDocumentUpserts|IndexWorkerRebuilds|UpdateDocumentChunk|SplitDocumentChunk|SQLStoreListKnowledgeDocumentChunks|SQLStoreUpdateKnowledgeDocumentChunk|QdrantVectorStoreUpserts|QdrantVectorStoreSearches|DeleteDocument)" -count=1 -v
result: passed
```

Covered tests:

```text
TestUpdateDocumentIncrementalReplacesQdrantDocumentVectorsToAvoidStaleChunks
TestUpdateDocumentFullReplaceDeletesStaleQdrantPointsBeforeUpsert
TestCreateDocumentUpsertsEmbeddedChunksToQdrantVectorStore
TestIndexWorkerRebuildsClaimedDocumentVectors
TestUpdateDocumentChunkReindexesEditedChunkInQdrantVectorStore
TestSplitDocumentChunkReindexesDocumentChunksInQdrantVectorStore
TestDeleteDocumentDeletesTenantScopedQdrantPointsBeforeSQLDocument
TestDeleteDocumentByIDResolvesScopeBeforeQdrantCleanup
TestSQLStoreListKnowledgeDocumentChunksReturnsTenantScopedChunkViews
TestSQLStoreUpdateKnowledgeDocumentChunkUpdatesContentHashAndReturnsChunk
TestQdrantVectorStoreUpsertsTenantChunkPoints
TestQdrantVectorStoreSearchesTenantChunkPoints
```

## Residual Risk

RAG upload is still request-synchronous for public parse/create compatibility. Vector repair jobs now have retry caps, owner lease reclaim, and `dead_letter`; later 2026-07-02 slices add transactional SQL enqueue for document create/update, chunk edit/split/merge, delete cleanup intent, and parsed-content ingestion jobs with a worker. Commercial release still needs upload enqueue/status, raw parser replay, retrieval-debug evidence, and target-runtime Qdrant/Postgres evidence.
