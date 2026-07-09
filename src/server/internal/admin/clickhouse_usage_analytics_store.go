package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type usageAnalyticsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type ClickHouseUsageAnalyticsStore struct {
	db usageAnalyticsQueryer
}

func NewClickHouseUsageAnalyticsStore(db usageAnalyticsQueryer) *ClickHouseUsageAnalyticsStore {
	return &ClickHouseUsageAnalyticsStore{db: db}
}

func (s *ClickHouseUsageAnalyticsStore) ListRequestLogEvidence(ctx context.Context, requestIDs []string) (map[string]RequestLogEvidence, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("clickhouse request log evidence store is unavailable")
	}
	requestIDs = uniqueRequestLogEvidenceIDs(requestIDs)
	if len(requestIDs) == 0 {
		return map[string]RequestLogEvidence{}, nil
	}
	placeholders := make([]string, 0, len(requestIDs))
	args := make([]any, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		placeholders = append(placeholders, "?")
		args = append(args, requestID)
	}
	query := fmt.Sprintf(`
		%s
		FROM request_logs
		WHERE request_id IN (%s)
		ORDER BY timestamp DESC
	`, clickHouseRequestLogEvidenceSelectColumns(), strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query clickhouse request log evidence: %w", err)
	}
	defer rows.Close()
	return scanRequestLogEvidenceRows(rows)
}

func (s *ClickHouseUsageAnalyticsStore) ListRelayRouteDecisionEvidence(ctx context.Context, requestIDs []string, apiType string, result string) (map[string]RequestLogEvidence, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("clickhouse request log evidence store is unavailable")
	}
	requestIDs = uniqueRequestLogEvidenceIDs(requestIDs)
	if len(requestIDs) == 0 {
		return map[string]RequestLogEvidence{}, nil
	}
	placeholders := make([]string, 0, len(requestIDs))
	args := make([]any, 0, len(requestIDs)+2)
	for _, requestID := range requestIDs {
		placeholders = append(placeholders, "?")
		args = append(args, requestID)
	}
	args = append(args, strings.TrimSpace(apiType), strings.TrimSpace(result))
	query := fmt.Sprintf(`
		%s
		FROM request_logs
		WHERE request_id IN (%s)
		  AND service = 'relay'
		  AND JSONExtractString(metadata, 'event') = 'relay.route_decision'
		  AND JSONExtractString(metadata, 'relay_api_type') = ?
		  AND JSONExtractString(metadata, 'relay_route_result') = ?
		ORDER BY timestamp DESC
	`, clickHouseRequestLogEvidenceSelectColumns(), strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query clickhouse relay route decision evidence: %w", err)
	}
	defer rows.Close()
	return scanRequestLogEvidenceRows(rows)
}

func clickHouseRequestLogEvidenceSelectColumns() string {
	return `SELECT
			request_id,
			toString(id),
			timestamp,
			service,
			endpoint,
			method,
			status_code,
			duration_ms,
			request_tokens,
			response_tokens,
			model,
			cost_usd,
			error,
			toString(trace_id),
			metadata`
}

func scanRequestLogEvidenceRows(rows *sql.Rows) (map[string]RequestLogEvidence, error) {
	evidence := map[string]RequestLogEvidence{}
	for rows.Next() {
		var item RequestLogEvidence
		var metadata string
		if err := rows.Scan(
			&item.RequestID,
			&item.RequestLogID,
			&item.Timestamp,
			&item.Service,
			&item.Endpoint,
			&item.Method,
			&item.StatusCode,
			&item.DurationMS,
			&item.RequestTokens,
			&item.ResponseTokens,
			&item.Model,
			&item.CostUSD,
			&item.Error,
			&item.TraceID,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan clickhouse request log evidence: %w", err)
		}
		if json.Valid([]byte(metadata)) {
			item.Metadata = json.RawMessage(metadata)
		}
		if _, exists := evidence[item.RequestID]; !exists {
			evidence[item.RequestID] = item
		}
	}
	return evidence, rows.Err()
}

func uniqueRequestLogEvidenceIDs(requestIDs []string) []string {
	seen := map[string]struct{}{}
	unique := []string{}
	for _, requestID := range requestIDs {
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			continue
		}
		if _, ok := seen[requestID]; ok {
			continue
		}
		seen[requestID] = struct{}{}
		unique = append(unique, requestID)
	}
	return unique
}

