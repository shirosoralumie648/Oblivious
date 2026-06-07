ALTER TABLE token_rate_limits
    ADD COLUMN IF NOT EXISTS max_tokens_per_request BIGINT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'token_rate_limits_max_tokens_per_request_check'
    ) THEN
        ALTER TABLE token_rate_limits
            ADD CONSTRAINT token_rate_limits_max_tokens_per_request_check
            CHECK (max_tokens_per_request >= 0);
    END IF;
END $$;
