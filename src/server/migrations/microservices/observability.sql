-- Observability Service Schema
-- Alert configurations and system logs

CREATE TABLE IF NOT EXISTS alert_configs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    metric_name VARCHAR(255) NOT NULL,
    threshold DECIMAL(10,2) NOT NULL,
    comparison_operator VARCHAR(10) NOT NULL CHECK (comparison_operator IN ('>', '<', '>=', '<=', '=')),
    enabled BOOLEAN DEFAULT true,
    notification_channels TEXT[], -- array of channel identifiers
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS system_logs (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    level VARCHAR(20) NOT NULL CHECK (level IN ('DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL')),
    service VARCHAR(100),
    message TEXT NOT NULL,
    metadata JSONB,
    trace_id VARCHAR(64),
    span_id VARCHAR(32)
);

CREATE INDEX idx_alert_configs_metric ON alert_configs(metric_name);
CREATE INDEX idx_alert_configs_enabled ON alert_configs(enabled);
CREATE INDEX idx_system_logs_timestamp ON system_logs(timestamp DESC);
CREATE INDEX idx_system_logs_level ON system_logs(level);
CREATE INDEX idx_system_logs_service ON system_logs(service);
CREATE INDEX idx_system_logs_trace ON system_logs(trace_id);
