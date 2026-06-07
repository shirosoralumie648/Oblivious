-- Channel-level cost metadata for relay billing margin reporting.

ALTER TABLE IF EXISTS channels
    ADD COLUMN IF NOT EXISTS estimated_cost_per_1k DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cost_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1;
