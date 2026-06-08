-- Backfill columns required before the enhanced knowledge migration creates
-- organization-scoped indexes on tables that may already exist from v03.

ALTER TABLE IF EXISTS knowledge_bases
    ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
