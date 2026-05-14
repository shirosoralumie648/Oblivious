-- pgvector Extension and Memory Tables
-- Enables vector similarity search for RAG

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Memory Documents
-- Stores user documents for RAG retrieval
CREATE TABLE IF NOT EXISTS memory_documents (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT,
    content TEXT NOT NULL,
    source_type TEXT DEFAULT 'manual',  -- 'manual' | 'upload' | 'url'
    source_url TEXT,
    metadata JSONB DEFAULT '{}',
    total_chunks INT DEFAULT 0,
    embedding_model TEXT DEFAULT 'text-embedding-3-small',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Memory Chunks
-- Stores document chunks with embeddings for vector search
CREATE TABLE IF NOT EXISTS memory_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES memory_documents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    chunk_index INT NOT NULL,
    embedding vector(1536),  -- OpenAI text-embedding-3-small dimension
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for memory_documents
CREATE INDEX IF NOT EXISTS idx_memory_documents_user_id ON memory_documents(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_documents_created_at ON memory_documents(created_at DESC);

-- Indexes for memory_chunks
CREATE INDEX IF NOT EXISTS idx_memory_chunks_user_id ON memory_chunks(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_chunks_document_id ON memory_chunks(document_id);

-- Vector index for similarity search (IVFFlat for approximate nearest neighbor)
-- Note: Requires sufficient data before creating (typically 1000+ rows)
-- For production, consider using HNSW index: USING hnsw(embedding vector_cosine_ops)
CREATE INDEX IF NOT EXISTS idx_memory_chunks_embedding ON memory_chunks
    USING ivfflat(embedding vector_cosine_ops) WITH (lists = 100);

-- Unique constraint for chunk index per document
CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_chunks_document_chunk ON memory_chunks(document_id, chunk_index);
