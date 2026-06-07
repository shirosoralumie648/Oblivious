package usage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"

	"oblivious/server/internal/relay"
)

func TestSQLRecorder_RecordRelayUsageWritesGatewayFields(t *testing.T) {
	driverName := "usage_record_relay_test"
	recorder := &execCapture{}
	registerCaptureDriver(driverName, recorder)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	err = NewSQLRecorder(db).RecordRelayUsage(context.Background(), relay.RelayUsageLogRecord{
		UserID:           "user_1",
		OrganizationID:   "org_1",
		APITokenID:       "tok_1",
		RequestID:        "req_1",
		APIType:          "chat",
		FeatureType:      "workspace_chat",
		QuotaMode:        "relay_billing",
		Model:            "gpt-4o",
		ChannelID:        "ch_1",
		Provider:         "openai",
		Status:           relay.RelayUsageStatusSuccess,
		StatusCode:       200,
		LatencyMS:        42,
		Cost:             2.8,
		ChannelCost:      1.4,
		PromptTokens:     1000,
		CompletionTokens: 100,
		TotalTokens:      1100,
	})
	if err != nil {
		t.Fatalf("record relay usage: %v", err)
	}

	recorder.mu.Lock()
	query := recorder.query
	args := append([]driver.NamedValue(nil), recorder.args...)
	recorder.mu.Unlock()

	for _, column := range []string{
		"api_type",
		"feature_type",
		"quota_mode",
		"channel_id",
		"provider",
		"api_token_id",
		"status",
		"status_code",
		"latency_ms",
		"cost",
		"channel_cost",
		"request_id",
		"total_tokens",
	} {
		if !strings.Contains(query, column) {
			t.Fatalf("expected insert query to include %s, got %s", column, query)
		}
	}

	if got := argString(args, 2); got != "user_1" {
		t.Fatalf("user arg = %q, want user_1", got)
	}
	if got := argString(args, 4); got != "gpt-4o" {
		t.Fatalf("model arg = %q, want gpt-4o", got)
	}
	if got := argString(args, 10); got != "tok_1" {
		t.Fatalf("api token arg = %q, want tok_1", got)
	}
	if got := argString(args, 11); got != "workspace_chat" {
		t.Fatalf("feature type arg = %q, want workspace_chat", got)
	}
	if got := argString(args, 12); got != "relay_billing" {
		t.Fatalf("quota mode arg = %q, want relay_billing", got)
	}
	if got := argString(args, 13); got != string(relay.RelayUsageStatusSuccess) {
		t.Fatalf("status arg = %q, want success", got)
	}
}

var captureDrivers sync.Map

func registerCaptureDriver(name string, capture *execCapture) {
	if _, loaded := captureDrivers.LoadOrStore(name, capture); loaded {
		return
	}
	sql.Register(name, captureDriver{name: name})
}

type execCapture struct {
	mu    sync.Mutex
	query string
	args  []driver.NamedValue
}

type captureDriver struct {
	name string
}

func (d captureDriver) Open(_ string) (driver.Conn, error) {
	capture, _ := captureDrivers.Load(d.name)
	return captureConn{capture: capture.(*execCapture)}, nil
}

type captureConn struct {
	capture *execCapture
}

func (c captureConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c captureConn) Close() error {
	return nil
}

func (c captureConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c captureConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.capture.mu.Lock()
	c.capture.query = query
	c.capture.args = append([]driver.NamedValue(nil), args...)
	c.capture.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c captureConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return emptyRows{}, nil
}

func (c captureConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string {
	return nil
}

func (emptyRows) Close() error {
	return nil
}

func (emptyRows) Next(_ []driver.Value) error {
	return io.EOF
}

func argString(args []driver.NamedValue, ordinal int) string {
	for _, arg := range args {
		if arg.Ordinal == ordinal {
			value, _ := arg.Value.(string)
			return value
		}
	}
	return ""
}
