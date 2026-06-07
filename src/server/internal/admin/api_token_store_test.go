package admin

import (
	"strings"
	"testing"
)

func TestAPITokenUsageStatsAggregatesRequestCounts(t *testing.T) {
	query := apiTokenUsageStatsCTE()

	if !strings.Contains(query, "SUM(request_count)") {
		t.Fatalf("expected API token usage stats to sum request_count, got %s", query)
	}
	if strings.Contains(query, "COUNT(*) AS request_count") {
		t.Fatalf("expected API token usage stats not to count rows as requests, got %s", query)
	}
}
