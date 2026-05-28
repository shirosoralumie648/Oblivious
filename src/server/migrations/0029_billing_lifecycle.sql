-- Billing lifecycle state for v06 Payment And Marketplace Operations.
-- Phase 18 applies verified provider events to local subscription, invoice,
-- top-up, failed-payment, plan-change, and refund state.

ALTER TABLE payment_intents DROP CONSTRAINT IF EXISTS payment_intents_status_check;
ALTER TABLE payment_intents
    ADD CONSTRAINT payment_intents_status_check
    CHECK (status IN ('pending', 'completed', 'failed', 'refunded', 'partially_refunded', 'cancelled'));

ALTER TABLE payment_intents ADD COLUMN IF NOT EXISTS provider_payment_intent_id TEXT;
ALTER TABLE payment_intents ADD COLUMN IF NOT EXISTS provider_subscription_id TEXT;
ALTER TABLE payment_intents ADD COLUMN IF NOT EXISTS provider_invoice_id TEXT;
ALTER TABLE payment_intents ADD COLUMN IF NOT EXISTS refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_payment_intents_provider_payment_intent ON payment_intents(provider, provider_payment_intent_id);
CREATE INDEX IF NOT EXISTS idx_payment_intents_provider_subscription ON payment_intents(provider, provider_subscription_id);
CREATE INDEX IF NOT EXISTS idx_payment_intents_provider_invoice ON payment_intents(provider, provider_invoice_id);

ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS provider_subscription_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS provider_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS provider_checkout_session_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS provider_latest_invoice_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS failed_payment_at TIMESTAMPTZ;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_provider_subscription ON subscriptions(provider_subscription_id) WHERE provider_subscription_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_provider_customer ON subscriptions(provider_customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_provider_invoice ON subscriptions(provider_latest_invoice_id);

ALTER TABLE topup_orders ADD COLUMN IF NOT EXISTS payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL;
ALTER TABLE topup_orders ADD COLUMN IF NOT EXISTS provider_checkout_session_id TEXT;
ALTER TABLE topup_orders ADD COLUMN IF NOT EXISTS refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_topup_orders_payment_intent ON topup_orders(payment_intent_id) WHERE payment_intent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_topup_orders_provider_checkout ON topup_orders(provider_checkout_session_id);

CREATE TABLE IF NOT EXISTS billing_lifecycle_events (
    id TEXT PRIMARY KEY,
    transition_key TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL DEFAULT 'stripe',
    provider_event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    from_state TEXT,
    to_state TEXT NOT NULL,
    reason TEXT,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider = 'stripe')
);

CREATE INDEX IF NOT EXISTS idx_billing_lifecycle_org_created ON billing_lifecycle_events(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_lifecycle_event ON billing_lifecycle_events(provider_event_id, event_type);
CREATE INDEX IF NOT EXISTS idx_billing_lifecycle_entity ON billing_lifecycle_events(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS billing_invoices (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'stripe',
    provider_invoice_id TEXT NOT NULL,
    provider_subscription_id TEXT,
    provider_payment_intent_id TEXT,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id TEXT REFERENCES subscriptions(id) ON DELETE SET NULL,
    payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    amount_due DECIMAL(15,6) NOT NULL DEFAULT 0,
    amount_paid DECIMAL(15,6) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'usd',
    hosted_invoice_url TEXT,
    invoice_pdf TEXT,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider = 'stripe'),
    CHECK (status IN ('draft', 'open', 'paid', 'failed', 'void', 'uncollectible'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_invoices_provider_invoice ON billing_invoices(provider, provider_invoice_id);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_org_created ON billing_invoices(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_subscription ON billing_invoices(subscription_id);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_payment_intent ON billing_invoices(payment_intent_id);

CREATE TABLE IF NOT EXISTS billing_refunds (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'stripe',
    provider_refund_id TEXT NOT NULL,
    provider_charge_id TEXT,
    provider_payment_intent_id TEXT,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL,
    topup_order_id TEXT REFERENCES topup_orders(id) ON DELETE SET NULL,
    amount DECIMAL(15,6) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL,
    reason TEXT,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (provider = 'stripe'),
    CHECK (status IN ('pending', 'succeeded', 'failed', 'canceled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_refunds_provider_refund ON billing_refunds(provider, provider_refund_id);
CREATE INDEX IF NOT EXISTS idx_billing_refunds_org_created ON billing_refunds(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_refunds_payment_intent ON billing_refunds(payment_intent_id);
