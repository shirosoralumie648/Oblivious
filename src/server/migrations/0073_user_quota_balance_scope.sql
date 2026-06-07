-- User-scoped quota balance isolation
-- Billing 6.2: organization/default balances and user-level balances must not share one row.

ALTER TABLE IF EXISTS quotas
    ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'organization';

UPDATE quotas
SET scope = 'organization'
WHERE scope IS NULL OR scope = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'quotas_scope_check'
    ) THEN
        ALTER TABLE quotas
            ADD CONSTRAINT quotas_scope_check CHECK (scope IN ('organization', 'user'));
    END IF;
END $$;

DROP INDEX IF EXISTS idx_quotas_unique_organization;

CREATE UNIQUE INDEX IF NOT EXISTS idx_quotas_unique_organization_scope
    ON quotas(organization_id)
    WHERE scope = 'organization';

CREATE UNIQUE INDEX IF NOT EXISTS idx_quotas_unique_user_scope
    ON quotas(organization_id, user_id)
    WHERE scope = 'user';

CREATE INDEX IF NOT EXISTS idx_quotas_scope_organization_user
    ON quotas(scope, organization_id, user_id);
