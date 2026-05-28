package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
)

const requestIDHeader = "X-Request-Id"

type contextKey string

const (
	requestIDContextKey          contextKey = "request-id"
	observabilityScopeContextKey contextKey = "observability-scope"
)

var requestCounter uint64
var observabilityLogger = observability.NewJSONLogger(os.Stdout)
var observabilityLoggerMu sync.RWMutex

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
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, stdhttp.StatusInternalServerError, "internal_error", fmt.Sprintf("panic recovered: %v", recovered))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func withRequestID(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		requestID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&requestCounter, 1))
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
		ctx := context.WithValue(r.Context(), observabilityScopeContextKey, scope)
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
		currentObservabilityLogger().Log(ctx, observability.Event{
			Component:      "http",
			Event:          "http.request",
			RequestID:      requestID,
			OrganizationID: organizationID,
			UserID:         userID,
			Method:         r.Method,
			Route:          route,
			Status:         recorder.status,
			Latency:        duration,
		})
	})
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
