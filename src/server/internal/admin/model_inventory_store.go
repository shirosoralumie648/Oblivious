package admin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"
)

// ModelInventoryStore defines admin model inventory inspection operations.
type ModelInventoryStore interface {
	ListModelInventory(ctx context.Context, filter ModelInventoryFilter) ([]*ModelInventoryEntry, int, error)
}

const modelInventoryCTE = `
	SELECT
		BTRIM(model_name.model) AS model,
		ch.organization_id,
		ch.id,
		ch.name,
		ch.provider,
		ch.groups,
		ch.enabled,
		ch.priority,
		ch.estimated_cost_per_1k,
		ch.cost_multiplier
	FROM channels ch
	CROSS JOIN LATERAL unnest(ch.models) AS model_name(model)
	WHERE BTRIM(model_name.model) <> ''
`

func modelInventoryUsageStatsCTE() string {
	return `
		usage_stats AS (
			SELECT
				organization_id,
				model_id AS model,
				COALESCE(SUM(request_count), 0)::int AS request_count,
				COALESCE(SUM(cost), 0)::double precision AS total_cost,
				COALESCE(SUM(channel_cost), 0)::double precision AS total_channel_cost
			FROM usage_records
			WHERE organization_id IS NOT NULL
			GROUP BY organization_id, model_id
		)
	`
}

func modelInventoryWhere(filter ModelInventoryFilter) (string, []any) {
	var conditions []string
	var args []any
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.OrganizationID != "" {
		add("mc.organization_id = $%d", filter.OrganizationID)
	}
	if filter.Provider != "" {
		add("mc.provider = $%d", filter.Provider)
	}
	if filter.Group != "" {
		add("$%d = ANY(mc.groups)", filter.Group)
	}
	switch filter.Status {
	case "enabled", "active", "online":
		conditions = append(conditions, "mc.enabled = true")
	case "disabled", "inactive", "offline":
		conditions = append(conditions, "mc.enabled = false")
	}
	if filter.Search != "" {
		args = append(args, filter.Search)
		searchArg := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"(mc.model ILIKE '%%' || $%d || '%%' OR mc.name ILIKE '%%' || $%d || '%%' OR mc.provider ILIKE '%%' || $%d || '%%')",
			searchArg,
			searchArg,
			searchArg,
		))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func appendModelPageCondition(where string, args []any, models []string) (string, []any) {
	args = append(args, pq.Array(models))
	condition := fmt.Sprintf("mc.model = ANY($%d)", len(args))
	if where == "" {
		return "WHERE " + condition, args
	}
	return where + " AND " + condition, args
}

func modelInventoryPageOrder(sort string) string {
	switch sort {
	case "model:desc":
		return "mc.model DESC"
	case "requests:asc":
		return "request_count ASC, mc.model ASC"
	case "requests:desc":
		return "request_count DESC, mc.model ASC"
	case "spend:asc":
		return "total_cost ASC, mc.model ASC"
	case "spend:desc":
		return "total_cost DESC, mc.model ASC"
	case "channelCost:asc":
		return "total_channel_cost ASC, mc.model ASC"
	case "channelCost:desc":
		return "total_channel_cost DESC, mc.model ASC"
	case "grossMargin:asc":
		return "gross_margin ASC, mc.model ASC"
	case "grossMargin:desc":
		return "gross_margin DESC, mc.model ASC"
	default:
		return "mc.model ASC"
	}
}

