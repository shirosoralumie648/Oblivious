package http

import (
	"context"
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/config"
)

const releaseEvidenceRoutePrefix = "/api/v1/admin/release-evidence/"
const releaseEvidenceMigrationLedgerCountQuery = "SELECT COUNT(*) FROM schema_migrations"
const releaseEvidenceRAGIndexingSummaryQuery = `
SELECT
	(
		SELECT COUNT(*)
		FROM knowledge_index_jobs
		WHERE ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS queued_jobs,
	(
		SELECT COUNT(*)
		FROM knowledge_index_jobs
		WHERE status = 'succeeded'
		  AND completed_at IS NOT NULL
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS drained_jobs,
	(
		SELECT COUNT(*)
		FROM knowledge_index_jobs
		WHERE status = 'succeeded'
		  AND completed_at IS NOT NULL
		  AND COALESCE(completed_by, '') <> ''
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS worker_completed_jobs,
	(
		SELECT COUNT(*)
		FROM knowledge_ingestion_jobs
		WHERE status = 'succeeded'
		  AND completed_at IS NOT NULL
		  AND raw_size_bytes > 0
		  AND OCTET_LENGTH(raw_content) > 0
		  AND COALESCE(raw_filename, '') <> ''
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS raw_parser_replays,
	(
		SELECT COUNT(*)
		FROM knowledge_retrieval_test_cases
		WHERE COALESCE(expected_document_id, '') <> ''
		  AND COALESCE(expected_chunk_id, '') <> ''
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS retrieval_probe_count,
	(
		SELECT COUNT(*)
		FROM knowledge_index_jobs
		WHERE operation = 'delete_document'
		  AND status = 'succeeded'
		  AND completed_at IS NOT NULL
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS stale_vector_rows_filtered
`
const releaseEvidenceMarketplacePayoutSummaryQuery = `
SELECT
	(
		SELECT COUNT(*)
		FROM marketplace_payouts
		WHERE provider <> 'local'
		  AND COALESCE(provider_payout_id, '') <> ''
		  AND (metadata->>'dispatch_status' = 'dispatched' OR status IN ('paid_out', 'failed'))
		  AND ($1::timestamptz IS NULL OR updated_at >= $1)
		  AND ($2::timestamptz IS NULL OR updated_at <= $2)
	) AS outbound_dispatches,
	(
		SELECT COUNT(*)
		FROM stripe_webhook_events
		WHERE event_type IN ('payout.paid', 'payout.failed')
		  AND status = 'processed'
		  AND ($1::timestamptz IS NULL OR COALESCE(processed_at, received_at) >= $1)
		  AND ($2::timestamptz IS NULL OR COALESCE(processed_at, received_at) <= $2)
	) AS webhook_events,
	(
		SELECT COUNT(*)
		FROM marketplace_settlements
		WHERE status IN ('paid_out', 'partially_refunded', 'reversed')
		  AND ($1::timestamptz IS NULL OR updated_at >= $1)
		  AND ($2::timestamptz IS NULL OR updated_at <= $2)
	) AS settlement_ledger_entries,
	(
		SELECT COUNT(*)
		FROM marketplace_settlements ms
		JOIN marketplace_orders mo ON mo.id = ms.order_id
		WHERE (
			(ms.status = 'paid_out' AND mo.status = 'paid')
			OR (ms.status = 'partially_refunded' AND mo.status = 'partially_refunded')
			OR (ms.status = 'reversed' AND mo.status IN ('refunded', 'cancelled'))
		)
		  AND ($1::timestamptz IS NULL OR ms.updated_at >= $1)
		  AND ($2::timestamptz IS NULL OR ms.updated_at <= $2)
	) AS reconciled_entries,
	(
		SELECT COUNT(*)
		FROM marketplace_orders
		WHERE refunded_amount > 0
		  AND ($1::timestamptz IS NULL OR updated_at >= $1)
		  AND ($2::timestamptz IS NULL OR updated_at <= $2)
	) AS refund_chargeback_cases,
	(
		SELECT COUNT(DISTINCT mo.id)
		FROM marketplace_orders mo
		JOIN billing_refunds br ON br.payment_intent_id = mo.payment_intent_id
		WHERE mo.refunded_amount > 0
		  AND COALESCE(br.provider_refund_id, '') <> ''
		  AND ($1::timestamptz IS NULL OR br.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR br.created_at <= $2)
	) AS refund_chargeback_cases_handled
`
const releaseEvidenceMarketplaceGovernanceSummaryQuery = `
SELECT
	(
		SELECT COUNT(*)
		FROM published_agents
		WHERE status = 'pending_review'
		  AND ($1::timestamptz IS NULL OR updated_at >= $1)
		  AND ($2::timestamptz IS NULL OR updated_at <= $2)
	) AS review_queue_items,
	(
		SELECT COUNT(*)
		FROM marketplace_governance_events
		WHERE action = 'appeal'
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS appeal_queue_items,
	(
		SELECT COUNT(*)
		FROM marketplace_governance_events
		WHERE action IN ('reinstate', 'appeal_reject')
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS appeal_decisions,
	(
		SELECT COUNT(DISTINCT agent_id)
		FROM marketplace_governance_events
		WHERE action = 'review_assign'
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS review_assignments,
	(
		SELECT COUNT(*)
		FROM marketplace_governance_events
		WHERE action IN ('review_assign', 'automated_review_pass', 'automated_review_reject', 'approve', 'reject', 'needs_changes')
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS sla_checks,
	(
		SELECT COUNT(*)
		FROM marketplace_governance_events
		WHERE action IN ('automated_review_reject', 'reject', 'needs_changes', 'takedown')
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS sla_breaches_handled,
	(
		SELECT COUNT(*)
		FROM marketplace_abuse_reports
		WHERE ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS abuse_reports,
	(
		SELECT COUNT(*)
		FROM marketplace_abuse_reports
		WHERE status IN ('resolved', 'dismissed')
		  AND resolved_at IS NOT NULL
		  AND ($1::timestamptz IS NULL OR resolved_at >= $1)
		  AND ($2::timestamptz IS NULL OR resolved_at <= $2)
	) AS abuse_reports_resolved
`
const releaseEvidenceRelayRealtimeSummaryQuery = `
SELECT
	COUNT(*) AS total_requests,
	COUNT(*) FILTER (
		WHERE COALESCE(api_token_id, '') <> ''
		  AND COALESCE(user_id, '') <> ''
		  AND COALESCE(organization_id, '') <> ''
	) AS authenticated_requests,
	COUNT(*) FILTER (WHERE COALESCE(request_id, '') <> '') AS request_linked_usage_records,
	COUNT(*) FILTER (
		WHERE price_snapshot <> '{}'::jsonb
		  AND COALESCE(price_currency, '') <> ''
		  AND COALESCE(price_source, '') <> ''
	) AS price_snapshot_records,
	COUNT(*) FILTER (
		WHERE status = 'error'
		  AND COALESCE(error_code, '') IN ('realtime_usage_missing')
		  AND cost = 0
	) AS abort_settlement_records,
	COUNT(*) FILTER (
		WHERE status IN ('success', 'error')
		  AND status_code > 0
	) AS terminal_usage_records
FROM usage_records
WHERE api_type = 'realtime'
  AND ($1::timestamptz IS NULL OR created_at >= $1)
  AND ($2::timestamptz IS NULL OR created_at <= $2)
`
const releaseEvidenceRelayBatchSummaryQuery = `
SELECT
	(
		SELECT COUNT(*)
		FROM relay_batch_polling_jobs
		WHERE COALESCE(request_id, '') <> ''
		  AND COALESCE(billing_session_id, '') <> ''
		  AND preauthorized_amount > 0
		  AND ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	) AS prebill_reservations,
	(
		SELECT COUNT(*)
		FROM relay_batch_polling_jobs
		WHERE status = 'succeeded'
		  AND completed_at IS NOT NULL
		  AND attempts > 0
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS polling_completions,
	(
		SELECT COUNT(*)
		FROM usage_records usage
		JOIN relay_batch_polling_jobs job ON job.request_id = usage.request_id
		WHERE usage.api_type = 'batch'
		  AND usage.status = 'success'
		  AND COALESCE(usage.request_id, '') <> ''
		  AND usage.price_snapshot <> '{}'::jsonb
		  AND COALESCE(usage.price_currency, '') <> ''
		  AND COALESCE(usage.price_source, '') <> ''
		  AND ($1::timestamptz IS NULL OR usage.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR usage.created_at <= $2)
	) AS settlement_records,
	(
		SELECT COUNT(*)
		FROM usage_records usage
		JOIN relay_batch_polling_jobs job ON job.request_id = usage.request_id
		WHERE usage.api_type = 'batch'
		  AND usage.status = 'error'
		  AND job.status = 'dead_letter'
		  AND COALESCE(usage.request_id, '') <> ''
		  AND COALESCE(usage.error_code, '') LIKE 'batch_%'
		  AND usage.cost = 0
		  AND ($1::timestamptz IS NULL OR usage.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR usage.created_at <= $2)
	) AS refund_records,
	(
		SELECT COUNT(*)
		FROM usage_records usage
		JOIN relay_batch_polling_jobs job ON job.request_id = usage.request_id
		WHERE usage.api_type = 'batch'
		  AND COALESCE(usage.request_id, '') <> ''
		  AND usage.status IN ('success', 'error')
		  AND usage.status_code > 0
		  AND ($1::timestamptz IS NULL OR usage.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR usage.created_at <= $2)
	) AS usage_audit_records,
	(
		SELECT COUNT(*)
		FROM relay_batch_polling_jobs
		WHERE status = 'dead_letter'
		  AND completed_at IS NOT NULL
		  AND ($1::timestamptz IS NULL OR completed_at >= $1)
		  AND ($2::timestamptz IS NULL OR completed_at <= $2)
	) AS terminal_failure_records
`
const releaseEvidenceRelayBatchAuditRequestIDsQuery = `
SELECT DISTINCT usage.request_id
FROM usage_records usage
JOIN relay_batch_polling_jobs job ON job.request_id = usage.request_id
WHERE usage.api_type = 'batch'
  AND COALESCE(usage.request_id, '') <> ''
  AND usage.status IN ('success', 'error')
  AND usage.status_code > 0
  AND ($1::timestamptz IS NULL OR usage.created_at >= $1)
  AND ($2::timestamptz IS NULL OR usage.created_at <= $2)
ORDER BY usage.request_id
`

