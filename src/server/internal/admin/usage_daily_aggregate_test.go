package admin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestSQLStoreRefreshUsageDailyAggregatesUpsertsDailyRows(t *testing.T) {
	driverName := "admin_usage_daily_aggregate_refresh"
	capture := &usageDailyAggregateCapture{}
	registerUsageDailyAggregateDriver(driverName, capture)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	if err := NewSQLStore(db).RefreshUsageDailyAggregates(context.Background(), from, to); err != nil {
		t.Fatalf("refresh daily usage aggregates: %v", err)
	}

	capture.mu.Lock()
	query := capture.execQuery
	args := append([]driver.NamedValue(nil), capture.execArgs...)
	capture.mu.Unlock()

	for _, want := range []string{
		"INSERT INTO usage_daily_aggregates",
		"date_trunc('day', created_at)::date",
		"SUM(request_count)",
		"SUM(COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens))",
		"SUM(cost)",
		"SUM(channel_cost)",
		"ON CONFLICT (usage_date, organization_id, user_id, model_id, feature_type, channel_id, provider, status)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected refresh query to contain %q, got %s", want, query)
		}
	}
	if len(args) != 2 || args[0].Value != from || args[1].Value != to {
		t.Fatalf("expected refresh range args, got %#v", args)
	}
}

func TestSQLStoreUsageAnalyticsDayGranularityUsesDailyAggregateTable(t *testing.T) {
	driverName := "admin_usage_daily_aggregate_analytics"
	capture := &usageDailyAggregateCapture{}
	registerUsageDailyAggregateDriver(driverName, capture)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	analytics, err := NewSQLStore(db).GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
		From:        from,
		To:          to,
		Granularity: "day",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("get daily aggregate usage analytics: %v", err)
	}

	capture.mu.Lock()
	queries := strings.Join(capture.queries, "\n")
	capture.mu.Unlock()

	if !strings.Contains(queries, "FROM usage_daily_aggregates") {
		t.Fatalf("expected day analytics to read daily aggregates, got %s", queries)
	}
	if strings.Contains(queries, "FROM usage_records") {
		t.Fatalf("day analytics should not scan raw usage_records when daily aggregates are available, got %s", queries)
	}
	if len(analytics.ByTime) != 1 || analytics.ByTime[0].Key != "2026-06-01" || analytics.ByTime[0].TotalCost != 0.42 {
		t.Fatalf("unexpected daily time analytics: %+v", analytics.ByTime)
	}
}

