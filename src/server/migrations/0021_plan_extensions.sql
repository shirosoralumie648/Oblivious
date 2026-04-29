-- Plan Extensions
-- Extended packages and subscriptions table for hybrid pricing model
-- D-10: token-based quota, D-11: model access + agent limits, D-14: public packages + scheduled plan changes

-- Helper function for auto-updating updated_at timestamps
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Extend packages table with hybrid pricing fields
ALTER TABLE packages ADD COLUMN IF NOT EXISTS token_quota INTEGER NOT NULL DEFAULT 1000000;
ALTER TABLE packages ADD COLUMN IF NOT EXISTS model_access TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS agent_limit INTEGER NOT NULL DEFAULT 10;
ALTER TABLE packages ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE packages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Trigger to auto-update updated_at on row change
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_packages_updated_at') THEN
        CREATE TRIGGER trg_packages_updated_at
            BEFORE UPDATE ON packages
            FOR EACH ROW
            EXECUTE FUNCTION update_timestamp();
    END IF;
END
$$;

-- Extend subscriptions table with billing period tracking and scheduled plan changes
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS next_plan_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ;

-- Indexes for plan extensions
CREATE INDEX IF NOT EXISTS idx_packages_is_public ON packages(is_public);
CREATE INDEX IF NOT EXISTS idx_subscriptions_next_plan_id ON subscriptions(next_plan_id);

-- Extend users table with plan assignment and status (D-12: user lifecycle)
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan_id TEXT REFERENCES packages(id);
ALTER TABLE users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
CREATE INDEX IF NOT EXISTS idx_users_plan_id ON users(plan_id);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
