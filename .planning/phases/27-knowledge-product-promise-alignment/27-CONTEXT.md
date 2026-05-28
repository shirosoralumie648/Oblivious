# Phase 27 Context: Knowledge Product Promise Alignment

## Milestone

v08 Product Completeness.

## Why This Phase Exists

Phase 25 closed `PROD-01` by making default MCP built-ins real or disabled. Phase 26 closed `PROD-02` by adding durable Agent run/tool-run state. `PROD-03` is now the next product-completeness gap: Knowledge is customer-facing, but its retrieval behavior is still a text/snippet search implementation while the commercial product promise allows RAG only if ingestion, embedding-backed retrieval, indexing, and source citation are real.

Current Knowledge behavior:

- `src/server/internal/knowledge/store.go` stores documents and `knowledge_document_chunks`.
- `CreateKnowledgeDocument` and `UpdateKnowledgeDocument` split document content into chunks, but chunks do not carry embeddings or embedding model metadata.
- `RetrieveKnowledge` builds an `ILIKE` pattern from the full normalized query, ranks title/chunk/body text hits in Go, and returns snippets with document ID/title only.
- `docs/API.md` describes the retrieve endpoint as "Retrieve relevant document chunks" and Relay `/v1/embeddings` exists, but Knowledge retrieval does not use Relay embeddings.
- `src/web/src/routes/workspace/KnowledgePage.tsx` says users can search "indexed snippets"; it does not display source citations, chunk indexes, similarity scores, or retrieval method.
- Memory already has a Relay embedder and pgvector-backed HNSW search, but Knowledge does not use that path.

For a commercial Knowledge feature, text snippets are not enough if the product is positioned as RAG. The customer-facing path must either honestly say "text retrieval" or implement embedding-backed retrieval with source citations. Because v08 is about removing MVP behavior and the platform already has Relay embeddings and pgvector, Phase 27 should implement real embedding-backed Knowledge retrieval instead of narrowing the product copy.

## Requirement

- **PROD-03:** Knowledge behavior matches customer-facing product copy. If marketed as RAG, ingestion, embedding-backed retrieval, indexing, and source citation must be implemented and verified; otherwise copy must explicitly describe text retrieval.

## Current Evidence And Gaps

Existing evidence:

- `knowledge_document_chunks` is already the right persistence boundary for ingestion/indexing.
- `memory.NewRelayEmbedder` already calls Relay `/v1/embeddings` and forwards trusted internal user/organization/request identity headers.
- `src/server/internal/http/router.go` already creates a Relay embedder for Memory when `RELAY_ENABLED=true`.
- Knowledge handlers and stores are already organization-scoped after v04.
- Existing Knowledge tests cover CRUD, query normalization, snippet rendering, and tenant-scoped store calls.

Gaps:

- Knowledge chunks have no `embedding vector(1536)` column, no embedding model, and no vector index.
- Knowledge service has no embedder dependency, so document ingestion cannot produce embeddings through Relay.
- Retrieval does not embed the query and does not run pgvector similarity search.
- Retrieval response lacks citation fields such as chunk ID, chunk index, source title, retrieval method, and similarity score.
- UI renders snippets without citations or RAG/source language.
- Docs and quality gates do not prevent future customer-facing RAG overclaims.
- Existing tests can pass even if retrieval remains pure text search.

## Design Direction

Phase 27 should choose the stronger commercial path:

- Add migration `0032_knowledge_rag_index.sql` that enables pgvector if needed, adds `embedding vector(1536)`, `embedding_model`, and `indexed_at` to `knowledge_document_chunks`, and adds a vector index.
- Add a Knowledge embedder dependency that uses Relay embeddings in production through `memory.NewRelayEmbedder`.
- Keep existing document CRUD APIs, but make create/update index chunks with embeddings before the document is considered successfully saved.
- Make retrieval embed the query through Relay and search chunk vectors with pgvector cosine similarity.
- Return source citations in every retrieval result: document ID/title, chunk ID, chunk index, snippet, similarity score, and retrieval method `embedding_rag`.
- Preserve organization filtering on all indexing and retrieval queries.
- Update the UI to display "RAG citations" or equivalent source-backed language and include chunk/source citation details.
- Update API docs, commercial gates, and quality gates so Knowledge cannot be described as commercial RAG unless embedding-backed retrieval and citations are present.

Fallback behavior should be explicit:

- In commercial/Relay-enabled runtime, Knowledge retrieval should use embeddings.
- If the server is started without a Knowledge embedder, retrieval should fail with an explicit configuration error rather than silently returning text-search results under RAG copy.
- Tests may use fake embedders and small deterministic vectors; no live provider key is required.

## Expected Code Areas

- `src/server/migrations/0032_knowledge_rag_index.sql`: chunk embedding columns and vector index.
- `src/server/internal/knowledge/service.go`: embedder interface, indexing orchestration, response shape with citations.
- `src/server/internal/knowledge/store.go`: chunk embedding persistence, pgvector retrieval, citation fields.
- `src/server/internal/knowledge/service_test.go`: fake embedder tests proving ingestion and retrieval use embeddings.
- `src/server/internal/knowledge/store_test.go`: helper/vector and DB-backed retrieval tests if `TEST_DATABASE_URL` is available.
- `src/server/internal/http/router.go`: inject Relay embedder into Knowledge service when Relay is enabled.
- `src/server/internal/http/knowledge_handler.go`: response shape remains wrapped by API envelope but returns citation fields.
- `src/server/internal/http/knowledge_handler_test.go`: retrieval response includes citations and retrieval method.
- `src/web/src/types/api.ts`, `src/web/src/features/knowledge/api.ts`, `src/web/src/routes/workspace/KnowledgePage.tsx`, `src/web/src/routes/workspace/KnowledgePage.test.tsx`: typed source citations and UI rendering.
- `docs/API.md`, `docs/release/commercial-gates.md`, `scripts/verify-quality-gates.sh`: docs and gates for `PROD-03`.

## Verification Design

Phase 27 must prove behavior, not just copy:

- RED service tests must fail before implementation because Knowledge has no embedder/index contract.
- Service tests must prove document creation/update calls `EmbedBatch` with indexed chunks and retrieval calls `Embed` for the query.
- Store tests must prove vector search orders by similarity and returns citation fields with organization filtering.
- Handler tests must prove retrieve responses include `source`, `chunkId`, `chunkIndex`, `similarity`, and `retrievalMethod`.
- Web tests must prove source citations render in the Knowledge page.
- Docs gates must assert RAG wording is coupled to embedding-backed retrieval/source citation artifacts.
- `git diff --check` must pass.

## Closeout Boundary

Phase 27 may close only `PROD-03`.

It must not claim:

- Full Chat/Admin/Marketplace UX hardening (`PROD-04`).
- Public onboarding, pricing, and operator guide completion (`PROD-05`).
- End-to-end commercial journeys or final commercial completion audit (`PROD-06`, `AUDIT-01`).

---

*Phase: 27-knowledge-product-promise-alignment*
*Context gathered: 2026-05-28 from current repository evidence*
