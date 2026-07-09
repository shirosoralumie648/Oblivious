package http

import (
	"context"
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/config"
)

func TestReleaseEvidenceRoutesRequireAdminWithoutDatabase(t *testing.T) {
	path := "/api/v1/admin/release-evidence/provider-runtime-config"

	anonymousRouter := NewRouterWithOptions(testConfig(), nil, RouterOptions{})
	anonymousRecorder := httptest.NewRecorder()
	anonymousRouter.ServeHTTP(anonymousRecorder, httptest.NewRequest(stdhttp.MethodGet, path, nil))
	if anonymousRecorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected anonymous release evidence route to return 401, got %d with body %s", anonymousRecorder.Code, anonymousRecorder.Body.String())
	}

	userSession := routeSurfaceUserSession()
	userRouter := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: userSession}})
	userRequest := httptest.NewRequest(stdhttp.MethodGet, path, nil)
	userRequest.AddCookie(routeSurfaceSignedSessionCookie(t, userSession))
	userRecorder := httptest.NewRecorder()
	userRouter.ServeHTTP(userRecorder, userRequest)
	if userRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-admin release evidence route to return 403, got %d with body %s", userRecorder.Code, userRecorder.Body.String())
	}
}

func TestReleaseEvidenceProviderRuntimeConfigReflectsConfiguredProviders(t *testing.T) {
	cfg := testConfig()
	cfg.StripeSecretKey = "sk_live_secret_should_not_leak"
	cfg.StripeSuccessURL = "https://app.example.com/billing/success"
	cfg.StripeCancelURL = "https://app.example.com/billing/cancel"
	cfg.StripeWebhookSecret = "whsec_secret_should_not_leak"
	cfg.AlipayCheckoutBaseURL = "https://payments.example.com/alipay"
	cfg.AlipayWebhookSecret = "alipay_secret_should_not_leak"
	cfg.WeChatPayCheckoutBaseURL = "https://payments.example.com/wechatpay"
	cfg.WeChatPayWebhookSecret = "wechat_secret_should_not_leak"

	recorder := releaseEvidenceGET(t, cfg, "/api/v1/admin/release-evidence/provider-runtime-config")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider runtime proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	for _, secret := range []string{
		cfg.StripeSecretKey,
		cfg.StripeWebhookSecret,
		cfg.AlipayWebhookSecret,
		cfg.WeChatPayWebhookSecret,
	} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("release evidence response leaked secret %q in body %s", secret, recorder.Body.String())
		}
	}

	var response struct {
		Data struct {
			Stripe              string `json:"stripe"`
			Alipay              string `json:"alipay"`
			WeChatPay           string `json:"wechatpay"`
			ProviderEnv         string `json:"providerEnv"`
			CheckoutBaseURLs    string `json:"checkoutBaseUrls"`
			WebhookRoutes       string `json:"webhookRoutes"`
			WebhookVerification string `json:"webhookVerification"`
			Summary             struct {
				ProvidersConfigured       int `json:"providersConfigured"`
				ProviderEnvVarsChecked    int `json:"providerEnvVarsChecked"`
				CheckoutBaseURLsChecked   int `json:"checkoutBaseUrlsChecked"`
				WebhookRoutesChecked      int `json:"webhookRoutesChecked"`
				WebhookVerificationChecks int `json:"webhookVerificationChecks"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode provider runtime proof: %v", err)
	}
	for field, value := range map[string]string{
		"stripe":              response.Data.Stripe,
		"alipay":              response.Data.Alipay,
		"wechatpay":           response.Data.WeChatPay,
		"providerEnv":         response.Data.ProviderEnv,
		"checkoutBaseUrls":    response.Data.CheckoutBaseURLs,
		"webhookRoutes":       response.Data.WebhookRoutes,
		"webhookVerification": response.Data.WebhookVerification,
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass, got %q in body %s", field, value, recorder.Body.String())
		}
	}
	if response.Data.Summary.ProvidersConfigured != 3 ||
		response.Data.Summary.ProviderEnvVarsChecked != 3 ||
		response.Data.Summary.CheckoutBaseURLsChecked != 3 ||
		response.Data.Summary.WebhookRoutesChecked != 3 ||
		response.Data.Summary.WebhookVerificationChecks != 3 {
		t.Fatalf("expected provider summary to cover all providers, got %+v", response.Data.Summary)
	}
}

func TestReleaseEvidenceRAGIndexingFailsClosedWithoutDatabase(t *testing.T) {
	recorder := releaseEvidenceGET(t, testConfig(), "/api/v1/admin/release-evidence/rag-indexing")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected RAG indexing proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			DurableQueueMigration string `json:"durableQueueMigration"`
			WorkerDeployment      string `json:"workerDeployment"`
			EnqueueDrainProbe     string `json:"enqueueDrainProbe"`
			RawParserReplay       string `json:"rawParserReplay"`
			RetrievalProbe        string `json:"retrievalProbe"`
			StaleVectorFilter     string `json:"staleVectorFilter"`
			Status                string `json:"status"`
			Summary               struct {
				QueuedJobs              int `json:"queuedJobs"`
				DrainedJobs             int `json:"drainedJobs"`
				WorkerCompletedJobs     int `json:"workerCompletedJobs"`
				RawParserReplayCount    int `json:"rawParserReplayCount"`
				RetrievalProbeCount     int `json:"retrievalProbeCount"`
				StaleVectorRowsFiltered int `json:"staleVectorRowsFiltered"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode RAG indexing proof: %v", err)
	}
	for field, value := range map[string]string{
		"durableQueueMigration": response.Data.DurableQueueMigration,
		"workerDeployment":      response.Data.WorkerDeployment,
		"enqueueDrainProbe":     response.Data.EnqueueDrainProbe,
		"rawParserReplay":       response.Data.RawParserReplay,
		"retrievalProbe":        response.Data.RetrievalProbe,
		"staleVectorFilter":     response.Data.StaleVectorFilter,
	} {
		if value != "fail" {
			t.Fatalf("expected %s to fail closed without database, got body %s", field, recorder.Body.String())
		}
	}
	if response.Data.Status != "not_ready" ||
		response.Data.Summary.QueuedJobs != 0 ||
		response.Data.Summary.DrainedJobs != 0 ||
		response.Data.Summary.WorkerCompletedJobs != 0 ||
		response.Data.Summary.RawParserReplayCount != 0 ||
		response.Data.Summary.RetrievalProbeCount != 0 ||
		response.Data.Summary.StaleVectorRowsFiltered != 0 {
		t.Fatalf("expected RAG indexing proof to stay not_ready with zero summary, got %+v", response.Data)
	}
}

func TestReleaseEvidenceRoutesRejectInvalidWindowScope(t *testing.T) {
	recorder := releaseEvidenceGET(t, testConfig(), "/api/v1/admin/release-evidence/rag-indexing?from=2026-07-08T00:00:00Z")
	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected incomplete release evidence window to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "from and to query parameters must be provided together") {
		t.Fatalf("expected window diagnostic, got %s", recorder.Body.String())
	}
}