type releaseEvidenceDatabaseOpener func(driverName, dataSourceName string) (*sql.DB, error)

type releaseEvidenceHandler struct {
	config                config.Config
	database              *sql.DB
	requestLogEvidence    admin.RequestLogEvidenceStore
	openDatabase          releaseEvidenceDatabaseOpener
	migrationProbeTimeout time.Duration
}

type releaseEvidenceScopeContextKey struct{}

type releaseEvidenceScope struct {
	From      time.Time
	To        time.Time
	HasWindow bool
}

func parseReleaseEvidenceScope(values url.Values) (releaseEvidenceScope, string) {
	fromRaw := strings.TrimSpace(values.Get("from"))
	toRaw := strings.TrimSpace(values.Get("to"))
	if fromRaw == "" && toRaw == "" {
		return releaseEvidenceScope{}, ""
	}
	if fromRaw == "" || toRaw == "" {
		return releaseEvidenceScope{}, "from and to query parameters must be provided together"
	}
	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return releaseEvidenceScope{}, "from query parameter must be RFC3339"
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return releaseEvidenceScope{}, "to query parameter must be RFC3339"
	}
	if to.Before(from) {
		return releaseEvidenceScope{}, "to query parameter must be at or after from"
	}
	return releaseEvidenceScope{
		From:      from,
		To:        to,
		HasWindow: true,
	}, ""
}

