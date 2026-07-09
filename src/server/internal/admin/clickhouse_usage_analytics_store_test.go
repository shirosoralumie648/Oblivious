package admin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClickHouseUsageAnalyticsStoreQueriesRequestLogsWithGranularity(t *testing.T) {
	driverName := "admin_clickhouse_usage_analytics_test"
	queryer := &clickHouseUsageAnalyticsQueryer{}
	registerClickHouseUsageAnalyticsDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	analytics, err := NewClickHouseUsageAnalyticsStore(db).GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
		OrganizationID: "750e8400-e29b-41d4-a716-446655440000",
		UserID:         "850e8400-e29b-41d4-a716-446655440000",
		APIType:        "chat",
		Model:          "gpt-5.4",
		ChannelID:      "ch_1",
		Provider:       "openai",
		Status:         "success",
		From:           from,
		To:             to,
		Limit:          5,
		Granularity:    "minute",
	})
	if err != nil {
		t.Fatalf("get clickhouse usage analytics: %v", err)
	}

	queryer.mu.Lock()
	queries := strings.Join(queryer.queries, "\n")
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"FROM request_logs",
		"toStartOfMinute(timestamp)",
		"count() AS request_count",
		"sum(request_tokens + response_tokens)",
		"sum(cost_usd)",
		"JSONExtractString(metadata, 'feature_type')",
		"JSONExtractString(metadata, 'relay_api_type')",
		"JSONExtractString(metadata, 'channel_id') = ?",
		"COALESCE(NULLIF(JSONExtractString(metadata, 'channel_id'), ''), 'unknown')",
		"JSONExtractString(metadata, 'provider') = ?",
		"COALESCE(NULLIF(JSONExtractString(metadata, 'provider'), ''), 'unknown')",
		"status_code >= 200",
		"status_code < 400",
	} {
		if !strings.Contains(queries, want) {
			t.Fatalf("expected ClickHouse analytics query bundle to include %q, got %s", want, queries)
		}
	}
	if len(args) < 9 || args[0].Value != from || args[1].Value != to || args[2].Value != "750e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected time and identity filters to be query args, got %#v", args)
	}
	if len(analytics.ByModel) != 1 || analytics.ByModel[0].Key != "gpt-5.4" || analytics.ByModel[0].RequestCount != 3 {
		t.Fatalf("unexpected model analytics: %+v", analytics.ByModel)
	}
	if len(analytics.ByFeature) != 1 || analytics.ByFeature[0].Key != "workflow" {
		t.Fatalf("unexpected feature analytics: %+v", analytics.ByFeature)
	}
	if len(analytics.ByUser) != 1 || analytics.ByUser[0].Key != "850e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected user analytics: %+v", analytics.ByUser)
	}
	if len(analytics.ByTime) != 1 || analytics.ByTime[0].Key != "2026-06-01T00:01:00Z" || !analytics.ByTime[0].StartedAt.Equal(from.Add(time.Minute)) {
		t.Fatalf("unexpected time analytics: %+v", analytics.ByTime)
	}
	if len(analytics.ByChannel) != 1 || analytics.ByChannel[0].Key != "ch_1" || analytics.ByChannel[0].TotalCost != 0.31 {
		t.Fatalf("unexpected channel analytics: %+v", analytics.ByChannel)
	}
	if len(analytics.ByProvider) != 1 || analytics.ByProvider[0].Key != "openai" || analytics.ByProvider[0].TotalCost != 0.31 {
		t.Fatalf("unexpected provider analytics: %+v", analytics.ByProvider)
	}
	if len(analytics.CrossDimensions) != 3 {
		t.Fatalf("expected three cross dimension buckets, got %+v", analytics.CrossDimensions)
	}
	if analytics.CrossDimensions[0].Dimension != "model_time" ||
		analytics.CrossDimensions[0].Primary != "gpt-5.4" ||
		analytics.CrossDimensions[0].Secondary != "2026-06-01T00:01:00Z" {
		t.Fatalf("unexpected model_time cross dimension bucket: %+v", analytics.CrossDimensions[0])
	}
	if analytics.CrossDimensions[1].Dimension != "user_feature" || analytics.CrossDimensions[1].Primary == "" || analytics.CrossDimensions[1].Secondary != "workflow" {
		t.Fatalf("unexpected user_feature cross dimension bucket: %+v", analytics.CrossDimensions[1])
	}
	if analytics.CrossDimensions[2].Dimension != "feature_time" || analytics.CrossDimensions[2].Primary != "workflow" || analytics.CrossDimensions[2].Secondary != "2026-06-01T00:01:00Z" {
		t.Fatalf("unexpected feature_time cross dimension bucket: %+v", analytics.CrossDimensions[2])
	}
}

