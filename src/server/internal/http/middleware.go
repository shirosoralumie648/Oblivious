package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
	relaytypes "oblivious/server/internal/relay/types"
)

const requestIDHeader = "X-Request-Id"

type contextKey string

const (
	requestIDContextKey          contextKey = "request-id"
	observabilityScopeContextKey contextKey = "observability-scope"
)

var observabilityLogger = observability.NewJSONLogger(os.Stdout)
var observabilityLoggerMu sync.RWMutex
var requestLogSink observability.RequestLogSink = observability.NoopRequestLogSink{}
var requestLogSinkMu sync.RWMutex
var httpAlertSink observability.AlertSink
var httpAlertSinkMu sync.RWMutex
var httpRecoveryController *observability.RecoveryController
var httpRecoveryControllerMu sync.RWMutex
var httpAlertLatencySLOThreshold = 5 * time.Second
var httpAlertLatencySLOThresholdMu sync.RWMutex

type statusRecorder struct {
	stdhttp.ResponseWriter
	status int
}

type observabilityScope struct {
	mu             sync.RWMutex
	organizationID string
	userID         string
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func applyMiddleware(handler stdhttp.Handler, middleware ...func(stdhttp.Handler) stdhttp.Handler) stdhttp.Handler {
	wrapped := handler
	for index := len(middleware) - 1; index >= 0; index-- {
		wrapped = middleware[index](wrapped)
	}
	return wrapped
}

func withRecover(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		startedAt := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, stdhttp.StatusInternalServerError, "internal_error", fmt.Sprintf("panic recovered: %v", recovered))
				requestID := w.Header().Get(requestIDHeader)
				if requestID == "" {
					requestID = requestIDFromContext(r.Context())
				}
				routeHTTPPanicAlert(r.Context(), r.Method, normalizeRoute(r.URL.Path), time.Since(startedAt), startedAt.UTC(), requestID, recovered)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func withRequestID(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		requestID := uuid.NewString()
		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	return requestID
}

func withLogging(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: stdhttp.StatusOK}
		scope := &observabilityScope{}
		relayRequestLogScope := relaytypes.NewRequestLogScope()
		ctx := context.WithValue(r.Context(), observabilityScopeContextKey, scope)
		ctx = relaytypes.WithRequestLogScope(ctx, relayRequestLogScope)
		route := normalizeRoute(r.URL.Path)
		ctx, span := observability.StartSpan(
			ctx,
			"http.request",
			observability.String("http.method", r.Method),
			observability.String("http.route", route),
		)
		defer span.End()

		next.ServeHTTP(recorder, r.WithContext(ctx))

		requestID := requestIDFromContext(r.Context())
		duration := time.Since(startedAt)
		organizationID, userID := scope.snapshot()
		metrics.RecordHTTPRequest(r.Method, route, recorder.status)
		metrics.ObserveHTTPRequestDuration(r.Method, route, duration.Seconds())
		component, featureType := classifyRequestFeature(r.URL.Path)
		traceID := requestID
		if _, err := uuid.Parse(traceID); err != nil {
			traceID = uuid.NewString()
		}
		spanID := uuid.NewString()
		event := observability.Event{
			Component:      component,
			Event:          "http.request",
			RequestID:      requestID,
			TraceID:        traceID,
			SpanID:         spanID,
			OrganizationID: organizationID,
			UserID:         userID,
			Method:         r.Method,
			Route:          route,
			Status:         recorder.status,
			Latency:        duration,
			Fields: map[string]any{
				"feature_type": featureType,
			},
		}
		if component == observability.ComponentRelay {
			event.RelayAPIType = featureType
			enrichRelayRequestLogEvent(&event, relayRequestLogScope)
		}
		currentObservabilityLogger().Log(ctx, event)
		if err := observability.WriteRequestLog(ctx, currentRequestLogSink(), event, startedAt.UTC()); err != nil {
			routeRequestLogSinkAlert(ctx, r.Method, route, startedAt.UTC(), requestID, err)
		}
		routeHTTPAlert(ctx, r.Method, route, recorder.status, duration, startedAt.UTC(), requestID)
		routeHTTPLatencySLOAlert(ctx, r.Method, route, recorder.status, duration, startedAt.UTC(), requestID)
	})
}

