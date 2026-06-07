-- Agent enhanced tables: runs, tool runs, and memories.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS agent_runs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    memory_enabled BOOLEAN NOT NULL DEFAULT false,
    memory_searched BOOLEAN NOT NULL DEFAULT false,
    memory_result_count INTEGER NOT NULL DEFAULT 0,
    iteration_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    final_message_id TEXT,
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('running', 'pending_approval', 'completed', 'failed', 'max_iterations_reached', 'token_budget_exceeded'))
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_org_conversation_created
    ON agent_runs(organization_id, conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_runs_org_status_created
    ON agent_runs(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_tool_runs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_type TEXT NOT NULL,
    server_id TEXT NOT NULL DEFAULT '',
    arguments JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    approval_status TEXT NOT NULL,
    approved_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    approval_decision_reason TEXT NOT NULL DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    result_content TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending_approval', 'running', 'completed', 'failed', 'rejected')),
    CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    UNIQUE (organization_id, run_id, tool_call_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_tool_runs_org_run_created
    ON agent_tool_runs(organization_id, run_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_agent_tool_runs_org_status_created
    ON agent_tool_runs(organization_id, status, created_at DESC);

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
