package http

import (
	"errors"
	stdhttp "net/http"

	"oblivious/server/internal/auth"
)

const csrfHeaderName = "X-CSRF-Token"

func (m authMiddleware) securityGuard(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if isSafeMethod(r.Method) || isPublicAuthMutation(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		session, ok := m.currentSession(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get(csrfHeaderName)
		if token == "" {
			writeError(w, stdhttp.StatusForbidden, "csrf_required", "csrf token required")
			return
		}
		if !m.validCSRFToken(session.ID, token) {
			writeError(w, stdhttp.StatusForbidden, "csrf_invalid", "invalid csrf token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m authMiddleware) rateLimit(scope string, policy auth.RateLimitPolicy, next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		session, ok := sessionFromContext(r)
		if !ok {
			writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		key := clientIP(r) + ":" + session.User.ID + ":" + r.Method + ":" + r.URL.Path
		if err := m.service.CheckRateLimit(r.Context(), scope, key, policy); err != nil {
			if errors.Is(err, auth.ErrRateLimited) {
				writeError(w, stdhttp.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
				return
			}
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "rate limit check failed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case stdhttp.MethodGet, stdhttp.MethodHead, stdhttp.MethodOptions:
		return true
	default:
		return false
	}
}

func isPublicAuthMutation(path string) bool {
	switch path {
	case "/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/password-reset/request",
		"/api/v1/auth/password-reset/confirm":
		return true
	default:
		return false
	}
}
