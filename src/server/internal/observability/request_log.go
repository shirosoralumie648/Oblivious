package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const nilUUID = "00000000-0000-0000-0000-000000000000"

type RequestLogRow struct {
	ID             string
	RequestID      string
	Timestamp      time.Time
	OrganizationID string
	UserID         string
	Service        string
	Endpoint       string
	Method         string
	StatusCode     uint16
	DurationMS     uint32
	RequestTokens  uint32
	ResponseTokens uint32
	Model          string
	CostUSD        float64
	Error          string
	TraceID        string
	Metadata       string
}

type RequestLogSink interface {
	InsertRequestLog(ctx context.Context, row RequestLogRow) error
}

type requestLogExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type NoopRequestLogSink struct{}

func (NoopRequestLogSink) InsertRequestLog(context.Context, RequestLogRow) error {
	return nil
}

type SQLRequestLogSink struct {
	db requestLogExecutor
}

func NewSQLRequestLogSink(db requestLogExecutor) *SQLRequestLogSink {
	return &SQLRequestLogSink{db: db}
}

func (s *SQLRequestLogSink) InsertRequestLog(ctx context.Context, row RequestLogRow) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO request_logs (
			id, request_id, timestamp, organization_id, user_id, service, endpoint, method,
			status_code, duration_ms, request_tokens, response_tokens, model,
			cost_usd, error, trace_id, metadata
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, row.ID, row.RequestID, row.Timestamp, row.OrganizationID, row.UserID, row.Service, row.Endpoint, row.Method,
		row.StatusCode, row.DurationMS, row.RequestTokens, row.ResponseTokens, row.Model,
		row.CostUSD, row.Error, row.TraceID, row.Metadata)
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}
	return nil
}

func WriteRequestLog(ctx context.Context, sink RequestLogSink, event Event, timestamp time.Time) error {
	if sink == nil {
		return nil
	}
	row, err := RequestLogRowFromEvent(event, timestamp)
	if err != nil {
		return err
	}
	return sink.InsertRequestLog(ctx, row)
}

func RequestLogRowFromEvent(event Event, timestamp time.Time) (RequestLogRow, error) {
	fields := event.Fields
	metadata, err := requestLogMetadata(event, sanitizeFields(fields))
	if err != nil {
		return RequestLogRow{}, err
	}

	row := RequestLogRow{
		ID:             normalizedUUID(event.RequestID),
		RequestID:      strings.TrimSpace(event.RequestID),
		Timestamp:      timestamp,
		OrganizationID: normalizedUUID(event.OrganizationID),
		UserID:         normalizedUUID(event.UserID),
		Service:        firstNonEmpty(event.Component, "unknown"),
		Endpoint:       event.Route,
		Method:         strings.ToUpper(strings.TrimSpace(event.Method)),
		StatusCode:     uint16(clampUint(event.Status, math.MaxUint16)),
		DurationMS:     uint32(clampUint(durationMillis(event.Latency), math.MaxUint32)),
		RequestTokens:  uint32(clampUint(firstInt(fields, "request_tokens", "input_tokens"), math.MaxUint32)),
		ResponseTokens: uint32(clampUint(firstInt(fields, "response_tokens", "output_tokens"), math.MaxUint32)),
		Model:          firstString(fields, "model"),
		CostUSD:        firstFloat(fields, "cost_usd", "cost"),
		Error:          firstNonEmpty(event.FailureReason, firstString(fields, "error"), firstString(fields, "error_code")),
		TraceID:        normalizedUUID(event.TraceID),
		Metadata:       metadata,
	}
	return row, nil
}

func requestLogMetadata(event Event, fields map[string]any) (string, error) {
	metadata := make(map[string]any, len(fields)+12)
	add := func(key string, value any) {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				metadata[key] = typed
			}
		case int:
			if typed != 0 {
				metadata[key] = typed
			}
		default:
			if value != nil {
				metadata[key] = value
			}
		}
	}

	add("event", event.Event)
	add("request_id", event.RequestID)
	add("trace_id", event.TraceID)
	add("span_id", event.SpanID)
	add("organization_id", event.OrganizationID)
	add("user_id", event.UserID)
	add("relay_route_class", event.RelayRouteClass)
	add("relay_api_type", event.RelayAPIType)
	add("billing_policy", event.BillingPolicy)
	add("billing_session_id", event.BillingSessionID)
	add("channel_id", event.ChannelID)
	add("provider", event.Provider)
	add("failure_reason", event.FailureReason)

	for key, value := range fields {
		if isSensitiveField(key) {
			continue
		}
		metadata[key] = sanitizeMetadataValue(value)
	}

	raw, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal request log metadata: %w", err)
	}
	return string(raw), nil
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, nested := range typed {
			if isSensitiveField(key) {
				continue
			}
			sanitized[key] = sanitizeMetadataValue(nested)
		}
		return sanitized
	case map[string]string:
		sanitized := make(map[string]string, len(typed))
		for key, nested := range typed {
			if isSensitiveField(key) {
				continue
			}
			sanitized[key] = nested
		}
		return sanitized
	case []any:
		sanitized := make([]any, 0, len(typed))
		for _, nested := range typed {
			sanitized = append(sanitized, sanitizeMetadataValue(nested))
		}
		return sanitized
	default:
		return value
	}
}

func normalizedUUID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nilUUID
	}
	if parsed, err := uuid.Parse(trimmed); err == nil {
		return parsed.String()
	}
	return nilUUID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		case fmt.Stringer:
			return strings.TrimSpace(typed.String())
		}
	}
	return ""
}

func firstInt(fields map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		maxInt := maxIntValue()
		minInt := minIntValue()
		switch typed := value.(type) {
		case int:
			return typed
		case int8:
			return int(typed)
		case int16:
			return int(typed)
		case int32:
			return int(typed)
		case int64:
			if typed > int64(maxInt) {
				return maxInt
			}
			if typed < int64(minInt) {
				return minInt
			}
			return int(typed)
		case uint:
			if uint64(typed) > uint64(maxInt) {
				return maxInt
			}
			return int(typed)
		case uint8:
			return int(typed)
		case uint16:
			return int(typed)
		case uint32:
			if uint64(typed) > uint64(maxInt) {
				return maxInt
			}
			return int(typed)
		case uint64:
			if typed > uint64(maxInt) {
				return maxInt
			}
			return int(typed)
		case float32:
			return int(typed)
		case float64:
			return int(typed)
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				if parsed > int64(maxInt) {
					return maxInt
				}
				if parsed < int64(minInt) {
					return minInt
				}
				return int(parsed)
			}
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func firstFloat(fields map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int8:
			return float64(typed)
		case int16:
			return float64(typed)
		case int32:
			return float64(typed)
		case int64:
			return float64(typed)
		case uint:
			return float64(typed)
		case uint8:
			return float64(typed)
		case uint16:
			return float64(typed)
		case uint32:
			return float64(typed)
		case uint64:
			return float64(typed)
		case json.Number:
			parsed, err := typed.Float64()
			if err == nil {
				return parsed
			}
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func clampUint(value int, max int) int {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func durationMillis(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	millis := int(duration / time.Millisecond)
	if millis == 0 {
		return 1
	}
	return millis
}

func maxIntValue() int {
	return int(^uint(0) >> 1)
}

func minIntValue() int {
	return -maxIntValue() - 1
}
