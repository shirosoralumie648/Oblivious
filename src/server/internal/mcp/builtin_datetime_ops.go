package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"timestamp_now":     &TimestampNowTool{},
		"timestamp_to_date": &TimestampToDateTool{},
		"date_to_timestamp": &DateToTimestampTool{},
		"date_format":       &DateFormatTool{},
		"date_add":          &DateAddTool{},
		"date_diff":         &DateDiffTool{},
		"timezone_convert":  &TimezoneConvertTool{},
		"duration_parse":    &DurationParseTool{},
		"duration_format":   &DurationFormatTool{},
		"date_weekday":      &DateWeekdayTool{},
		"date_is_weekend":   &DateIsWeekendTool{},
		"date_start_of_day": &DateStartOfDayTool{},
		"cron_next":         &CronNextTool{},
		"cron_describe":     &CronDescribeTool{},
	}, map[string]bool{
		"timestamp_now":     true,
		"timestamp_to_date": true,
		"date_to_timestamp": true,
		"date_format":       true,
		"date_add":          true,
		"date_diff":         true,
		"timezone_convert":  true,
		"duration_parse":    true,
		"duration_format":   true,
		"date_weekday":      true,
		"date_is_weekend":   true,
		"date_start_of_day": true,
		"cron_next":         true,
		"cron_describe":     true,
	})
}

const datetimeOpsEpoch = "1970-01-01T00:00:00Z"

func datetimeOpsString(args map[string]any, key, fallback string) string {
	if args == nil {
		return fallback
	}
	if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

var datetimeOpsNamedLayouts = map[string]string{
	"rfc3339":  time.RFC3339,
	"rfc1123":  time.RFC1123,
	"rfc822":   time.RFC822,
	"kitchen":  time.Kitchen,
	"dateonly": "2006-01-02",
	"datetime": "2006-01-02 15:04:05",
	"timeonly": "15:04:05",
	"unixdate": time.UnixDate,
}

func datetimeOpsLayout(name string) string {
	if layout, ok := datetimeOpsNamedLayouts[strings.ToLower(strings.TrimSpace(name))]; ok {
		return layout
	}
	if strings.TrimSpace(name) == "" {
		return time.RFC3339
	}
	return name
}

func datetimeOpsLocation(name string) (*time.Location, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.EqualFold(trimmed, "utc") {
		return time.UTC, nil
	}
	if strings.EqualFold(trimmed, "local") {
		return time.Local, nil
	}
	location, err := time.LoadLocation(trimmed)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q", name)
	}
	return location, nil
}

func datetimeOpsParse(date, format, timezone string) (time.Time, error) {
	location, err := datetimeOpsLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	layout := datetimeOpsLayout(format)
	parsed, err := time.ParseInLocation(layout, date, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse date %q with format %q", date, layout)
	}
	return parsed, nil
}

// TimestampNowTool returns the current Unix timestamp.
type TimestampNowTool struct{}

func (t *TimestampNowTool) Name() string        { return "timestamp_now" }
func (t *TimestampNowTool) Description() string { return "Get the current Unix timestamp in seconds" }
func (t *TimestampNowTool) InputSchema() any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *TimestampNowTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	_ = args
	return &ToolResult{Content: strconv.FormatInt(time.Now().Unix(), 10)}, nil
}

// TimestampToDateTool converts a Unix timestamp to a formatted date string.
type TimestampToDateTool struct{}

func (t *TimestampToDateTool) Name() string { return "timestamp_to_date" }
func (t *TimestampToDateTool) Description() string {
	return "Convert a Unix timestamp to a date string"
}
func (t *TimestampToDateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timestamp": map[string]any{"type": "integer", "description": "Unix timestamp in seconds", "default": 0},
			"format":    map[string]any{"type": "string", "description": "Output layout (RFC3339, RFC1123, DateOnly, DateTime or Go layout)", "default": "RFC3339"},
			"timezone":  map[string]any{"type": "string", "description": "IANA timezone name", "default": "UTC"},
		},
	}
}
func (t *TimestampToDateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	timestamp := int64(getFloat(args, "timestamp", 0))
	location, err := datetimeOpsLocation(datetimeOpsString(args, "timezone", "UTC"))
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	layout := datetimeOpsLayout(datetimeOpsString(args, "format", "RFC3339"))
	return &ToolResult{Content: time.Unix(timestamp, 0).In(location).Format(layout)}, nil
}

