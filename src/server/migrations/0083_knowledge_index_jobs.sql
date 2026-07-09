-- Durable Knowledge/RAG indexing job lifecycle.

ALTER TABLE IF EXISTS knowledge_documents
  ADD COLUMN IF NOT EXISTS index_status TEXT NOT NULL DEFAULT 'ready',
  ADD COLUMN IF NOT EXISTS index_error TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS indexed_at TIMESTAMPTZ;

ALTER TABLE IF EXISTS knowledge_documents
  DROP CONSTRAINT IF EXISTS knowledge_documents_index_status_check;

ALTER TABLE IF EXISTS knowledge_documents
  ADD CONSTRAINT knowledge_documents_index_status_check
  CHECK (index_status IN ('pending', 'indexing', 'ready', 'failed'));

CREATE TABLE IF NOT EXISTS knowledge_index_jobs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL,
  operation TEXT NOT NULL DEFAULT 'upsert_document',
  status TEXT NOT NULL DEFAULT 'pending',
  error TEXT NOT NULL DEFAULT '',
  attempts INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  locked_at TIMESTAMPTZ,
  locked_by TEXT NOT NULL DEFAULT '',
  completed_by TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (operation IN ('upsert_document', 'delete_document')),
  CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead_letter')),
  CHECK (attempts >= 0),
  CHECK (max_attempts > 0)
);

CREATE INDEX IF NOT EXISTS knowledge_index_jobs_claim_idx
  ON knowledge_index_jobs (status, available_at, created_at, id);

CREATE INDEX IF NOT EXISTS knowledge_index_jobs_org_document_idx
  ON knowledge_index_jobs (organization_id, knowledge_base_id, document_id, updated_at DESC);
