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

func TestRecordRelayRequestAndChannelHealthScore(t *testing.T) {
	before := testutil.ToFloat64(RelayRequestTotal.WithLabelValues("openai", "ch_1", "chat", "success", "miss"))
	RecordRelayRequest("openai", "ch_1", "chat", "success", "miss")
	after := testutil.ToFloat64(RelayRequestTotal.WithLabelValues("openai", "ch_1", "chat", "success", "miss"))
	if after != before+1 {
		t.Fatalf("expected relay request counter to increment by 1, before=%v after=%v", before, after)
	}

	SetRelayChannelHealthScore("ch_1", 0.72)
	if got := testutil.ToFloat64(RelayChannelHealthScore.WithLabelValues("ch_1")); got != 0.72 {
		t.Fatalf("expected relay channel health score 0.72, got %v", got)
	}

	SetRelayChannelHealthScore("ch_1", 2)
	if got := testutil.ToFloat64(RelayChannelHealthScore.WithLabelValues("ch_1")); got != 1 {
		t.Fatalf("expected relay channel health score to clamp to 1, got %v", got)
	}
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

func TestRecordRelaySemanticCacheMetrics(t *testing.T) {
	model := "gpt-4o-cache-metrics-test"
	beforeHit := testutil.ToFloat64(RelaySemanticCacheEventsTotal.WithLabelValues("hit", "chat", model))
	beforeMiss := testutil.ToFloat64(RelaySemanticCacheEventsTotal.WithLabelValues("miss", "chat", model))

	RecordRelaySemanticCacheLookup("chat", model, true)
	RecordRelaySemanticCacheLookup("chat", model, false)
	RecordRelaySemanticCacheLookup("chat", model, true)

	afterHit := testutil.ToFloat64(RelaySemanticCacheEventsTotal.WithLabelValues("hit", "chat", model))
	afterMiss := testutil.ToFloat64(RelaySemanticCacheEventsTotal.WithLabelValues("miss", "chat", model))
	if afterHit != beforeHit+2 {
		t.Fatalf("expected cache hit counter +2, before=%v after=%v", beforeHit, afterHit)
	}
	if afterMiss != beforeMiss+1 {
		t.Fatalf("expected cache miss counter +1, before=%v after=%v", beforeMiss, afterMiss)
	}

	hitRate := testutil.ToFloat64(RelayCacheHitRate.WithLabelValues("chat", model))
	if hitRate < 0.66 || hitRate > 0.67 {
		t.Fatalf("expected cache hit rate around 2/3, got %v", hitRate)
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

func TestFusionObservabilityMetricsRecordWorkflowRAGAndAgentSignals(t *testing.T) {
	workflowBefore := testutil.ToFloat64(WorkflowExecutionTotal.WithLabelValues("succeeded"))
	RecordWorkflowExecution("succeeded", 1.25)
	workflowAfter := testutil.ToFloat64(WorkflowExecutionTotal.WithLabelValues("succeeded"))
	if workflowAfter != workflowBefore+1 {
		t.Fatalf("expected workflow execution counter increment, before=%v after=%v", workflowBefore, workflowAfter)
	}
	if count := testutil.CollectAndCount(WorkflowExecutionDurationSeconds, "workflow_execution_duration_seconds"); count == 0 {
		t.Fatal("expected workflow execution duration histogram to be collectable")
	}

	SetWorkflowNodeErrorRate("agent", 0.25)
	if got := testutil.ToFloat64(WorkflowNodeErrorRate.WithLabelValues("agent")); got != 0.25 {
		t.Fatalf("expected workflow node error rate 0.25, got %v", got)
	}

	ObserveRAGRetrievalLatency("hybrid", 0.75)
	if count := testutil.CollectAndCount(RAGRetrievalLatencySeconds, "rag_retrieval_latency_seconds"); count == 0 {
		t.Fatal("expected RAG retrieval latency histogram to be collectable")
	}
	ObserveRAGDocumentProcessingDuration("template_based", 1.5)
	if count := testutil.CollectAndCount(RAGDocumentProcessingDurationSeconds, "rag_document_processing_duration_seconds"); count == 0 {
		t.Fatal("expected RAG document processing duration histogram to be collectable")
	}
	SetRAGChunkCount(12)
	if got := testutil.ToFloat64(RAGChunkCount); got != 12 {
		t.Fatalf("expected RAG chunk count 12, got %v", got)
	}

	agentBefore := testutil.ToFloat64(AgentRunTotal.WithLabelValues("completed"))
	RecordAgentRun("completed")
	agentAfter := testutil.ToFloat64(AgentRunTotal.WithLabelValues("completed"))
	if agentAfter != agentBefore+1 {
		t.Fatalf("expected agent run counter increment, before=%v after=%v", agentBefore, agentAfter)
	}

	toolBefore := testutil.ToFloat64(AgentToolCallTotal.WithLabelValues("datetime", "completed"))
	RecordAgentToolCall("datetime", "completed")
	toolAfter := testutil.ToFloat64(AgentToolCallTotal.WithLabelValues("datetime", "completed"))
	if toolAfter != toolBefore+1 {
		t.Fatalf("expected agent tool call counter increment, before=%v after=%v", toolBefore, toolAfter)
	}

	ObserveAgentIterationCount("completed", 2)
	if count := testutil.CollectAndCount(AgentIterationCount, "agent_iteration_count"); count == 0 {
		t.Fatal("expected agent iteration count histogram to be collectable")
	}
}

func TestSetWorkflowExecutionActiveHealthMetricsResetsStaleStatuses(t *testing.T) {
	t.Cleanup(func() {
		SetWorkflowExecutionActiveHealth(nil)
	})

	SetWorkflowExecutionActiveHealth(map[string]WorkflowExecutionActiveHealth{
		"running": {Count: 2, OldestAgeSeconds: 90},
		"queued":  {Count: 1, OldestAgeSeconds: 30},
	})

	if got := testutil.ToFloat64(WorkflowExecutionActive.WithLabelValues("running")); got != 2 {
		t.Fatalf("expected running active count 2, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActiveAgeSeconds.WithLabelValues("running")); got != 90 {
		t.Fatalf("expected running active age 90, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActive.WithLabelValues("queued")); got != 1 {
		t.Fatalf("expected queued active count 1, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActive.WithLabelValues("paused")); got != 0 {
		t.Fatalf("expected paused active count to reset to 0, got %v", got)
	}

	SetWorkflowExecutionActiveHealth(map[string]WorkflowExecutionActiveHealth{
		"paused": {Count: 3, OldestAgeSeconds: 180},
	})

	if got := testutil.ToFloat64(WorkflowExecutionActive.WithLabelValues("running")); got != 0 {
		t.Fatalf("expected running active count to reset to 0, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActiveAgeSeconds.WithLabelValues("running")); got != 0 {
		t.Fatalf("expected running active age to reset to 0, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActive.WithLabelValues("paused")); got != 3 {
		t.Fatalf("expected paused active count 3, got %v", got)
	}
	if got := testutil.ToFloat64(WorkflowExecutionActiveAgeSeconds.WithLabelValues("paused")); got != 180 {
		t.Fatalf("expected paused active age 180, got %v", got)
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
	r.MustRegister(RelayRequestTotal)
	r.MustRegister(RelayChannelHealthScore)
	r.MustRegister(RateLimitExceeded)
	r.MustRegister(HTTPRequestsTotal)
	r.MustRegister(HTTPRequestDuration)
	r.MustRegister(RelayRouteDecisionsTotal)
	r.MustRegister(ProviderFailuresTotal)
	r.MustRegister(ProviderRequestDuration)
	r.MustRegister(RelaySemanticCacheEventsTotal)
	r.MustRegister(RelayCacheHitRate)
	r.MustRegister(BillingLifecycleEventsTotal)
	r.MustRegister(QuotaSettlementFailuresTotal)
	r.MustRegister(StripeWebhookEventsTotal)
	r.MustRegister(StripeWebhookFailuresTotal)
	r.MustRegister(MarketplaceSettlementEventsTotal)
	r.MustRegister(JobEventsTotal)
	r.MustRegister(MigrationRunsTotal)
	r.MustRegister(WorkflowExecutionTotal)
	r.MustRegister(WorkflowExecutionDurationSeconds)
	r.MustRegister(WorkflowExecutionActive)
	r.MustRegister(WorkflowExecutionActiveAgeSeconds)
	r.MustRegister(WorkflowNodeErrorRate)
	r.MustRegister(RAGDocumentProcessingDurationSeconds)
	r.MustRegister(RAGRetrievalLatencySeconds)
	r.MustRegister(RAGChunkCount)
	r.MustRegister(AgentRunTotal)
	r.MustRegister(AgentToolCallTotal)
	r.MustRegister(AgentIterationCount)
}
