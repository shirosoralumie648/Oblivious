-- Channel-level default weight used when no explicit model route exists.

ALTER TABLE IF EXISTS channels
    ADD COLUMN IF NOT EXISTS weight INTEGER NOT NULL DEFAULT 100;

UPDATE channels
SET weight = 100
WHERE weight <= 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'channels_weight_positive'
    ) THEN
        ALTER TABLE channels
            ADD CONSTRAINT channels_weight_positive CHECK (weight > 0) NOT VALID;
    END IF;
END
$$;
