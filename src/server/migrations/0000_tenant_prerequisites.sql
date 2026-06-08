-- Tenant prerequisite table for early service migrations.
-- Several pre-v04 service migrations reference organizations before the
-- original tenant-foundation migration runs. Keep this forward-only migration
-- minimal and add cross-table constraints after users exists.

CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_by_user_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$'),
    CHECK (status IN ('active', 'disabled', 'archived'))
);
