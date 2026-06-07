-- Knowledge hybrid retrieval indexes

CREATE INDEX IF NOT EXISTS knowledge_document_chunks_fts_idx
  ON knowledge_document_chunks
  USING gin (to_tsvector('simple', content));

CREATE INDEX IF NOT EXISTS knowledge_documents_title_fts_idx
  ON knowledge_documents
  USING gin (to_tsvector('simple', title));