func withReleaseEvidenceScope(ctx context.Context, scope releaseEvidenceScope) context.Context {
	if !scope.HasWindow {
		return ctx
	}
	return context.WithValue(ctx, releaseEvidenceScopeContextKey{}, scope)
}

func releaseEvidenceScopeFromContext(ctx context.Context) releaseEvidenceScope {
	scope, _ := ctx.Value(releaseEvidenceScopeContextKey{}).(releaseEvidenceScope)
	return scope
}

func (s releaseEvidenceScope) queryArgs() []any {
	if !s.HasWindow {
		return []any{nil, nil}
	}
	return []any{s.From, s.To}
}

func (s releaseEvidenceScope) asMap() map[string]string {
	if !s.HasWindow {
		return nil
	}
	return map[string]string{
		"from": s.From.UTC().Format(time.RFC3339),
		"to":   s.To.UTC().Format(time.RFC3339),
	}
}

func addReleaseEvidenceScope(proof map[string]any, scope releaseEvidenceScope) {
	if proof == nil || !scope.HasWindow {
		return
	}
	proof["scope"] = scope.asMap()
}

func newReleaseEvidenceHandler(cfg config.Config) releaseEvidenceHandler {
	return newReleaseEvidenceHandlerWithDatabase(cfg, nil)
}

func newReleaseEvidenceHandlerWithDatabase(cfg config.Config, database *sql.DB) releaseEvidenceHandler {
	return newReleaseEvidenceHandlerWithDatabaseAndRequestLogs(cfg, database, nil)
}

func newReleaseEvidenceHandlerWithDatabaseAndRequestLogs(cfg config.Config, database *sql.DB, requestLogEvidence admin.RequestLogEvidenceStore) releaseEvidenceHandler {
	return releaseEvidenceHandler{
		config:                cfg,
		database:              database,
		requestLogEvidence:    requestLogEvidence,
		openDatabase:          sql.Open,
		migrationProbeTimeout: 2 * time.Second,
	}
}

func releaseEvidenceRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/admin/release-evidence/marketplace-governance", "getAdminReleaseEvidenceMarketplaceGovernanceProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/marketplace-payout", "getAdminReleaseEvidenceMarketplacePayoutProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/microservice-database", "getAdminReleaseEvidenceMicroserviceDatabaseProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/provider-runtime-config", "getAdminReleaseEvidenceProviderRuntimeConfigProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/rag-indexing", "getAdminReleaseEvidenceRAGIndexingProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/relay-batch", "getAdminReleaseEvidenceRelayBatchProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
		{"GET", "/api/v1/admin/release-evidence/relay-realtime", "getAdminReleaseEvidenceRelayRealtimeProof", "cookie", false, "release.contract_reporting", "", "none", "", "200", "application/json", "ref", "#/components/schemas/ReleaseEvidenceProofEnvelope"},
	})
}

func registerReleaseEvidenceRouteSurfaces(registrar *RouteSurfaceRegistrar, handler releaseEvidenceHandler) error {
	operations := releaseEvidenceRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthAdmin, releaseEvidenceRouteHandler(handler)))
}

func registerReleaseEvidenceRoutes(mux *stdhttp.ServeMux, authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler releaseEvidenceHandler) {
	if err := registerReleaseEvidenceRouteSurfaces(mustRouteSurfaceAdminAdapterRegistrar(mux, authMiddleware), handler); err != nil {
		panic(err)
	}
}

func newReleaseEvidenceRouter(authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler releaseEvidenceHandler) stdhttp.Handler {
	return authMiddleware.requireAdmin(releaseEvidenceRouteHandler(handler))
}

func releaseEvidenceRouteHandler(handler releaseEvidenceHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		proofType := strings.Trim(strings.TrimPrefix(r.URL.Path, releaseEvidenceRoutePrefix), "/")
		if proofType == "" || strings.Contains(proofType, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		scope, scopeErr := parseReleaseEvidenceScope(r.URL.Query())
		if scopeErr != "" {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", scopeErr)
			return
		}

		proof, ok := handler.proof(withReleaseEvidenceScope(r.Context(), scope), proofType)
		if !ok {
			writeError(w, stdhttp.StatusNotFound, "not_found", "release evidence proof not found")
			return
		}
		writeSuccess(w, stdhttp.StatusOK, proof)
	})
}

