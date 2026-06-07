-- Knowledge enhanced tables: knowledge bases, documents, document chunks.
-- knowledge_bases uses IF NOT EXISTS since it may already exist from earlier migration.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS knowledge_bases (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active',
    embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('active', 'archived', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_org_updated
    ON knowledge_bases(organization_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'manual',
    source_url TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    chunk_count INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_type IN ('manual', 'upload', 'url', 'api')),
    CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'archived')),
    CHECK (chunk_count >= 0),
    CHECK (total_tokens >= 0)
);

CREATE INDEX IF NOT EXISTS idx_documents_org_kb_created
    ON documents(organization_id, knowledge_base_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_documents_org_status
    ON documents(organization_id, status);

CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),
    embedding_model TEXT NOT NULL DEFAULT '',
    token_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (chunk_index >= 0),
    CHECK (token_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_chunks_doc_chunk
    ON document_chunks(document_id, chunk_index);

CREATE INDEX IF NOT EXISTS idx_document_chunks_org_doc
    ON document_chunks(organization_id, document_id, chunk_index);

CREATE INDEX IF NOT EXISTS idx_document_chunks_embedding
    ON document_chunks USING hnsw (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;
