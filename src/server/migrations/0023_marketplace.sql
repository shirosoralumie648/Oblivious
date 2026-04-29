-- Marketplace
-- D-17: Agent publishing and review pipeline
-- D-18: Publish metadata, D-19: Version management, D-20: Installs, D-27: Reviews

-- Published Agents
CREATE TABLE IF NOT EXISTS published_agents (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    icon_url TEXT,
    category_id TEXT,
    tags TEXT[] NOT NULL DEFAULT '{}',
    tools JSONB,
    example_conversations JSONB,
    system_prompt TEXT,
    visibility TEXT NOT NULL DEFAULT 'private',
    status TEXT NOT NULL DEFAULT 'draft',
    review_reason TEXT,
    pricing_type TEXT NOT NULL DEFAULT 'free',
    pricing_amount DECIMAL(10,2) DEFAULT 0,
    install_count INTEGER NOT NULL DEFAULT 0,
    rating_avg DECIMAL(3,2) DEFAULT 0,
    rating_count INTEGER NOT NULL DEFAULT 0,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Agent Versions (D-19)
CREATE TABLE IF NOT EXISTS agent_versions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    changelog TEXT,
    metadata JSONB,
    status TEXT NOT NULL DEFAULT 'pending_review',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, version)
);

-- Agent Reviews (D-27)
CREATE TABLE IF NOT EXISTS agent_reviews (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, user_id)
);

-- Agent Installs (D-20)
CREATE TABLE IF NOT EXISTS agent_installs (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    version_id TEXT REFERENCES agent_versions(id),
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, user_id)
);

-- Indexes for published_agents
CREATE INDEX IF NOT EXISTS idx_published_agents_owner ON published_agents(owner_id);
CREATE INDEX IF NOT EXISTS idx_published_agents_status ON published_agents(status);
CREATE INDEX IF NOT EXISTS idx_published_agents_visibility ON published_agents(visibility);
CREATE INDEX IF NOT EXISTS idx_published_agents_rating ON published_agents(rating_avg DESC);
CREATE INDEX IF NOT EXISTS idx_published_agents_installs ON published_agents(install_count DESC);

-- Indexes for agent versions, reviews, installs
CREATE INDEX IF NOT EXISTS idx_agent_versions_agent ON agent_versions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_reviews_agent ON agent_reviews(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_installs_user ON agent_installs(user_id);