func TestReleaseEvidenceRAGIndexingPassesFromTargetIndexingData(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("knowledge_retrieval_test_cases").
		WillReturnRows(sqlmock.NewRows([]string{
			"queued_jobs",
			"drained_jobs",
			"worker_completed_jobs",
			"raw_parser_replays",
			"retrieval_probe_count",
			"stale_vector_rows_filtered",
		}).AddRow(2, 2, 2, 1, 2, 1))

	handler := newReleaseEvidenceHandlerWithDatabase(testConfig(), database)
	proof := handler.ragIndexingProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("RAG indexing proof SQL expectations were not met: %v", err)
	}
	for field, value := range map[string]any{
		"durableQueueMigration": proof["durableQueueMigration"],
		"workerDeployment":      proof["workerDeployment"],
		"enqueueDrainProbe":     proof["enqueueDrainProbe"],
		"rawParserReplay":       proof["rawParserReplay"],
		"retrievalProbe":        proof["retrievalProbe"],
		"staleVectorFilter":     proof["staleVectorFilter"],
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass from target indexing data, got %+v", field, proof)
		}
	}
	if proof["status"] != nil {
		t.Fatalf("expected passing RAG indexing proof to omit not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["queuedJobs"] != 2 ||
		summary["drainedJobs"] != 2 ||
		summary["workerCompletedJobs"] != 2 ||
		summary["rawParserReplayCount"] != 1 ||
		summary["retrievalProbeCount"] != 2 ||
		summary["staleVectorRowsFiltered"] != 1 {
		t.Fatalf("unexpected RAG indexing summary: %+v", summary)
	}
}

func TestReleaseEvidenceRAGIndexingUsesWindowScope(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	from := time.Date(2026, time.July, 8, 8, 0, 0, 0, time.UTC)
	to := from.Add(30 * time.Minute)
	mock.ExpectQuery("knowledge_index_jobs").
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{
			"queued_jobs",
			"drained_jobs",
			"worker_completed_jobs",
			"raw_parser_replays",
			"retrieval_probe_count",
			"stale_vector_rows_filtered",
		}).AddRow(1, 1, 1, 1, 1, 1))

	handler := newReleaseEvidenceHandlerWithDatabase(testConfig(), database)
	ctx := withReleaseEvidenceScope(context.Background(), releaseEvidenceScope{
		From:      from,
		To:        to,
		HasWindow: true,
	})
	proof, ok := handler.proof(ctx, "rag-indexing")
	if !ok {
		t.Fatalf("expected RAG indexing proof")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("RAG indexing scoped SQL expectations were not met: %v", err)
	}
	scope, ok := proof["scope"].(map[string]string)
	if !ok {
		t.Fatalf("expected scoped proof to include scope, got %+v", proof)
	}
	if scope["from"] != from.Format(time.RFC3339) || scope["to"] != to.Format(time.RFC3339) {
		t.Fatalf("unexpected release evidence scope: %+v", scope)
	}
}

