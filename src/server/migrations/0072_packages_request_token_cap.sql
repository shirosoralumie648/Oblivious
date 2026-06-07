ALTER TABLE packages
    ADD COLUMN IF NOT EXISTS max_tokens_per_request INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'packages_max_tokens_per_request_check'
    ) THEN
        ALTER TABLE packages
            ADD CONSTRAINT packages_max_tokens_per_request_check
            CHECK (max_tokens_per_request >= 0);
    END IF;
END $$;