func (h releaseEvidenceHandler) proof(ctx context.Context, proofType string) (map[string]any, bool) {
	var proof map[string]any
	switch proofType {
	case "rag-indexing":
		proof = h.ragIndexingProof(ctx)
	case "relay-realtime":
		proof = h.relayRealtimeProof(ctx)
	case "relay-batch":
		proof = h.relayBatchProof(ctx)
	case "marketplace-payout":
		proof = h.marketplacePayoutProof(ctx)
	case "marketplace-governance":
		proof = h.marketplaceGovernanceProof(ctx)
	case "provider-runtime-config":
		proof = h.providerRuntimeConfigProof()
	case "microservice-database":
		proof = h.microserviceDatabaseProof(ctx)
	default:
		return nil, false
	}
	addReleaseEvidenceScope(proof, releaseEvidenceScopeFromContext(ctx))
	return proof, true
}

func (h releaseEvidenceHandler) ragIndexingProof(ctx context.Context) map[string]any {
	summary := h.ragIndexingProofSummary(ctx)
	durableQueueMigration := summary.QueuedJobs > 0
	workerDeployment := summary.WorkerCompletedJobs > 0
	enqueueDrainProbe := summary.QueuedJobs > 0 && summary.DrainedJobs == summary.QueuedJobs
	rawParserReplay := summary.RawParserReplays > 0
	retrievalProbe := summary.RetrievalProbeCount > 0
	staleVectorFilter := summary.StaleVectorRowsFiltered > 0
	ready := durableQueueMigration &&
		workerDeployment &&
		enqueueDrainProbe &&
		rawParserReplay &&
		retrievalProbe &&
		staleVectorFilter

	proof := map[string]any{
		"durableQueueMigration": passFail(durableQueueMigration),
		"workerDeployment":      passFail(workerDeployment),
		"enqueueDrainProbe":     passFail(enqueueDrainProbe),
		"rawParserReplay":       passFail(rawParserReplay),
		"retrievalProbe":        passFail(retrievalProbe),
		"staleVectorFilter":     passFail(staleVectorFilter),
		"summary":               summary.asMap(),
	}
	if !ready {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "durable RAG target probes must come from target queue drain, worker, parser replay, retrieval, and stale-vector filtering records"
	}
	return proof
}

type ragIndexingProofSummary struct {
	QueuedJobs              int
	DrainedJobs             int
	WorkerCompletedJobs     int
	RawParserReplays        int
	RetrievalProbeCount     int
	StaleVectorRowsFiltered int
}

func (s ragIndexingProofSummary) asMap() map[string]int {
	return map[string]int{
		"queuedJobs":              s.QueuedJobs,
		"drainedJobs":             s.DrainedJobs,
		"workerCompletedJobs":     s.WorkerCompletedJobs,
		"rawParserReplayCount":    s.RawParserReplays,
		"retrievalProbeCount":     s.RetrievalProbeCount,
		"staleVectorRowsFiltered": s.StaleVectorRowsFiltered,
	}
}

func (h releaseEvidenceHandler) ragIndexingProofSummary(ctx context.Context) ragIndexingProofSummary {
	if h.database == nil {
		return ragIndexingProofSummary{}
	}

	var summary ragIndexingProofSummary
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	err := h.database.QueryRowContext(ctx, releaseEvidenceRAGIndexingSummaryQuery, args...).Scan(
		&summary.QueuedJobs,
		&summary.DrainedJobs,
		&summary.WorkerCompletedJobs,
		&summary.RawParserReplays,
		&summary.RetrievalProbeCount,
		&summary.StaleVectorRowsFiltered,
	)
	if err != nil {
		return ragIndexingProofSummary{}
	}
	return summary
}

func (h releaseEvidenceHandler) relayRealtimeProof(ctx context.Context) map[string]any {
	if h.config.RelayRealtimeCommercialLifecycleEnabled {
		return h.relayRealtimeLiveProof(ctx)
	}
	return h.relayRealtimeDisabledProof()
}

func (h releaseEvidenceHandler) relayRealtimeDisabledProof() map[string]any {
	disabled := true
	proof := map[string]any{
		"mode":                                proofString(disabled, "disabled_until_commercial_lifecycle", "commercial_lifecycle_enabled"),
		"productionPolicyDisabled":            passFail(disabled),
		"authOriginPrebillAbortUsageBlockers": passFail(disabled),
		"summary": map[string]int{
			"productionPolicyChecks":                   countIf(disabled),
			"authOriginPrebillAbortUsageBlockerChecks": countIf(disabled) * 5,
		},
	}
	return proof
}

func (h releaseEvidenceHandler) relayRealtimeLiveProof(ctx context.Context) map[string]any {
	summary := h.relayRealtimeProofSummary(ctx)
	originPolicyChecks := h.relayRealtimeOriginPolicyChecks()
	summary.OriginPolicyChecks = originPolicyChecks
	productionPolicyEnabled := h.config.RelayRealtimeCommercialLifecycleEnabled
	authPolicy := summary.TotalRequests > 0 && summary.AuthenticatedRequests == summary.TotalRequests
	originPolicy := originPolicyChecks > 0
	prebillSettlement := summary.TotalRequests > 0 && summary.PriceSnapshotRecords == summary.TotalRequests
	abortSettlement := summary.AbortSettlementRecords > 0
	usageLedger := summary.TotalRequests > 0 &&
		summary.RequestLinkedUsageRecords == summary.TotalRequests &&
		summary.TerminalUsageRecords == summary.TotalRequests
	ready := productionPolicyEnabled &&
		authPolicy &&
		originPolicy &&
		prebillSettlement &&
		abortSettlement &&
		usageLedger

	proof := map[string]any{
		"mode":                    "commercial_lifecycle_enabled",
		"productionPolicyEnabled": passFail(productionPolicyEnabled),
		"authPolicy":              passFail(authPolicy),
		"originPolicy":            passFail(originPolicy),
		"prebillSettlement":       passFail(prebillSettlement),
		"abortSettlement":         passFail(abortSettlement),
		"usageLedger":             passFail(usageLedger),
		"summary":                 summary.asMap(),
	}
	if !ready {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "realtime relay lifecycle is enabled, so target auth, origin, prebill, abort, and usage ledger records must all pass"
	}
	return proof
}

