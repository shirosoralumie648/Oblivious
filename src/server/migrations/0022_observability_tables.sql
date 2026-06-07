-- Observability tables: alert rules.

CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    component TEXT NOT NULL DEFAULT '',
    metric_name TEXT NOT NULL,
    condition TEXT NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning',
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    notification_channels JSONB NOT NULL DEFAULT '[]',
    labels JSONB NOT NULL DEFAULT '{}',
    annotations JSONB NOT NULL DEFAULT '{}',
    last_triggered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (btrim(name) <> ''),
    CHECK (condition IN ('gt', 'gte', 'lt', 'lte', 'eq', 'neq')),
    CHECK (severity IN ('info', 'warning', 'critical')),
    CHECK (duration_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_org_enabled
    ON alert_rules(organization_id, enabled);
CREATE INDEX IF NOT EXISTS idx_alert_rules_org_component
    ON alert_rules(organization_id, component);
CREATE INDEX IF NOT EXISTS idx_alert_rules_org_severity
    ON alert_rules(organization_id, severity);
