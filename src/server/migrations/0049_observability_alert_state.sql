CREATE TABLE IF NOT EXISTS observability_alert_states (
    alert_key TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    severity TEXT NOT NULL,
    original_severity TEXT NOT NULL DEFAULT '',
    escalated BOOLEAN NOT NULL DEFAULT false,
    title TEXT NOT NULL DEFAULT '',
    component TEXT NOT NULL DEFAULT '',
    opened_at TIMESTAMPTZ NOT NULL,
    last_occurred_at TIMESTAMPTZ NOT NULL,
    occurrence_count INT NOT NULL DEFAULT 0,
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS observability_alert_occurrences (
    id BIGSERIAL PRIMARY KEY,
    alert_key TEXT NOT NULL REFERENCES observability_alert_states(alert_key) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_observability_alert_occurrences_key_time
    ON observability_alert_occurrences(alert_key, occurred_at DESC);

CREATE TABLE IF NOT EXISTS observability_notification_states (
    notify_key TEXT PRIMARY KEY,
    alert_key TEXT NOT NULL,
    severity TEXT NOT NULL,
    last_notified_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS observability_alert_delivery_attempts (
    id TEXT PRIMARY KEY,
    alert_key TEXT NOT NULL,
    severity TEXT NOT NULL,
    component TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL,
    provider_id TEXT NOT NULL DEFAULT '',
    provider_kind TEXT NOT NULL DEFAULT '',
    delivered BOOLEAN NOT NULL DEFAULT false,
    error TEXT NOT NULL DEFAULT '',
    attempted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_observability_alert_delivery_attempts_alert_time
    ON observability_alert_delivery_attempts(alert_key, attempted_at DESC);

CREATE TABLE IF NOT EXISTS observability_recovery_actions (
    id TEXT PRIMARY KEY,
    policy_name TEXT NOT NULL,
    alert_key TEXT NOT NULL,
    severity TEXT NOT NULL,
    component TEXT NOT NULL DEFAULT '',
    action_type TEXT NOT NULL,
    status TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    attempt INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_observability_recovery_actions_cooldown
    ON observability_recovery_actions(policy_name, alert_key, created_at DESC);

CREATE TABLE IF NOT EXISTS observability_alert_routing_rules (
    severity TEXT PRIMARY KEY,
    channels JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS observability_alert_provider_configs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    channel TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_observability_alert_provider_configs_channel
    ON observability_alert_provider_configs(channel, status);
