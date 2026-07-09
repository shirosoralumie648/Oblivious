# RAG Stale Vector Ready Filter

Date: 2026-07-04

## Scope

This slice closes a retrieval correctness gap in the RAG vector lifecycle. Qdrant can still contain stale points while SQL document updates, deletes, and async index jobs are in flight; vector-only and hybrid retrieval now filter Qdrant candidates against SQL live document/chunk rows whose `index_status` is `ready` before returning them.

## Code Evidence

- `src/server/internal/knowledge/service.go`: Qdrant vector-only and hybrid candidate paths now call `filterReadyKnowledgeRetrievalResults` before reranking or fusing results.
- `src/server/internal/knowledge/store.go`: `SQLStore.FilterReadyKnowledgeRetrievalResults` joins candidate `(document_id, chunk_id)` pairs against tenant-scoped `knowledge_documents`, `knowledge_document_chunks`, and `knowledge_bases`, and only returns rows with `index_status = ready`.
- `src/server/internal/knowledge/service_test.go`: `TestRetrieveWithOptionsFiltersStaleQdrantVectorCandidates` proves service-level vector retrieval drops deleted/pending stale Qdrant candidates.
- `src/server/internal/knowledge/store_test.go`: `TestSQLStoreFilterReadyKnowledgeRetrievalResultsKeepsOnlyLiveReadyChunks` proves the SQL filter only admits live ready chunks.

## Verification

- RED: `cd src/server && go test ./internal/knowledge -run TestRetrieveWithOptionsFiltersStaleQdrantVectorCandidates -count=1 -v`
  - Failed as expected before the filter because the service did not call the SQL ready/live candidate filter.
- GREEN focused: `gofmt -w src/server/internal/knowledge/service.go src/server/internal/knowledge/store.go src/server/internal/knowledge/service_test.go src/server/internal/knowledge/store_test.go && cd src/server && go test ./internal/knowledge -run 'TestRetrieveWithOptionsFiltersStaleQdrantVectorCandidates|TestSQLStoreFilterReadyKnowledgeRetrievalResultsKeepsOnlyLiveReadyChunks' -count=1 -v`
  - Passed both the service-level and SQLStore-level stale vector filter tests.
- Package regression: `cd src/server && go test ./internal/knowledge -count=1 && cd ../.. && git diff --check -- src/server/internal/knowledge/service.go src/server/internal/knowledge/store.go src/server/internal/knowledge/service_test.go src/server/internal/knowledge/store_test.go`
  - Passed for `internal/knowledge` and diff whitespace.

## Remaining Boundary

This closes the in-process stale vector guard. Target Postgres/Qdrant runtime proof remains open: the next release evidence should run the migrated SQL store and Qdrant-backed vector store together, process update/delete index jobs after restart, and demonstrate retrieval no longer returns stale points in the target deployment profile.
