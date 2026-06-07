package metricscollector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidMetricName      = errors.New("metric name is required")
	ErrInvalidMetricType      = errors.New("metric type must be counter, gauge, or histogram")
	ErrInvalidOrganization    = errors.New("organization is required")
	ErrInvalidService         = errors.New("service is required")
	ErrMetricsSinkUnavailable = errors.New("metrics sink is unavailable")
)

type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

type MetricSample struct {
	Name         string            `json:"name"`
	Type         MetricType        `json:"type"`
	Value        float64           `json:"value"`
	Labels       map[string]string `json:"labels,omitempty"`
	Organization string            `json:"organization,omitempty"`
	Service      string            `json:"service,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

type LogEntry struct {
	ID             string         `json:"id"`
	Timestamp      time.Time      `json:"timestamp"`
	Level          string         `json:"level"`
	OrganizationID string         `json:"organizationId"`
	Service        string         `json:"service"`
	Endpoint       string         `json:"endpoint,omitempty"`
	Method         string         `json:"method,omitempty"`
	StatusCode     int            `json:"statusCode,omitempty"`
	DurationMS     int64          `json:"durationMs,omitempty"`
	Message        string         `json:"message"`
	TraceID        string         `json:"traceId,omitempty"`
	SpanID         string         `json:"spanId,omitempty"`
	Fields         map[string]any `json:"fields,omitempty"`
}

type MetricsQuery struct {
	OrganizationID string
	Service        string
	MetricName     string
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
}

type LogsQuery struct {
	OrganizationID string
	Service        string
	Level          string
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Offset         int
}

type MetricsStore interface {
	InsertMetricSample(ctx context.Context, sample MetricSample) error
	ListMetricSamples(ctx context.Context, query MetricsQuery) ([]MetricSample, error)
}

type LogsStore interface {
	InsertLogEntry(ctx context.Context, entry LogEntry) error
	ListLogEntries(ctx context.Context, query LogsQuery) ([]LogEntry, error)
	CountLogEntries(ctx context.Context, query LogsQuery) (int, error)
}

type ObservabilityService struct {
	metricsStore MetricsStore
	logsStore    LogsStore
}

func NewObservabilityService(metricsStore MetricsStore, logsStore LogsStore) *ObservabilityService {
	return &ObservabilityService{
		metricsStore: metricsStore,
		logsStore:    logsStore,
	}
}

func (s *ObservabilityService) LogRequest(ctx context.Context, entry LogEntry) (LogEntry, error) {
	if s.logsStore == nil {
		return LogEntry{}, ErrMetricsSinkUnavailable
	}

	normalized := normalizeLogEntry(entry)
	if err := validateLogEntry(normalized); err != nil {
		return LogEntry{}, err
	}

	if err := s.logsStore.InsertLogEntry(ctx, normalized); err != nil {
		return LogEntry{}, fmt.Errorf("insert log entry: %w", err)
	}

	return normalized, nil
}

func (s *ObservabilityService) GetMetrics(ctx context.Context, query MetricsQuery) ([]MetricSample, error) {
	if s.metricsStore == nil {
		return nil, ErrMetricsSinkUnavailable
	}

	normalized := normalizeMetricsQuery(query)

	samples, err := s.metricsStore.ListMetricSamples(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("list metric samples: %w", err)
	}

	if samples == nil {
		return []MetricSample{}, nil
	}

	return samples, nil
}

func (s *ObservabilityService) GetLogs(ctx context.Context, query LogsQuery) ([]LogEntry, error) {
	if s.logsStore == nil {
		return nil, ErrMetricsSinkUnavailable
	}

	normalized := normalizeLogsQuery(query)

	entries, err := s.logsStore.ListLogEntries(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("list log entries: %w", err)
	}

	if entries == nil {
		return []LogEntry{}, nil
	}

	return entries, nil
}

func normalizeLogEntry(entry LogEntry) LogEntry {
	normalized := entry
	normalized.ID = strings.TrimSpace(entry.ID)
	normalized.Level = normalizeLogLevel(entry.Level)
	normalized.OrganizationID = strings.TrimSpace(entry.OrganizationID)
	normalized.Service = strings.TrimSpace(entry.Service)
	normalized.Endpoint = strings.TrimSpace(entry.Endpoint)
	normalized.Method = strings.ToUpper(strings.TrimSpace(entry.Method))
	normalized.Message = strings.TrimSpace(entry.Message)
	normalized.TraceID = strings.TrimSpace(entry.TraceID)
	normalized.SpanID = strings.TrimSpace(entry.SpanID)

	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = time.Now().UTC()
	}

	if normalized.Fields == nil {
		normalized.Fields = make(map[string]any)
	}

	return normalized
}

func normalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return "debug"
	case "warn", "warning":
		return "warning"
	case "error", "err":
		return "error"
	default:
		return "info"
	}
}

func validateLogEntry(entry LogEntry) error {
	if entry.OrganizationID == "" {
		return ErrInvalidOrganization
	}
	if entry.Service == "" {
		return ErrInvalidService
	}
	return nil
}

func normalizeMetricsQuery(query MetricsQuery) MetricsQuery {
	normalized := query
	normalized.OrganizationID = strings.TrimSpace(query.OrganizationID)
	normalized.Service = strings.TrimSpace(query.Service)
	normalized.MetricName = strings.TrimSpace(query.MetricName)

	if normalized.Limit <= 0 {
		normalized.Limit = 100
	}
	if normalized.Limit > 1000 {
		normalized.Limit = 1000
	}

	return normalized
}

func normalizeLogsQuery(query LogsQuery) LogsQuery {
	normalized := query
	normalized.OrganizationID = strings.TrimSpace(query.OrganizationID)
	normalized.Service = strings.TrimSpace(query.Service)
	normalized.Level = normalizeLogLevel(query.Level)

	if normalized.Limit <= 0 {
		normalized.Limit = 100
	}
	if normalized.Limit > 1000 {
		normalized.Limit = 1000
	}
	if normalized.Offset < 0 {
		normalized.Offset = 0
	}

	return normalized
}

type SQLMetricsStore struct {
	db *sql.DB
}

func NewSQLMetricsStore(db *sql.DB) *SQLMetricsStore {
	return &SQLMetricsStore{db: db}
}

func (s *SQLMetricsStore) InsertMetricSample(ctx context.Context, sample MetricSample) error {
	if s == nil || s.db == nil {
		return nil
	}

	labelsJSON := "{}"
	if len(sample.Labels) > 0 {
		labelsJSON = marshalStringLabels(sample.Labels)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO metric_samples (
			name, type, value, labels, organization, service, timestamp
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sample.Name, string(sample.Type), sample.Value, labelsJSON,
		sample.Organization, sample.Service, sample.Timestamp)
	if err != nil {
		return fmt.Errorf("insert metric sample: %w", err)
	}

	return nil
}

func (s *SQLMetricsStore) ListMetricSamples(ctx context.Context, query MetricsQuery) ([]MetricSample, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT name, type, value, labels, organization, service, timestamp
		FROM metric_samples
		WHERE ($1 = '' OR organization = $1)
		  AND ($2 = '' OR service = $2)
		  AND ($3 = '' OR name = $3)
		  AND ($4 = '0001-01-01' OR timestamp >= $4)
		  AND ($5 = '0001-01-01' OR timestamp <= $5)
		ORDER BY timestamp DESC
		LIMIT $6
	`, query.OrganizationID, query.Service, query.MetricName,
		query.StartTime, query.EndTime, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	samples := []MetricSample{}
	for rows.Next() {
		var sample MetricSample
		var labelsJSON string
		var metricType string
		if err := rows.Scan(
			&sample.Name,
			&metricType,
			&sample.Value,
			&labelsJSON,
			&sample.Organization,
			&sample.Service,
			&sample.Timestamp,
		); err != nil {
			return nil, err
		}

		sample.Type = MetricType(metricType)
		sample.Labels = unmarshalStringLabels(labelsJSON)
		samples = append(samples, sample)
	}

	return samples, rows.Err()
}

