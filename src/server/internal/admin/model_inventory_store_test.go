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
	if !strings.Contains(query, "organization_id") || !strings.Contains(query, "GROUP BY organization_id, model_id") {
		t.Fatalf("expected model inventory usage stats to preserve organization scope, got %s", query)
	}
	if strings.Contains(query, "COUNT(*)::int AS request_count") {
		t.Fatalf("expected model inventory usage stats not to count rows as requests, got %s", query)
	}
}

func TestModelInventoryWhereScopesToOrganization(t *testing.T) {
	where, args := modelInventoryWhere(ModelInventoryFilter{OrganizationID: "org_1", Provider: "openai"})

	if !strings.Contains(where, "mc.organization_id = $1") || !strings.Contains(where, "mc.provider = $2") {
		t.Fatalf("expected model inventory where to scope organization before filters, got where=%q args=%#v", where, args)
	}
	if len(args) != 2 || args[0] != "org_1" || args[1] != "openai" {
		t.Fatalf("expected organization and provider args, got %#v", args)
	}
}
