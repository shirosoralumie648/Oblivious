package usage

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

	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/types"
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
	effectiveFrom := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	err = NewSQLRecorder(db).RecordRelayUsage(context.Background(), relay.RelayUsageLogRecord{
		UserID:         "user_1",
		OrganizationID: "org_1",
		APITokenID:     "tok_1",
		RequestID:      "req_1",
		APIType:        "chat",
		FeatureType:    "workspace_chat",
		QuotaMode:      "relay_billing",
		Model:          "gpt-4o",
		ChannelID:      "ch_1",
		Provider:       "openai",
		Status:         relay.RelayUsageStatusSuccess,
		StatusCode:     200,
		LatencyMS:      42,
		Cost:           2.8,
		ChannelCost:    1.4,
		PriceSnapshot: &relay.PricingQuote{
			Model:             "gpt-4o",
			APIType:           "chat",
			Currency:          "quota",
			Source:            "initial_catalog",
			EffectiveFrom:     &effectiveFrom,
			Subtotal:          2.8,
			GroupMultiplier:   1,
			ChannelMultiplier: 1,
			TotalCost:         2.8,
			Dimensions: []relay.PricingQuoteDimension{
				{
					PricingEntryID: "rpe_chat_gpt4o_prompt_v1",
					Model:          "gpt-4o",
					APIType:        "chat",
					Dimension:      types.DimPromptTokens,
					UnitCost:       0.002,
					Markup:         1,
					UnitPrice:      0.002,
					Quantity:       1000,
					Amount:         2.0,
					Currency:       "quota",
					Source:         "initial_catalog",
					EffectiveFrom:  &effectiveFrom,
				},
			},
		},
		PriceCurrency:      "quota",
		PriceSource:        "initial_catalog",
		PriceEffectiveFrom: &effectiveFrom,
		PromptTokens:       1000,
		CompletionTokens:   100,
		TotalTokens:        1100,
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
		"price_snapshot",
		"price_currency",
		"price_source",
		"price_effective_from",
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
	if got := argString(args, 22); !strings.Contains(got, `"pricingEntryId":"rpe_chat_gpt4o_prompt_v1"`) || !strings.Contains(got, `"totalCost":2.8`) {
		t.Fatalf("price snapshot arg missing catalog entry and total cost: %s", got)
	}
	if got := argString(args, 23); got != "quota" {
		t.Fatalf("price currency arg = %q, want quota", got)
	}
	if got := argString(args, 24); got != "initial_catalog" {
		t.Fatalf("price source arg = %q, want initial_catalog", got)
	}
}

func TestRelayUsagePriceSnapshotsMigrationDeclaresImmutableFields(t *testing.T) {
	source, err := os.ReadFile("../../migrations/0088_relay_usage_price_snapshots.sql")
	if err != nil {
		t.Fatalf("read relay usage price snapshot migration: %v", err)
	}
	text := string(source)
	for _, needle := range []string{
		"price_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
		"price_currency TEXT NOT NULL DEFAULT ''",
		"price_source TEXT NOT NULL DEFAULT ''",
		"price_effective_from TIMESTAMPTZ",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected migration to contain %q, got %s", needle, text)
		}
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