// DateToTimestampTool converts a date string to a Unix timestamp.
type DateToTimestampTool struct{}

func (t *DateToTimestampTool) Name() string { return "date_to_timestamp" }
func (t *DateToTimestampTool) Description() string {
	return "Convert a date string to a Unix timestamp"
}
func (t *DateToTimestampTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":     map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"format":   map[string]any{"type": "string", "description": "Input layout", "default": "RFC3339"},
			"timezone": map[string]any{"type": "string", "description": "IANA timezone name", "default": "UTC"},
		},
	}
}
func (t *DateToTimestampTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		datetimeOpsString(args, "format", "RFC3339"),
		datetimeOpsString(args, "timezone", "UTC"),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: strconv.FormatInt(parsed.Unix(), 10)}, nil
}

// DateFormatTool reformats a date string between layouts.
type DateFormatTool struct{}

func (t *DateFormatTool) Name() string        { return "date_format" }
func (t *DateFormatTool) Description() string { return "Reformat a date string into another layout" }
func (t *DateFormatTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":          map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"input_format":  map[string]any{"type": "string", "description": "Input layout", "default": "RFC3339"},
			"output_format": map[string]any{"type": "string", "description": "Output layout", "default": "DateTime"},
			"timezone":      map[string]any{"type": "string", "description": "IANA timezone name", "default": "UTC"},
		},
	}
}
func (t *DateFormatTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		datetimeOpsString(args, "input_format", "RFC3339"),
		datetimeOpsString(args, "timezone", "UTC"),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	layout := datetimeOpsLayout(datetimeOpsString(args, "output_format", "DateTime"))
	return &ToolResult{Content: parsed.Format(layout)}, nil
}

// DateAddTool adds a Go duration to a date.
type DateAddTool struct{}

func (t *DateAddTool) Name() string        { return "date_add" }
func (t *DateAddTool) Description() string { return "Add a duration (e.g. 2h30m, -24h) to a date" }
func (t *DateAddTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":     map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"duration": map[string]any{"type": "string", "description": "Go duration string", "default": "0s"},
			"format":   map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
		},
	}
}
func (t *DateAddTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	format := datetimeOpsString(args, "format", "RFC3339")
	parsed, err := datetimeOpsParse(datetimeOpsString(args, "date", datetimeOpsEpoch), format, "UTC")
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	duration, err := time.ParseDuration(datetimeOpsString(args, "duration", "0s"))
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid duration %q", datetimeOpsString(args, "duration", "0s")), IsError: true}, nil
	}
	return &ToolResult{Content: parsed.Add(duration).Format(datetimeOpsLayout(format))}, nil
}

// DateDiffTool computes the difference between two dates.
type DateDiffTool struct{}

func (t *DateDiffTool) Name() string        { return "date_diff" }
func (t *DateDiffTool) Description() string { return "Difference between two dates in a chosen unit" }
func (t *DateDiffTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date1":  map[string]any{"type": "string", "description": "First date", "default": datetimeOpsEpoch},
			"date2":  map[string]any{"type": "string", "description": "Second date", "default": datetimeOpsEpoch},
			"format": map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
			"unit":   map[string]any{"type": "string", "description": "Result unit: seconds, minutes, hours or days", "default": "seconds"},
		},
	}
}
func (t *DateDiffTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	format := datetimeOpsString(args, "format", "RFC3339")
	date1, err := datetimeOpsParse(datetimeOpsString(args, "date1", datetimeOpsEpoch), format, "UTC")
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	date2, err := datetimeOpsParse(datetimeOpsString(args, "date2", datetimeOpsEpoch), format, "UTC")
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	diff := date2.Sub(date1)
	unit := strings.ToLower(datetimeOpsString(args, "unit", "seconds"))
	var value float64
	switch unit {
	case "seconds":
		value = diff.Seconds()
	case "minutes":
		value = diff.Minutes()
	case "hours":
		value = diff.Hours()
	case "days":
		value = diff.Hours() / 24
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported unit %q; supported units: seconds, minutes, hours, days", unit), IsError: true}, nil
	}
	return &ToolResult{Content: fmt.Sprintf("%g %s", value, unit)}, nil
}

