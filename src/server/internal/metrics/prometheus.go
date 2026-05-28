package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_requests_total",
			Help: "Total number of relay requests",
		},
		[]string{"channel_id", "model", "api_type", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "relay_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"channel_id", "model", "api_type"},
	)

	TokenUsageTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_tokens_total",
			Help: "Total number of tokens used",
		},
		[]string{"channel_id", "model", "api_type", "token_type"},
	)

	BillingAmountTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_billing_amount_total",
			Help: "Total billing amount in dollars",
		},
		[]string{"channel_id", "model", "api_type", "billing_status"},
	)

	ChannelHealthy = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_channel_healthy",
			Help: "Whether a channel is healthy (1) or not (0)",
		},
		[]string{"channel_id", "model"},
	)

	ChannelLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "relay_channel_latency_seconds",
			Help:    "Per-channel latency in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"channel_id"},
	)

	RateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_rate_limit_exceeded_total",
			Help: "Total number of rate limit exceeded events",
		},
		[]string{"channel_id", "model", "api_type"},
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status_class"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)

	RelayRouteDecisionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_route_decisions_total",
			Help: "Total number of Relay route policy decisions",
		},
		[]string{"class", "api_type", "result", "failure_reason"},
	)

	ProviderFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "provider_failures_total",
			Help: "Total number of upstream provider failures observed by Relay",
		},
		[]string{"provider", "channel_id", "api_type", "reason"},
	)

	ProviderRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "provider_request_duration_seconds",
			Help:    "Upstream provider request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"provider", "channel_id", "api_type"},
	)

	BillingLifecycleEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_lifecycle_events_total",
			Help: "Total number of billing lifecycle transitions",
		},
		[]string{"kind", "status"},
	)

	QuotaSettlementFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "quota_settlement_failures_total",
			Help: "Total number of quota settlement failures by stage",
		},
		[]string{"stage"},
	)

	StripeWebhookEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stripe_webhook_events_total",
			Help: "Total number of Stripe webhook events by type and status",
		},
		[]string{"event_type", "status"},
	)

	StripeWebhookFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stripe_webhook_failures_total",
			Help: "Total number of Stripe webhook failures by low-cardinality reason",
		},
		[]string{"reason"},
	)

	MarketplaceSettlementEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "marketplace_settlement_events_total",
			Help: "Total number of Marketplace settlement and governance events",
		},
		[]string{"event", "status"},
	)

	JobEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_events_total",
			Help: "Total number of task/job lifecycle events",
		},
		[]string{"kind", "status"},
	)

	MigrationRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "migration_runs_total",
			Help: "Total number of migration runs by status",
		},
		[]string{"status"},
	)
)

func RecordRequest(channelID, model, apiType, status string) {
	RequestsTotal.WithLabelValues(channelID, model, apiType, status).Inc()
}

func RecordDuration(channelID, model, apiType string, seconds float64) {
	RequestDuration.WithLabelValues(channelID, model, apiType).Observe(seconds)
}

func RecordTokenUsage(channelID, model, apiType, tokenType string, count int) {
	TokenUsageTotal.WithLabelValues(channelID, model, apiType, tokenType).Add(float64(count))
}

func RecordBillingAmount(channelID, model, apiType, status string, amount float64) {
	BillingAmountTotal.WithLabelValues(channelID, model, apiType, status).Add(amount)
}

func SetChannelHealth(channelID, model string, healthy bool) {
	val := 0.0
	if healthy {
		val = 1.0
	}
	ChannelHealthy.WithLabelValues(channelID, model).Set(val)
}

func RecordChannelLatency(channelID string, seconds float64) {
	ChannelLatency.WithLabelValues(channelID).Observe(seconds)
}

func RecordRateLimitExceeded(channelID, model, apiType string) {
	RateLimitExceeded.WithLabelValues(channelID, model, apiType).Inc()
}

func RecordHTTPRequest(method, route string, status int) {
	HTTPRequestsTotal.WithLabelValues(method, route, statusClass(status)).Inc()
}

func ObserveHTTPRequestDuration(method, route string, seconds float64) {
	HTTPRequestDuration.WithLabelValues(method, route).Observe(seconds)
}

func RecordRelayRouteDecision(class, apiType, result, failureReason string) {
	RelayRouteDecisionsTotal.WithLabelValues(class, apiType, result, lowCardinalityOrNone(failureReason)).Inc()
}

func RecordProviderFailure(provider, channelID, apiType, reason string) {
	ProviderFailuresTotal.WithLabelValues(lowCardinalityOrUnknown(provider), lowCardinalityOrUnknown(channelID), apiType, lowCardinalityOrUnknown(reason)).Inc()
}

func ObserveProviderRequestDuration(provider, channelID, apiType string, seconds float64) {
	ProviderRequestDuration.WithLabelValues(lowCardinalityOrUnknown(provider), lowCardinalityOrUnknown(channelID), apiType).Observe(seconds)
}

func RecordBillingLifecycleEvent(kind, status string) {
	BillingLifecycleEventsTotal.WithLabelValues(lowCardinalityOrUnknown(kind), lowCardinalityOrUnknown(status)).Inc()
}

func RecordQuotaSettlementFailure(stage string) {
	QuotaSettlementFailuresTotal.WithLabelValues(lowCardinalityOrUnknown(stage)).Inc()
}

func RecordStripeWebhookEvent(eventType, status string) {
	StripeWebhookEventsTotal.WithLabelValues(lowCardinalityOrUnknown(eventType), lowCardinalityOrUnknown(status)).Inc()
}

func RecordStripeWebhookFailure(reason string) {
	StripeWebhookFailuresTotal.WithLabelValues(lowCardinalityOrUnknown(reason)).Inc()
}

func RecordMarketplaceSettlementEvent(event, status string) {
	MarketplaceSettlementEventsTotal.WithLabelValues(lowCardinalityOrUnknown(event), lowCardinalityOrUnknown(status)).Inc()
}

func RecordJobEvent(kind, status string) {
	JobEventsTotal.WithLabelValues(lowCardinalityOrUnknown(kind), lowCardinalityOrUnknown(status)).Inc()
}

func RecordMigrationRun(status string) {
	MigrationRunsTotal.WithLabelValues(lowCardinalityOrUnknown(status)).Inc()
}

func statusClass(status int) string {
	if status < 100 {
		return "unknown"
	}
	return string(rune('0'+status/100)) + "xx"
}

func lowCardinalityOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func lowCardinalityOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
