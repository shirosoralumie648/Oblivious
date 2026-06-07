# Knowledge and RAG Requirement Audit - 2026-06-07

Scope:

- `docs/superpowers/specs/2026-06-04-functional-logic-details.md` sections 3.1-3.4 and 7.6.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design.md` section 3.3.

Status values:

- `Proven`: current code and focused tests prove the requirement.
- `Partial`: current code exists, but the requirement is only partly implemented or partly verified.
- `Gap`: current evidence contradicts or misses the requirement.
- `Unverified`: code may exist, but evidence is too weak for a completion claim.

## 3.1 Retrieval Strategy Configuration

| Requirement | Status | Evidence |
| --- | --- | --- |
| Knowledge-base retrieval mode supports `vector_only`, `hybrid`, and `hybrid_rerank`. | Proven for API/store contract | `KnowledgeRetrievalModeVector` now serializes as `vector_only`, while legacy `"vector"` input is still normalized to `vector_only`. `TestKnowledgeHandlerRetrieveAcceptsVectorOnlyMode` proves the HTTP retrieve API accepts the spec/frontend mode, and `TestSQLStoreRetrieveKnowledgeWithOptionsUsesCrossTenantSafeVectorSearch` proves SQL vector mode accepts `vector_only`. |
| Knowledge-base retrieval config is preserved after list/detail reload. | Proven | `ListKnowledgeBases` and `GetKnowledgeBase` now select and scan retrieval mode, limit, min score, vector/keyword weights, reranker config, chunk config, embedding model, and update strategy. `TestSQLStoreListAndGetKnowledgeBasesReturnRAGConfig` proves config readback. |
| Hybrid retrieval combines vector and keyword search. | Proven for configured SQL and Qdrant-backed service paths | `RetrieveKnowledgeWithOptions` has pgvector and full-text branches with weighted score fusion and tenant filters. When a Qdrant vector store and query embedding are configured, service retrieval now takes vector candidates from Qdrant, keyword candidates from the SQL keyword path, collapses duplicate chunks, and applies weighted reciprocal-rank fusion before optional reranking. Covered by SQL-shape tests and `TestRetrieveWithOptionsFusesQdrantVectorAndKeywordResultsForHybridMode`. This is functional coverage, not yet a curated production quality benchmark. |
| `hybrid_rerank` invokes a configured reranker model and falls back on outage. | Proven for configured HTTP reranker path and service fallback | `Service` now accepts a `KnowledgeReranker`, `hybrid_rerank` calls it after store retrieval, `retrieval.KnowledgeResultReranker` adapts `KnowledgeRetrievalResult` to the Cohere-compatible `/rerank` client, and `newKnowledgeService` wires it when `RAG_RERANKER_BASE_URL` is configured. Retrieval expands the store candidate pool to the knowledge-base `rerankTopK` before applying the final user-facing limit. If the reranker fails, service retrieval preserves original candidate ordering, applies the final limit, and increments `rag_reranker_fallback_total{mode="hybrid_rerank"}`. Covered by `TestRetrieveWithOptionsReranksHybridRerankResults`, `TestRetrieveWithOptionsExpandsHybridRerankCandidatePool`, `TestRetrieveWithOptionsFallsBackWhenHybridRerankerFails`, `TestKnowledgeHandlerRetrieveUsesKnowledgeBaseRerankTopK`, `TestKnowledgeResultRerankerCallsCohereCompatibleEndpoint`, `TestLoadRAGRerankerConfig`, `TestNewKnowledgeServiceWiresRAGReranker`, and `TestFusionObservabilityMetricsRecordWorkflowRAGAndAgentSignals`. |

## 3.2 Chunking Strategy Configuration

| Requirement | Status | Evidence |
| --- | --- | --- |
| Knowledge-base config stores chunk strategy, size, and overlap. | Proven for config persistence/readback | Create/update/list/detail paths include `chunk_strategy`, `chunk_size`, and `chunk_overlap`; `TestSQLStoreListAndGetKnowledgeBasesReturnRAGConfig` covers readback. |
| Main ingestion applies fixed, semantic, QA, or template chunking strategies. | Proven for service ingestion path | Create/update document paths load the knowledge-base chunk config before building chunks. `fixed_size` uses configured size/overlap with start/end rune metadata; `semantic` honors configured chunk size in the service path; `qa_split` groups question/answer pairs; and `template_based`/`template` split heading-based sections while keeping headings attached to body text. `TestCreateDocumentUsesKnowledgeBaseChunkingConfig`, `TestUpdateDocumentUsesKnowledgeBaseChunkingConfig`, `TestCreateDocumentUsesSemanticChunkingConfig`, `TestCreateDocumentUsesQAChunkingConfig`, `TestCreateDocumentUsesTemplateBasedChunkingConfig`, and `TestUpdateDocumentUsesTemplateBasedChunkingConfig` prove the main ingestion path. |
| Chunk visualization/editing is available end to end. | Partial | Workspace UI has retrieval result and chunk actions, and server chunk update/list paths exist. This audit did not prove original-document visual chunk boundaries, split/merge editing, or colored chunk overlays. |

## 3.3 Document Update Strategy

| Requirement | Status | Evidence |
| --- | --- | --- |
| Knowledge-base config stores full-replace, incremental, and versioned update strategy names. | Proven for config contract | `KnowledgeUpdateStrategyIncremental` is now defined alongside `full_replace` and `versioned`, and config readback preserves `incremental`. |
| Full replace document updates remove old chunks and write new chunks. | Proven for current default | `replaceKnowledgeDocumentChunksWithOptions` deletes chunks for the document and inserts replacement chunks with metadata; document write tests cover chunk and metadata persistence. |
| Incremental update performs diff-aware chunk updates. | Proven for chunk-hash update path | Chunk writes now persist a SHA-256 `contentHash` in chunk metadata. `UpdateKnowledgeDocumentWithOptions` with `incremental` compares new chunk hashes against existing tenant/version-scoped chunk metadata, deletes and reinserts only changed or removed chunk indexes, and leaves unchanged chunk rows untouched. Service updates also use the same diff path before Qdrant upsert, so unchanged chunks are not re-upserted to the vector store. Covered by `TestSQLStoreUpdateKnowledgeDocumentIncrementalReplacesOnlyChangedChunkHashes`, `TestSQLStoreDiffKnowledgeDocumentChunksReturnsOnlyChangedIncrementalChunks`, and `TestUpdateDocumentIncrementalUpsertsOnlyChangedChunksToQdrantVectorStore`. |
| Versioned update preserves multiple chunk versions and supports version-scoped retrieval. | Proven for chunk coexistence, partial for document version history | Document and chunk rows carry `document_version`, retrieval can filter by version, and `versioned` updates now replace only chunks matching the current document version instead of deleting all chunks for the document. `TestSQLStoreUpdateKnowledgeDocumentVersionedReplacesOnlyCurrentVersionChunks` proves the SQL write path preserves other chunk versions. The `knowledge_documents` row still acts as the current-version pointer rather than a full document-version history table. |

## 3.4 Citation and Vector Store

| Requirement | Status | Evidence |
| --- | --- | --- |
| Retrieval results include citation/source metadata and highlights. | Proven for SQL/API/UI citation surfaces | SQL retrieval scans document/chunk identity, version, page number, source URL, original text, matched snippet, confidence, and query highlight positions into `KnowledgeCitation`. `TestSQLStoreRetrieveKnowledgeWithOptionsUsesCrossTenantSafeVectorSearch` proves page/source metadata, `TestSQLStoreRetrieveKnowledgeWithOptionsPopulatesCitationHighlights` proves highlight offsets in the main SQL scan path, `TestKnowledgeHandlerRetrieveAcceptsHybridOptions` proves handler serialization, and `KnowledgePage` tests cover page/source/original text/highlight rendering plus original document preview links. |
| Qdrant vector store is wired for collections, chunk upsert, chunk search adapter calls, `vector_only` retrieval routing, and hybrid vector candidate routing. | Proven for Qdrant adapter, ingestion upsert, `vector_only`, `hybrid`, and `hybrid_rerank` candidate paths | `QdrantVectorStore` can ensure/delete tenant collections, upsert embedded document chunks as tenant-scoped points with document/chunk/version/source payload, and issue Qdrant point searches that map payloads back into `KnowledgeRetrievalResult`. Router wiring can attach the store to the knowledge service; `CreateDocumentWithOptions` upserts embedded chunks after the SQL write; `RetrieveWithOptions` routes `vector_only` retrieval through Qdrant when a vector store and query embedding are configured; configured `hybrid` retrieval gets vector candidates from Qdrant while preserving SQL keyword candidates and weighted fusion; and configured `hybrid_rerank` expands the Qdrant and keyword candidate pools before reranker invocation. Covered by `TestQdrantVectorStoreEnsuresTenantCollection`, `TestQdrantVectorStoreUpsertsTenantChunkPoints`, `TestQdrantVectorStoreSearchesTenantChunkPoints`, `TestCreateDocumentUpsertsEmbeddedChunksToQdrantVectorStore`, `TestRetrieveWithOptionsUsesQdrantSearchForVectorOnlyMode`, `TestRetrieveWithOptionsFusesQdrantVectorAndKeywordResultsForHybridMode`, `TestRetrieveWithOptionsReranksQdrantHybridCandidatesForHybridRerankMode`, and `TestNewKnowledgeServiceWiresQdrantVectorStore`. |
| HNSW/vector payload search is production-backed by Qdrant in the main retrieval path. | Proven for configured service retrieval modes | `vector_only`, `hybrid`, and `hybrid_rerank` can use Qdrant search in the main service path when a vector store and query embedding are configured. SQL keyword retrieval remains active for keyword and hybrid keyword candidates. |

## 7.6 Retrieval Testing UI and API

| Requirement | Status | Evidence |
| --- | --- | --- |
| Retrieval test UI can save scored results as test cases and run evaluations. | Proven for frontend/API flow | `KnowledgePage` exposes save/run controls, feature API tests cover create/list/run calls, and handler tests cover request/response behavior. |
| SQL-backed stores persist and list retrieval test cases. | Proven | `SQLStore.CreateRetrievalTestCase` now inserts into `knowledge_retrieval_test_cases` through a tenant-scoped `INSERT ... SELECT FROM knowledge_bases`, and `SQLStore.ListRetrievalTestCases` reads saved expected results by org/base. `TestSQLStoreCreateRetrievalTestCasePersistsExpectedResult` and `TestSQLStoreListRetrievalTestCasesReturnsSavedExpectedResults` prove the SQL path. |
| Retrieval test reports prove quality of configured RAG modes. | Partial | `RunRetrievalTestCases` compares expected document/chunk/version against actual retrieval results. This is a functional harness, not yet a curated quality benchmark across chunk/rerank/update policies. |

## Verification

Fresh checks for this slice:

- `go test ./internal/knowledge -run 'TestSQLStore(CreateRetrievalTestCasePersistsExpectedResult|ListRetrievalTestCasesReturnsSavedExpectedResults)' -count=1`
- `go test ./internal/knowledge -run 'TestSQLStoreRetrieveKnowledgeWithOptions(PopulatesCitationHighlights|UsesCrossTenantSafeVectorSearch)' -count=1`
- `go test ./internal/knowledge -run 'Test(Create|Update)DocumentUses.*ChunkingConfig' -count=1`
- `go test ./internal/knowledge -run 'TestSQLStore(UpdateKnowledgeDocumentVersionedReplacesOnlyCurrentVersionChunks|CreateKnowledgeDocumentWithOptionsPersistsCrossTenantChunksAndEmbeddings)' -count=1`
- `go test ./internal/knowledge -run 'Test(SQLStoreUpdateKnowledgeDocumentIncrementalReplacesOnlyChangedChunkHashes|SQLStoreDiffKnowledgeDocumentChunksReturnsOnlyChangedIncrementalChunks|UpdateDocumentIncrementalUpsertsOnlyChangedChunksToQdrantVectorStore)' -count=1`
- `go test ./internal/knowledge -run 'TestRetrieveWithOptions(FallsBackWhenHybridRerankerFails|ReranksHybridRerankResults|ExpandsHybridRerankCandidatePool)' -count=1`
- `go test ./internal/knowledge -run 'TestRetrieveWithOptions(UsesQdrantSearchForVectorOnlyMode|RecordsRAGRetrievalLatencyMetric|ReranksHybridRerankResults)' -count=1`
- `go test ./internal/knowledge -run 'TestRetrieveWithOptionsFusesQdrantVectorAndKeywordResultsForHybridMode' -count=1`
- `go test ./internal/knowledge -run 'TestRetrieveWithOptionsReranksQdrantHybridCandidatesForHybridRerankMode' -count=1`
- `go test ./internal/knowledge -run 'TestQdrantVectorStore(UpsertsTenantChunkPoints|SearchesTenantChunkPoints)' -count=1`
- `go test ./internal/knowledge -run 'Test(CreateDocumentUpsertsEmbeddedChunksToQdrantVectorStore|UpdateDocumentUsesTemplateBasedChunkingConfig|QdrantVectorStore)' -count=1`
- `go test ./internal/knowledge/retrieval -run TestKnowledgeResultRerankerCallsCohereCompatibleEndpoint -count=1`
- `go test ./internal/config -run 'TestLoadRAGRerankerConfig|TestLoadRejectsInvalidRAGRerankerTopK' -count=1`
- `go test ./internal/http -run 'TestKnowledgeHandlerRetrieve(UsesKnowledgeBaseRerankTopK|AcceptsHybridOptions|AcceptsVectorOnlyMode)|TestKnowledgeHandlerRunsRetrievalTestCases|TestNewKnowledgeServiceWiresRAGReranker' -count=1`
- `go test ./internal/metrics -run TestFusionObservabilityMetricsRecordWorkflowRAGAndAgentSignals -count=1`
- `go test ./internal/knowledge/... -count=1`
- `go test ./internal/http -run 'TestKnowledgeHandler|TestNewKnowledgeServiceWiresQdrantVectorStore|TestRegisterKnowledgeRoutes|TestKnowledgeRoutes|TestRegisterKnowledge' -count=1`

All Go commands above were run from `src/server` with absolute `GOCACHE=/tmp/oblivious-gocache` and `GOMODCACHE=/tmp/oblivious-gomodcache`.

## Current Conclusion

The Knowledge/RAG row remains `Partial`, not `Proven`.

The current Knowledge/RAG slices close the highest-priority API/store contract gaps for retrieval mode naming, knowledge-base RAG config readback, SQL-backed retrieval test case persistence, fixed/semantic/QA/template chunking config in the main document create/update ingestion path, configured HTTP reranking for `hybrid_rerank`, reranker outage fallback metrics, incremental chunk-hash update semantics, versioned chunk coexistence for update writes, citation highlight/source metadata in SQL/API/UI surfaces, Qdrant chunk upsert/search adapter behavior, Qdrant-backed `vector_only` retrieval, Qdrant-backed vector candidates for configured `hybrid`/`hybrid_rerank` retrieval while preserving SQL keyword fusion, and knowledge-base `rerankTopK` candidate-pool expansion before final result truncation. The remaining high-value work is:

1. Prove or complete chunk visualization/editing, full document-version history, and curated retrieval quality benchmark coverage.