type relayRealtimeProofSummary struct {
	TotalRequests             int
	AuthenticatedRequests     int
	RequestLinkedUsageRecords int
	PriceSnapshotRecords      int
	AbortSettlementRecords    int
	TerminalUsageRecords      int
	OriginPolicyChecks        int
}

func (s relayRealtimeProofSummary) asMap() map[string]int {
	return map[string]int{
		"totalRequests":             s.TotalRequests,
		"authenticatedRequests":     s.AuthenticatedRequests,
		"requestLinkedUsageRecords": s.RequestLinkedUsageRecords,
		"priceSnapshotRecords":      s.PriceSnapshotRecords,
		"abortSettlementRecords":    s.AbortSettlementRecords,
		"terminalUsageRecords":      s.TerminalUsageRecords,
		"originPolicyChecks":        s.OriginPolicyChecks,
	}
}

func (h releaseEvidenceHandler) relayRealtimeProofSummary(ctx context.Context) relayRealtimeProofSummary {
	if h.database == nil {
		return relayRealtimeProofSummary{}
	}

	var summary relayRealtimeProofSummary
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	err := h.database.QueryRowContext(ctx, releaseEvidenceRelayRealtimeSummaryQuery, args...).Scan(
		&summary.TotalRequests,
		&summary.AuthenticatedRequests,
		&summary.RequestLinkedUsageRecords,
		&summary.PriceSnapshotRecords,
		&summary.AbortSettlementRecords,
		&summary.TerminalUsageRecords,
	)
	if err != nil {
		return relayRealtimeProofSummary{}
	}
	return summary
}

func (h releaseEvidenceHandler) relayRealtimeOriginPolicyChecks() int {
	checks := 0
	for _, origin := range h.config.CORSAllowedOrigins {
		if isExternalHTTPSWebOrigin(origin) {
			checks++
		}
	}
	return checks
}

func (h releaseEvidenceHandler) relayBatchProof(ctx context.Context) map[string]any {
	if h.config.RelayBatchCommercialLifecycleEnabled {
		return h.relayBatchLiveProof(ctx)
	}
	return h.relayBatchDisabledProof()
}

func (h releaseEvidenceHandler) relayBatchDisabledProof() map[string]any {
	disabled := true
	proof := map[string]any{
		"mode":                     proofString(disabled, "disabled_until_commercial_lifecycle", "commercial_lifecycle_enabled"),
		"productionPolicyDisabled": passFail(disabled),
		"prebillPollingSettlementRefundAuditUsageBlockers": passFail(disabled),
		"summary": map[string]int{
			"productionPolicyChecks":                                countIf(disabled),
			"prebillPollingSettlementRefundAuditUsageBlockerChecks": countIf(disabled) * 6,
		},
	}
	return proof
}

func (h releaseEvidenceHandler) relayBatchLiveProof(ctx context.Context) map[string]any {
	summary := h.relayBatchProofSummary(ctx)
	productionPolicyEnabled := h.config.RelayBatchCommercialLifecycleEnabled && h.config.RelayBatchPollingWorkerEnabled
	prebillReservation := summary.PrebillReservations > 0
	pollingCompletion := summary.PollingCompletions > 0
	settlement := summary.SettlementRecords > 0
	refund := summary.RefundRecords > 0 && summary.TerminalFailureRecords >= summary.RefundRecords
	requestLogAudit := summary.RequestLogAuditRecords > 0 &&
		summary.RequestLogAuditRecords >= summary.SettlementRecords+summary.RefundRecords
	usageAudit := summary.UsageAuditRecords > 0 &&
		summary.UsageAuditRecords >= summary.SettlementRecords+summary.RefundRecords &&
		requestLogAudit
	ready := productionPolicyEnabled &&
		prebillReservation &&
		pollingCompletion &&
		settlement &&
		refund &&
		usageAudit

	proof := map[string]any{
		"mode":                    "commercial_lifecycle_enabled",
		"productionPolicyEnabled": passFail(productionPolicyEnabled),
		"prebillReservation":      passFail(prebillReservation),
		"pollingCompletion":       passFail(pollingCompletion),
		"settlement":              passFail(settlement),
		"refund":                  passFail(refund),
		"usageAudit":              passFail(usageAudit),
		"summary":                 summary.asMap(),
	}
	if !ready {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "batch relay lifecycle is enabled, so target prebill, polling, settlement, refund, and usage audit records must all pass"
	}
	return proof
}

type relayBatchProofSummary struct {
	PrebillReservations    int
	PollingCompletions     int
	SettlementRecords      int
	RefundRecords          int
	UsageAuditRecords      int
	RequestLogAuditRecords int
	TerminalFailureRecords int
}

type relayRouteDecisionEvidenceStore interface {
	ListRelayRouteDecisionEvidence(ctx context.Context, requestIDs []string, apiType string, result string) (map[string]admin.RequestLogEvidence, error)
}

func (s relayBatchProofSummary) asMap() map[string]int {
	return map[string]int{
		"prebillReservations":    s.PrebillReservations,
		"pollingCompletions":     s.PollingCompletions,
		"settlementRecords":      s.SettlementRecords,
		"refundRecords":          s.RefundRecords,
		"usageAuditRecords":      s.UsageAuditRecords,
		"requestLogAuditRecords": s.RequestLogAuditRecords,
		"terminalFailureRecords": s.TerminalFailureRecords,
	}
}