func TestSQLStoreUsageDailyAggregatesPostgresRefreshAndAnalytics(t *testing.T) {
	store, ctx := testUsageDailyAggregateSQLStore(t)

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	if err := store.RefreshUsageDailyAggregates(ctx, from, to); err != nil {
		t.Fatalf("refresh daily aggregates: %v", err)
	}

	var requestCount int
	var totalTokens int
	var totalCost float64
	var channelCost float64
	if err := store.db.QueryRowContext(ctx, `
		SELECT request_count, total_tokens, total_cost, channel_cost
		FROM usage_daily_aggregates
		WHERE usage_date = $1
		  AND organization_id = 'org_daily'
		  AND user_id = 'user_daily'
		  AND model_id = 'gpt-4o'
		  AND feature_type = 'chat'
		  AND channel_id = 'channel_openai'
		  AND provider = 'openai'
		  AND status = 'success'
	`, from).Scan(&requestCount, &totalTokens, &totalCost, &channelCost); err != nil {
		t.Fatalf("read daily aggregate: %v", err)
	}
	if requestCount != 3 || totalTokens != 230 || totalCost != 0.42 || channelCost != 0.21 {
		t.Fatalf("unexpected daily aggregate values: requests=%d tokens=%d cost=%v channelCost=%v", requestCount, totalTokens, totalCost, channelCost)
	}

	analytics, err := store.GetUsageAnalytics(ctx, UsageAnalyticsFilter{
		OrganizationID: "org_daily",
		From:           from,
		To:             to,
		Granularity:    "day",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("get daily analytics: %v", err)
	}

	if len(analytics.ByTime) != 2 {
		t.Fatalf("expected two daily time buckets, got %+v", analytics.ByTime)
	}
	first := analytics.ByTime[0]
	if first.Key != "2026-06-01" || first.RequestCount != 3 || first.TotalTokens != 230 || first.TotalCost != 0.42 {
		t.Fatalf("unexpected first daily bucket: %+v", first)
	}
	if len(analytics.ByModel) == 0 || analytics.ByModel[0].Key != "gpt-4o" || analytics.ByModel[0].TotalCost != 0.42 {
		t.Fatalf("expected model analytics from daily aggregate table, got %+v", analytics.ByModel)
	}
	if !hasUsageAnalyticsCrossBucket(analytics.CrossDimensions, "model_time", "gpt-4o", "2026-06-01", 0.42) {
		t.Fatalf("expected model_time cross bucket from daily aggregates, got %+v", analytics.CrossDimensions)
	}
}

func TestSQLStoreUsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens(t *testing.T) {
	store, ctx := testUsageDailyAggregateSQLStore(t)

	analytics, err := store.GetUsageAnalytics(ctx, UsageAnalyticsFilter{
		OrganizationID: "org_daily",
		Model:          "gpt-4o",
		From:           time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		To:             time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "hour",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("get raw usage analytics: %v", err)
	}
	if len(analytics.ByModel) != 1 {
		t.Fatalf("expected one model bucket, got %+v", analytics.ByModel)
	}
	if analytics.ByModel[0].TotalTokens != 230 {
		t.Fatalf("expected raw analytics to fall back to prompt plus completion tokens when total_tokens is zero, got %+v", analytics.ByModel[0])
	}
}

func TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens(t *testing.T) {
	store, ctx := testUsageDailyAggregateSQLStore(t)

	entries, _, err := store.ListUsageLogs(ctx, UsageLogFilter{
		OrganizationID: "org_daily",
		Model:          "gpt-4o",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list usage logs: %v", err)
	}
	for _, entry := range entries {
		if entry.ID == "usage_daily_1" {
			if entry.TotalTokens != 100 {
				t.Fatalf("expected usage log to fall back to prompt plus completion tokens when total_tokens is zero, got %+v", entry)
			}
			return
		}
	}
	t.Fatalf("expected usage_daily_1 in usage logs, got %+v", entries)
}

func hasUsageAnalyticsCrossBucket(buckets []UsageAnalyticsBucket, dimension, primary, secondary string, cost float64) bool {
	for _, bucket := range buckets {
		if bucket.Dimension == dimension && bucket.Primary == primary && bucket.Secondary == secondary && bucket.TotalCost == cost {
			return true
		}
	}
	return false
}

func testUsageDailyAggregateSQLStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for usage daily aggregate SQL tests")
		}
		t.Skip("TEST_DATABASE_URL is required for usage daily aggregate SQL tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104279)`); err != nil {
		t.Fatalf("lock usage daily aggregate test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104279)`); err != nil {
			t.Fatalf("unlock usage daily aggregate test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS usage_daily_aggregates CASCADE`,
		`DROP TABLE IF EXISTS usage_records CASCADE`,
		`DROP TABLE IF EXISTS conversations CASCADE`,
		`DROP TABLE IF EXISTS workspaces CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE usage_records (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
			organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
			conversation_id TEXT REFERENCES conversations(id) ON DELETE SET NULL,
			model_id TEXT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 1,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			api_type TEXT,
			feature_type TEXT,
			quota_mode TEXT,
			channel_id TEXT,
			provider TEXT,
			api_token_id TEXT,
			status TEXT,
			status_code INTEGER,
			latency_ms INTEGER,
			cost NUMERIC(15,6) NOT NULL DEFAULT 0,
			channel_cost NUMERIC(15,6) NOT NULL DEFAULT 0,
			request_id TEXT,
			error_code TEXT,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`INSERT INTO users (id, email, name) VALUES ('user_daily', 'daily@example.test', 'Daily User')`,
		`INSERT INTO organizations (id, name) VALUES ('org_daily', 'Daily Org')`,
		`INSERT INTO workspaces (id, user_id, organization_id, name) VALUES ('workspace_daily', 'user_daily', 'org_daily', 'Daily Workspace')`,
		`INSERT INTO conversations (id, user_id, workspace_id, organization_id, title) VALUES ('conversation_daily', 'user_daily', 'workspace_daily', 'org_daily', 'Daily Conversation')`,
		`INSERT INTO usage_records (id, user_id, workspace_id, organization_id, conversation_id, model_id, request_count, input_tokens, output_tokens, api_type, feature_type, channel_id, provider, api_token_id, status, cost, channel_cost, total_tokens, created_at) VALUES
			('usage_daily_1', 'user_daily', 'workspace_daily', 'org_daily', 'conversation_daily', 'gpt-4o', 1, 40, 60, 'chat', 'chat', 'channel_openai', 'openai', 'token_daily', 'success', 0.12, 0.06, 0, '2026-06-01T08:00:00Z'),
			('usage_daily_2', 'user_daily', 'workspace_daily', 'org_daily', 'conversation_daily', 'gpt-4o', 2, 50, 80, 'chat', 'chat', 'channel_openai', 'openai', 'token_daily', 'success', 0.30, 0.15, 130, '2026-06-01T12:00:00Z'),
			('usage_daily_3', 'user_daily', 'workspace_daily', 'org_daily', 'conversation_daily', 'gpt-4o-mini', 1, 10, 20, 'chat', 'chat', 'channel_openai', 'openai', 'token_daily', 'success', 0.05, 0.02, 30, '2026-06-02T08:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare usage daily aggregate database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0074_usage_daily_aggregates.sql")
	if err != nil {
		t.Fatalf("read usage daily aggregate migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply usage daily aggregate migration: %v", err)
	}

	return NewSQLStore(database), context.Background()
}

var usageDailyAggregateDrivers sync.Map

func registerUsageDailyAggregateDriver(name string, capture *usageDailyAggregateCapture) {
	if _, loaded := usageDailyAggregateDrivers.LoadOrStore(name, capture); loaded {
		return
	}
	sql.Register(name, usageDailyAggregateDriver{name: name})
}

type usageDailyAggregateCapture struct {
	mu        sync.Mutex
	execQuery string
	execArgs  []driver.NamedValue
	queries   []string
}

type usageDailyAggregateDriver struct {
	name string
}

func (d usageDailyAggregateDriver) Open(_ string) (driver.Conn, error) {
	capture, _ := usageDailyAggregateDrivers.Load(d.name)
	return usageDailyAggregateConn{capture: capture.(*usageDailyAggregateCapture)}, nil
}

type usageDailyAggregateConn struct {
	capture *usageDailyAggregateCapture
}

func (c usageDailyAggregateConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c usageDailyAggregateConn) Close() error {
	return nil
}

func (c usageDailyAggregateConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c usageDailyAggregateConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.capture.mu.Lock()
	c.capture.execQuery = query
	c.capture.execArgs = append([]driver.NamedValue(nil), args...)
	c.capture.mu.Unlock()
	return driver.RowsAffected(2), nil
}

func (c usageDailyAggregateConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.capture.mu.Lock()
	c.capture.queries = append(c.capture.queries, query)
	c.capture.mu.Unlock()

	switch {
	case strings.Contains(query, "AS primary_key"):
		return usageDailyAggregateRows([]string{"primary_key", "secondary_key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"gpt-4o", "2026-06-01", int64(2), int64(1200), 0.42, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		}), nil
	case strings.Contains(query, "usage_date AS key") || strings.Contains(query, "to_char(usage_date"):
		return usageDailyAggregateRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"2026-06-01", int64(2), int64(1200), 0.42, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		}), nil
	default:
		return usageDailyAggregateRows([]string{"key", "request_count", "total_tokens", "total_cost", "started_at"}, [][]driver.Value{
			{"gpt-4o", int64(2), int64(1200), 0.42, nil},
		}), nil
	}
}

func (c usageDailyAggregateConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

func usageDailyAggregateRows(columns []string, rows [][]driver.Value) driver.Rows {
	return &usageDailyAggregateRowSet{columns: columns, rows: rows}
}

type usageDailyAggregateRowSet struct {
	columns []string
	index   int
	rows    [][]driver.Value
}

func (r *usageDailyAggregateRowSet) Columns() []string {
	return r.columns
}

func (r *usageDailyAggregateRowSet) Close() error {
	return nil
}

func (r *usageDailyAggregateRowSet) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
