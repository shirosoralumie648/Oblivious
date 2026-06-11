-- Agent Microservice Schema

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    config JSONB,
    status VARCHAR(50) DEFAULT 'idle',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'running',
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    run_id UUID REFERENCES agent_runs(id) ON DELETE CASCADE,
    memory_type VARCHAR(100) NOT NULL,
    content JSONB NOT NULL,
    embedding vector(1536),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_plan_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_number INTEGER NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_plan_step_execution (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step_id UUID NOT NULL REFERENCES agent_plan_steps(id) ON DELETE CASCADE,
    execution_data JSONB,
    result JSONB,
    error TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agent_runs_agent_id ON agent_runs(agent_id);
CREATE INDEX idx_agent_runs_status ON agent_runs(status);
CREATE INDEX idx_agent_memories_agent_id ON agent_memories(agent_id);
CREATE INDEX idx_agent_memories_run_id ON agent_memories(run_id);
CREATE INDEX idx_agent_plan_steps_run_id ON agent_plan_steps(run_id);
CREATE INDEX idx_agent_plan_step_execution_step_id ON agent_plan_step_execution(step_id);
