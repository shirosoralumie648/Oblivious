-- Knowledge document update strategy and version metadata.

ALTER TABLE IF EXISTS knowledge_bases
  ADD COLUMN IF NOT EXISTS update_strategy TEXT NOT NULL DEFAULT 'full_replace';

ALTER TABLE IF EXISTS knowledge_documents
  ADD COLUMN IF NOT EXISTS document_version TEXT NOT NULL DEFAULT 'v1',
  ADD COLUMN IF NOT EXISTS update_strategy TEXT NOT NULL DEFAULT 'full_replace';

ALTER TABLE IF EXISTS knowledge_document_chunks
  ADD COLUMN IF NOT EXISTS document_version TEXT NOT NULL DEFAULT 'v1',
  ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS knowledge_documents_org_base_version_idx
  ON knowledge_documents (organization_id, knowledge_base_id, document_version);

CREATE INDEX IF NOT EXISTS knowledge_document_chunks_org_version_idx
  ON knowledge_document_chunks (organization_id, document_version);
