package trigger

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCronExpression = errors.New("invalid cron expression")
	ErrScheduleDisabled      = errors.New("schedule trigger is disabled")
)

// ScheduleTrigger manages cron-based workflow triggering.
type ScheduleTrigger struct {
	ID             string
	Name           string
	CronExpression string
	Timezone       string
	Enabled        bool
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	Definition     map[string]any
}

// CronField represents a parsed cron field (minute, hour, day-of-month, month, day-of-week).
type CronField struct {
	Values []int
	IsWild bool
}

// ParsedCron holds a fully parsed 5-field cron expression.
type ParsedCron struct {
	Minute     CronField
	Hour       CronField
	DayOfMonth CronField
	Month      CronField
	DayOfWeek  CronField
	Raw        string
}

// NewScheduleTrigger creates a new schedule trigger.
func NewScheduleTrigger(id, name, cronExpr string, timezone string, enabled bool) (*ScheduleTrigger, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("schedule trigger ID is required")
	}
	if _, err := ParseCronExpression(cronExpr); err != nil {
		return nil, err
	}
	tz := strings.TrimSpace(timezone)
	if tz == "" {
		tz = "UTC"
	}
	return &ScheduleTrigger{
		ID:             id,
		Name:           strings.TrimSpace(name),
		CronExpression: strings.TrimSpace(cronExpr),
		Timezone:       tz,
		Enabled:        enabled,
	}, nil
}

// ParseCronExpression parses a standard 5-field cron expression: minute hour day-of-month month day-of-week.
func ParseCronExpression(expr string) (*ParsedCron, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("%w: empty expression", ErrInvalidCronExpression)
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("%w: expected 5 fields, got %d", ErrInvalidCronExpression, len(fields))
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("%w: minute field: %v", ErrInvalidCronExpression, err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("%w: hour field: %v", ErrInvalidCronExpression, err)
	}
	dayOfMonth, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("%w: day-of-month field: %v", ErrInvalidCronExpression, err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("%w: month field: %v", ErrInvalidCronExpression, err)
	}
	dayOfWeek, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("%w: day-of-week field: %v", ErrInvalidCronExpression, err)
	}

	return &ParsedCron{
		Minute:     minute,
		Hour:       hour,
		DayOfMonth: dayOfMonth,
		Month:      month,
		DayOfWeek:  dayOfWeek,
		Raw:        expr,
	}, nil
}

