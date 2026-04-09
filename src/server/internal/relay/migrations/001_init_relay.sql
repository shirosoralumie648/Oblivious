-- Relay 模块初始迁移
-- 执行: psql $DATABASE_URL -f src/server/internal/relay/migrations/001_init_relay.sql

-- 渠道表
CREATE TABLE IF NOT EXISTS relay_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- 'openai'
    base_url VARCHAR(500) NOT NULL DEFAULT 'https://api.openai.com',
    api_key_encrypted TEXT,
    models TEXT[] DEFAULT '{}',
    rpm_limit INT DEFAULT 1000,
    tpm_limit INT DEFAULT 100000,
    markup DECIMAL(5,2) DEFAULT 1.0,
    cb_threshold INT DEFAULT 5,
    cb_timeout INT DEFAULT 30,
    health_check_strategy VARCHAR(20) DEFAULT 'models_api',
    probe_model VARCHAR(100) DEFAULT 'gpt-4o-mini',
    probe_prompt VARCHAR(255) DEFAULT 'hi',
    strategy VARCHAR(20) DEFAULT 'weighted',
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 定价表（按 APIType + Dimension 粒度定价）
CREATE TABLE IF NOT EXISTS relay_pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_type VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL DEFAULT '*',
    dimension VARCHAR(50) NOT NULL,
    unit_cost DECIMAL(15,8) NOT NULL,
    markup DECIMAL(5,2) DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 模型路由表
CREATE TABLE IF NOT EXISTS relay_model_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model VARCHAR(100) NOT NULL UNIQUE,
    strategy VARCHAR(20) DEFAULT 'weighted',
    created_at TIMESTAMP DEFAULT NOW()
);

-- 模型-渠道权重
CREATE TABLE IF NOT EXISTS relay_model_channel_weights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID REFERENCES relay_model_routes(id) ON DELETE CASCADE,
    channel_id UUID REFERENCES relay_channels(id) ON DELETE CASCADE,
    weight INT DEFAULT 100,
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_channels_enabled ON relay_channels(enabled);
CREATE INDEX IF NOT EXISTS idx_model_routes_model ON relay_model_routes(model);
CREATE INDEX IF NOT EXISTS idx_pricing_api_type ON relay_pricing_entries(api_type);
CREATE INDEX IF NOT EXISTS idx_pricing_model ON relay_pricing_entries(model);
CREATE INDEX IF NOT EXISTS idx_channel_weights_route ON relay_model_channel_weights(route_id);
CREATE INDEX IF NOT EXISTS idx_channel_weights_channel ON relay_model_channel_weights(channel_id);

-- 初始 OpenAI 定价数据
INSERT INTO relay_pricing_entries (api_type, model, dimension, unit_cost, markup) VALUES
-- Chat (prompt / completion 不对称)
('chat', 'gpt-4o', 'prompt_tokens', 0.0000025, 1.5),
('chat', 'gpt-4o', 'completion_tokens', 0.00001, 1.5),
('chat', 'gpt-4o-mini', 'prompt_tokens', 0.000000075, 1.5),
('chat', 'gpt-4o-mini', 'completion_tokens', 0.0000003, 1.5),
('chat', 'gpt-4o', 'total_tokens', 0.0, 1.5),
('responses', 'gpt-4o', 'prompt_tokens', 0.0000025, 1.5),
('responses', 'gpt-4o', 'completion_tokens', 0.00001, 1.5),
('responses', 'gpt-4o-mini', 'prompt_tokens', 0.000000075, 1.5),
('responses', 'gpt-4o-mini', 'completion_tokens', 0.0000003, 1.5),
('embeddings', '*', 'total_tokens', 0.0000001, 1.5),
('images_generations', 'dall-e-3', 'image_count', 0.040, 1.5),
('images_generations', 'dall-e-2', 'image_count', 0.020, 1.5),
('audio_speech', '*', 'audio_seconds', 0.015, 1.5),
('audio_transcriptions', '*', 'audio_seconds', 0.00006, 1.5),
('audio_translations', '*', 'audio_seconds', 0.00006, 1.5)
ON CONFLICT DO NOTHING;
