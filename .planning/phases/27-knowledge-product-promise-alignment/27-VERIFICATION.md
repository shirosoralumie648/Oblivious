# Phase 27 Verification — Knowledge Product Promise Alignment

## Scope

Phase 27 closes only `PROD-03`: Knowledge behavior must match customer-facing RAG copy by using Relay embeddings, pgvector chunk retrieval, and source citations.

Phase 28 commercial UX hardening, Phase 29 public docs/onboarding/pricing/operator guides, Phase 30 end-to-end commercial journeys, and the final commercial completion audit remain required before final commercial readiness.

## Evidence

### Focused Knowledge Service Tests

Command:

```bash
cd src/server && go test ./internal/knowledge -run 'CreateDocumentIndexes|UpdateDocumentReindexes|RetrieveUsesQueryEmbedding|EmbedderMissing|CreateDocumentCreates|UpdateDocumentUpdates|RetrieveNormalizes' -count=1
```

Result: PASS.

Evidence covered:
- Document create/update calls `EmbedBatch` and passes indexed chunks to the store.
- Retrieval calls `Embed` for the normalized query.
- Retrieval returns `embedding_rag` and source citation fields.
- Missing embedder returns an explicit configuration error instead of falling back to text retrieval.

### Focused HTTP Handler Tests

Command:

```bash
cd src/server && go test ./internal/http -run 'KnowledgeHandlerRetrieve|KnowledgeHandler' -count=1
```

Result: PASS.

Evidence covered:
- Knowledge retrieve responses preserve the API envelope.
- Retrieval payloads include `chunkId`, `chunkIndex`, `retrievalMethod`, `similarity`, and `source`.

### DB-Backed pgvector Knowledge Tests

Command:

```bash
cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32771/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/knowledge ./internal/http -run 'Knowledge|RAG|Retrieve|Citation|Source|Tenant' -count=1
```

Result: PASS.

Environment:
- PostgreSQL pgvector container: `oblivious-phase27-pgvector`
- Host port: `127.0.0.1:32771`

Evidence covered:
- `knowledge_document_chunks.embedding` stores vectors from indexed chunks.
- Retrieval orders chunks by pgvector similarity.
- Retrieval is organization-scoped and rejects cross-tenant chunk leakage.
- Source citation fields round trip through store and HTTP paths.

### Focused Web KnowledgePage Tests

Command:

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test -- KnowledgePage --runInBand
```

Result: PASS. The command ran 33 test files and 114 tests.

Evidence covered:
- Workspace Knowledge copy describes embedding-backed, source-cited RAG.
- Retrieved results render source citation text with document title, chunk index, retrieval method, and similarity.

### Docs And Gate Checks

Command:

```bash
bash scripts/check.sh docs
```

Result: PASS.

Command:

```bash
git diff --check
```

Result: PASS.

## Boundary

`PROD-03` is complete. The Product Completeness Gate and final commercial readiness remain open until Phase 28 through Phase 30 and `AUDIT-01` are complete.