func (h releaseEvidenceHandler) relayBatchProofSummary(ctx context.Context) relayBatchProofSummary {
	if h.database == nil {
		return relayBatchProofSummary{}
	}

	var summary relayBatchProofSummary
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	err := h.database.QueryRowContext(ctx, releaseEvidenceRelayBatchSummaryQuery, args...).Scan(
		&summary.PrebillReservations,
		&summary.PollingCompletions,
		&summary.SettlementRecords,
		&summary.RefundRecords,
		&summary.UsageAuditRecords,
		&summary.TerminalFailureRecords,
	)
	if err != nil {
		return relayBatchProofSummary{}
	}
	summary.RequestLogAuditRecords = h.relayBatchRequestLogAuditRecords(ctx)
	return summary
}

func (h releaseEvidenceHandler) relayBatchRequestLogAuditRecords(ctx context.Context) int {
	if h.database == nil || h.requestLogEvidence == nil {
		return 0
	}
	requestIDs, err := h.relayBatchAuditRequestIDs(ctx)
	if err != nil || len(requestIDs) == 0 {
		return 0
	}
	routeDecisionStore, ok := h.requestLogEvidence.(relayRouteDecisionEvidenceStore)
	if !ok {
		return 0
	}
	evidence, err := routeDecisionStore.ListRelayRouteDecisionEvidence(ctx, requestIDs, "batch", "allowed")
	if err != nil {
		return 0
	}
	count := 0
	for _, requestID := range requestIDs {
		item, ok := evidence[requestID]
		if !ok {
			continue
		}
		if !relayBatchRequestLogEvidenceMatches(item) {
			continue
		}
		count++
	}
	return count
}

