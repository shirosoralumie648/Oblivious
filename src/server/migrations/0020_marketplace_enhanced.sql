-- Marketplace enhanced tables: templates.

CREATE TABLE IF NOT EXISTS marketplace_templates (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    template_data JSONB NOT NULL,
    category TEXT,
    tags TEXT[] NOT NULL DEFAULT '{}',
    downloads_count INTEGER NOT NULL DEFAULT 0,
    rating_avg DECIMAL(3,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (type IN ('workflow', 'bot', 'plugin')),
    CHECK (btrim(name) <> '')
);

CREATE INDEX IF NOT EXISTS idx_marketplace_templates_org_type
    ON marketplace_templates(organization_id, type);
CREATE INDEX IF NOT EXISTS idx_marketplace_templates_type
    ON marketplace_templates(type);
CREATE INDEX IF NOT EXISTS idx_marketplace_templates_category
    ON marketplace_templates(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_templates_tags
    ON marketplace_templates USING GIN(tags);
