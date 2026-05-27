-- Payment authority for v06 Billing And Marketplace Operations.
-- Stripe checkout and webhook ingestion are recorded here before later phases
-- apply subscription, top-up, refund, invoice, and Marketplace settlement effects.

CREATE TABLE IF NOT EXISTS payment_intents (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'stripe',
    provider_checkout_session_id TEXT UNIQUE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_id TEXT REFERENCES packages(id),
    kind TEXT NOT NULL,
    amount DECIMAL(15,6) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL DEFAULT 'pending',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider <> ''),
    CHECK (kind IN ('subscription', 'topup')),
    CHECK (status IN ('pending', 'completed', 'failed', 'refunded', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_payment_intents_org_created ON payment_intents(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_intents_user_created ON payment_intents(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_intents_status ON payment_intents(status);

CREATE TABLE IF NOT EXISTS stripe_webhook_events (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'stripe',
    event_id TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL,
    payload JSONB NOT NULL,
    error TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    CHECK (provider = 'stripe'),
    CHECK (status IN ('processed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_stripe_webhook_events_type_received ON stripe_webhook_events(event_type, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_stripe_webhook_events_status_received ON stripe_webhook_events(status, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_stripe_webhook_events_org_received ON stripe_webhook_events(organization_id, received_at DESC);