// NextRun returns the next execution time after "from".
func (pc *ParsedCron) NextRun(from time.Time) time.Time {
	if pc == nil {
		return time.Time{}
	}
	loc := from.Location()

	// Start from the next minute after "from".
	t := time.Date(
		from.Year(), from.Month(), from.Day(),
		from.Hour(), from.Minute(), 0, 0, loc,
	).Add(time.Minute)

	// Guard against infinite loops: search up to 2 years out.
	maxTime := from.Add(2 * 365 * 24 * time.Hour)

	for t.Before(maxTime) {
		if !matchesCronField(pc.Month, int(t.Month())) {
			t = nextMonth(t)
			continue
		}
		if !matchesCronField(pc.DayOfMonth, t.Day()) {
			t = nextDay(t)
			continue
		}
		if !matchesCronField(pc.DayOfWeek, int(t.Weekday())) {
			t = nextDay(t)
			continue
		}
		if !matchesCronField(pc.Hour, t.Hour()) {
			t = nextHour(t)
			continue
		}
		if !matchesCronField(pc.Minute, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// CalculateNextRun computes the next run time for a schedule trigger.
func (st *ScheduleTrigger) CalculateNextRun(from time.Time) (time.Time, error) {
	if st == nil {
		return time.Time{}, fmt.Errorf("schedule trigger is nil")
	}
	if !st.Enabled {
		return time.Time{}, ErrScheduleDisabled
	}

	parsed, err := ParseCronExpression(st.CronExpression)
	if err != nil {
		return time.Time{}, err
	}

	loc := time.UTC
	if tz := strings.TrimSpace(st.Timezone); tz != "" {
		if parsedLoc, err := time.LoadLocation(tz); err == nil {
			loc = parsedLoc
		}
	}

	localFrom := from.In(loc)
	nextRun := parsed.NextRun(localFrom)
	if nextRun.IsZero() {
		return time.Time{}, fmt.Errorf("%w: no next run found within 2 years", ErrInvalidCronExpression)
	}

	return nextRun.UTC(), nil
}

// ShouldRun returns true if the trigger should execute at the given time.
func (st *ScheduleTrigger) ShouldRun(now time.Time) bool {
	if st == nil || !st.Enabled {
		return false
	}
	if st.NextRunAt == nil {
		return true
	}
	return !now.Before(*st.NextRunAt)
}

// MarkExecuted updates LastRunAt and recalculates NextRunAt.
func (st *ScheduleTrigger) MarkExecuted(executedAt time.Time) error {
	if st == nil {
		return fmt.Errorf("schedule trigger is nil")
	}
	st.LastRunAt = &executedAt
	nextRun, err := st.CalculateNextRun(executedAt)
	if err != nil {
		if errors.Is(err, ErrScheduleDisabled) {
			st.NextRunAt = nil
			return nil
		}
		return err
	}
	st.NextRunAt = &nextRun
	return nil
}

// ScheduleDescription returns a human-readable description of the schedule.
func (st *ScheduleTrigger) ScheduleDescription() string {
	if st == nil {
		return ""
	}
	return cronExpressionDescription(st.CronExpression)
}

func cronExpressionDescription(expr string) string {
	parsed, err := ParseCronExpression(expr)
	if err != nil {
		return expr
	}
	if parsed.Minute.IsWild && parsed.Hour.IsWild {
		return "every minute"
	}
	if parsed.Hour.IsWild {
		return fmt.Sprintf("every hour at minute %d", firstValue(parsed.Minute.Values))
	}
	if parsed.DayOfMonth.IsWild && parsed.Month.IsWild && parsed.DayOfWeek.IsWild {
		return fmt.Sprintf("daily at %02d:%02d", firstValue(parsed.Hour.Values), firstValue(parsed.Minute.Values))
	}
	return expr
}

func firstValue(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func parseCronField(field string, min, max int) (CronField, error) {
	if field == "*" {
		values := make([]int, max-min+1)
		for i := range values {
			values[i] = min + i
		}
		return CronField{Values: values, IsWild: true}, nil
	}

	// Handle */step wildcard.
	if strings.HasPrefix(field, "*/") {
		stepStr := strings.TrimPrefix(field, "*/")
		step, err := parseCronInt(stepStr)
		if err != nil || step <= 0 {
			return CronField{}, fmt.Errorf("invalid step value: %s", stepStr)
		}
		values := []int{}
		for v := min; v <= max; v += step {
			values = append(values, v)
		}
		return CronField{Values: values}, nil
	}

	// Handle comma-separated values and ranges.
	parts := strings.Split(field, ",")
	seen := map[int]bool{}
	allValues := []int{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err := parseCronInt(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return CronField{}, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}
			end, err := parseCronInt(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return CronField{}, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}
			if start < min || end > max || start > end {
				return CronField{}, fmt.Errorf("range %d-%d out of bounds [%d,%d]", start, end, min, max)
			}
			for v := start; v <= end; v++ {
				if !seen[v] {
					allValues = append(allValues, v)
					seen[v] = true
				}
			}
		} else {
			val, err := parseCronInt(part)
			if err != nil {
				return CronField{}, fmt.Errorf("invalid value: %s", part)
			}
			if val < min || val > max {
				return CronField{}, fmt.Errorf("value %d out of bounds [%d,%d]", val, min, max)
			}
			if !seen[val] {
				allValues = append(allValues, val)
				seen[val] = true
			}
		}
	}

	if len(allValues) == 0 {
		return CronField{}, fmt.Errorf("empty field")
	}
	return CronField{Values: allValues}, nil
}

func parseCronInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	val := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		val = val*10 + int(c-'0')
	}
	return val, nil
}

func matchesCronField(field CronField, value int) bool {
	for _, v := range field.Values {
		if v == value {
			return true
		}
	}
	return false
}

func nextMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, t.Location())
}

func nextDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}

func nextHour(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, t.Hour()+1, 0, 0, 0, t.Location())
}
