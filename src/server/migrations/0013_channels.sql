-- Relay channels and model routes

CREATE TABLE IF NOT EXISTS channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,  -- 'openai' | 'anthropic' | 'gemini'
    base_url TEXT NOT NULL DEFAULT 'https://api.openai.com',
    api_key_encrypted TEXT NOT NULL,
    models TEXT[] NOT NULL DEFAULT '{}',
    rpm_limit INTEGER DEFAULT 1000,
    tpm_limit INTEGER DEFAULT 100000,
    cb_threshold INTEGER DEFAULT 5,
    cb_timeout INTEGER DEFAULT 30,
    health_check_strategy TEXT DEFAULT 'models_api',
    probe_model TEXT,
    probe_prompt TEXT,
    strategy TEXT DEFAULT 'weighted',
    priority INTEGER DEFAULT 0,
    estimated_cost_per_1k DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_routes (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL UNIQUE,
    strategy TEXT DEFAULT 'weighted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_channel_weights (
    id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL REFERENCES model_routes(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    weight INTEGER DEFAULT 100,
    priority INTEGER DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    UNIQUE(route_id, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_channels_provider ON channels(provider);
CREATE INDEX IF NOT EXISTS idx_channels_enabled ON channels(enabled);
CREATE INDEX IF NOT EXISTS idx_model_routes_model ON model_routes(model);
CREATE INDEX IF NOT EXISTS idx_model_channel_weights_route_id ON model_channel_weights(route_id);
CREATE INDEX IF NOT EXISTS idx_model_channel_weights_channel_id ON model_channel_weights(channel_id);