func (s *SQLStore) ListModelInventory(ctx context.Context, filter ModelInventoryFilter) ([]*ModelInventoryEntry, int, error) {
	filter = normalizeModelInventoryFilter(filter)
	if filter.OrganizationID == "" {
		return nil, 0, fmt.Errorf("organization id is required")
	}
	where, args := modelInventoryWhere(filter)

	var total int
	if err := s.db.QueryRowContext(ctx, `
		WITH model_channels AS (`+modelInventoryCTE+`)
		SELECT COUNT(DISTINCT mc.model)
		FROM model_channels mc
		`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count model inventory: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	modelRows, err := s.db.QueryContext(ctx, `
		WITH model_channels AS (`+modelInventoryCTE+`),
		`+modelInventoryUsageStatsCTE()+`
		SELECT
			mc.model,
			COALESCE(MAX(us.request_count), 0) AS request_count,
			COALESCE(MAX(us.total_cost), 0) AS total_cost,
			COALESCE(MAX(us.total_channel_cost), 0) AS total_channel_cost,
			COALESCE(MAX(us.total_cost), 0) - COALESCE(MAX(us.total_channel_cost), 0) AS gross_margin
		FROM model_channels mc
		LEFT JOIN usage_stats us ON us.organization_id = mc.organization_id AND us.model = mc.model
		`+where+`
		GROUP BY mc.model
		ORDER BY `+modelInventoryPageOrder(filter.Sort)+`
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list model inventory page: %w", err)
	}
	defer modelRows.Close()

	var models []string
	for modelRows.Next() {
		var model string
		var requestCount int
		var totalCost, totalChannelCost, grossMargin float64
		if err := modelRows.Scan(&model, &requestCount, &totalCost, &totalChannelCost, &grossMargin); err != nil {
			return nil, 0, fmt.Errorf("scan model inventory page: %w", err)
		}
		models = append(models, model)
	}
	if err := modelRows.Err(); err != nil {
		return nil, 0, err
	}
	if len(models) == 0 {
		return []*ModelInventoryEntry{}, total, nil
	}

	detailWhere, detailArgs := modelInventoryWhere(filter)
	detailWhere, detailArgs = appendModelPageCondition(detailWhere, detailArgs, models)
	rows, err := s.db.QueryContext(ctx, `
		WITH model_channels AS (`+modelInventoryCTE+`),
		`+modelInventoryUsageStatsCTE()+`
		SELECT
			mc.model,
			mc.id,
			mc.name,
			mc.provider,
			mc.groups,
			mc.enabled,
			mc.priority,
			mc.estimated_cost_per_1k,
			mc.cost_multiplier,
			COALESCE(us.request_count, 0),
			COALESCE(us.total_cost, 0),
			COALESCE(us.total_channel_cost, 0)
		FROM model_channels mc
		LEFT JOIN usage_stats us ON us.organization_id = mc.organization_id AND us.model = mc.model
		`+detailWhere+`
		ORDER BY mc.model ASC, mc.enabled DESC, mc.priority DESC, mc.name ASC
	`, detailArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list model inventory: %w", err)
	}
	defer rows.Close()

	entriesByModel := map[string]*ModelInventoryEntry{}
	providerSets := map[string]map[string]struct{}{}
	groupSets := map[string]map[string]struct{}{}
	multiplierSums := map[string]float64{}

	for rows.Next() {
		var model string
		var channel ModelInventoryChannel
		var groups []string
		var requestCount int
		var totalCost, totalChannelCost float64
		if err := rows.Scan(
			&model,
			&channel.ID,
			&channel.Name,
			&channel.Provider,
			pq.Array(&groups),
			&channel.Enabled,
			&channel.Priority,
			&channel.EstimatedCostPer1K,
			&channel.CostMultiplier,
			&requestCount,
			&totalCost,
			&totalChannelCost,
		); err != nil {
			return nil, 0, fmt.Errorf("scan model inventory: %w", err)
		}
		channel.Groups = groups

		entry := entriesByModel[model]
		if entry == nil {
			entry = &ModelInventoryEntry{
				Model:                 model,
				MinEstimatedCostPer1K: channel.EstimatedCostPer1K,
				MaxEstimatedCostPer1K: channel.EstimatedCostPer1K,
				RequestCount:          requestCount,
				TotalCost:             totalCost,
				TotalChannelCost:      totalChannelCost,
			}
			entriesByModel[model] = entry
			providerSets[model] = map[string]struct{}{}
			groupSets[model] = map[string]struct{}{}
		}

		entry.ChannelCount++
		if channel.Enabled {
			entry.EnabledChannelCount++
		} else {
			entry.DisabledChannelCount++
		}
		if channel.EstimatedCostPer1K < entry.MinEstimatedCostPer1K {
			entry.MinEstimatedCostPer1K = channel.EstimatedCostPer1K
		}
		if channel.EstimatedCostPer1K > entry.MaxEstimatedCostPer1K {
			entry.MaxEstimatedCostPer1K = channel.EstimatedCostPer1K
		}
		multiplierSums[model] += channel.CostMultiplier
		providerSets[model][channel.Provider] = struct{}{}
		for _, group := range groups {
			if strings.TrimSpace(group) != "" {
				groupSets[model][group] = struct{}{}
			}
		}
		entry.Channels = append(entry.Channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	entries := make([]*ModelInventoryEntry, 0, len(models))
	for _, model := range models {
		entry := entriesByModel[model]
		if entry == nil {
			continue
		}
		entry.Providers = sortedKeys(providerSets[model])
		entry.Groups = sortedKeys(groupSets[model])
		if entry.ChannelCount > 0 {
			entry.AvgCostMultiplier = multiplierSums[model] / float64(entry.ChannelCount)
		}
		entries = append(entries, entry)
	}
	return entries, total, nil
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
