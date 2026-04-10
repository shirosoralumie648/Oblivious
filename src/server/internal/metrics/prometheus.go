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
