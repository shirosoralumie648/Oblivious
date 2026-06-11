package mcp

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

func executeDatetimeOpsTool(t *testing.T, name string, args map[string]any) *ToolResult {
	t.Helper()
	tool, ok := GetBuiltinTool(name)
	if !ok {
		t.Fatalf("builtin %s not registered", name)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("%s returned error: %v", name, err)
	}
	return result
}

var datetimeOpsToolNames = []string{
	"timestamp_now", "timestamp_to_date", "date_to_timestamp", "date_format",
	"date_add", "date_diff", "timezone_convert", "duration_parse",
	"duration_format", "date_weekday", "date_is_weekend", "date_start_of_day",
	"cron_next", "cron_describe",
}

func TestDatetimeOpsToolsRegisteredAndDefaultEnabled(t *testing.T) {
	for _, name := range datetimeOpsToolNames {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("expected %s to be registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("expected %s to be default commercial enabled", name)
		}
	}
}

func TestTimestampNowReturnsParseableUnixSeconds(t *testing.T) {
	before := time.Now().Unix() - 1
	result := executeDatetimeOpsTool(t, "timestamp_now", nil)
	value, err := strconv.ParseInt(result.Content, 10, 64)
	if err != nil {
		t.Fatalf("timestamp_now output %q is not an integer: %v", result.Content, err)
	}
	if value < before || value > time.Now().Unix()+1 {
		t.Fatalf("timestamp_now output %d outside expected range", value)
	}
}

