-- Billing Microservice Schema
-- Handles subscriptions, invoices, payments, quotas, usage tracking, and rate limiting

CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    plan_id VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'cancelled', 'expired', 'paused')),
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancel_at_period_end BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    amount_cents INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'open', 'paid', 'void', 'uncollectible')),
    due_date TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_invoices_subscription_id ON invoices(subscription_id);
CREATE INDEX idx_invoices_user_id ON invoices(user_id);
CREATE INDEX idx_invoices_status ON invoices(status);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID REFERENCES invoices(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    amount_cents INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    payment_method VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded')),
    transaction_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_payments_invoice_id ON payments(invoice_id);
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);

CREATE TABLE IF NOT EXISTS quota_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    amount INTEGER NOT NULL,
    operation VARCHAR(10) NOT NULL CHECK (operation IN ('add', 'subtract', 'reset')),
    reason VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_quota_transactions_user_id ON quota_transactions(user_id);
CREATE INDEX idx_quota_transactions_resource_type ON quota_transactions(resource_type);
CREATE INDEX idx_quota_transactions_created_at ON quota_transactions(created_at);

CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL,
    metadata JSONB,
    recorded_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_usage_records_user_id ON usage_records(user_id);
CREATE INDEX idx_usage_records_resource_type ON usage_records(resource_type);
CREATE INDEX idx_usage_records_recorded_at ON usage_records(recorded_at);

CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID REFERENCES payments(id) ON DELETE CASCADE,
    amount_cents INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    reason VARCHAR(255),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_refunds_payment_id ON refunds(payment_id);
CREATE INDEX idx_refunds_status ON refunds(status);

CREATE TABLE IF NOT EXISTS concurrency_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    max_concurrent INTEGER NOT NULL,
    current_count INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, resource_type)
);

CREATE INDEX idx_concurrency_limits_user_id ON concurrency_limits(user_id);

CREATE TABLE IF NOT EXISTS token_rate_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_seconds INTEGER NOT NULL,
    tokens_used INTEGER DEFAULT 0,
    tokens_limit INTEGER NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, window_start)
);

CREATE INDEX idx_token_rate_limits_user_id ON token_rate_limits(user_id);
CREATE INDEX idx_token_rate_limits_window_start ON token_rate_limits(window_start);