func (s *ClickHouseUsageAnalyticsStore) GetUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	if s == nil || s.db == nil {
		return UsageAnalytics{}, fmt.Errorf("clickhouse usage analytics store is unavailable")
	}
	filter = normalizeUsageAnalyticsFilter(filter)
	where, args := clickHouseUsageAnalyticsWhere(filter)
	bucketExpression := clickHouseUsageAnalyticsBucketExpression(filter.Granularity)

	byModel, err := s.queryBuckets(ctx, "model", "model", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	featureExpression := clickHouseUsageAnalyticsFeatureExpression()
	byFeature, err := s.queryBuckets(ctx, "feature", featureExpression, "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byUser, err := s.queryBuckets(ctx, "user", clickHouseUsageAnalyticsUserExpression(), "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byTime, err := s.queryBuckets(ctx, "time", fmt.Sprintf("formatDateTime(%s, '%%Y-%%m-%%dT%%H:%%i:%%SZ')", bucketExpression), bucketExpression, where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byChannel, err := s.queryBuckets(ctx, "channel", "COALESCE(NULLIF(JSONExtractString(metadata, 'channel_id'), ''), 'unknown')", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	byProvider, err := s.queryBuckets(ctx, "provider", "COALESCE(NULLIF(JSONExtractString(metadata, 'provider'), ''), 'unknown')", "", where, args, filter.Limit)
	if err != nil {
		return UsageAnalytics{}, err
	}
	crossDimensions, err := s.queryCrossBuckets(ctx, []usageAnalyticsCrossDimension{
		{
			dimension:           "model_time",
			primaryExpression:   "model",
			secondaryExpression: fmt.Sprintf("formatDateTime(%s, '%%Y-%%m-%%dT%%H:%%i:%%SZ')", bucketExpression),
			startedAtExpression: bucketExpression,
			orderBy:             "started_at ASC, total_cost DESC, request_count DESC, primary_key ASC",
		},
		{
			dimension:           "user_feature",
			primaryExpression:   clickHouseUsageAnalyticsUserExpression(),
			secondaryExpression: featureExpression,
			orderBy:             "total_cost DESC, request_count DESC, primary_key ASC, secondary_key ASC",
		},
		{
			dimension:           "feature_time",
			primaryExpression:   featureExpression,
			secondaryExpression: fmt.Sprintf("formatDateTime(%s, '%%Y-%%m-%%dT%%H:%%i:%%SZ')", bucketExpression),
			startedAtExpression: bucketExpression,
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

func clickHouseUsageAnalyticsWhere(filter UsageAnalyticsFilter) (string, []any) {
	conditions := []string{"timestamp >= ?", "timestamp < ?"}
	args := []any{filter.From, filter.To}
	add := func(condition, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		conditions = append(conditions, condition)
		args = append(args, strings.TrimSpace(value))
	}

	add("organization_id = ?", filter.OrganizationID)
	add(clickHouseUsageAnalyticsUserExpression()+" = ?", filter.UserID)
	add("JSONExtractString(metadata, 'relay_api_type') = ?", filter.APIType)
	add("JSONExtractString(metadata, 'feature_type') = ?", filter.FeatureType)
	add("JSONExtractString(metadata, 'quota_mode') = ?", filter.QuotaMode)
	add("model = ?", filter.Model)
	add("JSONExtractString(metadata, 'channel_id') = ?", filter.ChannelID)
	add("JSONExtractString(metadata, 'provider') = ?", filter.Provider)
	switch strings.ToLower(strings.TrimSpace(filter.Status)) {
	case "success", "succeeded", "ok":
		conditions = append(conditions, "status_code >= 200", "status_code < 400")
	case "failure", "failed", "error":
		conditions = append(conditions, "(status_code >= 400 OR error != '')")
	default:
		add("JSONExtractString(metadata, 'status') = ?", filter.Status)
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func clickHouseUsageAnalyticsUserExpression() string {
	return "COALESCE(NULLIF(JSONExtractString(metadata, 'user_id'), ''), toString(user_id))"
}

func clickHouseUsageAnalyticsFeatureExpression() string {
	return "COALESCE(NULLIF(JSONExtractString(metadata, 'feature_type'), ''), NULLIF(JSONExtractString(metadata, 'relay_api_type'), ''), NULLIF(service, ''), 'unknown')"
}

func clickHouseUsageAnalyticsBucketExpression(granularity string) string {
	switch normalizeUsageAnalyticsGranularity(granularity) {
	case "second":
		return "toStartOfSecond(timestamp)"
	case "minute":
		return "toStartOfMinute(timestamp)"
	case "hour":
		return "toStartOfHour(timestamp)"
	case "week":
		return "toStartOfWeek(timestamp)"
	case "month":
		return "toStartOfMonth(timestamp)"
	default:
		return "toStartOfDay(timestamp)"
	}
}

func (s *ClickHouseUsageAnalyticsStore) queryBuckets(
	ctx context.Context,
	dimension string,
	keyExpression string,
	startedAtExpression string,
	where string,
	args []any,
	limit int,
) ([]UsageAnalyticsBucket, error) {
	if limit <= 0 {
		limit = 10
	}
	selectStartedAt := "NULL AS started_at"
	groupStartedAt := ""
	orderBy := "total_cost DESC, request_count DESC, key ASC"
	if startedAtExpression != "" {
		selectStartedAt = startedAtExpression + " AS started_at"
		groupStartedAt = ", " + startedAtExpression
		orderBy = "started_at ASC"
	}
	query := fmt.Sprintf(`
		SELECT
			%[1]s AS key,
			count() AS request_count,
			sum(request_tokens + response_tokens) AS total_tokens,
			sum(cost_usd) AS total_cost,
			%[2]s
		FROM request_logs
		%[3]s
		GROUP BY %[1]s%[4]s
		ORDER BY %[5]s
		LIMIT ?
	`, keyExpression, selectStartedAt, where, groupStartedAt, orderBy)
	queryArgs := append(append([]any(nil), args...), limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query %s clickhouse usage analytics: %w", dimension, err)
	}
	defer rows.Close()

	buckets := []UsageAnalyticsBucket{}
	for rows.Next() {
		var bucket UsageAnalyticsBucket
		var startedAt any
		if err := rows.Scan(&bucket.Key, &bucket.RequestCount, &bucket.TotalTokens, &bucket.TotalCost, &startedAt); err != nil {
			return nil, fmt.Errorf("scan %s clickhouse usage analytics: %w", dimension, err)
		}
		bucket.Dimension = dimension
		bucket.StartedAt = scanUsageAnalyticsStartedAt(startedAt)
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (s *ClickHouseUsageAnalyticsStore) queryCrossBuckets(ctx context.Context, dimensions []usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	buckets := []UsageAnalyticsBucket{}
	for _, dimension := range dimensions {
		items, err := s.queryCrossBucket(ctx, dimension, where, args, limit)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, items...)
	}
	return buckets, nil
}

func (s *ClickHouseUsageAnalyticsStore) queryCrossBucket(ctx context.Context, dimension usageAnalyticsCrossDimension, where string, args []any, limit int) ([]UsageAnalyticsBucket, error) {
	if limit <= 0 {
		limit = 10
	}
	selectStartedAt := "NULL AS started_at"
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
			count() AS request_count,
			sum(request_tokens + response_tokens) AS total_tokens,
			sum(cost_usd) AS total_cost,
			%[3]s
		FROM request_logs
		%[4]s
		GROUP BY %[1]s, %[2]s%[5]s
		ORDER BY %[6]s
		LIMIT ?
	`, dimension.primaryExpression, dimension.secondaryExpression, selectStartedAt, where, groupStartedAt, orderBy)
	queryArgs := append(append([]any(nil), args...), limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query %s clickhouse usage analytics: %w", dimension.dimension, err)
	}
	defer rows.Close()

	buckets := []UsageAnalyticsBucket{}
	for rows.Next() {
		var bucket UsageAnalyticsBucket
		var startedAt any
		if err := rows.Scan(&bucket.Primary, &bucket.Secondary, &bucket.RequestCount, &bucket.TotalTokens, &bucket.TotalCost, &startedAt); err != nil {
			return nil, fmt.Errorf("scan %s clickhouse usage analytics: %w", dimension.dimension, err)
		}
		bucket.Dimension = dimension.dimension
		bucket.Key = bucket.Primary + "|" + bucket.Secondary
		bucket.StartedAt = scanUsageAnalyticsStartedAt(startedAt)
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func scanUsageAnalyticsStartedAt(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		parsed, _ := time.Parse(time.RFC3339, typed)
		return parsed
	case []byte:
		parsed, _ := time.Parse(time.RFC3339, string(typed))
		return parsed
	default:
		return time.Time{}
	}
}
