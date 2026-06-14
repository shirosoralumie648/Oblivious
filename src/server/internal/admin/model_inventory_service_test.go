package admin

import (
	"context"
	"testing"
)

func TestServiceListModelInventoryNormalizesFiltersAndReturnsOperationalFields(t *testing.T) {
	store := &modelInventoryStoreSpy{
		entries: []*ModelInventoryEntry{{
			Model:                 "gpt-4o",
			Providers:             []string{"openai", "azure"},
			Groups:                []string{"default", "vip"},
			ChannelCount:          2,
			EnabledChannelCount:   1,
			DisabledChannelCount:  1,
			MinEstimatedCostPer1K: 0.02,
			MaxEstimatedCostPer1K: 0.05,
			AvgCostMultiplier:     1.2,
			RequestCount:          30,
			TotalCost:             1.23,
			TotalChannelCost:      0.61,
			Channels: []ModelInventoryChannel{{
				ID:                 "ch_1",
				Name:               "OpenAI primary",
				Provider:           "openai",
				Groups:             []string{"default", "vip"},
				Enabled:            true,
				Priority:           10,
				EstimatedCostPer1K: 0.02,
				CostMultiplier:     1.1,
			}},
		}},
		total: 1,
	}
	service := NewService(store)

	entries, total, err := service.ListModelInventory(context.Background(), ModelInventoryFilter{
		OrganizationID: " org_1 ",
		Provider:       " openai ",
		Group:          " vip ",
		Status:         " ENABLED ",
		Search:         " gpt ",
		Sort:           " REQUESTS:DESC ",
		Limit:          250,
		Offset:         -10,
	})
	if err != nil {
		t.Fatalf("list model inventory: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("expected one model and total=1, got len=%d total=%d", len(entries), total)
	}
	if store.filter.Limit != 100 || store.filter.Offset != 0 {
		t.Fatalf("expected normalized limit=100 offset=0, got limit=%d offset=%d", store.filter.Limit, store.filter.Offset)
	}
	if store.filter.OrganizationID != "org_1" {
		t.Fatalf("expected organization scope to be normalized, got %q", store.filter.OrganizationID)
	}
	if store.filter.Provider != "openai" || store.filter.Group != "vip" || store.filter.Status != "enabled" || store.filter.Search != "gpt" {
		t.Fatalf("expected trimmed filters, got %#v", store.filter)
	}
	if store.filter.Sort != "requests:desc" {
		t.Fatalf("expected normalized sort requests:desc, got %q", store.filter.Sort)
	}
	if entries[0].Providers[0] != "openai" || entries[0].ChannelCount != 2 || entries[0].RequestCount != 30 || entries[0].TotalCost != 1.23 {
		t.Fatalf("expected model operational fields, got %#v", entries[0])
	}
}

func TestServiceListModelInventoryRequiresOrganizationScope(t *testing.T) {
	service := NewService(&modelInventoryStoreSpy{})

	_, _, err := service.ListModelInventory(context.Background(), ModelInventoryFilter{})
	if err == nil {
		t.Fatal("expected missing organization id to fail closed")
	}
}

type modelInventoryStoreSpy struct {
	Store
	filter  ModelInventoryFilter
	entries []*ModelInventoryEntry
	total   int
}

func (s *modelInventoryStoreSpy) ListModelInventory(_ context.Context, filter ModelInventoryFilter) ([]*ModelInventoryEntry, int, error) {
	s.filter = filter
	return s.entries, s.total, nil
}
