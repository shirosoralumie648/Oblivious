package console

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestSQLStoreGetUsageSummaryAggregatesOperatingDimensions(t *testing.T) {
	driverName := "console_usage_summary_test"
	queryer := &usageSummaryQueryer{}
	registerConsoleUsageDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	summary, err := NewSQLStore(db).GetUsageSummary(context.Background(), "org_1", "user_1")
	if err != nil {
		t.Fatalf("get usage summary: %v", err)
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	var decoded struct {
		ByModel []struct {
			Key          string  `json:"key"`
			RequestCount int     `json:"requestCount"`
			TotalCost    float64 `json:"totalCost"`
			TotalTokens  int     `json:"totalTokens"`
		} `json:"byModel"`
		ByFeature []struct {
			Key          string  `json:"key"`
			RequestCount int     `json:"requestCount"`
			TotalCost    float64 `json:"totalCost"`
			TotalTokens  int     `json:"totalTokens"`
		} `json:"byFeature"`
		ByUser []struct {
			Key          string  `json:"key"`
			RequestCount int     `json:"requestCount"`
			TotalCost    float64 `json:"totalCost"`
			TotalTokens  int     `json:"totalTokens"`
		} `json:"byUser"`
		TimeSeries []struct {
			Bucket       string  `json:"bucket"`
			RequestCount int     `json:"requestCount"`
			TotalCost    float64 `json:"totalCost"`
			TotalTokens  int     `json:"totalTokens"`
		} `json:"timeSeries"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode usage summary json: %v", err)
	}

	if len(decoded.ByModel) != 2 || decoded.ByModel[0].Key != "gpt-4o" || decoded.ByModel[0].RequestCount != 3 || decoded.ByModel[0].TotalTokens != 300 || decoded.ByModel[0].TotalCost != 0.09 {
		t.Fatalf("unexpected byModel summary: %+v", decoded.ByModel)
	}
	if len(decoded.ByFeature) != 2 || decoded.ByFeature[0].Key != "chat" || decoded.ByFeature[0].RequestCount != 4 {
		t.Fatalf("unexpected byFeature summary: %+v", decoded.ByFeature)
	}
	if len(decoded.ByUser) != 2 || decoded.ByUser[0].Key != "user_2" || decoded.ByUser[0].TotalCost != 0.12 {
		t.Fatalf("expected users sorted by cost descending, got %+v", decoded.ByUser)
	}
	if len(decoded.TimeSeries) != 2 || decoded.TimeSeries[0].Bucket != "2026-06-04" || decoded.TimeSeries[1].Bucket != "2026-06-05" {
		t.Fatalf("unexpected timeSeries summary: %+v", decoded.TimeSeries)
	}

	queryer.mu.Lock()
	queries := strings.Join(queryer.queries, "\n")
	queryer.mu.Unlock()
	for _, expected := range []string{"GROUP BY model_id", "feature_type", "GROUP BY user_id", "date_trunc('day', created_at)"} {
		if !strings.Contains(queries, expected) {
			t.Fatalf("expected query bundle to include %q, got %s", expected, queries)
		}
	}
}

var consoleUsageDrivers sync.Map

func registerConsoleUsageDriver(name string, queryer *usageSummaryQueryer) {
	if _, loaded := consoleUsageDrivers.LoadOrStore(name, queryer); loaded {
		return
	}
	sql.Register(name, consoleUsageDriver{name: name})
}

type usageSummaryQueryer struct {
	mu      sync.Mutex
	queries []string
}

type consoleUsageDriver struct {
	name string
}

func (d consoleUsageDriver) Open(_ string) (driver.Conn, error) {
	queryer, _ := consoleUsageDrivers.Load(d.name)
	return consoleUsageConn{queryer: queryer.(*usageSummaryQueryer)}, nil
}

type consoleUsageConn struct {
	queryer *usageSummaryQueryer
}

func (c consoleUsageConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c consoleUsageConn) Close() error {
	return nil
}

func (c consoleUsageConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c consoleUsageConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.queryer.mu.Lock()
	c.queryer.queries = append(c.queryer.queries, query)
	c.queryer.mu.Unlock()

	switch {
	case strings.Contains(query, "SELECT COALESCE(SUM(request_count), 0)") && !strings.Contains(query, "GROUP BY"):
		return &usageRows{columns: []string{"requests"}, rows: [][]driver.Value{{int64(5)}}}, nil
	case strings.Contains(query, "ORDER BY created_at DESC"):
		return &usageRows{columns: []string{
			"id", "api_token_id", "request_id", "api_type", "model_id", "status", "status_code", "error_code", "latency_ms", "cost", "input_tokens", "output_tokens", "total_tokens", "created_at",
		}}, nil
	case strings.Contains(query, "GROUP BY model_id"):
		return aggregateRows([][]driver.Value{
			{"gpt-4o", int64(3), int64(300), 0.09},
			{"gpt-4.1", int64(2), int64(180), 0.04},
		}), nil
	case strings.Contains(query, "feature_type"):
		return aggregateRows([][]driver.Value{
			{"chat", int64(4), int64(360), 0.1},
			{"workflow", int64(1), int64(120), 0.03},
		}), nil
	case strings.Contains(query, "GROUP BY user_id"):
		return aggregateRows([][]driver.Value{
			{"user_2", int64(2), int64(240), 0.12},
			{"user_1", int64(3), int64(240), 0.01},
		}), nil
	case strings.Contains(query, "date_trunc('day', created_at)"):
		return &usageRows{columns: []string{"bucket", "request_count", "total_tokens", "total_cost"}, rows: [][]driver.Value{
			{"2026-06-04", int64(2), int64(200), 0.04},
			{"2026-06-05", int64(3), int64(280), 0.09},
		}}, nil
	default:
		return &usageRows{}, nil
	}
}

func (c consoleUsageConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

func aggregateRows(rows [][]driver.Value) driver.Rows {
	return &usageRows{columns: []string{"key", "request_count", "total_tokens", "total_cost"}, rows: rows}
}

type usageRows struct {
	columns []string
	index   int
	rows    [][]driver.Value
}

func (r *usageRows) Columns() []string {
	return r.columns
}

func (r *usageRows) Close() error {
	return nil
}

func (r *usageRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
