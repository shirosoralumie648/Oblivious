-- Categories and Tags
-- D-28: Predefined categories + publisher tags, D-30: Full-text search

-- Categories table
CREATE TABLE IF NOT EXISTS categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default categories (D-28)
INSERT INTO categories (id, name, slug, display_order) VALUES
    ('cat_chat', 'Chat', 'chat', 1),
    ('cat_coding', 'Coding', 'coding', 2),
    ('cat_writing', 'Writing', 'writing', 3),
    ('cat_image', 'Image Generation', 'image-generation', 4),
    ('cat_data', 'Data Analysis', 'data-analysis', 5),
    ('cat_productivity', 'Productivity', 'productivity', 6),
    ('cat_entertainment', 'Entertainment', 'entertainment', 7),
    ('cat_education', 'Education', 'education', 8)
ON CONFLICT (id) DO NOTHING;

-- Agent Tags junction table
CREATE TABLE IF NOT EXISTS agent_tags (
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (agent_id, tag)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);
CREATE INDEX IF NOT EXISTS idx_agent_tags_tag ON agent_tags(tag);

-- Full-text search on published_agents (D-30)
CREATE INDEX IF NOT EXISTS idx_published_agents_fts ON published_agents
    USING gin(to_tsvector('english', name || ' ' || description));
