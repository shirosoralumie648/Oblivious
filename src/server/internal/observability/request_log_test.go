package observability

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type captureRequestLogSink struct {
	rows []RequestLogRow
}

func (s *captureRequestLogSink) InsertRequestLog(_ context.Context, row RequestLogRow) error {
	s.rows = append(s.rows, row)
	return nil
}

func TestRequestLogRowFromEventMapsRelayCompletion(t *testing.T) {
	timestamp := time.Date(2026, 6, 5, 9, 30, 0, 123000000, time.UTC)

	row, err := RequestLogRowFromEvent(Event{
		Component:      "relay",
		Event:          "request.completed",
		RequestID:      "550e8400-e29b-41d4-a716-446655440000",
		TraceID:        "650e8400-e29b-41d4-a716-446655440000",
		OrganizationID: "750e8400-e29b-41d4-a716-446655440000",
		UserID:         "850e8400-e29b-41d4-a716-446655440000",
		Method:         "POST",
		Route:          "/v1/chat/completions",
		Status:         200,
		Latency:        1250 * time.Millisecond,
		RelayAPIType:   "chat",
		ChannelID:      "channel_primary",
		Provider:       "openai",
		Fields: map[string]any{
			"model":         "gpt-5.4",
			"input_tokens":  321,
			"output_tokens": int64(45),
			"cost_usd":      0.0123,
			"cache_hit":     true,
			"prompt":        "do not persist this prompt",
			"body":          map[string]any{"messages": []string{"secret"}},
			"api_key":       "sk-secret",
		},
	}, timestamp)
	if err != nil {
		t.Fatalf("map request log row: %v", err)
	}

	if row.ID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected request id to become row id, got %q", row.ID)
	}
	if !row.Timestamp.Equal(timestamp) {
		t.Fatalf("expected timestamp %s, got %s", timestamp, row.Timestamp)
	}
	if row.OrganizationID != "750e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected organization id %q", row.OrganizationID)
	}
	if row.UserID != "850e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected user id %q", row.UserID)
	}
	if row.Service != "relay" || row.Endpoint != "/v1/chat/completions" || row.Method != "POST" {
		t.Fatalf("unexpected routing columns: %+v", row)
	}
	if row.StatusCode != 200 || row.DurationMS != 1250 {
		t.Fatalf("unexpected status/duration: %+v", row)
	}
	if row.RequestTokens != 321 || row.ResponseTokens != 45 {
		t.Fatalf("unexpected tokens: %+v", row)
	}
	if row.Model != "gpt-5.4" {
		t.Fatalf("unexpected model %q", row.Model)
	}
	if math.Abs(row.CostUSD-0.0123) > 0.000001 {
		t.Fatalf("unexpected cost %.8f", row.CostUSD)
	}
	if row.TraceID != "650e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected trace id %q", row.TraceID)
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		t.Fatalf("metadata should be JSON: %v\n%s", err, row.Metadata)
	}
	if metadata["event"] != "request.completed" ||
		metadata["relay_api_type"] != "chat" ||
		metadata["channel_id"] != "channel_primary" ||
		metadata["provider"] != "openai" ||
		metadata["cache_hit"] != true {
		t.Fatalf("expected operational metadata to survive, got %+v", metadata)
	}
	for _, forbidden := range []string{"prompt", "body", "api_key"} {
		if _, ok := metadata[forbidden]; ok {
			t.Fatalf("metadata leaked sensitive field %q: %+v", forbidden, metadata)
		}
	}
	for _, forbiddenText := range []string{"do not persist", "secret", "sk-secret"} {
		if strings.Contains(row.Metadata, forbiddenText) {
			t.Fatalf("metadata leaked sensitive text %q: %s", forbiddenText, row.Metadata)
		}
	}
}

func TestRequestLogRecorderNoopsWithoutSink(t *testing.T) {
	if err := WriteRequestLog(context.Background(), nil, Event{Component: "relay", Event: "request.completed"}, time.Now()); err != nil {
		t.Fatalf("nil sink should be safe, got %v", err)
	}
	if err := (NoopRequestLogSink{}).InsertRequestLog(context.Background(), RequestLogRow{}); err != nil {
		t.Fatalf("noop sink should not fail: %v", err)
	}
}

func TestWriteRequestLogInsertsSanitizedRow(t *testing.T) {
	sink := &captureRequestLogSink{}
	timestamp := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)

	err := WriteRequestLog(context.Background(), sink, Event{
		Component:      "workflow",
		Event:          "request.completed",
		RequestID:      "950e8400-e29b-41d4-a716-446655440000",
		TraceID:        "a50e8400-e29b-41d4-a716-446655440000",
		OrganizationID: "b50e8400-e29b-41d4-a716-446655440000",
		UserID:         "c50e8400-e29b-41d4-a716-446655440000",
		Method:         "POST",
		Route:          "/api/v1/workflows/run",
		Status:         502,
		Latency:        75 * time.Millisecond,
		FailureReason:  "upstream_timeout",
		Fields: map[string]any{
			"request_tokens":  12,
			"response_tokens": 8,
			"model":           "claude-4.5",
			"cost":            "0.0042",
			"response":        "do not persist model response",
		},
	}, timestamp)
	if err != nil {
		t.Fatalf("write request log: %v", err)
	}

	if len(sink.rows) != 1 {
		t.Fatalf("expected one inserted row, got %d", len(sink.rows))
	}
	row := sink.rows[0]
	if row.Service != "workflow" || row.Error != "upstream_timeout" {
		t.Fatalf("unexpected inserted row: %+v", row)
	}
	if row.RequestTokens != 12 || row.ResponseTokens != 8 {
		t.Fatalf("unexpected inserted token columns: %+v", row)
	}
	if math.Abs(row.CostUSD-0.0042) > 0.000001 {
		t.Fatalf("unexpected inserted cost %.8f", row.CostUSD)
	}
	if strings.Contains(row.Metadata, "do not persist") {
		t.Fatalf("inserted metadata leaked response body: %s", row.Metadata)
	}
}

