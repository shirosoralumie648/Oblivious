-- Knowledge RAG indexing
-- v08 Phase 27: PROD-03

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE IF EXISTS knowledge_document_chunks
  ADD COLUMN IF NOT EXISTS embedding vector(1536),
  ADD COLUMN IF NOT EXISTS embedding_model TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS indexed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS knowledge_document_chunks_embedding_hnsw_idx
  ON knowledge_document_chunks
  USING hnsw (embedding vector_cosine_ops)
  WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS knowledge_document_chunks_org_doc_idx
  ON knowledge_document_chunks (organization_id, document_id, chunk_index);