// TimezoneConvertTool converts a date between timezones.
type TimezoneConvertTool struct{}

func (t *TimezoneConvertTool) Name() string        { return "timezone_convert" }
func (t *TimezoneConvertTool) Description() string { return "Convert a date between timezones" }
func (t *TimezoneConvertTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":    map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"from_tz": map[string]any{"type": "string", "description": "Source IANA timezone", "default": "UTC"},
			"to_tz":   map[string]any{"type": "string", "description": "Target IANA timezone", "default": "UTC"},
			"format":  map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
		},
	}
}
func (t *TimezoneConvertTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	format := datetimeOpsString(args, "format", "RFC3339")
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		format,
		datetimeOpsString(args, "from_tz", "UTC"),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	target, err := datetimeOpsLocation(datetimeOpsString(args, "to_tz", "UTC"))
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: parsed.In(target).Format(datetimeOpsLayout(format))}, nil
}

// DurationParseTool parses a Go duration string into seconds.
type DurationParseTool struct{}

func (t *DurationParseTool) Name() string { return "duration_parse" }
func (t *DurationParseTool) Description() string {
	return "Parse a duration string (e.g. 1h30m) into seconds"
}
func (t *DurationParseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"duration": map[string]any{"type": "string", "description": "Go duration string", "default": "0s"},
		},
	}
}
func (t *DurationParseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	raw := datetimeOpsString(args, "duration", "0s")
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid duration %q", raw), IsError: true}, nil
	}
	return &ToolResult{Content: fmt.Sprintf("%g seconds", duration.Seconds())}, nil
}

// DurationFormatTool formats seconds as a Go duration string.
type DurationFormatTool struct{}

func (t *DurationFormatTool) Name() string { return "duration_format" }
func (t *DurationFormatTool) Description() string {
	return "Format a number of seconds as a duration string"
}
func (t *DurationFormatTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"seconds": map[string]any{"type": "number", "description": "Seconds to format", "default": 0},
		},
	}
}
func (t *DurationFormatTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	seconds := getFloat(args, "seconds", 0)
	duration := time.Duration(seconds * float64(time.Second))
	return &ToolResult{Content: duration.String()}, nil
}

// DateWeekdayTool returns the weekday name of a date.
type DateWeekdayTool struct{}

func (t *DateWeekdayTool) Name() string        { return "date_weekday" }
func (t *DateWeekdayTool) Description() string { return "Get the weekday name for a date" }
func (t *DateWeekdayTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":   map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"format": map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
		},
	}
}
func (t *DateWeekdayTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		datetimeOpsString(args, "format", "RFC3339"),
		"UTC",
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: parsed.Weekday().String()}, nil
}

// DateIsWeekendTool reports whether a date falls on a weekend.
type DateIsWeekendTool struct{}

func (t *DateIsWeekendTool) Name() string { return "date_is_weekend" }
func (t *DateIsWeekendTool) Description() string {
	return "Check whether a date falls on Saturday or Sunday"
}
func (t *DateIsWeekendTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":   map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"format": map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
		},
	}
}
func (t *DateIsWeekendTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		datetimeOpsString(args, "format", "RFC3339"),
		"UTC",
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	weekday := parsed.Weekday()
	return &ToolResult{Content: strconv.FormatBool(weekday == time.Saturday || weekday == time.Sunday)}, nil
}

