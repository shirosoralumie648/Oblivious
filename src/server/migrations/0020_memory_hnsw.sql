-- HNSW Index Migration
-- Replaces IVFFlat index with HNSW for better recall and performance
-- as decided in 02-CONTEXT.md (Decision D-13).

-- Drop the existing IVFFlat index created in 0016_pgvector.sql
DROP INDEX IF EXISTS idx_memory_chunks_embedding;

-- Create HNSW index with vector_cosine_ops operator class
-- HNSW provides faster build times and better recall than IVFFlat
-- for production workloads with growing data volumes
CREATE INDEX IF NOT EXISTS idx_memory_chunks_embedding ON memory_chunks
    USING hnsw(embedding vector_cosine_ops);