func TestClickHouseUsageAnalyticsStoreUsesMetadataUserIdentity(t *testing.T) {
	driverName := "admin_clickhouse_usage_analytics_user_identity"
	queryer := &clickHouseUsageAnalyticsQueryer{}
	registerClickHouseUsageAnalyticsDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	_, err = NewClickHouseUsageAnalyticsStore(db).GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
		UserID:      "user_1",
		From:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	})
	if err != nil {
		t.Fatalf("get clickhouse usage analytics: %v", err)
	}

	queryer.mu.Lock()
	queries := strings.Join(queryer.queries, "\n")
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"COALESCE(NULLIF(JSONExtractString(metadata, 'user_id'), ''), toString(user_id))",
		"COALESCE(NULLIF(JSONExtractString(metadata, 'user_id'), ''), toString(user_id)) = ?",
	} {
		if !strings.Contains(queries, want) {
			t.Fatalf("expected ClickHouse user analytics to include %q, got %s", want, queries)
		}
	}
	foundUserArg := false
	for _, arg := range args {
		if arg.Value == "user_1" {
			foundUserArg = true
			break
		}
	}
	if !foundUserArg {
		t.Fatalf("expected original user id to be query arg, got %#v", args)
	}
}

func TestClickHouseUsageAnalyticsStoreQueriesRelayRouteDecisionEvidence(t *testing.T) {
	driverName := "admin_clickhouse_route_decision_evidence"
	queryer := &clickHouseUsageAnalyticsQueryer{}
	registerClickHouseUsageAnalyticsDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	evidence, err := NewClickHouseUsageAnalyticsStore(db).ListRelayRouteDecisionEvidence(
		context.Background(),
		[]string{" req_batch_1 ", "req_batch_2", "req_batch_1"},
		"batch",
		"allowed",
	)
	if err != nil {
		t.Fatalf("list route decision evidence: %v", err)
	}

	queryer.mu.Lock()
	queries := strings.Join(queryer.queries, "\n")
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"FROM request_logs",
		"service = 'relay'",
		"JSONExtractString(metadata, 'event') = 'relay.route_decision'",
		"JSONExtractString(metadata, 'relay_api_type') = ?",
		"JSONExtractString(metadata, 'relay_route_result') = ?",
		"ORDER BY timestamp DESC",
	} {
		if !strings.Contains(queries, want) {
			t.Fatalf("expected route-decision evidence query to include %q, got %s", want, queries)
		}
	}
	if len(args) != 4 ||
		args[0].Value != "req_batch_1" ||
		args[1].Value != "req_batch_2" ||
		args[2].Value != "batch" ||
		args[3].Value != "allowed" {
		t.Fatalf("unexpected route-decision evidence query args: %#v", args)
	}
	item, ok := evidence["req_batch_1"]
	if !ok {
		t.Fatalf("expected evidence for req_batch_1, got %+v", evidence)
	}
	if item.Service != "relay" || item.Endpoint != "/v1/batch" || item.StatusCode != 200 || !strings.Contains(string(item.Metadata), `"relay_route_result":"allowed"`) {
		t.Fatalf("unexpected route-decision evidence row: %+v", item)
	}
}

func TestClickHouseUsageAnalyticsStoreMapsGranularityFunctions(t *testing.T) {
	for granularity, want := range map[string]string{
		"second": "toStartOfSecond(timestamp)",
		"minute": "toStartOfMinute(timestamp)",
		"hour":   "toStartOfHour(timestamp)",
		"day":    "toStartOfDay(timestamp)",
		"week":   "toStartOfWeek(timestamp)",
		"month":  "toStartOfMonth(timestamp)",
	} {
		driverName := "admin_clickhouse_usage_analytics_" + granularity
		queryer := &clickHouseUsageAnalyticsQueryer{}
		registerClickHouseUsageAnalyticsDriver(driverName, queryer)

		db, err := sql.Open(driverName, "")
		if err != nil {
			t.Fatalf("open capture db: %v", err)
		}
		_, err = NewClickHouseUsageAnalyticsStore(db).GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
			From:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			To:          time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			Granularity: granularity,
		})
		db.Close()
		if err != nil {
			t.Fatalf("get clickhouse usage analytics for %s: %v", granularity, err)
		}
		queryer.mu.Lock()
		queries := strings.Join(queryer.queries, "\n")
		queryer.mu.Unlock()
		if !strings.Contains(queries, want) {
			t.Fatalf("expected %s query to use %q, got %s", granularity, want, queries)
		}
	}
}

var clickHouseUsageAnalyticsDrivers sync.Map

func registerClickHouseUsageAnalyticsDriver(name string, queryer *clickHouseUsageAnalyticsQueryer) {
	if _, loaded := clickHouseUsageAnalyticsDrivers.LoadOrStore(name, queryer); loaded {
		return
	}
	sql.Register(name, clickHouseUsageAnalyticsDriver{name: name})
}

type clickHouseUsageAnalyticsQueryer struct {
	mu      sync.Mutex
	queries []string
	args    []driver.NamedValue
}

