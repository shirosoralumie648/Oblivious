-- Knowledge retrieval test cases saved from scored RAG results.

CREATE TABLE IF NOT EXISTS knowledge_retrieval_test_cases (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    expected_document_id TEXT NOT NULL,
    expected_document_title TEXT NOT NULL DEFAULT '',
    expected_document_version TEXT NOT NULL DEFAULT '',
    expected_chunk_id TEXT NOT NULL,
    expected_chunk_index INTEGER NOT NULL DEFAULT 0,
    expected_snippet TEXT NOT NULL DEFAULT '',
    expected_result JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_test_cases_org_base_created
    ON knowledge_retrieval_test_cases(organization_id, knowledge_base_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_test_cases_org_base_query
    ON knowledge_retrieval_test_cases(organization_id, knowledge_base_id, query);
