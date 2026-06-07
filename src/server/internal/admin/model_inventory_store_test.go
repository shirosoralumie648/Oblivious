package admin

import (
	"strings"
	"testing"
)

func TestModelInventoryUsageStatsAggregatesRequestCounts(t *testing.T) {
	query := modelInventoryUsageStatsCTE()

	if !strings.Contains(query, "SUM(request_count)") {
		t.Fatalf("expected model inventory usage stats to sum request_count, got %s", query)
	}
	if strings.Contains(query, "COUNT(*)::int AS request_count") {
		t.Fatalf("expected model inventory usage stats not to count rows as requests, got %s", query)
	}
}