func enrichRelayRequestLogEvent(event *observability.Event, scope *relaytypes.RequestLogScope) {
	if event == nil || scope == nil {
		return
	}
	metadata, ok := scope.Snapshot()
	if !ok {
		return
	}
	if strings.TrimSpace(metadata.RequestID) != "" {
		event.RequestID = metadata.RequestID
	}
	if strings.TrimSpace(metadata.OrganizationID) != "" {
		event.OrganizationID = metadata.OrganizationID
	}
	if strings.TrimSpace(metadata.UserID) != "" {
		event.UserID = metadata.UserID
	}
	event.ChannelID = metadata.ChannelID
	event.Provider = metadata.Provider
	event.BillingSessionID = metadata.BillingSessionID
	if event.Fields == nil {
		event.Fields = map[string]any{}
	}
	addStringField(event.Fields, "model", metadata.Model)
	addStringField(event.Fields, "relay_request_id", metadata.RequestID)
	addStringField(event.Fields, "relay_organization_id", metadata.OrganizationID)
	addStringField(event.Fields, "relay_user_id", metadata.UserID)
	addStringField(event.Fields, "requested_model", metadata.RequestedModel)
	addStringField(event.Fields, "resolved_model", metadata.ResolvedModel)
	addStringField(event.Fields, "relay_usage_status", metadata.Status)
	addStringField(event.Fields, "error_code", metadata.ErrorCode)
	addStringField(event.Fields, "price_currency", metadata.PriceCurrency)
	addStringField(event.Fields, "price_source", metadata.PriceSource)
	addPositiveFloatField(event.Fields, "preauthorized_amount", metadata.PreauthorizedAmount)
	addPositiveFloatField(event.Fields, "token_preauthorized_amount", metadata.TokenPreauthorizedAmount)
	addPositiveFloatField(event.Fields, "cost", metadata.Cost)
	addPositiveFloatField(event.Fields, "channel_cost", metadata.ChannelCost)
	addPositiveIntField(event.Fields, "request_tokens", metadata.RequestTokens)
	addPositiveIntField(event.Fields, "response_tokens", metadata.ResponseTokens)
	addPositiveIntField(event.Fields, "total_tokens", metadata.TotalTokens)
	if metadata.PriceSnapshot != nil {
		event.Fields["price_snapshot"] = metadata.PriceSnapshot
	}
}

func addStringField(fields map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fields[key] = value
}

func addPositiveFloatField(fields map[string]any, key string, value float64) {
	if value <= 0 {
		return
	}
	fields[key] = value
}

func addPositiveIntField(fields map[string]any, key string, value int) {
	if value <= 0 {
		return
	}
	fields[key] = value
}

func attachSessionToObservabilityScope(r *stdhttp.Request, session auth.Session) {
	scope, ok := r.Context().Value(observabilityScopeContextKey).(*observabilityScope)
	if !ok || scope == nil {
		return
	}
	scope.set(session.OrganizationID, session.User.ID)
}

func (s *observabilityScope) set(organizationID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.organizationID = organizationID
	s.userID = userID
}

func (s *observabilityScope) snapshot() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.organizationID, s.userID
}

func currentObservabilityLogger() *observability.Logger {
	observabilityLoggerMu.RLock()
	defer observabilityLoggerMu.RUnlock()
	return observabilityLogger
}

func setObservabilityLoggerForTest(logger *observability.Logger) func() {
	observabilityLoggerMu.Lock()
	previous := observabilityLogger
	observabilityLogger = logger
	observabilityLoggerMu.Unlock()

	return func() {
		observabilityLoggerMu.Lock()
		observabilityLogger = previous
		observabilityLoggerMu.Unlock()
	}
}

func currentRequestLogSink() observability.RequestLogSink {
	requestLogSinkMu.RLock()
	defer requestLogSinkMu.RUnlock()
	return requestLogSink
}

func setRequestLogSinkForTest(sink observability.RequestLogSink) func() {
	return setRequestLogSink(sink)
}

func setRequestLogSink(sink observability.RequestLogSink) func() {
	requestLogSinkMu.Lock()
	previous := requestLogSink
	requestLogSink = sink
	requestLogSinkMu.Unlock()

	return func() {
		requestLogSinkMu.Lock()
		requestLogSink = previous
		requestLogSinkMu.Unlock()
	}
}

func setHTTPAlertRouterForTest(sink observability.AlertSink) func() {
	return setHTTPAlertSink(sink)
}

func setHTTPAlertSink(sink observability.AlertSink) func() {
	httpAlertSinkMu.Lock()
	previous := httpAlertSink
	httpAlertSink = sink
	httpAlertSinkMu.Unlock()

	return func() {
		httpAlertSinkMu.Lock()
		httpAlertSink = previous
		httpAlertSinkMu.Unlock()
	}
}

func currentHTTPAlertSink() observability.AlertSink {
	httpAlertSinkMu.RLock()
	defer httpAlertSinkMu.RUnlock()
	return httpAlertSink
}

func setHTTPRecoveryControllerForTest(controller *observability.RecoveryController) func() {
	return setHTTPRecoveryController(controller)
}

