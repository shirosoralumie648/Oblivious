-- Preserve relay file mapping audit history while hiding provider-deleted
-- files from tenant list/get passthrough paths.
ALTER TABLE relay_file_mappings
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_org_created_active
    ON relay_file_mappings(organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_user_created_active
    ON relay_file_mappings(user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_deleted_at
    ON relay_file_mappings(deleted_at)
    WHERE deleted_at IS NOT NULL;
