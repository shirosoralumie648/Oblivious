package http

import (
	"context"
	"fmt"
	"log"
	stdhttp "net/http"
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