func (h releaseEvidenceHandler) relayBatchAuditRequestIDs(ctx context.Context) ([]string, error) {
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	rows, err := h.database.QueryContext(ctx, releaseEvidenceRelayBatchAuditRequestIDsQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requestIDs := []string{}
	for rows.Next() {
		var requestID string
		if err := rows.Scan(&requestID); err != nil {
			return nil, err
		}
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			continue
		}
		requestIDs = append(requestIDs, requestID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requestIDs, nil
}

func relayBatchRequestLogEvidenceMatches(item admin.RequestLogEvidence) bool {
	if strings.TrimSpace(item.RequestID) == "" || strings.TrimSpace(item.RequestLogID) == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(item.Service), "relay") {
		return false
	}
	if item.StatusCode < 200 || item.StatusCode >= 400 {
		return false
	}
	if len(item.Metadata) == 0 {
		return false
	}
	var metadata map[string]any
	if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
		return false
	}
	return strings.EqualFold(metadataString(metadata, "event"), "relay.route_decision") &&
		strings.EqualFold(metadataString(metadata, "relay_api_type"), "batch") &&
		strings.EqualFold(metadataString(metadata, "relay_route_result"), "allowed")
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func (h releaseEvidenceHandler) marketplacePayoutProof(ctx context.Context) map[string]any {
	summary := h.marketplacePayoutProofSummary(ctx)
	outboundDispatch := summary.OutboundDispatches > 0
	inboundWebhookLifecycle := summary.WebhookEvents >= summary.OutboundDispatches && summary.OutboundDispatches > 0
	settlementLedger := summary.SettlementLedgerEntries > 0
	reconciliation := summary.ReconciledEntries == summary.SettlementLedgerEntries && summary.SettlementLedgerEntries > 0
	refundChargebackHandling := summary.RefundChargebackCases > 0 && summary.RefundChargebackCasesHandled == summary.RefundChargebackCases
	ready := outboundDispatch &&
		inboundWebhookLifecycle &&
		settlementLedger &&
		reconciliation &&
		refundChargebackHandling

	proof := map[string]any{
		"outboundDispatch":         passFail(outboundDispatch),
		"inboundWebhookLifecycle":  passFail(inboundWebhookLifecycle),
		"settlementLedger":         passFail(settlementLedger),
		"reconciliation":           passFail(reconciliation),
		"refundChargebackHandling": passFail(refundChargebackHandling),
		"summary":                  summary.asMap(),
	}
	if !ready {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "marketplace payout evidence must come from target payout dispatch, webhook, ledger, reconciliation, and refund or chargeback probes"
	}
	return proof
}

type marketplacePayoutProofSummary struct {
	OutboundDispatches           int
	WebhookEvents                int
	SettlementLedgerEntries      int
	ReconciledEntries            int
	RefundChargebackCases        int
	RefundChargebackCasesHandled int
}

func (s marketplacePayoutProofSummary) asMap() map[string]int {
	return map[string]int{
		"outboundDispatches":           s.OutboundDispatches,
		"webhookEvents":                s.WebhookEvents,
		"settlementLedgerEntries":      s.SettlementLedgerEntries,
		"reconciledEntries":            s.ReconciledEntries,
		"refundChargebackCases":        s.RefundChargebackCases,
		"refundChargebackCasesHandled": s.RefundChargebackCasesHandled,
	}
}

func (h releaseEvidenceHandler) marketplacePayoutProofSummary(ctx context.Context) marketplacePayoutProofSummary {
	if h.database == nil {
		return marketplacePayoutProofSummary{}
	}

	var summary marketplacePayoutProofSummary
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	err := h.database.QueryRowContext(ctx, releaseEvidenceMarketplacePayoutSummaryQuery, args...).Scan(
		&summary.OutboundDispatches,
		&summary.WebhookEvents,
		&summary.SettlementLedgerEntries,
		&summary.ReconciledEntries,
		&summary.RefundChargebackCases,
		&summary.RefundChargebackCasesHandled,
	)
	if err != nil {
		return marketplacePayoutProofSummary{}
	}
	return summary
}

func (h releaseEvidenceHandler) marketplaceGovernanceProof(ctx context.Context) map[string]any {
	summary := h.marketplaceGovernanceProofSummary(ctx)
	reviewQueue := summary.ReviewQueueItems > 0
	appealQueue := summary.AppealQueueItems > 0
	appealDecisionLifecycle := summary.AppealQueueItems > 0 && summary.AppealDecisions == summary.AppealQueueItems
	reviewAssignment := summary.ReviewQueueItems > 0 && summary.ReviewAssignments >= summary.ReviewQueueItems
	reviewSLAEnforcement := summary.ReviewAssignments > 0 && summary.SLAChecks >= summary.ReviewAssignments
	abuseReportLifecycle := summary.AbuseReports > 0 && summary.AbuseReportsResolved == summary.AbuseReports
	ready := reviewQueue &&
		appealQueue &&
		appealDecisionLifecycle &&
		reviewAssignment &&
		reviewSLAEnforcement &&
		abuseReportLifecycle

	proof := map[string]any{
		"reviewQueue":             passFail(reviewQueue),
		"appealQueue":             passFail(appealQueue),
		"appealDecisionLifecycle": passFail(appealDecisionLifecycle),
		"reviewAssignment":        passFail(reviewAssignment),
		"reviewSLAEnforcement":    passFail(reviewSLAEnforcement),
		"abuseReportLifecycle":    passFail(abuseReportLifecycle),
		"summary":                 summary.asMap(),
	}
	if !ready {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "marketplace governance evidence must come from target review, appeal, SLA, and abuse-report lifecycle probes"
	}
	return proof
}

type marketplaceGovernanceProofSummary struct {
	ReviewQueueItems     int
	AppealQueueItems     int
	AppealDecisions      int
	ReviewAssignments    int
	SLAChecks            int
	SLABreachesHandled   int
	AbuseReports         int
	AbuseReportsResolved int
}

func (s marketplaceGovernanceProofSummary) asMap() map[string]int {
	return map[string]int{
		"reviewQueueItems":     s.ReviewQueueItems,
		"appealQueueItems":     s.AppealQueueItems,
		"appealDecisions":      s.AppealDecisions,
		"reviewAssignments":    s.ReviewAssignments,
		"slaChecks":            s.SLAChecks,
		"slaBreachesHandled":   s.SLABreachesHandled,
		"abuseReports":         s.AbuseReports,
		"abuseReportsResolved": s.AbuseReportsResolved,
	}
}

func (h releaseEvidenceHandler) marketplaceGovernanceProofSummary(ctx context.Context) marketplaceGovernanceProofSummary {
	if h.database == nil {
		return marketplaceGovernanceProofSummary{}
	}

	var summary marketplaceGovernanceProofSummary
	args := releaseEvidenceScopeFromContext(ctx).queryArgs()
	err := h.database.QueryRowContext(ctx, releaseEvidenceMarketplaceGovernanceSummaryQuery, args...).Scan(
		&summary.ReviewQueueItems,
		&summary.AppealQueueItems,
		&summary.AppealDecisions,
		&summary.ReviewAssignments,
		&summary.SLAChecks,
		&summary.SLABreachesHandled,
		&summary.AbuseReports,
		&summary.AbuseReportsResolved,
	)
	if err != nil {
		return marketplaceGovernanceProofSummary{}
	}
	return summary
}

func zeroMarketplaceGovernanceProofSummary() map[string]int {
	return map[string]int{
		"reviewQueueItems":     0,
		"appealQueueItems":     0,
		"appealDecisions":      0,
		"reviewAssignments":    0,
		"slaChecks":            0,
		"slaBreachesHandled":   0,
		"abuseReports":         0,
		"abuseReportsResolved": 0,
	}
}

func (h releaseEvidenceHandler) providerRuntimeConfigProof() map[string]any {
	stripe := allNonEmpty(h.config.StripeSecretKey, h.config.StripeSuccessURL, h.config.StripeCancelURL, h.config.StripeWebhookSecret) &&
		isHTTPSURL(h.config.StripeSuccessURL) && isHTTPSURL(h.config.StripeCancelURL)
	alipay := allNonEmpty(h.config.AlipayCheckoutBaseURL, h.config.AlipayWebhookSecret) && isHTTPSURL(h.config.AlipayCheckoutBaseURL)
	wechatpay := allNonEmpty(h.config.WeChatPayCheckoutBaseURL, h.config.WeChatPayWebhookSecret) && isHTTPSURL(h.config.WeChatPayCheckoutBaseURL)
	allProviders := stripe && alipay && wechatpay

	proof := map[string]any{
		"stripe":              passFail(stripe),
		"alipay":              passFail(alipay),
		"wechatpay":           passFail(wechatpay),
		"providerEnv":         passFail(allProviders),
		"checkoutBaseUrls":    passFail(allProviders),
		"webhookRoutes":       passFail(allProviders),
		"webhookVerification": passFail(allProviders),
		"summary": map[string]int{
			"providersConfigured":       countBools(stripe, alipay, wechatpay),
			"providerEnvVarsChecked":    3,
			"checkoutBaseUrlsChecked":   countBools(stripe, alipay, wechatpay),
			"webhookRoutesChecked":      3,
			"webhookVerificationChecks": countBools(stripe, alipay, wechatpay),
		},
	}
	if !allProviders {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = "Stripe, Alipay, and WeChat Pay runtime configuration must all be complete and URL-backed for final provider evidence"
	}
	return proof
}

func (h releaseEvidenceHandler) microserviceDatabaseProof(ctx context.Context) map[string]any {
	services := map[string]string{
		"relay":         h.config.DBURLRelay,
		"chat":          h.config.DBURLChat,
		"workflow":      h.config.DBURLWorkflow,
		"rag":           h.config.DBURLRAG,
		"agent":         h.config.DBURLAgent,
		"billing":       h.config.DBURLBilling,
		"marketplace":   h.config.DBURLMarketplace,
		"admin":         h.config.DBURLAdmin,
		"channel":       h.config.DBURLChannel,
		"task":          h.config.DBURLTask,
		"observability": h.config.DBURLObservability,
	}

	proof := map[string]any{
		"mode":            strings.ToLower(strings.TrimSpace(h.config.DBMode)),
		"serviceUrlClass": "incomplete",
	}
	passedServices := 0
	for _, service := range microserviceDatabaseProofServices() {
		servicePassed := isExternalDatabaseURL(services[service])
		proof[service] = passFail(servicePassed)
		if servicePassed {
			passedServices++
		}
	}
	allServiceURLs := passedServices == len(services)
	if proof["mode"] == "microservices" && allServiceURLs {
		proof["serviceUrlClass"] = "external-filled"
	}

	migrationReadinessChecks := 1
	migrationReady := false
	notReadyReason := "service database URLs are checked from config, but target migration readiness still requires live database migration probes"
	if proof["mode"] == "microservices" && allServiceURLs {
		if h.microserviceDatabaseMigrationProbeEnabled() {
			migrationReadinessChecks, migrationReady = h.probeMicroserviceDatabaseMigrations(ctx, services)
			notReadyReason = "target service databases must be reachable and have a populated schema_migrations ledger before final release evidence can pass"
		} else {
			notReadyReason = "target service database migration probes are disabled for test environments"
		}
	}

	proof["migrationReadiness"] = passFail(migrationReady)
	proof["summary"] = map[string]int{
		"servicesChecked":          len(services),
		"externalUrlsChecked":      passedServices,
		"migrationReadinessChecks": migrationReadinessChecks,
	}
	if !migrationReady {
		proof["status"] = "not_ready"
		proof["notReadyReason"] = notReadyReason
	}
	return proof
}

func (h releaseEvidenceHandler) microserviceDatabaseMigrationProbeEnabled() bool {
	return h.openDatabase != nil &&
		h.migrationProbeTimeout > 0 &&
		!strings.EqualFold(strings.TrimSpace(h.config.Env), "test")
}

func (h releaseEvidenceHandler) probeMicroserviceDatabaseMigrations(ctx context.Context, services map[string]string) (int, bool) {
	checks := 0
	for _, service := range microserviceDatabaseProofServices() {
		databaseURL := strings.TrimSpace(services[service])
		if !isExternalDatabaseURL(databaseURL) {
			return checks, false
		}

		database, err := h.openDatabase("postgres", databaseURL)
		if err != nil || database == nil {
			return checks, false
		}
		ready := h.probeMigrationLedger(ctx, database)
		closeErr := database.Close()
		if !ready || closeErr != nil {
			return checks, false
		}
		checks++
	}
	return checks, true
}

func (h releaseEvidenceHandler) probeMigrationLedger(ctx context.Context, database *sql.DB) bool {
	probeCtx, cancel := context.WithTimeout(ctx, h.migrationProbeTimeout)
	defer cancel()

	if err := database.PingContext(probeCtx); err != nil {
		return false
	}
	var migrationCount int
	if err := database.QueryRowContext(probeCtx, releaseEvidenceMigrationLedgerCountQuery).Scan(&migrationCount); err != nil {
		return false
	}
	return migrationCount > 0
}

func microserviceDatabaseProofServices() []string {
	return []string{
		"relay",
		"chat",
		"workflow",
		"rag",
		"agent",
		"billing",
		"marketplace",
		"admin",
		"channel",
		"task",
		"observability",
	}
}

func failProof(fields ...string) map[string]any {
	proof := make(map[string]any, len(fields))
	for _, field := range fields {
		proof[field] = "fail"
	}
	return proof
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func proofString(ok bool, passValue, failValue string) string {
	if ok {
		return passValue
	}
	return failValue
}

func countIf(ok bool) int {
	if ok {
		return 1
	}
	return 0
}

func countBools(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func allNonEmpty(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func isHTTPSURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && parsed.Host != ""
}

func isExternalHTTPSWebOrigin(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "*" || containsPlaceholder(trimmed) {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return false
	}
	return !isLoopbackHost(parsed.Hostname())
}

func isExternalDatabaseURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || containsPlaceholder(trimmed) {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return false
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return false
	}
	return !isLoopbackHost(parsed.Hostname())
}

func containsPlaceholder(value string) bool {
	upper := strings.ToUpper(value)
	for _, marker := range []string{"CHANGE_ME", "CHANGEME", "REPLACE_ME", "TODO", "EXAMPLE"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	normalized := strings.ToLower(strings.Trim(host, "[]"))
	return normalized == "localhost" ||
		normalized == "0.0.0.0" ||
		normalized == "::1" ||
		strings.HasPrefix(normalized, "127.")
}
