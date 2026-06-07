package gateway

import (
	stdhttp "net/http"
	"time"
)

// ServiceTarget identifies a downstream microservice.
type ServiceTarget string

const (
	ServiceChat          ServiceTarget = "chat"
	ServiceWorkflow      ServiceTarget = "workflow"
	ServiceKnowledge     ServiceTarget = "knowledge"
	ServiceAgent         ServiceTarget = "agent"
	ServiceBilling       ServiceTarget = "billing"
	ServiceMarketplace   ServiceTarget = "marketplace"
	ServiceChannel       ServiceTarget = "channel"
	ServiceTask          ServiceTarget = "task"
	ServiceObservability ServiceTarget = "observability"
	ServiceRelay         ServiceTarget = "relay"
	ServiceAdmin         ServiceTarget = "admin"
)

// RouteEntry maps a URL path prefix to a downstream service handler.
type RouteEntry struct {
	Prefix  string
	Target  ServiceTarget
	Handler stdhttp.Handler
}

// Claims represents decoded JWT claims.
type Claims struct {
	UserID         string `json:"sub"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationID string `json:"org_id"`
	Exp            int64  `json:"exp"`
	Iat            int64  `json:"iat"`
}

// HealthStatus represents the health of a single downstream service.
type HealthStatus struct {
	Service ServiceTarget `json:"service"`
	Status  string        `json:"status"`
	Latency string        `json:"latency,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// AggregatedHealth is the overall health check response.
type AggregatedHealth struct {
	Status   string         `json:"status"`
	Services []HealthStatus `json:"services"`
}

// contextKey is a private type for context keys in this package.
type contextKey string

const (
	claimsContextKey    contextKey = "gateway-claims"
	requestIDContextKey contextKey = "gateway-request-id"
)

// SlidingWindowConfig holds rate limiter defaults.
type SlidingWindowConfig struct {
	DefaultRPM    int
	DefaultTPM    int
	DefaultOrgRPM int
	DefaultOrgTPM int
	// WindowSize is the sliding window duration (default 1 minute).
	WindowSize time.Duration
}
