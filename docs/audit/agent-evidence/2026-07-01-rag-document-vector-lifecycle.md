# Agent Evidence: RAG document vector lifecycle

Date: 2026-07-01

Agent: main

Commit: pending

## Runtime Claim

Knowledge document deletion now cleans tenant-scoped Qdrant points for the document before deleting the SQL document row.

The service deletes document vectors through `KnowledgeDocumentVectorDeleter` for both knowledge-base-scoped deletion and document-id-only deletion. The document-id-only path resolves the document's knowledge base first, then uses the organization, knowledge base, and document id to delete Qdrant points.

Keyword-only retrieval now reports `retrievalMethod: "keyword"` instead of labeling keyword fallback as `embedding_rag`.

## Reference Inputs

```text
docs/audit/product-roadmap-v2-from-reference.md - P0 durable RAG ingestion and retrieval evidence requirement.
src/server/internal/knowledge/service.go - document deletion, retrieval normalization, and Qdrant integration.
src/server/internal/knowledge/store.go - SQL document delete and retrieval queries.
src/server/internal/knowledge/qdrant_store.go - document point delete API.
```

## Oblivious Files Changed

```text
src/server/internal/knowledge/service.go
src/server/internal/knowledge/store.go
src/server/internal/knowledge/service_test.go
src/server/internal/knowledge/store_test.go
docs/audit/agent-evidence/2026-07-01-rag-document-vector-lifecycle.md
```

## Contract Changes

Retrieval method values now distinguish keyword-only retrieval:

```text
embedding_rag
keyword
```

Document deletion with a configured vector store now fails closed if Qdrant document point deletion fails.

## Verification Commands

```text
command: git diff --check -- src/server/internal/knowledge/service.go src/server/internal/knowledge/store.go src/server/internal/knowledge/service_test.go src/server/internal/knowledge/store_test.go
result: passed; Git reported LF-to-CRLF warnings only.

command: go test ./internal/knowledge -run 'TestDeleteDocumentDeletesTenantScopedQdrantPointsBeforeSQLDocument|TestDeleteDocumentByIDResolvesScopeBeforeQdrantCleanup|TestRetrieveWithKeywordModeBackfillsKeywordRetrievalMethod|TestSQLStoreRetrieveKnowledgeWithOptionsWithoutEmbeddingUsesKeywordOnlyQuery' -count=1 -v
result: blocked; Go is not on PATH. Error: /usr/bin/bash: line 1: go: command not found.
```

## Runtime Evidence IDs

```text
organization_id: org_knowledge
knowledge_base_id: kb_qdrant
document_id: doc_qdrant
document_id: doc_by_id
```

## Failure Evidence

The service deletion path now returns Qdrant deletion errors before SQL document deletion can proceed, preventing the zombie-vector case where SQL no longer owns the document but Qdrant can still retrieve it.

## Unsupported / Deferred Surfaces

Document create/update ingestion is still synchronous in the request path. A full commercial RAG ingestion lifecycle still needs durable index jobs, retryable worker ownership, and a pending-to-ready document status gate.

## Known Residual Risk

Vector deletion and SQL deletion are still two separate external operations. Without a durable outbox/job table, a process crash between Qdrant deletion and SQL deletion can leave a document row that needs reindexing. This is less dangerous than deleted SQL with live vectors, but it is not a complete commercial ingestion ledger.
