package admin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UsageLogStore defines read-only Relay usage inspection operations.
type UsageLogStore interface {
	ListUsageLogs(ctx context.Context, filter UsageLogFilter) ([]*UsageLogEntry, int, error)
	GetUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error)
}

// UsageAnalyticsStore defines aggregated Relay usage inspection operations.
type UsageAnalyticsStore interface {
	GetUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error)
}

func usageLogWhere(filter UsageLogFilter) (string, []any) {
	conditions := []string{"(api_type IS NOT NULL OR api_token_id IS NOT NULL OR channel_id IS NOT NULL)"}
	var args []any
	add := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	add("organization_id", filter.OrganizationID)
	add("user_id", filter.UserID)
	add("api_token_id", filter.APITokenID)
	add("request_id", filter.RequestID)
	add("api_type", filter.APIType)
	add("feature_type", filter.FeatureType)
	add("quota_mode", filter.QuotaMode)
	add("model_id", filter.Model)
	add("channel_id", filter.ChannelID)
	add("provider", filter.Provider)
	add("status", filter.Status)

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func usageAnalyticsWhere(filter UsageAnalyticsFilter) (string, []any) {
	conditions := []string{"(api_type IS NOT NULL OR api_token_id IS NOT NULL OR channel_id IS NOT NULL)"}
	args := []any{filter.From, filter.To}
	conditions = append(conditions, "created_at >= $1", "created_at < $2")
	add := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, strings.TrimSpace(value))
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("organization_id", filter.OrganizationID)
	add("user_id", filter.UserID)
	add("api_type", filter.APIType)
	add("feature_type", filter.FeatureType)
	add("quota_mode", filter.QuotaMode)
	add("model_id", filter.Model)
	add("channel_id", filter.ChannelID)
	add("provider", filter.Provider)
	add("status", filter.Status)
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (s *SQLStore) ListUsageLogs(ctx context.Context, filter UsageLogFilter) ([]*UsageLogEntry, int, error) {
	filter = normalizeUsageLogFilter(filter)
	where, args := usageLogWhere(filter)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_records `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count usage logs: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			COALESCE(organization_id, ''),
			user_id,
			COALESCE(api_token_id, ''),
			COALESCE(request_id, ''),
			COALESCE(api_type, ''),
			COALESCE(feature_type, ''),
			COALESCE(quota_mode, ''),
			model_id,
			COALESCE(channel_id, ''),
			COALESCE(provider, ''),
			COALESCE(status, ''),
			COALESCE(status_code, 0),
			COALESCE(error_code, ''),
			COALESCE(latency_ms, 0),
			COALESCE(cost, 0),
			COALESCE(channel_cost, 0),
			input_tokens,
			output_tokens,
			COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens),
			created_at
		FROM usage_records
		`+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list usage logs: %w", err)
	}
	defer rows.Close()

	var entries []*UsageLogEntry
	for rows.Next() {
		var entry UsageLogEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.OrganizationID,
			&entry.UserID,
			&entry.APITokenID,
			&entry.RequestID,
			&entry.APIType,
			&entry.FeatureType,
			&entry.QuotaMode,
			&entry.Model,
			&entry.ChannelID,
			&entry.Provider,
			&entry.Status,
			&entry.StatusCode,
			&entry.ErrorCode,
			&entry.LatencyMS,
			&entry.Cost,
			&entry.ChannelCost,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.TotalTokens,
			&entry.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan usage log: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, total, rows.Err()
}

func (s *SQLStore) GetUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	filter = normalizeUsageAnalyticsFilter(filter)
	if usageAnalyticsGranularityUsesDailyAggregates(filter.Granularity) {
		return s.getDailyUsageAnalytics(ctx, filter)
	}
	where, args := usageAnalyticsWhere(filter)
	timeKeyExpression, timeStartedAtExpression := postgresUsageAnalyticsTimeExpressions(filter.Granularity)

	byModel, err := s.queryUsageAnalyticsBuckets(ctx, "model", "model_id", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byFeature, err := s.queryUsageAnalyticsBuckets(ctx, "feature", "COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'chat')", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byUser, err := s.queryUsageAnalyticsBuckets(ctx, "user", "user_id", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byTime, err := s.queryUsageAnalyticsBuckets(ctx, "time", timeKeyExpression, timeStartedAtExpression, where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byChannel, err := s.queryUsageAnalyticsBuckets(ctx, "channel", "COALESCE(NULLIF(channel_id, ''), 'unknown')", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byProvider, err := s.queryUsageAnalyticsBuckets(ctx, "provider", "COALESCE(NULLIF(provider, ''), 'unknown')", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	crossDimensions, err := s.queryUsageAnalyticsCrossBuckets(ctx, []usageAnalyticsCrossDimension{
		{
			dimension:           "model_time",
			primaryExpression:   "model_id",
			secondaryExpression: timeKeyExpression,
			startedAtExpression: timeStartedAtExpression,
			orderBy:             "started_at ASC, total_cost DESC, request_count DESC, primary_key ASC",
		},
		{
			dimension:           "user_feature",
			primaryExpression:   "user_id",
			secondaryExpression: "COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'chat')",
			orderBy:             "total_cost DESC, request_count DESC, primary_key ASC, secondary_key ASC",
		},
		{
			dimension:           "feature_time",
			primaryExpression:   "COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'chat')",
			secondaryExpression: timeKeyExpression,
			startedAtExpression: timeStartedAtExpression,
			orderBy:             "started_at ASC, total_cost DESC, request_count DESC, primary_key ASC",
		},
	}, where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}

	return UsageAnalytics{
		ByModel:         byModel,
		ByFeature:       byFeature,
		ByUser:          byUser,
		ByTime:          byTime,
		ByChannel:       byChannel,
		ByProvider:      byProvider,
		CrossDimensions: crossDimensions,
	}, nil
}

type usageAnalyticsCrossDimension struct {
	dimension           string
	primaryExpression   string
	secondaryExpression string
	startedAtExpression string
	orderBy             string
}

type usageAnalyticsSource struct {
	table           string
	totalTokensExpr string
	totalCostExpr   string
}

var usageRecordsAnalyticsSource = usageAnalyticsSource{
	table:           "usage_records",
	totalTokensExpr: "COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens)",
	totalCostExpr:   "cost",
}

func postgresUsageAnalyticsTimeExpressions(granularity string) (string, string) {
	unit := normalizeUsageAnalyticsGranularity(granularity)
	startedAtExpression := fmt.Sprintf("date_trunc('%s', created_at)", unit)
	switch unit {
	case "second":
		return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')", startedAtExpression), startedAtExpression
	case "minute":
		return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD\"T\"HH24:MI:00\"Z\"')", startedAtExpression), startedAtExpression
	case "hour":
		return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD\"T\"HH24:00:00\"Z\"')", startedAtExpression), startedAtExpression
	case "week":
		return fmt.Sprintf("to_char(%s, 'IYYY-\"W\"IW')", startedAtExpression), startedAtExpression
	case "month":
		return fmt.Sprintf("to_char(%s, 'YYYY-MM')", startedAtExpression), startedAtExpression
	default:
		return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", startedAtExpression), startedAtExpression
	}
}

func usageAnalyticsGranularityUsesDailyAggregates(granularity string) bool {
	switch normalizeUsageAnalyticsGranularity(granularity) {
	case "day", "week", "month":
		return true
	default:
		return false
	}
}

func (s *SQLStore) queryUsageAnalyticsBuckets(ctx context.Context, dimension, keyExpression, timeExpression, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	return s.queryUsageAnalyticsBucketsFrom(ctx, usageRecordsAnalyticsSource, dimension, keyExpression, timeExpression, where, args, limit)
}

func (s *SQLStore) queryUsageAnalyticsBucketsFrom(ctx context.Context, source usageAnalyticsSource, dimension, keyExpression, timeExpression, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	if limit <= 0 {
		limit = 10
	}
	selectStartedAt := "NULL::timestamptz AS started_at"
	groupStartedAt := ""
	orderBy := "total_cost DESC, request_count DESC, key ASC"
	if timeExpression != "" {
		selectStartedAt = timeExpression + " AS started_at"
		groupStartedAt = ", " + timeExpression
		orderBy = "started_at ASC"
	}
	query := fmt.Sprintf(`
		SELECT
			%[1]s AS key,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(%[7]s), 0)::int AS total_tokens,
			COALESCE(SUM(%[8]s), 0)::double precision AS total_cost,
			%[2]s
		FROM %[9]s
		%[3]s
		GROUP BY %[1]s%[4]s
		ORDER BY %[5]s
		LIMIT $%[6]d
	`, keyExpression, selectStartedAt, where, groupStartedAt, orderBy, len(args)+1, source.totalTokensExpr, source.totalCostExpr, source.table)
	queryArgs := append(append([]any(nil), args...), limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query %s usage analytics: %w", dimension, err)
	}
	defer rows.Close()

	buckets := []UsageAnalyticsBucket{}
	for rows.Next() {
		var bucket UsageAnalyticsBucket
		var startedAt *time.Time
		if err := rows.Scan(&bucket.Key, &bucket.RequestCount, &bucket.TotalTokens, &bucket.TotalCost, &startedAt); err != nil {
			return nil, fmt.Errorf("scan %s usage analytics: %w", dimension, err)
		}
		bucket.Dimension = dimension
		if startedAt != nil {
			bucket.StartedAt = *startedAt
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (s *SQLStore) queryUsageAnalyticsCrossBuckets(ctx context.Context, dimensions []usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	return s.queryUsageAnalyticsCrossBucketsFrom(ctx, usageRecordsAnalyticsSource, dimensions, where, args, limit)
}

func (s *SQLStore) queryUsageAnalyticsCrossBucketsFrom(ctx context.Context, source usageAnalyticsSource, dimensions []usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	buckets := []UsageAnalyticsBucket{}
	for _, dimension := range dimensions {
		items, err := s.queryUsageAnalyticsCrossBucketFrom(ctx, source, dimension, where, args, limit)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, items...)
	}
	return buckets, nil
}

func (s *SQLStore) queryUsageAnalyticsCrossBucket(ctx context.Context, dimension usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	return s.queryUsageAnalyticsCrossBucketFrom(ctx, usageRecordsAnalyticsSource, dimension, where, args, limit)
}

func (s *SQLStore) queryUsageAnalyticsCrossBucketFrom(ctx context.Context, source usageAnalyticsSource, dimension usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	if limit <= 0 {
		limit = 10
	}
	selectStartedAt := "NULL::timestamptz AS started_at"
	groupStartedAt := ""
	if dimension.startedAtExpression != "" {
		selectStartedAt = dimension.startedAtExpression + " AS started_at"
		groupStartedAt = ", " + dimension.startedAtExpression
	}
	orderBy := dimension.orderBy
	if orderBy == "" {
		orderBy = "total_cost DESC, request_count DESC, primary_key ASC, secondary_key ASC"
	}
	query := fmt.Sprintf(`
		SELECT
			%[1]s AS primary_key,
			%[2]s AS secondary_key,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(%[8]s), 0)::int AS total_tokens,
			COALESCE(SUM(%[9]s), 0)::double precision AS total_cost,
			%[3]s
		FROM %[10]s
		%[4]s
		GROUP BY %[1]s, %[2]s%[5]s
		ORDER BY %[6]s
		LIMIT $%[7]d
	`, dimension.primaryExpression, dimension.secondaryExpression, selectStartedAt, where, groupStartedAt, orderBy, len(args)+1, source.totalTokensExpr, source.totalCostExpr, source.table)
	queryArgs := append(append([]any(nil), args...), limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query %s usage analytics: %w", dimension.dimension, err)
	}
	defer rows.Close()

	buckets := []UsageAnalyticsBucket{}
	for rows.Next() {
		var bucket UsageAnalyticsBucket
		var startedAt *time.Time
		if err := rows.Scan(&bucket.Primary, &bucket.Secondary, &bucket.RequestCount, &bucket.TotalTokens, &bucket.TotalCost, &startedAt); err != nil {
			return nil, fmt.Errorf("scan %s usage analytics: %w", dimension.dimension, err)
		}
		bucket.Dimension = dimension.dimension
		bucket.Key = bucket.Primary + "|" + bucket.Secondary
		if startedAt != nil {
			bucket.StartedAt = *startedAt
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}