type clickHouseUsageAnalyticsDriver struct {
	name string
}

func (d clickHouseUsageAnalyticsDriver) Open(_ string) (driver.Conn, error) {
	queryer, _ := clickHouseUsageAnalyticsDrivers.Load(d.name)
	return clickHouseUsageAnalyticsConn{queryer: queryer.(*clickHouseUsageAnalyticsQueryer)}, nil
}

type clickHouseUsageAnalyticsConn struct {
	queryer *clickHouseUsageAnalyticsQueryer
}

func (c clickHouseUsageAnalyticsConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c clickHouseUsageAnalyticsConn) Close() error {
	return nil
}

func (c clickHouseUsageAnalyticsConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c clickHouseUsageAnalyticsConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.queryer.mu.Lock()
	c.queryer.queries = append(c.queryer.queries, query)
	c.queryer.args = append(c.queryer.args, args...)
	c.queryer.mu.Unlock()

	switch {
	case strings.Contains(query, "relay.route_decision"):
		return clickHouseUsageAnalyticsRows([]string{
			"request_id",
			"toString(id)",
			"timestamp",
			"service",
			"endpoint",
			"method",
			"status_code",
			"duration_ms",
			"request_tokens",
			"response_tokens",
			"model",
			"cost_usd",
			"error",
			"toString(trace_id)",
			"metadata",
		}, [][]driver.Value{
			{
				"req_batch_1",
				"550e8400-e29b-41d4-a716-446655440000",
				time.Date(2026, 6, 16, 0, 1, 0, 0, time.UTC),
				"relay",
				"/v1/batch",
				"POST",
				int64(200),
				int64(42),
				int64(0),
				int64(0),
				"",
				float64(0),
				"",
				"00000000-0000-0000-0000-000000000000",
				`{"event":"relay.route_decision","relay_api_type":"batch","relay_route_result":"allowed"}`,
			},
		}), nil
	case strings.Contains(query, "AS primary_key") && strings.Contains(query, "model AS primary_key"):
		startedAt := time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC)
		return clickHouseUsageAnalyticsRows([]string{"primary_key", "secondary_key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"gpt-5.4", "2026-06-01T00:01:00Z", int64(3), int64(1200), 0.42, startedAt},
		}), nil
	case strings.Contains(query, "AS primary_key") && strings.Contains(query, "feature_type") && strings.Contains(query, "toStartOf") && strings.Contains(query, " AS started_at"):
		startedAt := time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC)
		return clickHouseUsageAnalyticsRows([]string{"primary_key", "secondary_key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"workflow", "2026-06-01T00:01:00Z", int64(3), int64(1200), 0.42, startedAt},
		}), nil
	case strings.Contains(query, "AS primary_key") && strings.Contains(query, "JSONExtractString(metadata, 'user_id')"):
		return clickHouseUsageAnalyticsRows([]string{"primary_key", "secondary_key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"850e8400-e29b-41d4-a716-446655440000", "workflow", int64(3), int64(1200), 0.42, nil},
		}), nil
	case strings.Contains(query, "GROUP BY model"):
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"gpt-5.4", int64(3), int64(1200), 0.42, nil},
		}), nil
	case strings.Contains(query, "GROUP BY COALESCE(NULLIF(JSONExtractString(metadata, 'user_id'), ''), toString(user_id))"):
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"850e8400-e29b-41d4-a716-446655440000", int64(3), int64(1200), 0.42, nil},
		}), nil
	case strings.Contains(query, "toStartOfMinute(timestamp) AS started_at"):
		startedAt := time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC)
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"2026-06-01T00:01:00Z", int64(3), int64(1200), 0.42, startedAt},
		}), nil
	case strings.Contains(query, "GROUP BY COALESCE(NULLIF(JSONExtractString(metadata, 'channel_id'), ''), 'unknown')"):
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"ch_1", int64(2), int64(700), 0.31, nil},
		}), nil
	case strings.Contains(query, "GROUP BY COALESCE(NULLIF(JSONExtractString(metadata, 'provider'), ''), 'unknown')"):
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"openai", int64(2), int64(700), 0.31, nil},
		}), nil
	case strings.Contains(query, "feature_type"):
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"workflow", int64(3), int64(1200), 0.42, nil},
		}), nil
	default:
		return clickHouseUsageAnalyticsRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, nil), nil
	}
}

func (c clickHouseUsageAnalyticsConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

func clickHouseUsageAnalyticsRows(columns []string, rows [][]driver.Value) driver.Rows {
	return &clickHouseRows{columns: columns, rows: rows}
}

type clickHouseRows struct {
	columns []string
	index   int
	rows    [][]driver.Value
}

func (r *clickHouseRows) Columns() []string {
	return r.columns
}

func (r *clickHouseRows) Close() error {
	return nil
}

func (r *clickHouseRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