type SQLLogsStore struct {
	db *sql.DB
}

func NewSQLLogsStore(db *sql.DB) *SQLLogsStore {
	return &SQLLogsStore{db: db}
}

func (s *SQLLogsStore) InsertLogEntry(ctx context.Context, entry LogEntry) error {
	if s == nil || s.db == nil {
		return nil
	}

	fieldsJSON := "{}"
	if len(entry.Fields) > 0 {
		raw, err := json.Marshal(entry.Fields)
		if err != nil {
			return fmt.Errorf("marshal log fields: %w", err)
		}
		fieldsJSON = string(raw)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO log_entries (
			id, timestamp, level, organization_id, service, endpoint, method,
			status_code, duration_ms, message, trace_id, span_id, fields
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, entry.ID, entry.Timestamp, entry.Level, entry.OrganizationID,
		entry.Service, entry.Endpoint, entry.Method,
		entry.StatusCode, entry.DurationMS, entry.Message,
		entry.TraceID, entry.SpanID, fieldsJSON)
	if err != nil {
		return fmt.Errorf("insert log entry: %w", err)
	}

	return nil
}

func (s *SQLLogsStore) ListLogEntries(ctx context.Context, query LogsQuery) ([]LogEntry, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, timestamp, level, organization_id, service, endpoint, method,
			   status_code, duration_ms, message, trace_id, span_id, fields
		FROM log_entries
		WHERE ($1 = '' OR organization_id = $1)
		  AND ($2 = '' OR service = $2)
		  AND ($3 = '' OR level = $3)
		  AND ($4 = '0001-01-01' OR timestamp >= $4)
		  AND ($5 = '0001-01-01' OR timestamp <= $5)
		ORDER BY timestamp DESC
		LIMIT $6 OFFSET $7
	`, query.OrganizationID, query.Service, query.Level,
		query.StartTime, query.EndTime, query.Limit, query.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []LogEntry{}
	for rows.Next() {
		var entry LogEntry
		var fieldsJSON string
		if err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Level,
			&entry.OrganizationID,
			&entry.Service,
			&entry.Endpoint,
			&entry.Method,
			&entry.StatusCode,
			&entry.DurationMS,
			&entry.Message,
			&entry.TraceID,
			&entry.SpanID,
			&fieldsJSON,
		); err != nil {
			return nil, err
		}

		entry.Fields = unmarshalAnyLabels(fieldsJSON)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func (s *SQLLogsStore) CountLogEntries(ctx context.Context, query LogsQuery) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}

	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM log_entries
		WHERE ($1 = '' OR organization_id = $1)
		  AND ($2 = '' OR service = $2)
		  AND ($3 = '' OR level = $3)
		  AND ($4 = '0001-01-01' OR timestamp >= $4)
		  AND ($5 = '0001-01-01' OR timestamp <= $5)
	`, query.OrganizationID, query.Service, query.Level,
		query.StartTime, query.EndTime).Scan(&count)

	return count, err
}

func marshalStringLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "{}"
	}

	raw, err := json.Marshal(labels)
	if err != nil {
		return "{}"
	}

	return string(raw)
}

func unmarshalStringLabels(jsonStr string) map[string]string {
	if jsonStr == "" || jsonStr == "{}" {
		return nil
	}

	result := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	return result
}

func unmarshalAnyLabels(jsonStr string) map[string]any {
	if jsonStr == "" || jsonStr == "{}" {
		return nil
	}

	result := make(map[string]any)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	return result
}
