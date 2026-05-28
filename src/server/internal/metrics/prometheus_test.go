package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func TestRecordHTTPRequest(t *testing.T) {
	RecordHTTPRequest("POST", "/api/v1/app/agents/:id", 201)
	ObserveHTTPRequestDuration("POST", "/api/v1/app/agents/:id", 0.125)
}

func TestRelayObservabilityMetricsRecordRouteDecisionAndProviderFailure(t *testing.T) {
	beforeDecision := testutil.ToFloat64(RelayRouteDecisionsTotal.WithLabelValues("commercial_supported_billed", "chat", "allowed", "none"))
	RecordRelayRouteDecision("commercial_supported_billed", "chat", "allowed", "")
	afterDecision := testutil.ToFloat64(RelayRouteDecisionsTotal.WithLabelValues("commercial_supported_billed", "chat", "allowed", "none"))
	if afterDecision != beforeDecision+1 {
		t.Fatalf("expected relay route decision counter to increment by 1, before=%v after=%v", beforeDecision, afterDecision)
	}

	beforeFailure := testutil.ToFloat64(ProviderFailuresTotal.WithLabelValues("openai", "ch_1", "chat", "request_error"))
	RecordProviderFailure("openai", "ch_1", "chat", "request_error")
	afterFailure := testutil.ToFloat64(ProviderFailuresTotal.WithLabelValues("openai", "ch_1", "chat", "request_error"))
	if afterFailure != beforeFailure+1 {
		t.Fatalf("expected provider failure counter to increment by 1, before=%v after=%v", beforeFailure, afterFailure)
	}

	ObserveProviderRequestDuration("openai", "ch_1", "chat", 0.25)
	if count := testutil.CollectAndCount(ProviderRequestDuration, "provider_request_duration_seconds"); count == 0 {
		t.Fatal("expected provider request duration histogram to be collectable")
	}
}

func TestCommercialBoundaryMetricsRecordBillingWebhookSettlementJobAndMigration(t *testing.T) {
	billingBefore := testutil.ToFloat64(BillingLifecycleEventsTotal.WithLabelValues("checkout", "completed"))
	RecordBillingLifecycleEvent("checkout", "completed")
	billingAfter := testutil.ToFloat64(BillingLifecycleEventsTotal.WithLabelValues("checkout", "completed"))
	if billingAfter != billingBefore+1 {
		t.Fatalf("expected billing lifecycle counter increment, before=%v after=%v", billingBefore, billingAfter)
	}

	quotaBefore := testutil.ToFloat64(QuotaSettlementFailuresTotal.WithLabelValues("settlement"))
	RecordQuotaSettlementFailure("settlement")
	quotaAfter := testutil.ToFloat64(QuotaSettlementFailuresTotal.WithLabelValues("settlement"))
	if quotaAfter != quotaBefore+1 {
		t.Fatalf("expected quota failure counter increment, before=%v after=%v", quotaBefore, quotaAfter)
	}

	webhookBefore := testutil.ToFloat64(StripeWebhookEventsTotal.WithLabelValues("checkout.session.completed", "processed"))
	RecordStripeWebhookEvent("checkout.session.completed", "processed")
	webhookAfter := testutil.ToFloat64(StripeWebhookEventsTotal.WithLabelValues("checkout.session.completed", "processed"))
	if webhookAfter != webhookBefore+1 {
		t.Fatalf("expected webhook event counter increment, before=%v after=%v", webhookBefore, webhookAfter)
	}

	webhookFailureBefore := testutil.ToFloat64(StripeWebhookFailuresTotal.WithLabelValues("invalid_signature"))
	RecordStripeWebhookFailure("invalid_signature")
	webhookFailureAfter := testutil.ToFloat64(StripeWebhookFailuresTotal.WithLabelValues("invalid_signature"))
	if webhookFailureAfter != webhookFailureBefore+1 {
		t.Fatalf("expected webhook failure counter increment, before=%v after=%v", webhookFailureBefore, webhookFailureAfter)
	}

	settlementBefore := testutil.ToFloat64(MarketplaceSettlementEventsTotal.WithLabelValues("paid_install", "paid"))
	RecordMarketplaceSettlementEvent("paid_install", "paid")
	settlementAfter := testutil.ToFloat64(MarketplaceSettlementEventsTotal.WithLabelValues("paid_install", "paid"))
	if settlementAfter != settlementBefore+1 {
		t.Fatalf("expected marketplace settlement counter increment, before=%v after=%v", settlementBefore, settlementAfter)
	}

	jobBefore := testutil.ToFloat64(JobEventsTotal.WithLabelValues("task.start", "running"))
	RecordJobEvent("task.start", "running")
	jobAfter := testutil.ToFloat64(JobEventsTotal.WithLabelValues("task.start", "running"))
	if jobAfter != jobBefore+1 {
		t.Fatalf("expected job event counter increment, before=%v after=%v", jobBefore, jobAfter)
	}

	migrationBefore := testutil.ToFloat64(MigrationRunsTotal.WithLabelValues("success"))
	RecordMigrationRun("success")
	migrationAfter := testutil.ToFloat64(MigrationRunsTotal.WithLabelValues("success"))
	if migrationAfter != migrationBefore+1 {
		t.Fatalf("expected migration run counter increment, before=%v after=%v", migrationBefore, migrationAfter)
	}
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
	r.MustRegister(HTTPRequestsTotal)
	r.MustRegister(HTTPRequestDuration)
	r.MustRegister(RelayRouteDecisionsTotal)
	r.MustRegister(ProviderFailuresTotal)
	r.MustRegister(ProviderRequestDuration)
	r.MustRegister(BillingLifecycleEventsTotal)
	r.MustRegister(QuotaSettlementFailuresTotal)
	r.MustRegister(StripeWebhookEventsTotal)
	r.MustRegister(StripeWebhookFailuresTotal)
	r.MustRegister(MarketplaceSettlementEventsTotal)
	r.MustRegister(JobEventsTotal)
	r.MustRegister(MigrationRunsTotal)
}
