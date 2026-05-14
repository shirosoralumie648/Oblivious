-- Quotas and Billing Sessions
-- User quota management and billing tracking

-- Quotas
-- Stores user balance and usage tracking
CREATE TABLE IF NOT EXISTS quotas (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance DECIMAL(15,6) DEFAULT 0,  -- Balance in USD
    used DECIMAL(15,6) DEFAULT 0,     -- Total used amount
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

-- Billing Sessions
-- Tracks individual billing sessions with idempotency
CREATE TABLE IF NOT EXISTS billing_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id TEXT,
    model TEXT,
    api_type TEXT,
    idempotency_key TEXT NOT NULL,
    pre_authorized_amt DECIMAL(15,6) DEFAULT 0,  -- Pre-authorized amount
    settled_amt DECIMAL(15,6) DEFAULT 0,         -- Actually settled amount
    status TEXT DEFAULT 'preauthorized',         -- 'preauthorized' | 'settled' | 'refunded'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ,
    UNIQUE(idempotency_key)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_quotas_user_id ON quotas(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_sessions_user_id ON billing_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_sessions_idempotency ON billing_sessions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_billing_sessions_status ON billing_sessions(status);
CREATE INDEX IF NOT EXISTS idx_billing_sessions_created_at ON billing_sessions(created_at DESC);

-- Packages
-- Subscription packages for quota allocation
CREATE TABLE IF NOT EXISTS packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    quota_amount DECIMAL(15,6) NOT NULL,  -- Quota amount in USD
    price DECIMAL(10,2) NOT NULL,        -- Price in USD
    duration_days INT,                   -- NULL means permanent
    is_active BOOLEAN DEFAULT true,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Subscriptions
-- User subscriptions to packages
CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_id TEXT NOT NULL REFERENCES packages(id),
    status TEXT DEFAULT 'active',        -- 'active' | 'expired' | 'cancelled'
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Topup Orders
-- One-time quota topup orders
CREATE TABLE IF NOT EXISTS topup_orders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(15,6) NOT NULL,       -- Quota amount to add
    money DECIMAL(10,2) NOT NULL,        -- Money paid in USD
    status TEXT DEFAULT 'pending',       -- 'pending' | 'paid' | 'failed'
    trade_no TEXT,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for packages/subscriptions/topup
CREATE INDEX IF NOT EXISTS idx_packages_is_active ON packages(is_active);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_topup_orders_user_id ON topup_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_topup_orders_status ON topup_orders(status);