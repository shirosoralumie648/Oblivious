package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRecordRequest(t *testing.T) {
	// Just verify no panic
	RecordRequest("ch_1", "gpt-4o", "chat", "success")
}

func TestRecordDuration(t *testing.T) {
	RecordDuration("ch_1", "gpt-4o", "chat", 0.5)
}

func TestRecordTokenUsage(t *testing.T) {
	RecordTokenUsage("ch_1", "gpt-4o", "chat", "prompt", 1000)
	RecordTokenUsage("ch_1", "gpt-4o", "chat", "completion", 500)
}

func TestRecordBillingAmount(t *testing.T) {
	RecordBillingAmount("ch_1", "gpt-4o", "chat", "settled", 10.5)
}

func TestSetChannelHealth(t *testing.T) {
	SetChannelHealth("ch_1", "gpt-4o", true)
	SetChannelHealth("ch_1", "gpt-4o", false)
}

func TestRecordChannelLatency(t *testing.T) {
	RecordChannelLatency("ch_1", 0.25)
}

func TestRecordRateLimitExceeded(t *testing.T) {
	RecordRateLimitExceeded("ch_1", "gpt-4o", "chat")
}

func TestMetricsRegistered(t *testing.T) {
	// Verify metrics are registered
	r := prometheus.NewRegistry()
	r.MustRegister(RequestsTotal)
	r.MustRegister(RequestDuration)
	r.MustRegister(TokenUsageTotal)
	r.MustRegister(BillingAmountTotal)
	r.MustRegister(ChannelHealthy)
	r.MustRegister(ChannelLatency)
	r.MustRegister(RateLimitExceeded)
}