// DateStartOfDayTool truncates a date to 00:00:00 in the chosen timezone.
type DateStartOfDayTool struct{}

func (t *DateStartOfDayTool) Name() string { return "date_start_of_day" }
func (t *DateStartOfDayTool) Description() string {
	return "Get the start of day (00:00:00) for a date"
}
func (t *DateStartOfDayTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":     map[string]any{"type": "string", "description": "Date string", "default": datetimeOpsEpoch},
			"format":   map[string]any{"type": "string", "description": "Date layout", "default": "RFC3339"},
			"timezone": map[string]any{"type": "string", "description": "IANA timezone name", "default": "UTC"},
		},
	}
}
func (t *DateStartOfDayTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	format := datetimeOpsString(args, "format", "RFC3339")
	parsed, err := datetimeOpsParse(
		datetimeOpsString(args, "date", datetimeOpsEpoch),
		format,
		datetimeOpsString(args, "timezone", "UTC"),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	year, month, day := parsed.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, parsed.Location())
	return &ToolResult{Content: start.Format(datetimeOpsLayout(format))}, nil
}

// cronField is a parsed set of allowed values for one cron field.
type cronField map[int]bool

func parseCronField(field string, min, max int) (cronField, error) {
	values := cronField{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty cron field segment")
		}
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			parsedStep, err := strconv.Atoi(part[idx+1:])
			if err != nil || parsedStep < 1 {
				return nil, fmt.Errorf("invalid cron step %q", part)
			}
			step = parsedStep
			part = part[:idx]
		}
		start, end := min, max
		switch {
		case part == "*":
			// full range
		case strings.Contains(part, "-"):
			bounds := strings.SplitN(part, "-", 2)
			parsedStart, err1 := strconv.Atoi(bounds[0])
			parsedEnd, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || parsedStart > parsedEnd {
				return nil, fmt.Errorf("invalid cron range %q", part)
			}
			start, end = parsedStart, parsedEnd
		default:
			value, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid cron value %q", part)
			}
			start, end = value, value
		}
		if start < min || end > max {
			return nil, fmt.Errorf("cron value out of range %d-%d: %q", min, max, part)
		}
		for v := start; v <= end; v += step {
			values[v] = true
		}
	}
	return values, nil
}

type cronSchedule struct {
	minute, hour, dom, month, dow cronField
	domStar, dowStar              bool
}

func parseCronExpr(expr string) (*cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, err
	}
	dom, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, err
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, err
	}
	dow, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return nil, err
	}
	if dow[7] {
		dow[0] = true
	}
	return &cronSchedule{
		minute: minute, hour: hour, dom: dom, month: month, dow: dow,
		domStar: fields[2] == "*", dowStar: fields[4] == "*",
	}, nil
}

func (s *cronSchedule) matches(t time.Time) bool {
	if !s.minute[t.Minute()] || !s.hour[t.Hour()] || !s.month[int(t.Month())] {
		return false
	}
	domMatch := s.dom[t.Day()]
	dowMatch := s.dow[int(t.Weekday())]
	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return dowMatch
	case s.dowStar:
		return domMatch
	default:
		// Standard cron semantics: either field may match when both restricted.
		return domMatch || dowMatch
	}
}

