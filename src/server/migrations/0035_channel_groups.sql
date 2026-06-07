-- User-group aware relay channel routing.

ALTER TABLE IF EXISTS channels
    ADD COLUMN IF NOT EXISTS groups TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_channels_groups
    ON channels USING GIN (groups);