func TestReleaseEvidenceMarketplaceGovernanceFailsClosedWithoutDatabase(t *testing.T) {
	recorder := releaseEvidenceGET(t, testConfig(), "/api/v1/admin/release-evidence/marketplace-governance")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected marketplace governance proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			ReviewQueue             string `json:"reviewQueue"`
			AppealQueue             string `json:"appealQueue"`
			AppealDecisionLifecycle string `json:"appealDecisionLifecycle"`
			ReviewAssignment        string `json:"reviewAssignment"`
			ReviewSLAEnforcement    string `json:"reviewSLAEnforcement"`
			AbuseReportLifecycle    string `json:"abuseReportLifecycle"`
			Status                  string `json:"status"`
			Summary                 struct {
				ReviewQueueItems     int `json:"reviewQueueItems"`
				AppealQueueItems     int `json:"appealQueueItems"`
				AppealDecisions      int `json:"appealDecisions"`
				ReviewAssignments    int `json:"reviewAssignments"`
				SLAChecks            int `json:"slaChecks"`
				SLABreachesHandled   int `json:"slaBreachesHandled"`
				AbuseReports         int `json:"abuseReports"`
				AbuseReportsResolved int `json:"abuseReportsResolved"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode marketplace governance proof: %v", err)
	}
	for field, value := range map[string]string{
		"reviewQueue":             response.Data.ReviewQueue,
		"appealQueue":             response.Data.AppealQueue,
		"appealDecisionLifecycle": response.Data.AppealDecisionLifecycle,
		"reviewAssignment":        response.Data.ReviewAssignment,
		"reviewSLAEnforcement":    response.Data.ReviewSLAEnforcement,
		"abuseReportLifecycle":    response.Data.AbuseReportLifecycle,
	} {
		if value != "fail" {
			t.Fatalf("expected %s to fail closed without database, got body %s", field, recorder.Body.String())
		}
	}
	if response.Data.Status != "not_ready" || response.Data.Summary.ReviewQueueItems != 0 || response.Data.Summary.AbuseReports != 0 {
		t.Fatalf("expected marketplace governance proof to stay not_ready with zero summary, got %+v", response.Data)
	}
}

func TestReleaseEvidenceMarketplacePayoutFailsClosedWithoutDatabase(t *testing.T) {
	recorder := releaseEvidenceGET(t, testConfig(), "/api/v1/admin/release-evidence/marketplace-payout")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected marketplace payout proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			OutboundDispatch        string `json:"outboundDispatch"`
			InboundWebhookLifecycle string `json:"inboundWebhookLifecycle"`
			SettlementLedger        string `json:"settlementLedger"`
			Reconciliation          string `json:"reconciliation"`
			RefundChargeback        string `json:"refundChargebackHandling"`
			Status                  string `json:"status"`
			Summary                 struct {
				OutboundDispatches           int `json:"outboundDispatches"`
				WebhookEvents                int `json:"webhookEvents"`
				SettlementLedgerEntries      int `json:"settlementLedgerEntries"`
				ReconciledEntries            int `json:"reconciledEntries"`
				RefundChargebackCases        int `json:"refundChargebackCases"`
				RefundChargebackCasesHandled int `json:"refundChargebackCasesHandled"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode marketplace payout proof: %v", err)
	}
	for field, value := range map[string]string{
		"outboundDispatch":         response.Data.OutboundDispatch,
		"inboundWebhookLifecycle":  response.Data.InboundWebhookLifecycle,
		"settlementLedger":         response.Data.SettlementLedger,
		"reconciliation":           response.Data.Reconciliation,
		"refundChargebackHandling": response.Data.RefundChargeback,
	} {
		if value != "fail" {
			t.Fatalf("expected %s to fail closed without database, got body %s", field, recorder.Body.String())
		}
	}
	if response.Data.Status != "not_ready" || response.Data.Summary.OutboundDispatches != 0 || response.Data.Summary.SettlementLedgerEntries != 0 {
		t.Fatalf("expected marketplace payout proof to stay not_ready with zero summary, got %+v", response.Data)
	}
}