func (s *cronSchedule) next(from time.Time) (time.Time, bool) {
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	limit := from.Add(4 * 366 * 24 * time.Hour)
	for candidate.Before(limit) {
		if s.matches(candidate) {
			return candidate, true
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, false
}

// CronNextTool calculates the next execution time of a cron expression.
type CronNextTool struct{}

func (t *CronNextTool) Name() string { return "cron_next" }
func (t *CronNextTool) Description() string {
	return "Calculate the next execution time for a 5-field cron expression"
}
func (t *CronNextTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cron_expr": map[string]any{"type": "string", "description": "5-field cron expression (minute hour dom month dow)", "default": "* * * * *"},
			"from_date": map[string]any{"type": "string", "description": "RFC3339 date to search from (defaults to now)"},
		},
	}
}
func (t *CronNextTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	schedule, err := parseCronExpr(datetimeOpsString(args, "cron_expr", "* * * * *"))
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	from := time.Now().UTC()
	if raw := datetimeOpsString(args, "from_date", ""); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return &ToolResult{Content: fmt.Sprintf("cannot parse from_date %q as RFC3339", raw), IsError: true}, nil
		}
		from = parsed.UTC()
	}
	next, ok := schedule.next(from)
	if !ok {
		return &ToolResult{Content: "no execution time found within 4 years", IsError: true}, nil
	}
	return &ToolResult{Content: next.Format(time.RFC3339)}, nil
}

func describeCronField(field string, unit string, formatter func(int) string) string {
	if field == "*" {
		return ""
	}
	parts := strings.Split(field, ",")
	described := make([]string, 0, len(parts))
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "*/"):
			described = append(described, fmt.Sprintf("every %s %s", strings.TrimPrefix(part, "*/"), pluralizeCronUnit(unit)))
		case strings.Contains(part, "-") && strings.Contains(part, "/"):
			described = append(described, part)
		case strings.Contains(part, "-"):
			bounds := strings.SplitN(part, "-", 2)
			startValue, err1 := strconv.Atoi(bounds[0])
			endValue, err2 := strconv.Atoi(bounds[1])
			if err1 == nil && err2 == nil {
				described = append(described, fmt.Sprintf("%s through %s", formatter(startValue), formatter(endValue)))
			} else {
				described = append(described, part)
			}
		default:
			if value, err := strconv.Atoi(part); err == nil {
				described = append(described, formatter(value))
			} else {
				described = append(described, part)
			}
		}
	}
	return strings.Join(described, ", ")
}

func pluralizeCronUnit(unit string) string {
	if strings.HasSuffix(unit, "s") {
		return unit
	}
	return unit + "s"
}

// CronDescribeTool renders a cron expression as English text.
type CronDescribeTool struct{}

func (t *CronDescribeTool) Name() string { return "cron_describe" }
func (t *CronDescribeTool) Description() string {
	return "Describe a 5-field cron expression in plain English"
}
func (t *CronDescribeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cron_expr": map[string]any{"type": "string", "description": "5-field cron expression", "default": "* * * * *"},
		},
	}
}
func (t *CronDescribeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	expr := datetimeOpsString(args, "cron_expr", "* * * * *")
	if _, err := parseCronExpr(expr); err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	fields := strings.Fields(expr)
	monthNames := []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

	segments := []string{}
	if minute := describeCronField(fields[0], "minute", func(v int) string { return fmt.Sprintf("minute %d", v) }); minute != "" {
		segments = append(segments, "at "+minute)
	} else {
		segments = append(segments, "every minute")
	}
	if hour := describeCronField(fields[1], "hour", func(v int) string { return fmt.Sprintf("hour %d", v) }); hour != "" {
		segments = append(segments, "past "+hour)
	}
	if dom := describeCronField(fields[2], "day", func(v int) string { return fmt.Sprintf("day-of-month %d", v) }); dom != "" {
		segments = append(segments, "on "+dom)
	}
	if month := describeCronField(fields[3], "month", func(v int) string {
		if v >= 1 && v <= 12 {
			return monthNames[v]
		}
		return strconv.Itoa(v)
	}); month != "" {
		segments = append(segments, "in "+month)
	}
	if dow := describeCronField(fields[4], "weekday", func(v int) string {
		if v >= 0 && v <= 7 {
			return dayNames[v]
		}
		return strconv.Itoa(v)
	}); dow != "" {
		segments = append(segments, "on "+dow)
	}
	description := strings.Join(segments, " ")
	description = strings.ToUpper(description[:1]) + description[1:]
	return &ToolResult{Content: description}, nil
}
