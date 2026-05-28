# Phase 27 Summary — Knowledge Product Promise Alignment

## Result

Phase 27 implements embedding-backed Knowledge RAG evidence for `PROD-03`.

Knowledge document create and update paths now index document chunks through the configured embedder. Retrieval embeds the query, searches stored chunk vectors with pgvector under organization scope, and returns source-cited results with `embedding_rag` retrieval metadata.

## Delivered

- Added `src/server/migrations/0032_knowledge_rag_index.sql` with pgvector extension setup, `knowledge_document_chunks.embedding`, embedding model metadata, index timestamps, and an HNSW vector index.
- Added `KnowledgeEmbedder`, `NewServiceWithEmbedder`, indexed chunk models, and `KnowledgeCitation` response fields.
- Updated Knowledge document create/update to call `EmbedBatch` and persist indexed chunks.
- Updated Knowledge retrieval to require a configured embedder, call `Embed`, and delegate vector retrieval to the SQL store.
- Updated SQL retrieval to rank chunks by `embedding <=> query_vector` under organization scope and return bounded snippets plus source citations.
- Wired the Knowledge service to the Relay embedder when Relay is enabled.
- Updated the workspace Knowledge page to render RAG citation lines with source document, chunk index, retrieval method, and similarity.
- Updated API docs, commercial gate docs, and quality gates for Relay embedding-backed RAG with citations.

## Verification

Focused Knowledge service, HTTP, DB-backed pgvector, and web tests passed. Docs and diff hygiene are recorded in `27-VERIFICATION.md`.

## Boundary

Phase 27 closes only `PROD-03` after final docs and diff checks pass. Phase 28 Commercial UX and Journey Hardening is next. v08 Product Completeness and the final commercial completion audit remain open.
