package http

import (
	"context"
	"fmt"
	"log"
	stdhttp "net/http"
	"strings"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-Id"

type contextKey string

const requestIDContextKey contextKey = "request-id"

var requestCounter uint64

type statusRecorder struct {
	stdhttp.ResponseWriter
	status int
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

func withLogging(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: stdhttp.StatusOK}

		next.ServeHTTP(recorder, r)

		requestID, _ := r.Context().Value(requestIDContextKey).(string)
		log.Printf("method=%s path=%s status=%d duration=%s request_id=%s", r.Method, r.URL.Path, recorder.status, time.Since(startedAt), requestID)
	})
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
