-- Marketplace Microservice Schema

-- Marketplace Agents (AI agents available for purchase/install)
CREATE TABLE IF NOT EXISTS marketplace_agents (
    agent_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(50) NOT NULL,
    author_id UUID NOT NULL,
    price DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    category VARCHAR(100),
    tags TEXT[],
    metadata JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Agent Installations
CREATE TABLE IF NOT EXISTS marketplace_installs (
    install_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES marketplace_agents(agent_id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    installed_version VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uninstalled_at TIMESTAMPTZ,
    UNIQUE(agent_id, user_id)
);

-- Agent Reviews
CREATE TABLE IF NOT EXISTS marketplace_reviews (
    review_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES marketplace_agents(agent_id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, user_id)
);

-- Transactions
CREATE TABLE IF NOT EXISTS marketplace_transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES marketplace_agents(agent_id) ON DELETE CASCADE,
    buyer_id UUID NOT NULL,
    seller_id UUID NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    payment_method VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Settlements (payouts to sellers)
CREATE TABLE IF NOT EXISTS marketplace_settlements (
    settlement_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id UUID NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    transaction_ids UUID[],
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ
);

-- Agent Templates (reusable agent configurations)
CREATE TABLE IF NOT EXISTS marketplace_templates (
    template_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    config JSONB NOT NULL,
    category VARCHAR(100),
    is_public BOOLEAN NOT NULL DEFAULT false,
    author_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_marketplace_agents_author ON marketplace_agents(author_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_agents_status ON marketplace_agents(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_installs_user ON marketplace_installs(user_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_reviews_agent ON marketplace_reviews(agent_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_transactions_buyer ON marketplace_transactions(buyer_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_transactions_seller ON marketplace_transactions(seller_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_settlements_seller ON marketplace_settlements(seller_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_templates_author ON marketplace_templates(author_id);