func setHTTPRecoveryController(controller *observability.RecoveryController) func() {
	httpRecoveryControllerMu.Lock()
	previous := httpRecoveryController
	httpRecoveryController = controller
	httpRecoveryControllerMu.Unlock()

	return func() {
		httpRecoveryControllerMu.Lock()
		httpRecoveryController = previous
		httpRecoveryControllerMu.Unlock()
	}
}

func currentHTTPRecoveryController() *observability.RecoveryController {
	httpRecoveryControllerMu.RLock()
	defer httpRecoveryControllerMu.RUnlock()
	return httpRecoveryController
}

func setHTTPAlertLatencySLOThresholdForTest(threshold time.Duration) func() {
	return setHTTPAlertLatencySLOThreshold(threshold)
}

func setHTTPAlertLatencySLOThreshold(threshold time.Duration) func() {
	httpAlertLatencySLOThresholdMu.Lock()
	previous := httpAlertLatencySLOThreshold
	httpAlertLatencySLOThreshold = threshold
	httpAlertLatencySLOThresholdMu.Unlock()

	return func() {
		httpAlertLatencySLOThresholdMu.Lock()
		httpAlertLatencySLOThreshold = previous
		httpAlertLatencySLOThresholdMu.Unlock()
	}
}

func currentHTTPAlertLatencySLOThreshold() time.Duration {
	httpAlertLatencySLOThresholdMu.RLock()
	defer httpAlertLatencySLOThresholdMu.RUnlock()
	return httpAlertLatencySLOThreshold
}

func routeHTTPAlert(ctx context.Context, method, route string, status int, latency time.Duration, occurredAt time.Time, requestID string) {
	if status < stdhttp.StatusInternalServerError {
		return
	}
	alertSink := currentHTTPAlertSink()
	recoveryController := currentHTTPRecoveryController()
	if alertSink == nil && recoveryController == nil {
		return
	}
	event := httpAlertEvent(method, route, status, latency, occurredAt, requestID)
	if alertSink != nil {
		_ = alertSink.Notify(ctx, event)
	}
	if recoveryController != nil {
		_, _ = recoveryController.HandleAlert(ctx, event)
	}
}

func routeHTTPLatencySLOAlert(ctx context.Context, method, route string, status int, latency time.Duration, occurredAt time.Time, requestID string) {
	threshold := currentHTTPAlertLatencySLOThreshold()
	if threshold <= 0 || latency <= threshold || route == "/healthz" || route == "/metrics" {
		return
	}
	alertSink := currentHTTPAlertSink()
	recoveryController := currentHTTPRecoveryController()
	if alertSink == nil && recoveryController == nil {
		return
	}
	event := httpLatencySLOAlertEvent(method, route, status, latency, threshold, occurredAt, requestID)
	if alertSink != nil {
		_ = alertSink.Notify(ctx, event)
	}
	if recoveryController != nil {
		_, _ = recoveryController.HandleAlert(ctx, event)
	}
}

func httpLatencySLOAlertEvent(method, route string, status int, latency time.Duration, threshold time.Duration, occurredAt time.Time, requestID string) observability.AlertEvent {
	return observability.AlertEvent{
		Key:        fmt.Sprintf("http-slo:%s:latency", route),
		Severity:   observability.AlertSeverityWarning,
		Title:      fmt.Sprintf("HTTP latency SLO exceeded on %s", route),
		Message:    fmt.Sprintf("%s %s returned %d in %dms over %dms SLO", method, route, status, latency.Milliseconds(), threshold.Milliseconds()),
		Component:  observability.ComponentHTTP,
		OccurredAt: occurredAt,
		Fields: map[string]any{
			"method":       method,
			"route":        route,
			"status":       status,
			"latency_ms":   latency.Milliseconds(),
			"threshold_ms": threshold.Milliseconds(),
			"request_id":   requestID,
			"source":       "http.slo",
			"slo":          "latency",
		},
	}
}

func routeHTTPPanicAlert(ctx context.Context, method, route string, latency time.Duration, occurredAt time.Time, requestID string, recovered any) {
	alertSink := currentHTTPAlertSink()
	recoveryController := currentHTTPRecoveryController()
	if alertSink == nil && recoveryController == nil {
		return
	}
	event := observability.AlertEvent{
		Key:        fmt.Sprintf("http:%s:panic", route),
		Severity:   observability.AlertSeverityCritical,
		Title:      fmt.Sprintf("HTTP panic recovered on %s", route),
		Message:    fmt.Sprintf("%s %s panic recovered in %dms: %v", method, route, latency.Milliseconds(), recovered),
		Component:  observability.ComponentHTTP,
		OccurredAt: occurredAt,
		Fields: map[string]any{
			"method":          method,
			"route":           route,
			"status":          stdhttp.StatusInternalServerError,
			"latency_ms":      latency.Milliseconds(),
			"request_id":      requestID,
			"source":          "http.recover",
			"failure_kind":    "panic",
			"panic_recovered": true,
		},
	}
	if alertSink != nil {
		_ = alertSink.Notify(ctx, event)
	}
	if recoveryController != nil {
		_, _ = recoveryController.HandleAlert(ctx, event)
	}
}

