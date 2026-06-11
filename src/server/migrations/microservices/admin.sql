-- Admin Microservice Schema
-- Handles: plans, pricing, audit logs, API tokens, system configuration

-- Plans table
CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('free', 'pro', 'enterprise')),
    price_monthly DECIMAL(10,2),
    price_yearly DECIMAL(10,2),
    features JSONB NOT NULL DEFAULT '{}',
    limits JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_plans_type ON plans(type);
CREATE INDEX idx_plans_active ON plans(is_active);

-- Pricing settings
CREATE TABLE pricing_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    region VARCHAR(10) NOT NULL DEFAULT 'global',
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    price DECIMAL(10,2) NOT NULL,
    billing_period VARCHAR(20) NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'lifetime')),
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(plan_id, region, currency, billing_period, valid_from)
);

CREATE INDEX idx_pricing_plan ON pricing_settings(plan_id);
CREATE INDEX idx_pricing_region ON pricing_settings(region);
CREATE INDEX idx_pricing_validity ON pricing_settings(valid_from, valid_until);

-- Audit logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

-- API tokens
CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    scopes JSONB NOT NULL DEFAULT '[]',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tokens_user ON api_tokens(user_id);
CREATE INDEX idx_tokens_hash ON api_tokens(token_hash);
CREATE INDEX idx_tokens_active ON api_tokens(is_active);
CREATE INDEX idx_tokens_expires ON api_tokens(expires_at);

-- System configuration
CREATE TABLE system_config (
    key VARCHAR(100) PRIMARY KEY,
    value JSONB NOT NULL,
    category VARCHAR(50) NOT NULL,
    description TEXT,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_config_category ON system_config(category);

-- Triggers for updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER plans_updated_at BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER api_tokens_updated_at BEFORE UPDATE ON api_tokens
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER system_config_updated_at BEFORE UPDATE ON system_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
