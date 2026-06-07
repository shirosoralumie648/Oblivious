package admin

import (
	"context"
	"strings"
)

func normalizeModelInventoryFilter(filter ModelInventoryFilter) ModelInventoryFilter {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.Provider = strings.TrimSpace(filter.Provider)
	filter.Group = strings.TrimSpace(filter.Group)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Sort = normalizeModelInventorySort(filter.Sort)
	return filter
}

func (s *Service) ListModelInventory(ctx context.Context, filter ModelInventoryFilter) ([]*ModelInventoryEntry, int, error) {
	return s.store.ListModelInventory(ctx, normalizeModelInventoryFilter(filter))
}

func normalizeModelInventorySort(sort string) string {
	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		return "model:asc"
	}

	parts := strings.SplitN(sort, ":", 2)
	key := strings.TrimSpace(parts[0])
	dir := "asc"
	if len(parts) == 2 {
		dir = strings.TrimSpace(parts[1])
	}
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	switch key {
	case "model", "name":
		key = "model"
	case "requestcount", "requests", "request_count":
		key = "requests"
	case "totalcost", "spend", "cost":
		key = "spend"
	case "totalchannelcost", "channelcost", "channel_cost":
		key = "channelCost"
	case "grossmargin", "gross_margin", "margin":
		key = "grossMargin"
	default:
		key = "model"
	}

	if key != "model" && len(parts) == 1 {
		dir = "desc"
	}
	return key + ":" + dir
}
