-- Marketplace recommendation operational signals.
-- Event ingestion can update these aggregates without coupling search to raw
-- exposure/click/install event volume.
CREATE TABLE IF NOT EXISTS marketplace_agent_ranking_signals (
    agent_id TEXT PRIMARY KEY REFERENCES published_agents(id) ON DELETE CASCADE,
    impression_count BIGINT NOT NULL DEFAULT 0,
    click_count BIGINT NOT NULL DEFAULT 0,
    install_conversion_count BIGINT NOT NULL DEFAULT 0,
    curated_weight DECIMAL(8,4) NOT NULL DEFAULT 1.0,
    governance_weight DECIMAL(8,4) NOT NULL DEFAULT 1.0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (impression_count >= 0),
    CHECK (click_count >= 0),
    CHECK (install_conversion_count >= 0),
    CHECK (curated_weight >= 0),
    CHECK (governance_weight >= 0)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_ranking_signals_updated
    ON marketplace_agent_ranking_signals(updated_at DESC);
