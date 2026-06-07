package marketplace

import (
	"context"
	"testing"
)

func TestRecordAgentRankingSignalAggregatesRecommendationCounters(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewService(NewSQLStore(database), nil)
	ctx := context.Background()

	for _, event := range []AgentRankingSignalEvent{
		AgentRankingSignalImpression,
		AgentRankingSignalClick,
		AgentRankingSignalInstallConversion,
		AgentRankingSignalImpression,
	} {
		if err := service.RecordAgentRankingSignal(ctx, "agent_invoice_relevant", event); err != nil {
			t.Fatalf("record %s: %v", event, err)
		}
	}

	var impressions, clicks, installs int
	if err := database.QueryRowContext(ctx, `
		SELECT impression_count, click_count, install_conversion_count
		FROM marketplace_agent_ranking_signals
		WHERE agent_id = $1
	`, "agent_invoice_relevant").Scan(&impressions, &clicks, &installs); err != nil {
		t.Fatalf("read ranking signal aggregate: %v", err)
	}

	if impressions != 2 || clicks != 1 || installs != 1 {
		t.Fatalf("expected ranking counters impressions=2 clicks=1 installs=1, got impressions=%d clicks=%d installs=%d", impressions, clicks, installs)
	}
}
