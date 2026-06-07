package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type relaySemanticCacheMetricKey struct {
	APIType string
	Model   string
}

type relaySemanticCacheMetricCounts struct {
	Hits   float64
	Misses float64
}

type workflowNodeMetricCounts struct {
	Failed float64
	Total  float64
}

type WorkflowExecutionActiveHealth struct {
	Count            int
	OldestAgeSeconds float64
}

var (
	relaySemanticCacheMetricsMu      sync.Mutex
	relaySemanticCacheMetricCountsBy = map[relaySemanticCacheMetricKey]relaySemanticCacheMetricCounts{}
	workflowNodeMetricsMu            sync.Mutex
	workflowNodeMetricCountsBy       = map[string]workflowNodeMetricCounts{}
)

var workflowExecutionActiveStatuses = []string{"running", "queued", "paused"}

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

	RelayRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_request_total",
			Help: "Total number of Relay user-level requests by provider, channel, API type, status, and semantic cache outcome",
		},
		[]string{"provider", "channel_id", "api_type", "status", "cache_status"},
	)

	RelayChannelHealthScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_channel_health_score",
			Help: "Relay channel health score normalized from 0 to 1",
		},
		[]string{"channel_id"},
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

	RelaySemanticCacheEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_semantic_cache_events_total",
			Help: "Total number of Relay semantic cache lookup outcomes",
		},
		[]string{"result", "api_type", "model"},
	)

	RelayCacheHitRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_cache_hit_rate",
			Help: "Cumulative Relay semantic cache hit rate by API type and model",
		},
		[]string{"api_type", "model"},
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

	WorkflowExecutionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_execution_total",
			Help: "Total number of workflow executions by terminal status",
		},
		[]string{"status"},
	)

	WorkflowExecutionDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "workflow_execution_duration_seconds",
			Help:    "Workflow execution duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300, 900, 3600},
		},
		[]string{"status"},
	)

	WorkflowExecutionActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workflow_execution_active",
			Help: "Current active workflow execution count by low-cardinality status",
		},
		[]string{"status"},
	)

	WorkflowExecutionActiveAgeSeconds = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workflow_execution_active_age_seconds",
			Help: "Age in seconds of the oldest active workflow execution by low-cardinality status",
		},
		[]string{"status"},
	)

	WorkflowNodeErrorRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workflow_node_error_rate",
			Help: "Workflow node error rate by low-cardinality node type",
		},
		[]string{"node_type"},
	)

	RAGDocumentProcessingDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rag_document_processing_duration_seconds",
			Help:    "RAG document processing duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
		},
		[]string{"strategy"},
	)

	RAGRetrievalLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rag_retrieval_latency_seconds",
			Help:    "RAG retrieval latency in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
		[]string{"mode"},
	)

	RAGRerankerFallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rag_reranker_fallback_total",
			Help: "Total number of RAG reranker fallbacks by retrieval mode",
		},
		[]string{"mode"},
	)

	RAGChunkCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rag_chunk_count",
			Help: "Most recent RAG document chunk count",
		},
	)

	AgentRunTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_run_total",
			Help: "Total number of agent runs by terminal status",
		},
		[]string{"status"},
	)

	AgentToolCallTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_tool_call_total",
			Help: "Total number of agent tool calls by low-cardinality tool name and status",
		},
		[]string{"tool_name", "status"},
	)

	AgentIterationCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_iteration_count",
			Help:    "Agent run iteration count by terminal status",
			Buckets: []float64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 100},
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

func RecordRelayRequest(provider, channelID, apiType, status, cacheStatus string) {
	RelayRequestTotal.WithLabelValues(
		lowCardinalityOrUnknown(provider),
		lowCardinalityOrUnknown(channelID),
		lowCardinalityOrUnknown(apiType),
		lowCardinalityOrUnknown(status),
		lowCardinalityOrNone(cacheStatus),
	).Inc()
}

