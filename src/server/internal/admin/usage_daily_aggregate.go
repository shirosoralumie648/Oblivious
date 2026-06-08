package admin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var usageDailyAggregatesAnalyticsSource = usageAnalyticsSource{
	table:           "usage_daily_aggregates",
	totalTokensExpr: "total_tokens",
	totalCostExpr:   "total_cost",
}

func (s *SQLStore) RefreshUsageDailyAggregates(ctx context.Context, from time.Time, to time.Time) error {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() || !from.Before(to) {
		from = to.AddDate(0, 0, -1)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO usage_daily_aggregates (
			usage_date,
			organization_id,
			user_id,
			model_id,
			feature_type,
			channel_id,
			provider,
			status,
			request_count,
			total_tokens,
			total_cost,
			channel_cost,
			refreshed_at
		)
		SELECT
			date_trunc('day', created_at)::date AS usage_date,
			organization_id,
			user_id,
			model_id,
			COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'unknown') AS feature_type,
			COALESCE(NULLIF(channel_id, ''), 'unknown') AS channel_id,
			COALESCE(NULLIF(provider, ''), 'unknown') AS provider,
			COALESCE(NULLIF(status, ''), 'unknown') AS status,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens)), 0)::int AS total_tokens,
			COALESCE(SUM(cost), 0)::numeric(15,6) AS total_cost,
			COALESCE(SUM(channel_cost), 0)::numeric(15,6) AS channel_cost,
			NOW() AS refreshed_at
		FROM usage_records
		WHERE created_at >= $1
		  AND created_at < $2
		  AND organization_id IS NOT NULL
		  AND user_id IS NOT NULL
		  AND (api_type IS NOT NULL OR api_token_id IS NOT NULL OR channel_id IS NOT NULL)
		GROUP BY
			date_trunc('day', created_at)::date,
			organization_id,
			user_id,
			model_id,
			COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'unknown'),
			COALESCE(NULLIF(channel_id, ''), 'unknown'),
			COALESCE(NULLIF(provider, ''), 'unknown'),
			COALESCE(NULLIF(status, ''), 'unknown')
		ON CONFLICT (usage_date, organization_id, user_id, model_id, feature_type, channel_id, provider, status)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			total_tokens = EXCLUDED.total_tokens,
			total_cost = EXCLUDED.total_cost,
			channel_cost = EXCLUDED.channel_cost,
			refreshed_at = EXCLUDED.refreshed_at
	`, from, to)
	if err != nil {
		return fmt.Errorf("refresh usage daily aggregates: %w", err)
	}
	return nil
}

func usageDailyAggregateWhere(filter UsageAnalyticsFilter) (string, []any) {
	conditions := []string{"usage_date >= $1::date", "usage_date < $2::date"}
	args := []any{filter.From, filter.To}
	add := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, strings.TrimSpace(value))
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("organization_id", filter.OrganizationID)
	add("user_id", filter.UserID)
	add("feature_type", filter.FeatureType)
	add("model_id", filter.Model)
	add("channel_id", filter.ChannelID)
	add("provider", filter.Provider)
	add("status", filter.Status)
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func usageDailyAggregateTimeExpressions(granularity string) (string, string) {
	switch normalizeUsageAnalyticsGranularity(granularity) {
	case "week":
		startedAtExpression := "date_trunc('week', usage_date::timestamptz)"
		return "to_char(" + startedAtExpression + ", 'IYYY-\"W\"IW')", startedAtExpression
	case "month":
		startedAtExpression := "date_trunc('month', usage_date::timestamptz)"
		return "to_char(" + startedAtExpression + ", 'YYYY-MM')", startedAtExpression
	default:
		return "to_char(usage_date, 'YYYY-MM-DD')", "usage_date::timestamptz"
	}
}

func (s *SQLStore) getDailyUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	where, args := usageDailyAggregateWhere(filter)
	timeKeyExpression, timeStartedAtExpression := usageDailyAggregateTimeExpressions(filter.Granularity)

	byModel, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "model", "model_id", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byFeature, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "feature", "feature_type", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byUser, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "user", "user_id", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byTime, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "time", timeKeyExpression, timeStartedAtExpression, where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byChannel, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "channel", "channel_id", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byProvider, err := s.queryUsageAnalyticsBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, "provider", "provider", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	crossDimensions, err := s.queryUsageAnalyticsCrossBucketsFrom(ctx, usageDailyAggregatesAnalyticsSource, []usageAnalyticsCrossDimension{
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
			secondaryExpression: "feature_type",
			orderBy:             "total_cost DESC, request_count DESC, primary_key ASC, secondary_key ASC",
		},
		{
			dimension:           "feature_time",
			primaryExpression:   "feature_type",
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
