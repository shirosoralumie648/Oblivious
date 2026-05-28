package observability

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Event struct {
	Component        string
	Event            string
	RequestID        string
	TraceID          string
	SpanID           string
	OrganizationID   string
	UserID           string
	Method           string
	Route            string
	Status           int
	Latency          time.Duration
	RelayRouteClass  string
	RelayAPIType     string
	BillingPolicy    string
	BillingSessionID string
	ChannelID        string
	Provider         string
	FailureReason    string
	Fields           map[string]any
}

type Logger struct {
	logger *slog.Logger
}

func NewJSONLogger(output io.Writer) *Logger {
	return &Logger{
		logger: slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{})),
	}
}

func (l *Logger) Log(ctx context.Context, event Event) {
	if l == nil || l.logger == nil {
		return
	}

	event = sanitizeEvent(event)
	attrs := eventAttrs(event)
	l.logger.LogAttrs(ctx, slog.LevelInfo, event.Event, attrs...)
}

type Span interface {
	End()
}

type span struct {
	inner oteltrace.Span
}

func (s span) End() {
	if s.inner != nil {
		s.inner.End()
	}
}

func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
	tracer := otel.Tracer("oblivious/server")
	ctx, inner := tracer.Start(ctx, name, oteltrace.WithAttributes(attrs...))
	return ctx, span{inner: inner}
}

func String(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

func Int(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

type Reporter interface {
	ReportError(ctx context.Context, event Event)
}

type NoopReporter struct{}

func (NoopReporter) ReportError(context.Context, Event) {}

type MemoryReporter struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryReporter() *MemoryReporter {
	return &MemoryReporter{}
}

func (r *MemoryReporter) ReportError(_ context.Context, event Event) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, sanitizeEvent(event))
}

func (r *MemoryReporter) Events() []Event {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]Event, len(r.events))
	copy(events, r.events)
	for index := range events {
		events[index].Fields = cloneFields(events[index].Fields)
	}
	return events
}

func eventAttrs(event Event) []slog.Attr {
	attrs := make([]slog.Attr, 0, 18+len(event.Fields))
	addString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			attrs = append(attrs, slog.String(key, value))
		}
	}

	addString("component", event.Component)
	addString("event", event.Event)
	addString("request_id", event.RequestID)
	addString("trace_id", event.TraceID)
	addString("span_id", event.SpanID)
	addString("organization_id", event.OrganizationID)
	addString("user_id", event.UserID)
	addString("method", event.Method)
	addString("route", event.Route)
	if event.Status != 0 {
		attrs = append(attrs, slog.Int("status", event.Status))
	}
	if event.Latency > 0 {
		attrs = append(attrs, slog.Float64("latency_ms", float64(event.Latency)/float64(time.Millisecond)))
	}
	addString("relay_route_class", event.RelayRouteClass)
	addString("relay_api_type", event.RelayAPIType)
	addString("billing_policy", event.BillingPolicy)
	addString("billing_session_id", event.BillingSessionID)
	addString("channel_id", event.ChannelID)
	addString("provider", event.Provider)
	addString("failure_reason", event.FailureReason)

	for key, value := range event.Fields {
		if isSensitiveField(key) {
			continue
		}
		attrs = append(attrs, slog.Any(key, value))
	}
	return attrs
}

func sanitizeEvent(event Event) Event {
	event.Fields = sanitizeFields(event.Fields)
	return event
}

func sanitizeFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(fields))
	for key, value := range fields {
		if isSensitiveField(key) {
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

func cloneFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}

func isSensitiveField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, token := range []string{
		"authorization",
		"cookie",
		"api_key",
		"apikey",
		"token",
		"secret",
		"password",
		"database_url",
		"db_url",
		"payload",
		"prompt",
		"response",
		"body",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
