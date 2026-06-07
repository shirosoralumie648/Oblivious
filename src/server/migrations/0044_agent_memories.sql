-- First-class Agent memory records.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS agent_memories (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (type IN ('short_term', 'long_term', 'user_managed'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memories_org_user_created
    ON agent_memories(organization_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_memories_org_agent_created
    ON agent_memories(organization_id, agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_memories_org_type_created
    ON agent_memories(organization_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_memories_embedding
    ON agent_memories USING hnsw (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;
