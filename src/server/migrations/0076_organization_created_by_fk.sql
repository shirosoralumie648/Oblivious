-- Add the tenant creator foreign key after users exists.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'organizations_created_by_user_id_fkey'
          AND conrelid = 'organizations'::regclass
    ) THEN
        ALTER TABLE organizations
        ADD CONSTRAINT organizations_created_by_user_id_fkey
        FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
END
$$;