func TestReleaseEvidenceMarketplacePayoutPassesFromTargetLedgerData(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("billing_refunds").
		WillReturnRows(sqlmock.NewRows([]string{
			"outbound_dispatches",
			"webhook_events",
			"settlement_ledger_entries",
			"reconciled_entries",
			"refund_chargeback_cases",
			"refund_chargeback_cases_handled",
		}).AddRow(3, 3, 3, 3, 1, 1))

	handler := newReleaseEvidenceHandlerWithDatabase(testConfig(), database)
	proof := handler.marketplacePayoutProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("marketplace payout proof SQL expectations were not met: %v", err)
	}
	for field, value := range map[string]any{
		"outboundDispatch":         proof["outboundDispatch"],
		"inboundWebhookLifecycle":  proof["inboundWebhookLifecycle"],
		"settlementLedger":         proof["settlementLedger"],
		"reconciliation":           proof["reconciliation"],
		"refundChargebackHandling": proof["refundChargebackHandling"],
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass from target ledger data, got %+v", field, proof)
		}
	}
	if proof["status"] != nil {
		t.Fatalf("expected passing marketplace payout proof to omit not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["outboundDispatches"] != 3 ||
		summary["webhookEvents"] != 3 ||
		summary["settlementLedgerEntries"] != 3 ||
		summary["reconciledEntries"] != 3 ||
		summary["refundChargebackCases"] != 1 ||
		summary["refundChargebackCasesHandled"] != 1 {
		t.Fatalf("unexpected marketplace payout summary: %+v", summary)
	}
}

func TestReleaseEvidenceMarketplaceGovernancePassesFromTargetLifecycleData(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("marketplace_abuse_reports").
		WillReturnRows(sqlmock.NewRows([]string{
			"review_queue_items",
			"appeal_queue_items",
			"appeal_decisions",
			"review_assignments",
			"sla_checks",
			"sla_breaches_handled",
			"abuse_reports",
			"abuse_reports_resolved",
		}).AddRow(2, 1, 1, 2, 2, 1, 1, 1))

	handler := newReleaseEvidenceHandlerWithDatabase(testConfig(), database)
	proof := handler.marketplaceGovernanceProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("marketplace governance proof SQL expectations were not met: %v", err)
	}
	for field, value := range map[string]any{
		"reviewQueue":             proof["reviewQueue"],
		"appealQueue":             proof["appealQueue"],
		"appealDecisionLifecycle": proof["appealDecisionLifecycle"],
		"reviewAssignment":        proof["reviewAssignment"],
		"reviewSLAEnforcement":    proof["reviewSLAEnforcement"],
		"abuseReportLifecycle":    proof["abuseReportLifecycle"],
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass from target lifecycle data, got %+v", field, proof)
		}
	}
	if proof["status"] != nil {
		t.Fatalf("expected passing marketplace governance proof to omit not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["reviewQueueItems"] != 2 ||
		summary["appealQueueItems"] != 1 ||
		summary["appealDecisions"] != 1 ||
		summary["reviewAssignments"] != 2 ||
		summary["slaChecks"] != 2 ||
		summary["slaBreachesHandled"] != 1 ||
		summary["abuseReports"] != 1 ||
		summary["abuseReportsResolved"] != 1 {
		t.Fatalf("unexpected marketplace governance summary: %+v", summary)
	}
}

func TestReleaseEvidenceMicroserviceDatabaseFailsClosedOnMigrationReadiness(t *testing.T) {
	cfg := releaseEvidenceMicroserviceConfig()

	recorder := releaseEvidenceGET(t, cfg, "/api/v1/admin/release-evidence/microservice-database")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected microservice database proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), cfg.DBURLRelay) {
		t.Fatalf("release evidence response leaked database URL in body %s", recorder.Body.String())
	}

	var response struct {
		Data struct {
			Mode               string `json:"mode"`
			ServiceURLClass    string `json:"serviceUrlClass"`
			Relay              string `json:"relay"`
			Observability      string `json:"observability"`
			MigrationReadiness string `json:"migrationReadiness"`
			Status             string `json:"status"`
			Summary            struct {
				ServicesChecked          int `json:"servicesChecked"`
				ExternalURLsChecked      int `json:"externalUrlsChecked"`
				MigrationReadinessChecks int `json:"migrationReadinessChecks"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode microservice database proof: %v", err)
	}
	if response.Data.Mode != "microservices" || response.Data.ServiceURLClass != "external-filled" {
		t.Fatalf("expected microservice external-filled proof, got %+v", response.Data)
	}
	if response.Data.Relay != "pass" || response.Data.Observability != "pass" {
		t.Fatalf("expected configured service URLs to pass, got body %s", recorder.Body.String())
	}
	if response.Data.MigrationReadiness != "fail" || response.Data.Status != "not_ready" {
		t.Fatalf("expected migration readiness to fail closed until live probes exist, got body %s", recorder.Body.String())
	}
	if response.Data.Summary.ServicesChecked != 11 ||
		response.Data.Summary.ExternalURLsChecked != 11 ||
		response.Data.Summary.MigrationReadinessChecks != 1 {
		t.Fatalf("unexpected microservice database summary: %+v", response.Data.Summary)
	}
}

func TestReleaseEvidenceMicroserviceDatabasePassesLiveMigrationReadiness(t *testing.T) {
	cfg := releaseEvidenceMicroserviceConfig()
	cfg.Env = "production"

	handler := newReleaseEvidenceHandler(cfg)
	handler.migrationProbeTimeout = time.Second

	opened := 0
	var databases []*sql.DB
	var verifyMocks []func()
	handler.openDatabase = func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName != "postgres" {
			t.Fatalf("expected postgres driver, got %q", driverName)
		}
		if strings.TrimSpace(dataSourceName) == "" || strings.Contains(dataSourceName, "should_not_leak") {
			t.Fatalf("unexpected database source name %q", dataSourceName)
		}

		database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("create sqlmock database: %v", err)
		}
		mock.ExpectPing()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM schema_migrations`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectClose()

		databases = append(databases, database)
		verifyMocks = append(verifyMocks, func() {
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("migration probe SQL expectations were not met: %v", err)
			}
		})
		opened++
		return database, nil
	}

	proof := handler.microserviceDatabaseProof(context.Background())
	for _, verify := range verifyMocks {
		verify()
	}
	if len(databases) != len(microserviceDatabaseProofServices()) || opened != len(microserviceDatabaseProofServices()) {
		t.Fatalf("expected a migration probe for every service, opened=%d databases=%d", opened, len(databases))
	}
	if proof["migrationReadiness"] != "pass" || proof["status"] != nil {
		t.Fatalf("expected live migration readiness proof to pass without not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["servicesChecked"] != 11 ||
		summary["externalUrlsChecked"] != 11 ||
		summary["migrationReadinessChecks"] != 11 {
		t.Fatalf("unexpected migration readiness summary: %+v", summary)
	}
}

func TestReleaseEvidenceRelayDisabledProofsPassFromConfig(t *testing.T) {
	cfg := testConfig()
	cfg.RelayRealtimeCommercialLifecycleEnabled = false
	cfg.RelayBatchCommercialLifecycleEnabled = false

	for _, tt := range []struct {
		name         string
		path         string
		blockerField string
		blockerCount string
		minBlockers  int
	}{
		{
			name:         "realtime",
			path:         "/api/v1/admin/release-evidence/relay-realtime",
			blockerField: "authOriginPrebillAbortUsageBlockers",
			blockerCount: "authOriginPrebillAbortUsageBlockerChecks",
			minBlockers:  5,
		},
		{
			name:         "batch",
			path:         "/api/v1/admin/release-evidence/relay-batch",
			blockerField: "prebillPollingSettlementRefundAuditUsageBlockers",
			blockerCount: "prebillPollingSettlementRefundAuditUsageBlockerChecks",
			minBlockers:  6,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := releaseEvidenceGET(t, cfg, tt.path)
			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected relay proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode relay proof: %v", err)
			}
			if response.Data["mode"] != "disabled_until_commercial_lifecycle" ||
				response.Data["productionPolicyDisabled"] != "pass" ||
				response.Data[tt.blockerField] != "pass" {
				t.Fatalf("expected disabled relay proof to pass, got %s", recorder.Body.String())
			}
			summary, ok := response.Data["summary"].(map[string]any)
			if !ok {
				t.Fatalf("expected summary object, got %s", recorder.Body.String())
			}
			if int(summary[tt.blockerCount].(float64)) < tt.minBlockers {
				t.Fatalf("expected blocker count to cover disabled relay boundary, got %s", recorder.Body.String())
			}
		})
	}
}

