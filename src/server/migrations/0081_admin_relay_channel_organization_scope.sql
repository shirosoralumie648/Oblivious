-- Tenant scope for Admin-managed Relay channels.

ALTER TABLE IF EXISTS channels
    ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE CASCADE;

UPDATE channels
SET organization_id = (
    SELECT id
    FROM organizations
    ORDER BY created_at ASC, id ASC
    LIMIT 1
)
WHERE organization_id IS NULL
  AND EXISTS (SELECT 1 FROM organizations);

CREATE INDEX IF NOT EXISTS idx_channels_organization_provider
    ON channels(organization_id, provider);

CREATE INDEX IF NOT EXISTS idx_channels_organization_enabled
    ON channels(organization_id, enabled);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'channels_organization_id_fkey'
          AND conrelid = 'channels'::regclass
    ) THEN
        ALTER TABLE channels
            ADD CONSTRAINT channels_organization_id_fkey
            FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
            NOT VALID;
    END IF;
END
$$;