func httpAlertEvent(method, route string, status int, latency time.Duration, occurredAt time.Time, requestID string) observability.AlertEvent {
	severity := observability.AlertSeverityWarning
	if status >= stdhttp.StatusServiceUnavailable {
		severity = observability.AlertSeverityCritical
	}
	return observability.AlertEvent{
		Key:        fmt.Sprintf("http:%s:%d", route, status),
		Severity:   severity,
		Title:      fmt.Sprintf("HTTP %d on %s", status, route),
		Message:    fmt.Sprintf("%s %s returned %d in %dms", method, route, status, latency.Milliseconds()),
		Component:  observability.ComponentHTTP,
		OccurredAt: occurredAt,
		Fields: map[string]any{
			"method":      method,
			"route":       route,
			"status":      status,
			"latency_ms":  latency.Milliseconds(),
			"request_id":  requestID,
			"source":      "http.middleware",
			"status_text": stdhttp.StatusText(status),
		},
	}
}

func normalizeRoute(path string) string {
	if path == "" {
		return "/"
	}
	if path == "/healthz" || path == "/metrics" {
		return path
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	dynamicParents := map[string]struct{}{
		"agents":                   {},
		"conversations":            {},
		"knowledge-bases":          {},
		"documents":                {},
		"tasks":                    {},
		"mcp-servers":              {},
		"notifications":            {},
		"organizations":            {},
		"organization-invitations": {},
		"users":                    {},
		"plans":                    {},
		"reviews":                  {},
		"channels":                 {},
		"routes":                   {},
		"packages":                 {},
		"installs":                 {},
		"payouts":                  {},
		"settlements":              {},
		"refunds":                  {},
		"subscriptions":            {},
		"topups":                   {},
		"invoices":                 {},
	}
	for index := range segments {
		if index == 0 {
			continue
		}
		if _, ok := dynamicParents[segments[index-1]]; ok {
			segments[index] = ":id"
		}
	}
	return "/" + strings.Join(segments, "/")
}

func classifyRequestFeature(path string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(path))
	if normalized == "" {
		return observability.ComponentHTTP, "http"
	}

	switch {
	case strings.HasPrefix(normalized, "/v1/") || strings.HasPrefix(normalized, "/api/v1/relay/"):
		return observability.ComponentRelay, classifyRelayFeature(normalized)
	case strings.HasPrefix(normalized, "/api/v1/workflows"):
		return observability.ComponentWorkflow, "workflow"
	case strings.HasPrefix(normalized, "/api/v1/agent/") || normalized == "/api/v1/agent" ||
		strings.HasPrefix(normalized, "/api/v1/app/agents"):
		return observability.ComponentAgent, "agent"
	case strings.Contains(normalized, "/knowledge-bases") || strings.HasPrefix(normalized, "/api/v1/documents/"):
		return "rag", "rag"
	case strings.Contains(normalized, "/conversations") ||
		strings.Contains(normalized, "/message-shares") ||
		strings.Contains(normalized, "/conversation-shares") ||
		strings.Contains(normalized, "/personas"):
		return "chat", "chat"
	default:
		return observability.ComponentHTTP, "http"
	}
}

func classifyRelayFeature(path string) string {
	switch {
	case strings.Contains(path, "/images"):
		return "image"
	case strings.Contains(path, "/audio") ||
		strings.Contains(path, "/speech") ||
		strings.Contains(path, "/transcriptions") ||
		strings.Contains(path, "/translations"):
		return "audio"
	case strings.Contains(path, "/embeddings"):
		return "rag"
	case strings.Contains(path, "/chat") ||
		strings.Contains(path, "/responses") ||
		strings.Contains(path, "/completions"):
		return "chat"
	default:
		return "relay"
	}
}

func withCORS(allowedOrigins []string) func(stdhttp.Handler) stdhttp.Handler {
	normalizedOrigins := map[string]struct{}{}
	for _, origin := range allowedOrigins {
		trimmedOrigin := strings.TrimSpace(origin)
		if trimmedOrigin == "" {
			continue
		}

		normalizedOrigins[trimmedOrigin] = struct{}{}
	}

	if len(normalizedOrigins) == 0 {
		return func(next stdhttp.Handler) stdhttp.Handler {
			return next
		}
	}

	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			_, originAllowed := normalizedOrigins[origin]

			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Vary", "Origin")

				requestHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
				if requestHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				}
			}

			if r.Method == stdhttp.MethodOptions && originAllowed {
				w.WriteHeader(stdhttp.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
