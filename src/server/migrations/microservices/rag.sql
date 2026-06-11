-- RAG Microservice Schema
-- Tables: knowledge_bases, documents, document_chunks, document_versions, retrieval_test_cases

CREATE TABLE IF NOT EXISTS knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    embedding_model VARCHAR(100) NOT NULL,
    chunk_size INTEGER NOT NULL DEFAULT 512,
    chunk_overlap INTEGER NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::JSONB
);

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    source_url TEXT,
    content_type VARCHAR(100) NOT NULL,
    file_size BIGINT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::JSONB,
    CONSTRAINT fk_knowledge_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id)
);

CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),
    token_count INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::JSONB,
    CONSTRAINT fk_document FOREIGN KEY (document_id) REFERENCES documents(id),
    CONSTRAINT unique_document_chunk UNIQUE (document_id, chunk_index)
);

CREATE TABLE IF NOT EXISTS document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    content TEXT NOT NULL,
    change_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    metadata JSONB DEFAULT '{}'::JSONB,
    CONSTRAINT fk_document_version FOREIGN KEY (document_id) REFERENCES documents(id),
    CONSTRAINT unique_document_version UNIQUE (document_id, version_number)
);

CREATE TABLE IF NOT EXISTS retrieval_test_cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    expected_document_ids UUID[] NOT NULL,
    expected_rank INTEGER[],
    actual_document_ids UUID[],
    relevance_scores FLOAT[],
    passed BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::JSONB,
    CONSTRAINT fk_kb_test FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id)
);

CREATE INDEX idx_documents_kb ON documents(knowledge_base_id);
CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_chunks_document ON document_chunks(document_id);
CREATE INDEX idx_chunks_embedding ON document_chunks USING ivfflat (embedding vector_cosine_ops);
CREATE INDEX idx_versions_document ON document_versions(document_id);
CREATE INDEX idx_tests_kb ON retrieval_test_cases(knowledge_base_id);
