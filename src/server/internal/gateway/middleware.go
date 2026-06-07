package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	stdhttp "net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var gatewayRequestCounter uint64
var gatewayLogger = log.New(os.Stdout, "[gateway] ", log.LstdFlags)

// WithRequestID generates or extracts an X-Request-ID and injects it into the context.
func WithRequestID(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = fmt.Sprintf("gw-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&gatewayRequestCounter, 1))
		}
		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithLogging records request method, path, status code, and duration.
func WithLogging(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		startedAt := time.Now()
		recorder := &gatewayStatusRecorder{ResponseWriter: w, status: stdhttp.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(startedAt)
		requestID, _ := r.Context().Value(requestIDContextKey).(string)
		gatewayLogger.Printf("%s %s status=%d duration=%s request_id=%s",
			r.Method, r.URL.Path, recorder.status, duration.Truncate(time.Millisecond), requestID)
	})
}

// WithCORS applies CORS headers. Requests from origins not in the allow list
// are passed through without CORS headers (not blocked), matching the existing
// project convention.
func WithCORS(allowedOrigins []string) Middleware {
	normalized := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			normalized[trimmed] = struct{}{}
		}
	}

	return func(next stdhttp.Handler) stdhttp.Handler {
		if len(normalized) == 0 {
			return next
		}

		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if _, ok := normalized[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Vary", "Origin")

				if reqHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); reqHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				}
			}

			if r.Method == stdhttp.MethodOptions {
				if _, ok := normalized[origin]; ok {
					w.WriteHeader(stdhttp.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WithJWTAuth extracts a Bearer token from the Authorization header, validates
// the JWT signature, and injects Claims into the request context.
func WithJWTAuth(secret []byte) Middleware {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeGatewayError(w, stdhttp.StatusUnauthorized, "unauthorized", "missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeGatewayError(w, stdhttp.StatusUnauthorized, "unauthorized", "invalid Authorization format, expected Bearer token")
				return
			}

			token := strings.TrimSpace(parts[1])
			claims, err := validateJWT(token, secret)
			if err != nil {
				writeGatewayError(w, stdhttp.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithRateLimit applies per-user and per-org rate limiting using the sliding window limiter.
func WithRateLimit(limiter *SlidingWindowLimiter) Middleware {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			claims, ok := r.Context().Value(claimsContextKey).(*Claims)
			if !ok || claims == nil {
				// No claims means auth was skipped; pass through.
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Check user-level RPM.
			if err := limiter.Allow(ctx, "user:"+claims.UserID+":rpm", limiter.cfg.DefaultRPM); err != nil {
				w.Header().Set("Retry-After", "60")
				writeGatewayError(w, stdhttp.StatusTooManyRequests, "rate_limited", "user request rate limit exceeded")
				return
			}

			// Check org-level RPM if org is present.
			if claims.OrganizationID != "" {
				if err := limiter.Allow(ctx, "org:"+claims.OrganizationID+":rpm", limiter.cfg.DefaultOrgRPM); err != nil {
					w.Header().Set("Retry-After", "60")
					writeGatewayError(w, stdhttp.StatusTooManyRequests, "rate_limited", "organization request rate limit exceeded")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// --- JWT validation (HMAC-SHA256) ---

// validateJWT validates a compact JWT (header.payload.signature) using HMAC-SHA256.
// This is a minimal implementation; for production, consider a full JWT library.
func validateJWT(tokenString string, secret []byte) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed jwt: expected 3 parts, got %d", len(parts))
	}

	// Verify signature.
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	// Check expiration.
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// ClaimsFromContext extracts the gateway Claims from the request context.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsContextKey).(*Claims)
	return claims
}

// RequestIDFromContext extracts the gateway request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey).(string)
	return id
}
