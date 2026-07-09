package observability

import (
	"time"
)

// ObservabilityComponent constants define component identifiers.
const (
	ComponentHTTP          = "http"
	ComponentRelay         = "relay"
	ComponentBilling       = "billing"
	ComponentWorkflow      = "workflow"
	ComponentAgent         = "agent"
	ComponentSchedule      = "schedule"
	ComponentTask          = "task"
	ComponentAuth          = "auth"
	ComponentGateway       = "gateway"
	ComponentObservability = "observability"
	ComponentUnknown       = "unknown"
)

// RequestEvent constants define event types for request logging.
const (
	RequestEventStarted   = "request.started"
	RequestEventCompleted = "request.completed"
	RequestEventFailed    = "request.failed"
)

// HealthStatus represents the health state of a component.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheck represents a health check result for a component.
type HealthCheck struct {
	Component string         `json:"component"`
	Status    HealthStatus   `json:"status"`
	Message   string         `json:"message,omitempty"`
	Latency   time.Duration  `json:"latency,omitempty"`
	CheckedAt time.Time      `json:"checkedAt"`
	Details   map[string]any `json:"details,omitempty"`
}

// TraceContext holds distributed tracing identifiers.
type TraceContext struct {
	TraceID      string `json:"traceId"`
	SpanID       string `json:"spanId"`
	ParentSpanID string `json:"parentSpanId,omitempty"`
	Sampled      bool   `json:"sampled"`
}

// RequestMetrics contains aggregated metrics for a request.
type RequestMetrics struct {
	TotalRequests int64         `json:"totalRequests"`
	SuccessCount  int64         `json:"successCount"`
	FailureCount  int64         `json:"failureCount"`
	AvgDuration   time.Duration `json:"avgDuration"`
	P50Duration   time.Duration `json:"p50Duration"`
	P95Duration   time.Duration `json:"p95Duration"`
	P99Duration   time.Duration `json:"p99Duration"`
	TotalTokens   int64         `json:"totalTokens"`
	TotalCostUSD  float64       `json:"totalCostUsd"`
}

// ErrorClassification categorizes errors for observability.
type ErrorClassification string

const (
	ErrorClassTransient  ErrorClassification = "transient"
	ErrorClassPermanent  ErrorClassification = "permanent"
	ErrorClassRateLimit  ErrorClassification = "rate_limit"
	ErrorClassAuth       ErrorClassification = "auth"
	ErrorClassValidation ErrorClassification = "validation"
	ErrorClassUnknown    ErrorClassification = "unknown"
)

// ClassifiedError represents an error with its classification for observability.
type ClassifiedError struct {
	Error          string              `json:"error"`
	Code           string              `json:"code,omitempty"`
	Classification ErrorClassification `json:"classification"`
	Retryable      bool                `json:"retryable"`
	OccurredAt     time.Time           `json:"occurredAt"`
}

// ObservabilityConfig holds configuration for the observability stack.
type ObservabilityConfig struct {
	LogLevel        string        `json:"logLevel"`
	EnableTracing   bool          `json:"enableTracing"`
	EnableMetrics   bool          `json:"enableMetrics"`
	EnableAlerting  bool          `json:"enableAlerting"`
	MetricsInterval time.Duration `json:"metricsInterval"`
	TraceSampleRate float64       `json:"traceSampleRate"`
	MaxLogRetention time.Duration `json:"maxLogRetention"`
}

// DefaultObservabilityConfig returns a configuration with sensible defaults.
func DefaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		LogLevel:        "info",
		EnableTracing:   true,
		EnableMetrics:   true,
		EnableAlerting:  true,
		MetricsInterval: time.Minute,
		TraceSampleRate: 0.1,
		MaxLogRetention: 7 * 24 * time.Hour,
	}
}