func SetRelayChannelHealthScore(channelID string, score float64) {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	RelayChannelHealthScore.WithLabelValues(lowCardinalityOrUnknown(channelID)).Set(score)
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

func RecordRelaySemanticCacheLookup(apiType, model string, hit bool) {
	result := "miss"
	if hit {
		result = "hit"
	}
	apiType = lowCardinalityOrUnknown(apiType)
	model = lowCardinalityOrUnknown(model)
	RelaySemanticCacheEventsTotal.WithLabelValues(result, apiType, model).Inc()

	key := relaySemanticCacheMetricKey{APIType: apiType, Model: model}
	relaySemanticCacheMetricsMu.Lock()
	counts := relaySemanticCacheMetricCountsBy[key]
	if hit {
		counts.Hits++
	} else {
		counts.Misses++
	}
	relaySemanticCacheMetricCountsBy[key] = counts

	total := counts.Hits + counts.Misses
	if total > 0 {
		RelayCacheHitRate.WithLabelValues(apiType, model).Set(counts.Hits / total)
	}
	relaySemanticCacheMetricsMu.Unlock()
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

func RecordWorkflowExecution(status string, seconds float64) {
	status = lowCardinalityOrUnknown(status)
	WorkflowExecutionTotal.WithLabelValues(status).Inc()
	if seconds >= 0 {
		WorkflowExecutionDurationSeconds.WithLabelValues(status).Observe(seconds)
	}
}

func SetWorkflowExecutionActiveHealth(healthByStatus map[string]WorkflowExecutionActiveHealth) {
	for _, status := range workflowExecutionActiveStatuses {
		health := healthByStatus[status]
		count := float64(health.Count)
		if count < 0 {
			count = 0
		}
		age := health.OldestAgeSeconds
		if age < 0 {
			age = 0
		}
		WorkflowExecutionActive.WithLabelValues(status).Set(count)
		WorkflowExecutionActiveAgeSeconds.WithLabelValues(status).Set(age)
	}
}

func SetWorkflowNodeErrorRate(nodeType string, rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	WorkflowNodeErrorRate.WithLabelValues(lowCardinalityOrUnknown(nodeType)).Set(rate)
}

func RecordWorkflowNodeExecutionResult(nodeType string, failed bool) {
	nodeType = lowCardinalityOrUnknown(nodeType)
	workflowNodeMetricsMu.Lock()
	counts := workflowNodeMetricCountsBy[nodeType]
	counts.Total++
	if failed {
		counts.Failed++
	}
	workflowNodeMetricCountsBy[nodeType] = counts

	rate := 0.0
	if counts.Total > 0 {
		rate = counts.Failed / counts.Total
	}
	WorkflowNodeErrorRate.WithLabelValues(nodeType).Set(rate)
	workflowNodeMetricsMu.Unlock()
}

func ObserveRAGDocumentProcessingDuration(strategy string, seconds float64) {
	if seconds < 0 {
		return
	}
	RAGDocumentProcessingDurationSeconds.WithLabelValues(lowCardinalityOrUnknown(strategy)).Observe(seconds)
}

func ObserveRAGRetrievalLatency(mode string, seconds float64) {
	if seconds < 0 {
		return
	}
	RAGRetrievalLatencySeconds.WithLabelValues(lowCardinalityOrUnknown(mode)).Observe(seconds)
}

func RecordRAGRerankerFallback(mode string) {
	RAGRerankerFallbackTotal.WithLabelValues(lowCardinalityOrUnknown(mode)).Inc()
}

func SetRAGChunkCount(count int) {
	if count < 0 {
		count = 0
	}
	RAGChunkCount.Set(float64(count))
}

func RecordAgentRun(status string) {
	AgentRunTotal.WithLabelValues(lowCardinalityOrUnknown(status)).Inc()
}

func RecordAgentToolCall(toolName, status string) {
	AgentToolCallTotal.WithLabelValues(lowCardinalityOrUnknown(toolName), lowCardinalityOrUnknown(status)).Inc()
}

func ObserveAgentIterationCount(status string, iterations int) {
	if iterations < 0 {
		return
	}
	AgentIterationCount.WithLabelValues(lowCardinalityOrUnknown(status)).Observe(float64(iterations))
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