func TestReleaseEvidenceRelayRealtimeEnabledFailsClosedWithoutTargetLedger(t *testing.T) {
	cfg := testConfig()
	cfg.RelayRealtimeCommercialLifecycleEnabled = true
	cfg.CORSAllowedOrigins = []string{"https://console.oblivious.release.test"}

	recorder := releaseEvidenceGET(t, cfg, "/api/v1/admin/release-evidence/relay-realtime")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected relay realtime proof 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Mode                    string `json:"mode"`
			ProductionPolicyEnabled string `json:"productionPolicyEnabled"`
			AuthPolicy              string `json:"authPolicy"`
			OriginPolicy            string `json:"originPolicy"`
			PrebillSettlement       string `json:"prebillSettlement"`
			AbortSettlement         string `json:"abortSettlement"`
			UsageLedger             string `json:"usageLedger"`
			Status                  string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode relay realtime proof: %v", err)
	}
	if response.Data.Mode != "commercial_lifecycle_enabled" ||
		response.Data.ProductionPolicyEnabled != "pass" ||
		response.Data.OriginPolicy != "pass" ||
		response.Data.AuthPolicy != "fail" ||
		response.Data.PrebillSettlement != "fail" ||
		response.Data.AbortSettlement != "fail" ||
		response.Data.UsageLedger != "fail" ||
		response.Data.Status != "not_ready" {
		t.Fatalf("expected enabled realtime proof to fail closed without target ledger, got %+v body=%s", response.Data, recorder.Body.String())
	}
}

