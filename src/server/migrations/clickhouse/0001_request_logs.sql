CREATE TABLE IF NOT EXISTS request_logs (
    id UUID,
    request_id String,
    timestamp DateTime64(3),
    organization_id UUID,
    user_id UUID,
    service String,
    endpoint String,
    method String,
    status_code UInt16,
    duration_ms UInt32,
    request_tokens UInt32,
    response_tokens UInt32,
    model String,
    cost_usd Float64,
    error String,
    trace_id UUID,
    metadata String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (organization_id, timestamp, service);
