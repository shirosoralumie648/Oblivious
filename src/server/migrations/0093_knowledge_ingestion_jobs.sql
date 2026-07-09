-- Durable Knowledge/RAG ingestion job lifecycle for async parse/chunk/embed/create.

CREATE TABLE IF NOT EXISTS knowledge_ingestion_jobs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  raw_content BYTEA NOT NULL DEFAULT '',
  raw_filename TEXT NOT NULL DEFAULT '',
  raw_content_type TEXT NOT NULL DEFAULT '',
  raw_size_bytes BIGINT NOT NULL DEFAULT 0,
  document_version TEXT NOT NULL DEFAULT 'v1',
  update_strategy TEXT NOT NULL DEFAULT 'full_replace',
  source_url TEXT NOT NULL DEFAULT '',
  page_number INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  error TEXT NOT NULL DEFAULT '',
  attempts INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  locked_at TIMESTAMPTZ,
  locked_by TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead_letter')),
  CHECK (attempts >= 0),
  CHECK (max_attempts > 0),
  CHECK (raw_size_bytes >= 0),
  CHECK (page_number >= 0)
);

CREATE INDEX IF NOT EXISTS knowledge_ingestion_jobs_claim_idx
  ON knowledge_ingestion_jobs (status, available_at, created_at, id);

CREATE INDEX IF NOT EXISTS knowledge_ingestion_jobs_org_base_idx
  ON knowledge_ingestion_jobs (organization_id, knowledge_base_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS knowledge_ingestion_jobs_document_idx
  ON knowledge_ingestion_jobs (organization_id, document_id, updated_at DESC)
  WHERE document_id <> '';