func TestSQLRequestLogSinkInsertsClickHouseRequestLogs(t *testing.T) {
	driverName := "observability_request_log_test"
	capture := &requestLogExecCapture{}
	registerRequestLogCaptureDriver(driverName, capture)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	timestamp := time.Date(2026, 6, 5, 11, 0, 0, 456000000, time.UTC)
	row := RequestLogRow{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		Timestamp:      timestamp,
		OrganizationID: "750e8400-e29b-41d4-a716-446655440000",
		UserID:         "850e8400-e29b-41d4-a716-446655440000",
		Service:        "relay",
		Endpoint:       "/v1/chat/completions",
		Method:         "POST",
		StatusCode:     200,
		DurationMS:     123,
		RequestTokens:  10,
		ResponseTokens: 20,
		Model:          "gpt-5.4",
		CostUSD:        0.034,
		Error:          "",
		TraceID:        "650e8400-e29b-41d4-a716-446655440000",
		Metadata:       `{"provider":"openai"}`,
	}

	if err := NewSQLRequestLogSink(db).InsertRequestLog(context.Background(), row); err != nil {
		t.Fatalf("insert request log row: %v", err)
	}

	capture.mu.Lock()
	query := capture.query
	args := append([]driver.NamedValue(nil), capture.args...)
	capture.mu.Unlock()

	for _, want := range []string{
		"INSERT INTO request_logs",
		"id",
		"timestamp",
		"organization_id",
		"user_id",
		"service",
		"endpoint",
		"status_code",
		"duration_ms",
		"request_tokens",
		"response_tokens",
		"cost_usd",
		"metadata",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected insert query to include %q, got %s", want, query)
		}
	}

	if got := requestLogArg(args, 1); got != row.ID {
		t.Fatalf("id arg = %#v, want %q", got, row.ID)
	}
	if got := requestLogArg(args, 2); got != timestamp {
		t.Fatalf("timestamp arg = %#v, want %s", got, timestamp)
	}
	if got := requestLogArg(args, 7); got != row.Method {
		t.Fatalf("method arg = %#v, want %q", got, row.Method)
	}
	if got := requestLogArg(args, 16); got != row.Metadata {
		t.Fatalf("metadata arg = %#v, want %q", got, row.Metadata)
	}
}

func TestClickHouseRequestLogsMigrationMatchesSpec(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/clickhouse/0001_request_logs.sql")
	if err != nil {
		t.Fatalf("read ClickHouse request logs migration: %v", err)
	}
	ddl := string(raw)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS request_logs",
		"id UUID",
		"timestamp DateTime64(3)",
		"organization_id UUID",
		"user_id UUID",
		"service String",
		"endpoint String",
		"method String",
		"status_code UInt16",
		"duration_ms UInt32",
		"request_tokens UInt32",
		"response_tokens UInt32",
		"model String",
		"cost_usd Float64",
		"error String",
		"trace_id UUID",
		"metadata String",
		"ENGINE = MergeTree()",
		"PARTITION BY toYYYYMM(timestamp)",
		"ORDER BY (organization_id, timestamp, service)",
	} {
		if !strings.Contains(ddl, want) {
			t.Fatalf("ClickHouse DDL missing %q:\n%s", want, ddl)
		}
	}
}

var requestLogCaptureDrivers sync.Map

func registerRequestLogCaptureDriver(name string, capture *requestLogExecCapture) {
	if _, loaded := requestLogCaptureDrivers.LoadOrStore(name, capture); loaded {
		return
	}
	sql.Register(name, requestLogCaptureDriver{name: name})
}

type requestLogExecCapture struct {
	mu    sync.Mutex
	query string
	args  []driver.NamedValue
}

type requestLogCaptureDriver struct {
	name string
}

func (d requestLogCaptureDriver) Open(_ string) (driver.Conn, error) {
	capture, _ := requestLogCaptureDrivers.Load(d.name)
	return requestLogCaptureConn{capture: capture.(*requestLogExecCapture)}, nil
}

type requestLogCaptureConn struct {
	capture *requestLogExecCapture
}

func (c requestLogCaptureConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c requestLogCaptureConn) Close() error {
	return nil
}

func (c requestLogCaptureConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c requestLogCaptureConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.capture.mu.Lock()
	c.capture.query = query
	c.capture.args = append([]driver.NamedValue(nil), args...)
	c.capture.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c requestLogCaptureConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return requestLogEmptyRows{}, nil
}

func (c requestLogCaptureConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type requestLogEmptyRows struct{}

func (requestLogEmptyRows) Columns() []string {
	return nil
}

func (requestLogEmptyRows) Close() error {
	return nil
}

func (requestLogEmptyRows) Next(_ []driver.Value) error {
	return io.EOF
}

func requestLogArg(args []driver.NamedValue, ordinal int) any {
	for _, arg := range args {
		if arg.Ordinal == ordinal {
			return arg.Value
		}
	}
	return nil
}