func TestReleaseEvidenceRelayRealtimeEnabledPassesFromTargetLedger(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("usage_records").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests",
			"authenticated_requests",
			"request_linked_usage_records",
			"price_snapshot_records",
			"abort_settlement_records",
			"terminal_usage_records",
		}).AddRow(2, 2, 2, 2, 1, 2))

	cfg := testConfig()
	cfg.RelayRealtimeCommercialLifecycleEnabled = true
	cfg.CORSAllowedOrigins = []string{"https://console.oblivious.release.test"}
	handler := newReleaseEvidenceHandlerWithDatabase(cfg, database)
	proof := handler.relayRealtimeProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("relay realtime proof SQL expectations were not met: %v", err)
	}
	for field, value := range map[string]any{
		"productionPolicyEnabled": proof["productionPolicyEnabled"],
		"authPolicy":              proof["authPolicy"],
		"originPolicy":            proof["originPolicy"],
		"prebillSettlement":       proof["prebillSettlement"],
		"abortSettlement":         proof["abortSettlement"],
		"usageLedger":             proof["usageLedger"],
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass from target ledger data, got %+v", field, proof)
		}
	}
	if proof["mode"] != "commercial_lifecycle_enabled" || proof["status"] != nil {
		t.Fatalf("expected passing realtime proof without not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["totalRequests"] != 2 ||
		summary["authenticatedRequests"] != 2 ||
		summary["requestLinkedUsageRecords"] != 2 ||
		summary["priceSnapshotRecords"] != 2 ||
		summary["abortSettlementRecords"] != 1 ||
		summary["terminalUsageRecords"] != 2 ||
		summary["originPolicyChecks"] != 1 {
		t.Fatalf("unexpected realtime summary: %+v", summary)
	}
}

func TestReleaseEvidenceRelayRealtimeAbortSettlementRequiresRealtimeErrorCode(t *testing.T) {
	if strings.Contains(releaseEvidenceRelayRealtimeSummaryQuery, "upstream_error") {
		t.Fatalf("realtime release evidence must not treat generic upstream_error as abort settlement proof:\n%s", releaseEvidenceRelayRealtimeSummaryQuery)
	}
	if !strings.Contains(releaseEvidenceRelayRealtimeSummaryQuery, "realtime_usage_missing") {
		t.Fatalf("realtime release evidence must require explicit realtime abort error codes:\n%s", releaseEvidenceRelayRealtimeSummaryQuery)
	}

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("usage_records").
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests",
			"authenticated_requests",
			"request_linked_usage_records",
			"price_snapshot_records",
			"abort_settlement_records",
			"terminal_usage_records",
		}).AddRow(2, 2, 2, 2, 0, 2))

	cfg := testConfig()
	cfg.RelayRealtimeCommercialLifecycleEnabled = true
	cfg.CORSAllowedOrigins = []string{"https://console.oblivious.release.test"}
	handler := newReleaseEvidenceHandlerWithDatabase(cfg, database)
	proof := handler.relayRealtimeProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("relay realtime proof SQL expectations were not met: %v", err)
	}
	if proof["abortSettlement"] != "fail" || proof["status"] != "not_ready" {
		t.Fatalf("expected realtime proof to fail when no explicit abort settlement record exists, got %+v", proof)
	}
}

