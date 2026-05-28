-- Marketplace settlement and governance state for v06.
-- Paid Marketplace operation remains disabled until verified payment events can
-- create orders, installs, settlements, refund effects, and governance evidence.

CREATE TABLE IF NOT EXISTS marketplace_orders (
    id TEXT PRIMARY KEY,
    buyer_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    buyer_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    version_id TEXT REFERENCES agent_versions(id) ON DELETE SET NULL,
    payment_intent_id TEXT NOT NULL UNIQUE REFERENCES payment_intents(id) ON DELETE CASCADE,
    provider_checkout_session_id TEXT,
    provider_payment_intent_id TEXT,
    install_id TEXT REFERENCES agent_installs(id) ON DELETE SET NULL,
    gross_amount DECIMAL(15,6) NOT NULL,
    platform_fee_amount DECIMAL(15,6) NOT NULL,
    publisher_net_amount DECIMAL(15,6) NOT NULL,
    refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL DEFAULT 'pending_payment',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ,
    CHECK (status IN ('pending_payment', 'paid', 'partially_refunded', 'refunded', 'cancelled')),
    CHECK (gross_amount >= 0),
    CHECK (platform_fee_amount >= 0),
    CHECK (publisher_net_amount >= 0),
    CHECK (refunded_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_orders_buyer_created ON marketplace_orders(buyer_organization_id, buyer_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_publisher_created ON marketplace_orders(publisher_organization_id, publisher_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_agent ON marketplace_orders(agent_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_provider_payment_intent ON marketplace_orders(provider_payment_intent_id);

CREATE TABLE IF NOT EXISTS marketplace_payouts (
    id TEXT PRIMARY KEY,
    publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(15,6) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    provider TEXT NOT NULL DEFAULT 'local',
    provider_payout_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending', 'processing', 'payout_pending', 'paid_out', 'failed', 'cancelled')),
    CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_payouts_publisher_created ON marketplace_payouts(publisher_organization_id, publisher_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_payouts_status ON marketplace_payouts(status);

CREATE TABLE IF NOT EXISTS marketplace_settlements (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL UNIQUE REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    gross_amount DECIMAL(15,6) NOT NULL,
    platform_fee_amount DECIMAL(15,6) NOT NULL,
    publisher_net_amount DECIMAL(15,6) NOT NULL,
    refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0,
    payout_id TEXT REFERENCES marketplace_payouts(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    hold_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending', 'available', 'payout_pending', 'paid_out', 'partially_refunded', 'reversed')),
    CHECK (gross_amount >= 0),
    CHECK (platform_fee_amount >= 0),
    CHECK (publisher_net_amount >= 0),
    CHECK (refunded_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_settlements_publisher_created ON marketplace_settlements(publisher_organization_id, publisher_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_settlements_agent ON marketplace_settlements(agent_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_settlements_status ON marketplace_settlements(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_settlements_payout ON marketplace_settlements(payout_id);

CREATE TABLE IF NOT EXISTS marketplace_governance_events (
    id TEXT PRIMARY KEY,
    actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    actor_organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    from_status TEXT,
    to_status TEXT,
    reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (action IN ('publish', 'approve', 'reject', 'takedown', 'appeal', 'reinstate', 'abuse_report', 'abuse_resolve', 'abuse_dismiss', 'payout_state'))
);

CREATE INDEX IF NOT EXISTS idx_marketplace_governance_agent_created ON marketplace_governance_events(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_governance_actor_created ON marketplace_governance_events(actor_organization_id, actor_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_governance_action ON marketplace_governance_events(action);

CREATE TABLE IF NOT EXISTS marketplace_abuse_reports (
    id TEXT PRIMARY KEY,
    reporter_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    reporter_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    details TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    resolution TEXT,
    reviewer_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    CHECK (status IN ('open', 'reviewing', 'resolved', 'dismissed'))
);

CREATE INDEX IF NOT EXISTS idx_marketplace_abuse_agent_created ON marketplace_abuse_reports(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_abuse_reporter_created ON marketplace_abuse_reports(reporter_organization_id, reporter_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_abuse_status ON marketplace_abuse_reports(status);
