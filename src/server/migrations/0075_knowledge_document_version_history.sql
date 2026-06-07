CREATE TABLE IF NOT EXISTS knowledge_document_versions (
  id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
  knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  document_version TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  update_strategy TEXT NOT NULL DEFAULT 'versioned',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, knowledge_base_id, document_id, document_version)
);

CREATE INDEX IF NOT EXISTS knowledge_document_versions_org_base_doc_updated_idx
  ON knowledge_document_versions (organization_id, knowledge_base_id, document_id, updated_at DESC);

INSERT INTO knowledge_document_versions (
  id,
  document_id,
  knowledge_base_id,
  organization_id,
  document_version,
  title,
  content,
  update_strategy,
  created_at,
  updated_at
)
SELECT
  'kdv_' || md5(d.organization_id || ':' || d.knowledge_base_id || ':' || d.id || ':' || COALESCE(NULLIF(d.document_version, ''), 'v1')) AS id,
  d.id,
  d.knowledge_base_id,
  d.organization_id,
  COALESCE(NULLIF(d.document_version, ''), 'v1') AS document_version,
  d.title,
  d.content,
  COALESCE(NULLIF(d.update_strategy, ''), kb.update_strategy, 'versioned') AS update_strategy,
  d.created_at,
  d.updated_at
FROM knowledge_documents d
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE d.organization_id IS NOT NULL
ON CONFLICT (organization_id, knowledge_base_id, document_id, document_version)
DO UPDATE SET
  title = EXCLUDED.title,
  content = EXCLUDED.content,
  update_strategy = EXCLUDED.update_strategy,
  updated_at = EXCLUDED.updated_at;