func TestReleaseEvidenceRelayBatchEnabledPassesFromTargetLedger(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("relay_batch_polling_jobs").
		WillReturnRows(sqlmock.NewRows([]string{
			"prebill_reservations",
			"polling_completions",
			"settlement_records",
			"refund_records",
			"usage_audit_records",
			"terminal_failure_records",
		}).AddRow(2, 1, 1, 1, 2, 1))
	mock.ExpectQuery("SELECT DISTINCT usage.request_id").
		WillReturnRows(sqlmock.NewRows([]string{"request_id"}).
			AddRow("req_batch_refund").
			AddRow("req_batch_success"))

	cfg := testConfig()
	cfg.RelayBatchCommercialLifecycleEnabled = true
	cfg.RelayBatchPollingWorkerEnabled = true
	requestLogs := &releaseEvidenceRequestLogStore{
		evidence: map[string]admin.RequestLogEvidence{
			"req_batch_refund":  batchRequestLogEvidence("req_batch_refund"),
			"req_batch_success": batchRequestLogEvidence("req_batch_success"),
		},
	}
	handler := newReleaseEvidenceHandlerWithDatabaseAndRequestLogs(cfg, database, requestLogs)
	proof := handler.relayBatchProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("relay batch proof SQL expectations were not met: %v", err)
	}
	if requestLogs.routeDecisionAPIType != "batch" || requestLogs.routeDecisionResult != "allowed" {
		t.Fatalf("expected route-decision evidence lookup for allowed batch decisions, got apiType=%q result=%q", requestLogs.routeDecisionAPIType, requestLogs.routeDecisionResult)
	}
	for field, value := range map[string]any{
		"productionPolicyEnabled": proof["productionPolicyEnabled"],
		"prebillReservation":      proof["prebillReservation"],
		"pollingCompletion":       proof["pollingCompletion"],
		"settlement":              proof["settlement"],
		"refund":                  proof["refund"],
		"usageAudit":              proof["usageAudit"],
	} {
		if value != "pass" {
			t.Fatalf("expected %s to pass from target ledger data, got %+v", field, proof)
		}
	}
	if proof["mode"] != "commercial_lifecycle_enabled" || proof["status"] != nil {
		t.Fatalf("expected passing batch proof without not_ready status, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["prebillReservations"] != 2 ||
		summary["pollingCompletions"] != 1 ||
		summary["settlementRecords"] != 1 ||
		summary["refundRecords"] != 1 ||
		summary["usageAuditRecords"] != 2 ||
		summary["requestLogAuditRecords"] != 2 ||
		summary["terminalFailureRecords"] != 1 {
		t.Fatalf("unexpected batch summary: %+v", summary)
	}
}

func TestReleaseEvidenceRelayBatchUsageAuditRequiresRequestLogEvidence(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("relay_batch_polling_jobs").
		WillReturnRows(sqlmock.NewRows([]string{
			"prebill_reservations",
			"polling_completions",
			"settlement_records",
			"refund_records",
			"usage_audit_records",
			"terminal_failure_records",
		}).AddRow(2, 1, 1, 1, 2, 1))
	mock.ExpectQuery("SELECT DISTINCT usage.request_id").
		WillReturnRows(sqlmock.NewRows([]string{"request_id"}).
			AddRow("req_batch_missing_log").
			AddRow("req_batch_success"))

	cfg := testConfig()
	cfg.RelayBatchCommercialLifecycleEnabled = true
	cfg.RelayBatchPollingWorkerEnabled = true
	handler := newReleaseEvidenceHandlerWithDatabaseAndRequestLogs(cfg, database, &releaseEvidenceRequestLogStore{
		evidence: map[string]admin.RequestLogEvidence{
			"req_batch_success": batchRequestLogEvidence("req_batch_success"),
		},
	})
	proof := handler.relayBatchProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("relay batch proof SQL expectations were not met: %v", err)
	}
	if proof["usageAudit"] != "fail" || proof["status"] != "not_ready" {
		t.Fatalf("expected batch usageAudit to fail without full request-log evidence, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["usageAuditRecords"] != 2 || summary["requestLogAuditRecords"] != 1 {
		t.Fatalf("expected request-log audit coverage gap in summary, got %+v", summary)
	}
}

func TestReleaseEvidenceRelayBatchUsageAuditRequiresRouteDecisionRequestLogEvidence(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	mock.ExpectQuery("relay_batch_polling_jobs").
		WillReturnRows(sqlmock.NewRows([]string{
			"prebill_reservations",
			"polling_completions",
			"settlement_records",
			"refund_records",
			"usage_audit_records",
			"terminal_failure_records",
		}).AddRow(2, 1, 1, 1, 2, 1))
	mock.ExpectQuery("SELECT DISTINCT usage.request_id").
		WillReturnRows(sqlmock.NewRows([]string{"request_id"}).
			AddRow("req_batch_generic_log").
			AddRow("req_batch_success"))

	cfg := testConfig()
	cfg.RelayBatchCommercialLifecycleEnabled = true
	cfg.RelayBatchPollingWorkerEnabled = true
	handler := newReleaseEvidenceHandlerWithDatabaseAndRequestLogs(cfg, database, &releaseEvidenceRequestLogStore{
		evidence: map[string]admin.RequestLogEvidence{
			"req_batch_generic_log": genericBatchRequestLogEvidence("req_batch_generic_log"),
			"req_batch_success":     batchRequestLogEvidence("req_batch_success"),
		},
	})
	proof := handler.relayBatchProof(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("relay batch proof SQL expectations were not met: %v", err)
	}
	if proof["usageAudit"] != "fail" || proof["status"] != "not_ready" {
		t.Fatalf("expected batch usageAudit to fail without route-decision request-log coverage, got %+v", proof)
	}
	summary, ok := proof["summary"].(map[string]int)
	if !ok {
		t.Fatalf("expected integer summary, got %+v", proof["summary"])
	}
	if summary["requestLogAuditRecords"] != 1 {
		t.Fatalf("expected only route-decision request log to count, got %+v", summary)
	}
}

func TestReleaseEvidenceUnknownProofReturnsNotFound(t *testing.T) {
	recorder := releaseEvidenceGET(t, testConfig(), "/api/v1/admin/release-evidence/unknown-proof")
	if recorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected unknown proof to return 404, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

type releaseEvidenceRequestLogStore struct {
	evidence             map[string]admin.RequestLogEvidence
	routeDecisionAPIType string
	routeDecisionResult  string
}

func (s *releaseEvidenceRequestLogStore) ListRequestLogEvidence(_ context.Context, requestIDs []string) (map[string]admin.RequestLogEvidence, error) {
	result := map[string]admin.RequestLogEvidence{}
	for _, requestID := range requestIDs {
		if item, ok := s.evidence[requestID]; ok {
			result[requestID] = item
		}
	}
	return result, nil
}

func (s *releaseEvidenceRequestLogStore) ListRelayRouteDecisionEvidence(_ context.Context, requestIDs []string, apiType string, result string) (map[string]admin.RequestLogEvidence, error) {
	s.routeDecisionAPIType = apiType
	s.routeDecisionResult = result
	return s.ListRequestLogEvidence(context.Background(), requestIDs)
}

func batchRequestLogEvidence(requestID string) admin.RequestLogEvidence {
	return admin.RequestLogEvidence{
		RequestID:    requestID,
		RequestLogID: "550e8400-e29b-41d4-a716-446655440000",
		Service:      "relay",
		Endpoint:     "/v1/batch",
		Method:       stdhttp.MethodPost,
		StatusCode:   stdhttp.StatusOK,
		Metadata:     json.RawMessage(`{"event":"relay.route_decision","relay_api_type":"batch","relay_route_result":"allowed"}`),
	}
}

func genericBatchRequestLogEvidence(requestID string) admin.RequestLogEvidence {
	return admin.RequestLogEvidence{
		RequestID:    requestID,
		RequestLogID: "650e8400-e29b-41d4-a716-446655440000",
		Service:      "relay",
		Endpoint:     "/v1/batch",
		Method:       stdhttp.MethodPost,
		StatusCode:   stdhttp.StatusOK,
		Metadata:     json.RawMessage(`{"relay_api_type":"batch","relay_route_result":"allowed"}`),
	}
}

func releaseEvidenceGET(t *testing.T, cfg config.Config, path string) *httptest.ResponseRecorder {
	t.Helper()

	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(cfg, nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	request := httptest.NewRequest(stdhttp.MethodGet, path, nil)
	request.AddCookie(routeSurfaceSignedSessionCookie(t, session))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func releaseEvidenceMicroserviceConfig() config.Config {
	cfg := testConfig()
	cfg.DBMode = "microservices"
	cfg.DBURLRelay = "postgres://relay-db.oblivious.internal/relay"
	cfg.DBURLChat = "postgres://chat-db.oblivious.internal/chat"
	cfg.DBURLWorkflow = "postgres://workflow-db.oblivious.internal/workflow"
	cfg.DBURLRAG = "postgres://rag-db.oblivious.internal/rag"
	cfg.DBURLAgent = "postgres://agent-db.oblivious.internal/agent"
	cfg.DBURLBilling = "postgres://billing-db.oblivious.internal/billing"
	cfg.DBURLMarketplace = "postgres://marketplace-db.oblivious.internal/marketplace"
	cfg.DBURLAdmin = "postgres://admin-db.oblivious.internal/admin"
	cfg.DBURLChannel = "postgres://channel-db.oblivious.internal/channel"
	cfg.DBURLTask = "postgres://task-db.oblivious.internal/task"
	cfg.DBURLObservability = "postgres://observability-db.oblivious.internal/observability"
	return cfg
}