func TestTimestampDateRoundTrip(t *testing.T) {
	result := executeDatetimeOpsTool(t, "timestamp_to_date", map[string]any{"timestamp": 1718064000})
	if result.Content != "2024-06-11T00:00:00Z" {
		t.Fatalf("timestamp_to_date(1718064000) = %q, want 2024-06-11T00:00:00Z", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_to_timestamp", map[string]any{"date": "2024-06-11T00:00:00Z"})
	if result.Content != "1718064000" {
		t.Fatalf("date_to_timestamp = %q, want 1718064000", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_to_timestamp", map[string]any{"date": "not-a-date"})
	if !result.IsError {
		t.Fatalf("expected parse error, got %q", result.Content)
	}
}

func TestDateFormatReformatsBetweenLayouts(t *testing.T) {
	result := executeDatetimeOpsTool(t, "date_format", map[string]any{
		"date":          "2024-06-11T08:30:00Z",
		"input_format":  "RFC3339",
		"output_format": "DateOnly",
	})
	if result.Content != "2024-06-11" {
		t.Fatalf("date_format = %q, want 2024-06-11", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_format", map[string]any{"date": "junk"})
	if !result.IsError {
		t.Fatalf("expected parse error, got %q", result.Content)
	}
}

func TestDateAddAndDiff(t *testing.T) {
	result := executeDatetimeOpsTool(t, "date_add", map[string]any{
		"date":     "2024-06-11T00:00:00Z",
		"duration": "2h30m",
	})
	if result.Content != "2024-06-11T02:30:00Z" {
		t.Fatalf("date_add = %q, want 2024-06-11T02:30:00Z", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_add", map[string]any{"date": "2024-06-11T00:00:00Z", "duration": "nope"})
	if !result.IsError {
		t.Fatalf("expected invalid duration error, got %q", result.Content)
	}

	result = executeDatetimeOpsTool(t, "date_diff", map[string]any{
		"date1": "2024-06-11T00:00:00Z",
		"date2": "2024-06-12T12:00:00Z",
		"unit":  "hours",
	})
	if result.Content != "36 hours" {
		t.Fatalf("date_diff = %q, want 36 hours", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_diff", map[string]any{"unit": "fortnights"})
	if !result.IsError {
		t.Fatalf("expected unsupported unit error, got %q", result.Content)
	}
}

func TestTimezoneConvert(t *testing.T) {
	result := executeDatetimeOpsTool(t, "timezone_convert", map[string]any{
		"date":    "2024-06-11T00:00:00Z",
		"from_tz": "UTC",
		"to_tz":   "UTC",
	})
	if result.Content != "2024-06-11T00:00:00Z" {
		t.Fatalf("timezone_convert UTC->UTC = %q", result.Content)
	}
	result = executeDatetimeOpsTool(t, "timezone_convert", map[string]any{"to_tz": "Not/AZone"})
	if !result.IsError {
		t.Fatalf("expected unknown timezone error, got %q", result.Content)
	}
}

func TestDurationParseAndFormat(t *testing.T) {
	result := executeDatetimeOpsTool(t, "duration_parse", map[string]any{"duration": "1h30m"})
	if result.Content != "5400 seconds" {
		t.Fatalf("duration_parse(1h30m) = %q, want 5400 seconds", result.Content)
	}
	result = executeDatetimeOpsTool(t, "duration_parse", map[string]any{"duration": "bogus"})
	if !result.IsError {
		t.Fatalf("expected invalid duration error, got %q", result.Content)
	}
	result = executeDatetimeOpsTool(t, "duration_format", map[string]any{"seconds": 5400.0})
	if result.Content != "1h30m0s" {
		t.Fatalf("duration_format(5400) = %q, want 1h30m0s", result.Content)
	}
}

func TestDateWeekdayWeekendStartOfDay(t *testing.T) {
	result := executeDatetimeOpsTool(t, "date_weekday", map[string]any{"date": "2024-06-11T10:00:00Z"})
	if result.Content != "Tuesday" {
		t.Fatalf("date_weekday = %q, want Tuesday", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_is_weekend", map[string]any{"date": "2024-06-15T10:00:00Z"})
	if result.Content != "true" {
		t.Fatalf("date_is_weekend(Saturday) = %q, want true", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_is_weekend", map[string]any{"date": "2024-06-11T10:00:00Z"})
	if result.Content != "false" {
		t.Fatalf("date_is_weekend(Tuesday) = %q, want false", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_start_of_day", map[string]any{"date": "2024-06-11T15:45:12Z"})
	if result.Content != "2024-06-11T00:00:00Z" {
		t.Fatalf("date_start_of_day = %q, want 2024-06-11T00:00:00Z", result.Content)
	}
	result = executeDatetimeOpsTool(t, "date_weekday", map[string]any{"date": "garbage"})
	if !result.IsError {
		t.Fatalf("expected parse error, got %q", result.Content)
	}
}

func TestCronNext(t *testing.T) {
	cases := []struct {
		expr string
		from string
		want string
	}{
		{"* * * * *", "2024-06-11T10:00:30Z", "2024-06-11T10:01:00Z"},
		{"0 9 * * *", "2024-06-11T10:00:00Z", "2024-06-12T09:00:00Z"},
		{"30 14 1 * *", "2024-06-11T10:00:00Z", "2024-07-01T14:30:00Z"},
		{"*/15 * * * *", "2024-06-11T10:05:00Z", "2024-06-11T10:15:00Z"},
		{"0 0 * * 0", "2024-06-11T10:00:00Z", "2024-06-16T00:00:00Z"},
	}
	for _, tc := range cases {
		result := executeDatetimeOpsTool(t, "cron_next", map[string]any{"cron_expr": tc.expr, "from_date": tc.from})
		if result.IsError {
			t.Fatalf("cron_next(%q) returned tool error: %s", tc.expr, result.Content)
		}
		if result.Content != tc.want {
			t.Fatalf("cron_next(%q from %s) = %q, want %q", tc.expr, tc.from, result.Content, tc.want)
		}
	}

	result := executeDatetimeOpsTool(t, "cron_next", map[string]any{"cron_expr": "61 * * * *"})
	if !result.IsError {
		t.Fatalf("expected out-of-range cron error, got %q", result.Content)
	}
	result = executeDatetimeOpsTool(t, "cron_next", map[string]any{"cron_expr": "* * *"})
	if !result.IsError {
		t.Fatalf("expected field-count cron error, got %q", result.Content)
	}
}

func TestCronDescribe(t *testing.T) {
	result := executeDatetimeOpsTool(t, "cron_describe", map[string]any{"cron_expr": "* * * * *"})
	if result.Content != "Every minute" {
		t.Fatalf("cron_describe(* * * * *) = %q, want Every minute", result.Content)
	}
	result = executeDatetimeOpsTool(t, "cron_describe", map[string]any{"cron_expr": "0 9 * * 1-5"})
	want := "At minute 0 past hour 9 on Monday through Friday"
	if result.Content != want {
		t.Fatalf("cron_describe(0 9 * * 1-5) = %q, want %q", result.Content, want)
	}
	result = executeDatetimeOpsTool(t, "cron_describe", map[string]any{"cron_expr": "bad"})
	if !result.IsError {
		t.Fatalf("expected invalid cron error, got %q", result.Content)
	}
}

func TestDatetimeOpsToolsSucceedWithEmptyArgs(t *testing.T) {
	for _, name := range datetimeOpsToolNames {
		result := executeDatetimeOpsTool(t, name, map[string]any{})
		if result.IsError {
			t.Fatalf("%s with empty args returned tool error: %s", name, result.Content)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s with empty args returned placeholder output: %q", name, result.Content)
		}
	}
}
