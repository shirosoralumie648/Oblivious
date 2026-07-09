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

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/config"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/payment"
	"oblivious/server/internal/quota"
	relaytypes "oblivious/server/internal/relay/types"
	stripebilling "oblivious/server/internal/stripe"
)

func TestAdminHandlerExposesPhase31Operations(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channels?limit=200&offset=3", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	listRecorder := httptest.NewRecorder()
	handler.listChannels(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list channels expected 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if store.channelFilter.Limit != 100 || store.channelFilter.Offset != 3 {
		t.Fatalf("expected clamped channel filter limit=100 offset=3, got limit=%d offset=%d", store.channelFilter.Limit, store.channelFilter.Offset)
	}

	batchRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels/batch", strings.NewReader(`{"ids":["ch_1"],"action":"disable"}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	batchRecorder := httptest.NewRecorder()
	handler.batchUpdateChannels(batchRecorder, batchRequest)
	if batchRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("batch update expected 200, got %d: %s", batchRecorder.Code, batchRecorder.Body.String())
	}
	if store.batchAction != "disable" {
		t.Fatalf("expected disable batch action, got %q", store.batchAction)
	}

	updateRouteRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/routes/route_1", strings.NewReader(`{"model":"gpt-4.1"}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	updateRouteRecorder := httptest.NewRecorder()
	handler.updateRoute(updateRouteRecorder, updateRouteRequest, "route_1")
	if updateRouteRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update route expected 200, got %d: %s", updateRouteRecorder.Code, updateRouteRecorder.Body.String())
	}
	if store.routeUpdate.Model == nil || *store.routeUpdate.Model != "gpt-4.1" {
		t.Fatalf("expected route update model to be decoded, got %#v", store.routeUpdate.Model)
	}

	approveRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/approve", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	approveRecorder := httptest.NewRecorder()
	handler.approveAgent(approveRecorder, approveRequest, "agent_1")
	if approveRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("approve agent expected 200, got %d: %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	if store.approvedAgentID != "agent_1" {
		t.Fatalf("expected approved agent agent_1, got %q", store.approvedAgentID)
	}

	claimRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/claim", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	claimRecorder := httptest.NewRecorder()
	handler.claimReview(claimRecorder, claimRequest, "agent_1")
	if claimRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("claim review expected 200, got %d: %s", claimRecorder.Code, claimRecorder.Body.String())
	}
	if store.claimedAgentID != "agent_1" || store.claimedReviewerID != "user_admin" {
		t.Fatalf("expected review claim agent/reviewer to be stored, got agent=%q reviewer=%q", store.claimedAgentID, store.claimedReviewerID)
	}
	if !strings.Contains(claimRecorder.Body.String(), `"status":"claimed"`) {
		t.Fatalf("expected claimed response status, got %s", claimRecorder.Body.String())
	}

	needsChangesRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/needs-changes", strings.NewReader(`{"reason":"Add data retention details."}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	needsChangesRecorder := httptest.NewRecorder()
	handler.needsChangesAgent(needsChangesRecorder, needsChangesRequest, "agent_1")
	if needsChangesRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("needs changes expected 200, got %d: %s", needsChangesRecorder.Code, needsChangesRecorder.Body.String())
	}
	if store.needsChangesAgentID != "agent_1" || store.needsChangesReason != "Add data retention details." {
		t.Fatalf("expected needs changes agent/reason to be stored, got id=%q reason=%q", store.needsChangesAgentID, store.needsChangesReason)
	}
	if !strings.Contains(needsChangesRecorder.Body.String(), `"status":"needs_changes"`) {
		t.Fatalf("expected needs_changes response status, got %s", needsChangesRecorder.Body.String())
	}
}

func TestAdminHandlerListsReviewsWithMarketplaceReviewSLA(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	store := &fakeAdminStore{
		pendingReviews: []*marketplace.PublishedAgent{{
			ID:        "agent_sla",
			Name:      "Review me",
			Status:    "pending_review",
			CreatedAt: now.Add(-71 * time.Hour),
			UpdatedAt: now.Add(-71 * time.Hour),
			ReviewSLA: &marketplace.ReviewSLA{
				ManualDeadlineAt:          now.Add(time.Hour),
				ManualSlaHours:            72,
				ManualSlaStatus:           "due_soon",
				MinutesUntilDeadline:      60,
				AutomatedReviewDeadlineAt: now.Add(-70*time.Hour - 55*time.Minute),
				AutomatedReviewSlaMinutes: 5,
				AutomatedReviewSlaStatus:  "overdue",
				VIPPublisher:              false,
				PublisherTier:             "standard",
				PublisherTierSource:       "default",
			},
		}},
	}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/reviews", nil)
	recorder := httptest.NewRecorder()
	handler.listReviews(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected reviews 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"reviewSLA"`) ||
		!strings.Contains(recorder.Body.String(), `"manualDeadlineAt":"2026-06-05T13:00:00Z"`) ||
		!strings.Contains(recorder.Body.String(), `"manualSlaHours":72`) ||
		!strings.Contains(recorder.Body.String(), `"manualSlaStatus":"due_soon"`) ||
		!strings.Contains(recorder.Body.String(), `"minutesUntilDeadline":60`) {
		t.Fatalf("expected pending review response to expose reviewSLA, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerEnforcesMarketplaceReviewSLA(t *testing.T) {
	enforcer := &fakeReviewSLAEnforcer{
		result: marketplace.ReviewSLAEnforcementResult{Scanned: 3, Alerted: 2},
	}
	handler := newAdminHandlerWithReviewSLA(admin.NewService(&fakeAdminStore{}), enforcer)

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce?limit=50&offset=2", nil)
	recorder := httptest.NewRecorder()

	handler.enforceReviewSLA(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected SLA enforcement 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if enforcer.options.Limit != 50 || enforcer.options.Offset != 2 {
		t.Fatalf("expected limit/offset to be passed through, got %+v", enforcer.options)
	}
	if !strings.Contains(recorder.Body.String(), `"scanned":3`) || !strings.Contains(recorder.Body.String(), `"alerted":2`) {
		t.Fatalf("expected enforcement result in response, got %s", recorder.Body.String())
	}
}

func TestAdminReviewSLAEnforceRouteScansPendingReviewsAndAlerts(t *testing.T) {
	database := testDatabase(t)
	alerts := &captureReviewSLAAlertSink{}
	restoreAlerts := setHTTPAlertRouterForTest(alerts)
	defer restoreAlerts()
	router := NewRouter(testConfig(), database)

	userCookie, userCSRF, _ := registerHTTPUser(t, router, "marketplace-sla-user@example.com")
	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "marketplace-sla-admin@example.com")
	promoteHTTPUserToAdmin(t, database, adminUserID)
	_, organizationID := queryHTTPUserScope(t, database, adminUserID)
	if _, err := database.Exec(`
			INSERT INTO published_agents (
				id, owner_id, organization_id, name, description, category_id, tools, example_conversations,
				visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
			)
			VALUES ('agent_sla_overdue_http', $1, $2, 'SLA Overdue HTTP', 'Pending review past manual SLA.',
			        'cat_productivity', '{"tools":[{"name":"sla"}]}'::jsonb, '[]'::jsonb, 'public', 'pending_review',
			        'free', 0, 0, 0, 0, NOW() - INTERVAL '80 hours', NOW() - INTERVAL '80 hours')
	`, adminUserID, organizationID); err != nil {
		t.Fatalf("insert pending review agent: %v", err)
	}

	forbiddenRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce?limit=10", nil)
	forbiddenRequest.AddCookie(userCookie)
	addCSRF(forbiddenRequest, userCSRF)
	router.ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-admin SLA enforcement route 403, got %d: %s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
	}
	if len(alerts.events) != 0 {
		t.Fatalf("non-admin request must not emit SLA alerts, got %+v", alerts.events)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce?limit=10", nil)
	request.AddCookie(adminCookie)
	addCSRF(request, adminCSRF)
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected SLA enforcement route 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"scanned":1`) || !strings.Contains(recorder.Body.String(), `"alerted":1`) {
		t.Fatalf("expected scanned/alerted response from route, got %s", recorder.Body.String())
	}
	if len(alerts.events) != 1 {
		t.Fatalf("expected one routed SLA alert, got %+v", alerts.events)
	}
	if alerts.events[0].Key != "marketplace_review_sla:agent_sla_overdue_http:manual:overdue" ||
		alerts.events[0].Severity != observability.AlertSeverityCritical ||
		alerts.events[0].Component != "marketplace.review" {
		t.Fatalf("unexpected routed SLA alert: %+v", alerts.events[0])
	}
}

func TestAdminHandlerSyncsChannelModels(t *testing.T) {
	store := &fakeAdminStore{
		channelTestResult: &admin.ChannelTestResult{
			Success: true,
			Models:  []string{"gpt-4o", "gpt-4o-mini"},
		},
	}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/sync-models", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	recorder := httptest.NewRecorder()

	handler.syncChannelModels(recorder, request, "ch_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("sync models expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !equalStrings(store.updatedChannelModels, []string{"gpt-4o", "gpt-4o-mini"}) {
		t.Fatalf("expected channel models to be persisted, got %#v", store.updatedChannelModels)
	}
	if !strings.Contains(recorder.Body.String(), `"channel"`) ||
		!strings.Contains(recorder.Body.String(), `"testResult"`) ||
		!strings.Contains(recorder.Body.String(), `"models":["gpt-4o","gpt-4o-mini"]`) {
		t.Fatalf("expected sync response with channel and probe models, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerDetectsAndAppliesChannelModelUpdates(t *testing.T) {
	store := &fakeAdminStore{
		currentChannelModels: []string{"gpt-4o", "legacy-model"},
		channelTestResult: &admin.ChannelTestResult{
			Success: true,
			Models:  []string{"gpt-4o", "gpt-4.1"},
		},
	}
	handler := newAdminHandler(admin.NewService(store))

	detectRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/model-updates/detect", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	detectRecorder := httptest.NewRecorder()
	handler.detectChannelModelUpdates(detectRecorder, detectRequest, "ch_1")
	if detectRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("detect model updates expected 200, got %d: %s", detectRecorder.Code, detectRecorder.Body.String())
	}
	if !strings.Contains(detectRecorder.Body.String(), `"added":["gpt-4.1"]`) ||
		!strings.Contains(detectRecorder.Body.String(), `"removed":["legacy-model"]`) {
		t.Fatalf("expected model update diff, got %s", detectRecorder.Body.String())
	}
	if store.updatedChannelModels != nil {
		t.Fatalf("detect must not persist channel models, got %#v", store.updatedChannelModels)
	}

	applyRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/model-updates/apply", strings.NewReader(`{"mode":"replace"}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	applyRecorder := httptest.NewRecorder()
	handler.applyChannelModelUpdates(applyRecorder, applyRequest, "ch_1")
	if applyRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("apply model updates expected 200, got %d: %s", applyRecorder.Code, applyRecorder.Body.String())
	}
	if !equalStrings(store.updatedChannelModels, []string{"gpt-4o", "gpt-4.1"}) {
		t.Fatalf("expected replaced channel models, got %#v", store.updatedChannelModels)
	}
	if !strings.Contains(applyRecorder.Body.String(), `"mode":"replace"`) ||
		!strings.Contains(applyRecorder.Body.String(), `"appliedModels":["gpt-4o","gpt-4.1"]`) {
		t.Fatalf("expected apply response with replaced models, got %s", applyRecorder.Body.String())
	}
}

func TestAdminHandlerRefreshesChannelBalance(t *testing.T) {
	store := &fakeAdminStore{
		channelTestResult: &admin.ChannelTestResult{
			Success: true,
			Latency: 42,
			Balance: &admin.ChannelBalance{
				Amount:   8.5,
				Currency: "USD",
				Source:   "provider_balance",
			},
			Health: &admin.ChannelHealthDetail{Status: "online", CheckedAt: time.Now().UTC()},
		},
	}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/refresh-balance", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	recorder := httptest.NewRecorder()

	handler.refreshChannelBalance(recorder, request, "ch_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("refresh balance expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.channelDiagnostics == nil || store.channelDiagnostics.Balance == nil || store.channelDiagnostics.Balance.Amount != 8.5 {
		t.Fatalf("expected balance diagnostics to be persisted, got %#v", store.channelDiagnostics)
	}
	if !strings.Contains(recorder.Body.String(), `"balance"`) || !strings.Contains(recorder.Body.String(), `"amount":8.5`) {
		t.Fatalf("expected balance refresh response, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerListsChannelRuntimeStats(t *testing.T) {
	until := time.Date(2026, 6, 4, 12, 30, 0, 0, time.UTC)
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store, admin.WithChannelRuntimeStatsProvider(fakeRuntimeStatsProvider{
		stats: map[string]*relaytypes.ChannelStats{
			"ch_1": {
				ChannelID:        "ch_1",
				RPMCurrent:       7,
				TPMCurrent:       321,
				TotalRequests:    12,
				SuccessCount:     10,
				FailureCount:     2,
				LatencySumUs:     900_000,
				LatencyCount:     3,
				RateLimitedUntil: until,
			},
		},
	})))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channels/stats", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	recorder := httptest.NewRecorder()

	handler.listChannelRuntimeStats(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("runtime stats expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"channelID":"ch_1"`) ||
		!strings.Contains(recorder.Body.String(), `"rpmCurrent":7`) ||
		!strings.Contains(recorder.Body.String(), `"tpmCurrent":321`) ||
		!strings.Contains(recorder.Body.String(), `"avgLatencyMs":300`) ||
		!strings.Contains(recorder.Body.String(), `"rateLimitedUntil":"2026-06-04T12:30:00Z"`) {
		t.Fatalf("runtime stats response missing expected fields: %s", recorder.Body.String())
	}
	if store.channelFilter.OrganizationID != "org_1" {
		t.Fatalf("expected runtime stats to use active organization scope, got %#v", store.channelFilter)
	}
}

func TestAdminChannelOperatorRoutesDispatchThroughRouter(t *testing.T) {
	session := routeSurfaceAdminSession()
	store := &fakeAdminStore{
		currentChannelModels: []string{"gpt-4o", "legacy-model"},
		channelTestResult: &admin.ChannelTestResult{
			Success: true,
			Latency: 37,
			Models:  []string{"gpt-4o", "gpt-4.1"},
			Balance: &admin.ChannelBalance{Amount: 12.5, Currency: "USD", Source: "provider_balance"},
			Health:  &admin.ChannelHealthDetail{Status: "online", CheckedAt: time.Date(2026, 6, 4, 12, 30, 0, 0, time.UTC)},
		},
	}
	service := admin.NewService(store, admin.WithChannelRuntimeStatsProvider(fakeRuntimeStatsProvider{
		stats: map[string]*relaytypes.ChannelStats{
			"ch_1": {
				ChannelID:     "ch_1",
				RPMCurrent:    4,
				TPMCurrent:    128,
				TotalRequests: 9,
				SuccessCount:  8,
				FailureCount:  1,
			},
		},
	}))
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AdminService: service,
		AuthStore:    stubAuthStore{session: session},
	})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected []string
	}{
		{"stats", stdhttp.MethodGet, "/api/v1/admin/channels/stats", "", []string{`"channelID":"ch_1"`, `"rpmCurrent":4`}},
		{"sync models", stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/sync-models", "", []string{`"testResult"`, `"models":["gpt-4o","gpt-4.1"]`}},
		{"detect model updates", stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/model-updates/detect", "", []string{`"added":["gpt-4.1"]`, `"removed":["legacy-model"]`}},
		{"apply model updates", stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/model-updates/apply", `{"mode":"replace"}`, []string{`"mode":"replace"`, `"appliedModels":["gpt-4o","gpt-4.1"]`}},
		{"refresh balance", stdhttp.MethodPost, "/api/v1/admin/channels/ch_1/refresh-balance", "", []string{`"balance"`, `"amount":12.5`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if tt.method != stdhttp.MethodGet {
				addCSRF(request, csrfToken)
			}
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected %s %s to dispatch with 200, got %d: %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
			body := recorder.Body.String()
			for _, expected := range tt.expected {
				if !strings.Contains(body, expected) {
					t.Fatalf("expected response to contain %s, got %s", expected, body)
				}
			}
		})
	}
	if !equalStrings(store.updatedChannelModels, []string{"gpt-4o", "gpt-4.1"}) {
		t.Fatalf("expected apply route to persist replaced models, got %#v", store.updatedChannelModels)
	}
	if store.channelDiagnostics == nil || store.channelDiagnostics.Balance == nil || store.channelDiagnostics.Balance.Amount != 12.5 {
		t.Fatalf("expected refresh-balance route to persist diagnostics, got %#v", store.channelDiagnostics)
	}
}

func TestAdminHandlerUpdatesRelayPricingSettings(t *testing.T) {
	store := &fakeAdminStore{}
	var applied admin.RelayPricingSettings
	handler := newAdminHandler(admin.NewService(store, admin.WithRelayPricingSettingsApplier(func(settings admin.RelayPricingSettings) {
		applied = settings
	})))

	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/settings/relay-pricing", strings.NewReader(`{
		"modelMultipliers": {"gpt-4o": 1.5},
		"groupMultipliers": {"vip": 0.8}
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	recorder := httptest.NewRecorder()

	handler.updateRelayPricingSettings(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected settings update 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.relayPricingSettings.ModelMultipliers["gpt-4o"] != 1.5 || store.relayPricingSettings.GroupMultipliers["vip"] != 0.8 {
		t.Fatalf("store relay pricing settings not updated: %#v", store.relayPricingSettings)
	}
	if applied.ModelMultipliers["gpt-4o"] != 1.5 || applied.GroupMultipliers["vip"] != 0.8 {
		t.Fatalf("relay pricing settings not applied: %#v", applied)
	}
	if !strings.Contains(recorder.Body.String(), `"modelMultipliers"`) || !strings.Contains(recorder.Body.String(), `"groupMultipliers"`) {
		t.Fatalf("response should include settings maps, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerManagesUsageLimitSettings(t *testing.T) {
	quotaService := &fakeAdminQuotaSettingsService{
		settings: []quota.UsageLimitSettings{{
			OrganizationID:        "org_admin",
			QuotaMode:             "organization",
			MaxConcurrentRequests: 10,
			WindowSeconds:         60,
			MaxTokensPerWindow:    1000,
			MaxTokensPerRequest:   250,
		}},
	}
	handler := newAdminHandlerWithQuota(admin.NewService(&fakeAdminStore{}), quotaService)
	adminSession := testAdminSession()
	adminSession.OrganizationID = "org_admin"

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/settings/usage-limits", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, adminSession))
	listRecorder := httptest.NewRecorder()
	handler.listUsageLimitSettings(listRecorder, listRequest)

	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage limit list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if quotaService.listOrganizationID != "org_admin" {
		t.Fatalf("expected list to use session organization org_admin, got %q", quotaService.listOrganizationID)
	}
	if !strings.Contains(listRecorder.Body.String(), `"maxConcurrentRequests":10`) ||
		!strings.Contains(listRecorder.Body.String(), `"maxTokensPerWindow":1000`) ||
		!strings.Contains(listRecorder.Body.String(), `"maxTokensPerRequest":250`) ||
		!strings.Contains(listRecorder.Body.String(), `"quotaMode":"organization"`) {
		t.Fatalf("expected usage limit settings in response, got %s", listRecorder.Body.String())
	}

	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits", strings.NewReader(`{
		"userId": "user_1",
		"quotaMode": "user",
		"maxConcurrentRequests": 3,
		"windowSeconds": 30,
		"maxTokensPerWindow": 300,
		"maxTokensPerRequest": 75
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, adminSession))
	updateRecorder := httptest.NewRecorder()
	handler.updateUsageLimitSettings(updateRecorder, updateRequest)

	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage limit update 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if quotaService.saved.OrganizationID != "org_admin" || quotaService.saved.UserID != "user_1" {
		t.Fatalf("expected update to scope to session org and requested user, got %+v", quotaService.saved)
	}
	if quotaService.saved.QuotaMode != "user" {
		t.Fatalf("expected update to preserve quota mode, got %+v", quotaService.saved)
	}
	if quotaService.saved.MaxConcurrentRequests != 3 || quotaService.saved.WindowSeconds != 30 || quotaService.saved.MaxTokensPerWindow != 300 || quotaService.saved.MaxTokensPerRequest != 75 {
		t.Fatalf("unexpected saved usage limit settings: %+v", quotaService.saved)
	}
}

func TestAdminUsageLimitSettingsRoutePersistsWithPostgres(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "admin-usage-limits@example.com")
	promoteHTTPUserToAdmin(t, database, adminUserID)

	var adminOrganizationID string
	if err := database.QueryRow(`
		SELECT organization_id
		FROM organization_memberships
		WHERE user_id = $1 AND removed_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
	`, adminUserID).Scan(&adminOrganizationID); err != nil {
		t.Fatalf("query admin organization: %v", err)
	}

	orgBody := `{
		"organizationId": "org_spoofed",
		"quotaMode": "organization",
		"maxConcurrentRequests": 11,
		"windowSeconds": 60,
		"maxTokensPerWindow": 12000,
		"maxTokensPerRequest": 1000
	}`
	orgResponseBody := commercialDoJSON(t, router, stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits", orgBody, adminCookie, adminCSRF, stdhttp.StatusOK)
	var orgResponse struct {
		Data quota.UsageLimitSettings `json:"data"`
	}
	if err := json.Unmarshal(orgResponseBody, &orgResponse); err != nil {
		t.Fatalf("decode organization usage-limit response: %v", err)
	}
	if orgResponse.Data.OrganizationID != adminOrganizationID || orgResponse.Data.UserID != "" || orgResponse.Data.QuotaMode != "organization" {
		t.Fatalf("expected organization usage limit scoped to session org, got %+v", orgResponse.Data)
	}
	if orgResponse.Data.MaxConcurrentRequests != 11 || orgResponse.Data.WindowSeconds != 60 ||
		orgResponse.Data.MaxTokensPerWindow != 12000 || orgResponse.Data.MaxTokensPerRequest != 1000 {
		t.Fatalf("unexpected organization usage-limit response caps: %+v", orgResponse.Data)
	}

	userBody := `{
		"organizationId": "org_spoofed",
		"userId": "` + adminUserID + `",
		"quotaMode": "user",
		"maxConcurrentRequests": 4,
		"windowSeconds": 45,
		"maxTokensPerWindow": 2500,
		"maxTokensPerRequest": 250
	}`
	userResponseBody := commercialDoJSON(t, router, stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits", userBody, adminCookie, adminCSRF, stdhttp.StatusOK)
	var userResponse struct {
		Data quota.UsageLimitSettings `json:"data"`
	}
	if err := json.Unmarshal(userResponseBody, &userResponse); err != nil {
		t.Fatalf("decode user usage-limit response: %v", err)
	}
	if userResponse.Data.OrganizationID != adminOrganizationID || userResponse.Data.UserID != adminUserID || userResponse.Data.QuotaMode != "user" {
		t.Fatalf("expected user usage limit scoped to session org and requested user, got %+v", userResponse.Data)
	}
	if userResponse.Data.MaxConcurrentRequests != 4 || userResponse.Data.WindowSeconds != 45 ||
		userResponse.Data.MaxTokensPerWindow != 2500 || userResponse.Data.MaxTokensPerRequest != 250 {
		t.Fatalf("unexpected user usage-limit response caps: %+v", userResponse.Data)
	}

	var orgConcurrentOrganizationID string
	var orgConcurrentLimit int
	if err := database.QueryRow(`
		SELECT organization_id, max_concurrent_requests
		FROM concurrency_limits
		WHERE organization_id = $1 AND user_id IS NULL
	`, adminOrganizationID).Scan(&orgConcurrentOrganizationID, &orgConcurrentLimit); err != nil {
		t.Fatalf("query persisted organization concurrency limit: %v", err)
	}
	if orgConcurrentOrganizationID != adminOrganizationID || orgConcurrentLimit != 11 {
		t.Fatalf("unexpected persisted organization concurrency row: org=%q max=%d", orgConcurrentOrganizationID, orgConcurrentLimit)
	}

	var orgWindowSeconds int
	var orgTokensPerWindow int
	var orgTokensPerRequest int
	if err := database.QueryRow(`
		SELECT window_seconds, max_tokens_per_window, max_tokens_per_request
		FROM token_rate_limits
		WHERE organization_id = $1 AND user_id IS NULL
	`, adminOrganizationID).Scan(&orgWindowSeconds, &orgTokensPerWindow, &orgTokensPerRequest); err != nil {
		t.Fatalf("query persisted organization token limit: %v", err)
	}
	if orgWindowSeconds != 60 || orgTokensPerWindow != 12000 || orgTokensPerRequest != 1000 {
		t.Fatalf("unexpected persisted organization token row: window=%d maxWindow=%d maxRequest=%d", orgWindowSeconds, orgTokensPerWindow, orgTokensPerRequest)
	}

	var userConcurrentOrganizationID string
	var userConcurrentUserID string
	var userConcurrentLimit int
	if err := database.QueryRow(`
		SELECT organization_id, user_id, max_concurrent_requests
		FROM concurrency_limits
		WHERE organization_id = $1 AND user_id = $2
	`, adminOrganizationID, adminUserID).Scan(&userConcurrentOrganizationID, &userConcurrentUserID, &userConcurrentLimit); err != nil {
		t.Fatalf("query persisted user concurrency limit: %v", err)
	}
	if userConcurrentOrganizationID != adminOrganizationID || userConcurrentUserID != adminUserID || userConcurrentLimit != 4 {
		t.Fatalf("unexpected persisted user concurrency row: org=%q user=%q max=%d", userConcurrentOrganizationID, userConcurrentUserID, userConcurrentLimit)
	}

	var userWindowSeconds int
	var userTokensPerWindow int
	var userTokensPerRequest int
	if err := database.QueryRow(`
		SELECT window_seconds, max_tokens_per_window, max_tokens_per_request
		FROM token_rate_limits
		WHERE organization_id = $1 AND user_id = $2
	`, adminOrganizationID, adminUserID).Scan(&userWindowSeconds, &userTokensPerWindow, &userTokensPerRequest); err != nil {
		t.Fatalf("query persisted user token limit: %v", err)
	}
	if userWindowSeconds != 45 || userTokensPerWindow != 2500 || userTokensPerRequest != 250 {
		t.Fatalf("unexpected persisted user token row: window=%d maxWindow=%d maxRequest=%d", userWindowSeconds, userTokensPerWindow, userTokensPerRequest)
	}

	var spoofedRows int
	if err := database.QueryRow(`
		SELECT (
			SELECT COUNT(*) FROM concurrency_limits WHERE organization_id = 'org_spoofed'
		) + (
			SELECT COUNT(*) FROM token_rate_limits WHERE organization_id = 'org_spoofed'
		)
	`).Scan(&spoofedRows); err != nil {
		t.Fatalf("count spoofed usage-limit rows: %v", err)
	}
	if spoofedRows != 0 {
		t.Fatalf("expected session organization override to prevent spoofed rows, got %d", spoofedRows)
	}

	listBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/settings/usage-limits", "", adminCookie, "", stdhttp.StatusOK)
	var listResponse struct {
		Data struct {
			UsageLimits []quota.UsageLimitSettings `json:"usageLimits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &listResponse); err != nil {
		t.Fatalf("decode usage-limit list response: %v", err)
	}
	if len(listResponse.Data.UsageLimits) != 2 {
		t.Fatalf("expected organization and user usage limits in list, got %+v", listResponse.Data.UsageLimits)
	}
	var sawOrganization bool
	var sawUser bool
	for _, item := range listResponse.Data.UsageLimits {
		switch {
		case item.UserID == "":
			sawOrganization = item.OrganizationID == adminOrganizationID &&
				item.QuotaMode == "organization" &&
				item.MaxConcurrentRequests == 11 &&
				item.WindowSeconds == 60 &&
				item.MaxTokensPerWindow == 12000 &&
				item.MaxTokensPerRequest == 1000
		case item.UserID == adminUserID:
			sawUser = item.OrganizationID == adminOrganizationID &&
				item.QuotaMode == "user" &&
				item.MaxConcurrentRequests == 4 &&
				item.WindowSeconds == 45 &&
				item.MaxTokensPerWindow == 2500 &&
				item.MaxTokensPerRequest == 250
		default:
			t.Fatalf("unexpected usage-limit row in response: %+v", item)
		}
	}
	if !sawOrganization || !sawUser {
		t.Fatalf("expected persisted organization and user usage limits, got %+v", listResponse.Data.UsageLimits)
	}
}

func TestAdminHandlerUpdateUserQuotaValidatesAndAudits(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))
	adminSession := testAdminSession()

	request := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/admin/users/user_1", strings.NewReader(`{"balance":42.5}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, adminSession))
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.2")
	recorder := httptest.NewRecorder()

	handler.updateUserQuota(recorder, request, "user_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected quota update 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.updatedQuotaUserID != "user_1" || store.updatedQuotaBalance != 42.5 {
		t.Fatalf("expected quota update forwarded to store, got user=%q balance=%f", store.updatedQuotaUserID, store.updatedQuotaBalance)
	}
	if len(store.auditEntries) != 1 {
		t.Fatalf("expected one quota audit entry, got %#v", store.auditEntries)
	}
	entry := store.auditEntries[0]
	if entry.ActorID != adminSession.User.ID || entry.ActorEmail != adminSession.User.Email ||
		entry.Action != "user.quota.update" || entry.ResourceID != "user_1" ||
		entry.IPAddress != "203.0.113.10" || !strings.Contains(entry.Changes, `"balance":42.5`) {
		t.Fatalf("unexpected quota audit entry: %#v", entry)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"user_1"`) ||
		!strings.Contains(recorder.Body.String(), `"quotaBalance":42.5`) {
		t.Fatalf("expected updated user detail response, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerUpdateUserQuotaRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		body          string
		withSession   bool
		missingUserID string
		wantStatus    int
		wantMessage   string
	}{
		{
			name:        "missing session",
			userID:      "user_1",
			body:        `{"balance":42.5}`,
			wantStatus:  stdhttp.StatusUnauthorized,
			wantMessage: "authentication required",
		},
		{
			name:        "negative balance",
			userID:      "user_1",
			body:        `{"balance":-1}`,
			withSession: true,
			wantStatus:  stdhttp.StatusBadRequest,
			wantMessage: "balance must be a non-negative finite number",
		},
		{
			name:        "missing balance",
			userID:      "user_1",
			body:        `{}`,
			withSession: true,
			wantStatus:  stdhttp.StatusBadRequest,
			wantMessage: "balance is required",
		},
		{
			name:          "missing user",
			userID:        "missing_user",
			body:          `{"balance":10}`,
			withSession:   true,
			missingUserID: "missing_user",
			wantStatus:    stdhttp.StatusNotFound,
			wantMessage:   "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeAdminStore{missingUserID: tt.missingUserID}
			handler := newAdminHandler(admin.NewService(store))
			request := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/admin/users/"+tt.userID, strings.NewReader(tt.body))
			if tt.withSession {
				request = request.WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
			}
			recorder := httptest.NewRecorder()

			handler.updateUserQuota(recorder, request, tt.userID)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantMessage) {
				t.Fatalf("expected response to contain %q, got %s", tt.wantMessage, recorder.Body.String())
			}
			if store.updatedQuotaUserID != "" {
				t.Fatalf("invalid quota update should not reach store, got user=%q balance=%f", store.updatedQuotaUserID, store.updatedQuotaBalance)
			}
			if len(store.auditEntries) != 0 {
				t.Fatalf("invalid quota update should not audit, got %#v", store.auditEntries)
			}
		})
	}
}

func TestAdminUserQuotaRoutePersistsWithPostgres(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "admin-user-quota-admin@example.com")
	promoteHTTPUserToAdmin(t, database, adminUserID)
	_, _, targetUserID := registerHTTPUser(t, router, "admin-user-quota-target@example.com")

	request := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/admin/users/"+targetUserID, strings.NewReader(`{"balance":2500.75}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", "203.0.113.25, 198.51.100.2")
	request.AddCookie(adminCookie)
	addCSRF(request, adminCSRF)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("admin user quota update expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data admin.UserDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode admin user quota response: %v", err)
	}
	if response.Data.ID != targetUserID || response.Data.QuotaBalance != 2500.75 || response.Data.Status != "active" {
		t.Fatalf("unexpected admin user quota response: %+v", response.Data)
	}

	var quotaOrganizationID string
	var quotaUserID string
	var quotaScope string
	var quotaBalance float64
	var quotaUsed float64
	if err := database.QueryRow(`
		SELECT organization_id, user_id, scope, balance, used
		FROM quotas
		WHERE user_id = $1 AND scope = 'user'
	`, targetUserID).Scan(&quotaOrganizationID, &quotaUserID, &quotaScope, &quotaBalance, &quotaUsed); err != nil {
		t.Fatalf("query persisted user quota: %v", err)
	}
	if quotaUserID != targetUserID || quotaScope != "user" || quotaBalance != 2500.75 || quotaUsed != 0 {
		t.Fatalf("unexpected persisted quota row: org=%q user=%q scope=%q balance=%.2f used=%.2f", quotaOrganizationID, quotaUserID, quotaScope, quotaBalance, quotaUsed)
	}

	var quotaRows int
	if err := database.QueryRow(`SELECT COUNT(*) FROM quotas WHERE user_id = $1 AND scope = 'user'`, targetUserID).Scan(&quotaRows); err != nil {
		t.Fatalf("count user quota rows: %v", err)
	}
	if quotaRows != 1 {
		t.Fatalf("expected one user-scoped quota row, got %d", quotaRows)
	}

	var membershipRows int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
	`, quotaOrganizationID, targetUserID).Scan(&membershipRows); err != nil {
		t.Fatalf("count quota organization membership rows: %v", err)
	}
	if membershipRows != 1 {
		t.Fatalf("expected quota row to target an active membership organization, got %d memberships", membershipRows)
	}

	var auditActorID string
	var auditActorEmail string
	var auditAction string
	var auditResourceType string
	var auditResourceID string
	var auditBalance string
	var auditIP string
	if err := database.QueryRow(`
		SELECT actor_id, actor_email, action, resource_type, resource_id, changes->>'balance', ip_address
		FROM audit_logs
		WHERE action = 'user.quota.update' AND resource_id = $1
	`, targetUserID).Scan(&auditActorID, &auditActorEmail, &auditAction, &auditResourceType, &auditResourceID, &auditBalance, &auditIP); err != nil {
		t.Fatalf("query quota audit row: %v", err)
	}
	if auditActorID != adminUserID ||
		auditActorEmail != "admin-user-quota-admin@example.com" ||
		auditAction != "user.quota.update" ||
		auditResourceType != "user" ||
		auditResourceID != targetUserID ||
		auditBalance != "2500.75" ||
		auditIP != "203.0.113.25" {
		t.Fatalf("unexpected quota audit row: actor=%q email=%q action=%q resource=%q/%q balance=%q ip=%q", auditActorID, auditActorEmail, auditAction, auditResourceType, auditResourceID, auditBalance, auditIP)
	}
}

func TestAdminHandlerListsUsageLogsWithFilters(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/usage-logs?organizationID=org_1&userID=user_1&apiTokenID=tok_1&channelID=ch_1&provider=openai&status=success&model=gpt-4o&featureType=workspace_chat&quotaMode=relay_billing&limit=250&offset=-1", nil)
	recorder := httptest.NewRecorder()

	handler.listUsageLogs(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage logs 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.usageLogFilter.Limit != 100 || store.usageLogFilter.Offset != 0 {
		t.Fatalf("expected clamped usage log pagination, got limit=%d offset=%d", store.usageLogFilter.Limit, store.usageLogFilter.Offset)
	}
	if store.usageLogFilter.OrganizationID != "org_1" || store.usageLogFilter.UserID != "user_1" || store.usageLogFilter.APITokenID != "tok_1" {
		t.Fatalf("expected identity filters to be passed, got %#v", store.usageLogFilter)
	}
	if store.usageLogFilter.FeatureType != "workspace_chat" || store.usageLogFilter.QuotaMode != "relay_billing" {
		t.Fatalf("expected usage classification filters to be passed, got %#v", store.usageLogFilter)
	}
	if !strings.Contains(recorder.Body.String(), `"usageLogs"`) ||
		!strings.Contains(recorder.Body.String(), `"requestId":"req_1"`) ||
		!strings.Contains(recorder.Body.String(), `"featureType":"workspace_chat"`) ||
		!strings.Contains(recorder.Body.String(), `"quotaMode":"relay_billing"`) {
		t.Fatalf("expected usage logs response payload, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerGetsUsageAnalyticsWithFilters(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/usage-analytics?organizationID=org_1&userID=user_1&apiType=chat&model=gpt-4o&channelID=ch_1&provider=openai&status=success&granularity=minute&from=2026-06-01T00:00:00Z&to=2026-06-04T00:00:00Z&limit=250", nil)
	recorder := httptest.NewRecorder()

	handler.getUsageAnalytics(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage analytics 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.usageAnalyticsFilter.OrganizationID != "org_1" || store.usageAnalyticsFilter.UserID != "user_1" {
		t.Fatalf("expected identity filters to be passed, got %#v", store.usageAnalyticsFilter)
	}
	if store.usageAnalyticsFilter.APIType != "chat" || store.usageAnalyticsFilter.Model != "gpt-4o" || store.usageAnalyticsFilter.ChannelID != "ch_1" ||
		store.usageAnalyticsFilter.Provider != "openai" || store.usageAnalyticsFilter.Status != "success" {
		t.Fatalf("expected gateway analytics filters to be passed, got %#v", store.usageAnalyticsFilter)
	}
	if store.usageAnalyticsFilter.Limit != 100 {
		t.Fatalf("expected clamped usage analytics limit, got %d", store.usageAnalyticsFilter.Limit)
	}
	if store.usageAnalyticsFilter.Granularity != "minute" {
		t.Fatalf("expected usage analytics granularity to be passed, got %q", store.usageAnalyticsFilter.Granularity)
	}
	if !store.usageAnalyticsFilter.From.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) ||
		!store.usageAnalyticsFilter.To.Equal(time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected parsed time filters, got from=%s to=%s", store.usageAnalyticsFilter.From, store.usageAnalyticsFilter.To)
	}
	if !strings.Contains(recorder.Body.String(), `"byModel"`) ||
		!strings.Contains(recorder.Body.String(), `"byFeature"`) ||
		!strings.Contains(recorder.Body.String(), `"byUser"`) ||
		!strings.Contains(recorder.Body.String(), `"byTime"`) ||
		!strings.Contains(recorder.Body.String(), `"crossDimensions"`) {
		t.Fatalf("expected four analytics dimensions, got %s", recorder.Body.String())
	}
}

func TestAdminHandlerListsAndRevokesAPITokens(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/api-tokens?organizationID=org_1&userID=user_1&status=active&userGroup=vip&search=Production&model=gpt-4o&limit=250&offset=-1", nil)
	listRecorder := httptest.NewRecorder()

	handler.listAPITokens(listRecorder, listRequest)

	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected api tokens 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if store.apiTokenFilter.Limit != 100 || store.apiTokenFilter.Offset != 0 {
		t.Fatalf("expected clamped api token pagination, got limit=%d offset=%d", store.apiTokenFilter.Limit, store.apiTokenFilter.Offset)
	}
	if store.apiTokenFilter.OrganizationID != "org_1" || store.apiTokenFilter.UserID != "user_1" || store.apiTokenFilter.Status != "active" || store.apiTokenFilter.UserGroup != "vip" {
		t.Fatalf("expected identity/status filters to be passed, got %#v", store.apiTokenFilter)
	}
	if store.apiTokenFilter.Search != "Production" || store.apiTokenFilter.Model != "gpt-4o" {
		t.Fatalf("expected search/model filters to be passed, got %#v", store.apiTokenFilter)
	}
	if !strings.Contains(listRecorder.Body.String(), `"apiTokens"`) ||
		!strings.Contains(listRecorder.Body.String(), `"tokenPrefix":"sk-oblv"`) ||
		!strings.Contains(listRecorder.Body.String(), `"userEmail":"user@example.com"`) ||
		!strings.Contains(listRecorder.Body.String(), `"userGroup":"vip"`) ||
		!strings.Contains(listRecorder.Body.String(), `"requestCount":12`) ||
		!strings.Contains(listRecorder.Body.String(), `"totalCost":1.23`) {
		t.Fatalf("expected api token operational fields in response, got %s", listRecorder.Body.String())
	}

	revokeRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/api-tokens/tok_1/revoke", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	revokeRecorder := httptest.NewRecorder()

	handler.revokeAPIToken(revokeRecorder, revokeRequest, "tok_1")

	if revokeRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected revoke 200, got %d: %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}
	if store.revokedAPITokenID != "tok_1" {
		t.Fatalf("expected token tok_1 to be revoked, got %q", store.revokedAPITokenID)
	}
	if !strings.Contains(revokeRecorder.Body.String(), `"status":"revoked"`) {
		t.Fatalf("expected revoke response status, got %s", revokeRecorder.Body.String())
	}
}

func TestAdminHandlerListsModelInventoryWithFilters(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/models?provider=openai&group=vip&status=enabled&search=gpt&sort=requestCount:desc&limit=250&offset=-1", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	recorder := httptest.NewRecorder()

	handler.listModelInventory(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected model inventory 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.modelInventoryFilter.Limit != 100 || store.modelInventoryFilter.Offset != 0 {
		t.Fatalf("expected clamped model inventory pagination, got limit=%d offset=%d", store.modelInventoryFilter.Limit, store.modelInventoryFilter.Offset)
	}
	if store.modelInventoryFilter.OrganizationID != "org_1" {
		t.Fatalf("expected model inventory to use active organization scope, got %#v", store.modelInventoryFilter)
	}
	if store.modelInventoryFilter.Provider != "openai" || store.modelInventoryFilter.Group != "vip" || store.modelInventoryFilter.Status != "enabled" || store.modelInventoryFilter.Search != "gpt" {
		t.Fatalf("expected model inventory filters to be passed, got %#v", store.modelInventoryFilter)
	}
	if store.modelInventoryFilter.Sort != "requests:desc" {
		t.Fatalf("expected model inventory sort to be normalized and passed, got %q", store.modelInventoryFilter.Sort)
	}
	if !strings.Contains(recorder.Body.String(), `"models"`) ||
		!strings.Contains(recorder.Body.String(), `"model":"gpt-4o"`) ||
		!strings.Contains(recorder.Body.String(), `"providers":["openai"]`) ||
		!strings.Contains(recorder.Body.String(), `"channelCount":1`) ||
		!strings.Contains(recorder.Body.String(), `"totalCost":1.23`) {
		t.Fatalf("expected model inventory operational fields, got %s", recorder.Body.String())
	}
}

func TestAdminRoutesRequireAdminRole(t *testing.T) {
	handler := newAdminHandler(admin.NewService(&fakeAdminStore{}))

	userSession := testAdminSession()
	userSession.ID = "session_user"
	userSession.User.Role = "user"
	userMiddleware := newTestAuthMiddleware(userSession)
	userRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channels", nil)
	addSignedSessionCookie(t, userMiddleware, userRequest, userSession)
	userRecorder := httptest.NewRecorder()
	userMiddleware.requireAdmin(stdhttp.HandlerFunc(handler.listChannels)).ServeHTTP(userRecorder, userRequest)
	if userRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("requireAdmin should reject non-admin sessions with 403, got %d: %s", userRecorder.Code, userRecorder.Body.String())
	}

	adminSession := testAdminSession()
	adminMiddleware := newTestAuthMiddleware(adminSession)
	adminRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channels", nil)
	addSignedSessionCookie(t, adminMiddleware, adminRequest, adminSession)
	adminRecorder := httptest.NewRecorder()
	adminMiddleware.requireAdmin(stdhttp.HandlerFunc(handler.listChannels)).ServeHTTP(adminRecorder, adminRequest)
	if adminRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("requireAdmin should accept admin sessions, got %d: %s", adminRecorder.Code, adminRecorder.Body.String())
	}
}

func TestAdminHandlerCoversReleaseListSurfaces(t *testing.T) {
	handler := newAdminHandler(admin.NewService(&fakeAdminStore{}))

	tests := []struct {
		name string
		path string
		call func(stdhttp.ResponseWriter, *stdhttp.Request)
	}{
		{name: "channels", path: "/api/v1/admin/channels", call: handler.listChannels},
		{name: "routes", path: "/api/v1/admin/routes", call: handler.listRoutes},
		{name: "plans", path: "/api/v1/admin/plans", call: handler.listPlans},
		{name: "users", path: "/api/v1/admin/users", call: handler.listUsers},
		{name: "audit logs", path: "/api/v1/admin/audit-logs", call: handler.listAuditLogs},
		{name: "reviews", path: "/api/v1/admin/reviews", call: handler.listReviews},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil).
				WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
			recorder := httptest.NewRecorder()

			tt.call(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("%s expected 200, got %d: %s", tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAdminHandlerRedactsChannelAPIKeysFromAuditLogResponses(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))
	session := testAdminSession()
	ctx := context.WithValue(context.Background(), sessionContextKey, session)

	createRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/channels", strings.NewReader(`{
		"name": "Secret OpenAI",
		"provider": "openai",
		"apiKey": "sk-create-secret"
	}`)).WithContext(ctx)
	createRecorder := httptest.NewRecorder()
	handler.createChannel(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create channel expected 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if store.createdChannelAPIKey != "sk-create-secret" {
		t.Fatalf("expected create store to receive raw API key, got %q", store.createdChannelAPIKey)
	}

	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/channels/ch_1", strings.NewReader(`{"apiKey":"sk-update-secret"}`)).WithContext(ctx)
	updateRecorder := httptest.NewRecorder()
	handler.updateChannel(updateRecorder, updateRequest, "ch_1")
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update channel expected 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if store.updatedChannelAPIKey == nil || *store.updatedChannelAPIKey != "sk-update-secret" {
		t.Fatalf("expected update store to receive raw API key, got %#v", store.updatedChannelAPIKey)
	}

	auditRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/audit-logs?resourceType=channel&resourceID=ch_1", nil)
	auditRecorder := httptest.NewRecorder()
	handler.listAuditLogs(auditRecorder, auditRequest)
	if auditRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("audit list expected 200, got %d: %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	body := auditRecorder.Body.String()
	if strings.Contains(body, "sk-create-secret") || strings.Contains(body, "sk-update-secret") {
		t.Fatalf("audit response leaked channel API key: %s", body)
	}
	var auditResponse struct {
		Data struct {
			Entries []admin.AuditEntry `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(auditRecorder.Body.Bytes(), &auditResponse); err != nil {
		t.Fatalf("decode audit list: %v", err)
	}
	redactedEntries := 0
	for _, entry := range auditResponse.Data.Entries {
		if strings.Contains(entry.Changes, `"apiKey":"********"`) {
			redactedEntries++
		}
	}
	if redactedEntries != 2 {
		t.Fatalf("expected both channel audit entries to redact apiKey, got %+v", auditResponse.Data.Entries)
	}
}

func TestAdminHandlerCreatesPlanWithRequestTokenCap(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))
	adminSession := testAdminSession()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/plans", strings.NewReader(`{
		"name": "Pro",
		"description": "Production plan",
		"quotaAmount": 500,
		"tokenQuota": 1000000,
		"price": 29,
		"modelAccess": ["gpt-4o"],
		"agentLimit": 10,
		"maxTokensPerRequest": 32000,
		"isPublic": true,
		"sortOrder": 1
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, adminSession))
	recorder := httptest.NewRecorder()

	handler.createPlan(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected create plan 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.createdPlan.MaxTokensPerRequest != 32000 {
		t.Fatalf("expected request token cap to reach plan store, got %+v", store.createdPlan)
	}
	if !strings.Contains(recorder.Body.String(), `"maxTokensPerRequest":32000`) {
		t.Fatalf("expected response to include request token cap, got %s", recorder.Body.String())
	}
}

func TestMarketplaceHandlerExposesPublicAndSessionOperations(t *testing.T) {
	store := &fakeMarketplaceStore{}
	handler := newMarketplaceHandler(marketplace.NewService(store, nil), nil)

	agentRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/agents/agent_1", nil)
	agentRecorder := httptest.NewRecorder()
	handler.getAgent(agentRecorder, agentRequest, "agent_1")
	if agentRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("get agent expected 200, got %d: %s", agentRecorder.Code, agentRecorder.Body.String())
	}

	installRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_1/install?versionID=ver_1", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, testAdminSession()))
	installRecorder := httptest.NewRecorder()
	handler.installAgent(installRecorder, installRequest, "agent_1")
	if installRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("install expected 201, got %d: %s", installRecorder.Code, installRecorder.Body.String())
	}
	if store.installedAgentID != "agent_1" || store.installedUserID != "user_admin" || store.installedVersionID != "ver_1" {
		t.Fatalf("install call mismatch: agent=%q user=%q version=%q", store.installedAgentID, store.installedUserID, store.installedVersionID)
	}

	categoriesRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/categories", nil)
	categoriesRecorder := httptest.NewRecorder()
	handler.listCategories(categoriesRecorder, categoriesRequest)
	if categoriesRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("categories expected 200, got %d: %s", categoriesRecorder.Code, categoriesRecorder.Body.String())
	}
	var envelope Envelope
	if err := json.Unmarshal(categoriesRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode categories envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope")
	}
}

func TestMarketplaceHandlerPublicBrowseAndSessionGuards(t *testing.T) {
	handler := newMarketplaceHandler(marketplace.NewService(&fakeMarketplaceStore{}, nil), nil)

	publicCases := []struct {
		name string
		path string
		call func(stdhttp.ResponseWriter, *stdhttp.Request)
	}{
		{
			name: "featured",
			path: "/api/v1/marketplace/featured",
			call: handler.getFeaturedAgents,
		},
		{
			name: "categories",
			path: "/api/v1/marketplace/categories",
			call: handler.listCategories,
		},
		{
			name: "curated",
			path: "/api/v1/marketplace/curated",
			call: handler.getCuratedSections,
		},
		{
			name: "search",
			path: "/api/v1/marketplace/search?query=agent",
			call: handler.searchAgents,
		},
		{
			name: "agents",
			path: "/api/v1/marketplace/agents?query=agent",
			call: handler.listAgents,
		},
		{
			name: "agent detail",
			path: "/api/v1/marketplace/agents/agent_1",
			call: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				handler.getAgent(w, r, "agent_1")
			},
		},
		{
			name: "agent reviews",
			path: "/api/v1/marketplace/agents/agent_1/reviews",
			call: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				handler.listReviews(w, r, "agent_1")
			},
		},
		{
			name: "agent versions",
			path: "/api/v1/marketplace/agents/agent_1/versions",
			call: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				handler.getAgentVersions(w, r, "agent_1")
			},
		},
		{
			name: "templates",
			path: "/api/v1/marketplace/templates",
			call: handler.listTemplates,
		},
		{
			name: "template detail",
			path: "/api/v1/marketplace/templates/tpl_1",
			call: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				handler.getTemplate(w, r, "tpl_1")
			},
		},
	}

	for _, tt := range publicCases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			tt.call(recorder, request)

			if recorder.Code == stdhttp.StatusUnauthorized {
				t.Fatalf("%s should not require a session, got 401: %s", tt.path, recorder.Body.String())
			}
		})
	}

	protectedCases := []struct {
		name   string
		method string
		path   string
		call   func(stdhttp.ResponseWriter, *stdhttp.Request)
	}{
		{
			name:   "publish",
			method: stdhttp.MethodPost,
			path:   "/api/v1/marketplace/agents",
			call:   handler.publishAgent,
		},
		{
			name:   "my agents",
			method: stdhttp.MethodGet,
			path:   "/api/v1/marketplace/my-agents",
			call:   handler.listMyAgents,
		},
		{
			name:   "installs",
			method: stdhttp.MethodGet,
			path:   "/api/v1/marketplace/installs",
			call:   handler.listInstalledAgents,
		},
		{
			name:   "publisher stats",
			method: stdhttp.MethodGet,
			path:   "/api/v1/marketplace/publisher/stats",
			call:   handler.getPublisherStats,
		},
	}

	for _, tt := range protectedCases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()

			tt.call(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("%s should reject unauthenticated requests with 401, got %d: %s", tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestMarketplaceSearchFilterIncludesRequesterScopeWhenSessionExists(t *testing.T) {
	session := testAdminSession()
	session.OrganizationID = "org_marketplace_recs"
	session.User.ID = "user_marketplace_recs"
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/search?query=invoice&sort=featured&limit=6", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))

	filter := marketplaceSearchFilter(request)

	if filter.Sort != "recommended" {
		t.Fatalf("expected featured alias to normalize to recommended, got %q", filter.Sort)
	}
	if filter.RequesterOrganizationID != "org_marketplace_recs" || filter.RequesterUserID != "user_marketplace_recs" {
		t.Fatalf("expected requester scope from session, got org=%q user=%q", filter.RequesterOrganizationID, filter.RequesterUserID)
	}
}

func TestMarketplaceSearchFilterKeepsAnonymousRequesterScopeEmpty(t *testing.T) {
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/search?query=invoice&sort=recommended", nil)

	filter := marketplaceSearchFilter(request)

	if filter.RequesterOrganizationID != "" || filter.RequesterUserID != "" {
		t.Fatalf("anonymous marketplace search must not set requester scope, got org=%q user=%q", filter.RequesterOrganizationID, filter.RequesterUserID)
	}
}

func TestMarketplaceHandlerRejectsUnconfiguredPaidInstallProviderBeforeSettlement(t *testing.T) {
	store := &fakeMarketplaceStore{
		agent: &marketplace.PublishedAgent{
			ID:             "agent_paid",
			OrganizationID: "org_publisher",
			OwnerID:        "publisher_1",
			Name:           "Paid Agent",
			Status:         "approved",
			Visibility:     "public",
			PricingType:    "one_time",
			PricingAmount:  25,
		},
	}
	settlement := &fakeMarketplaceSettlementService{}
	checkoutCreator := &fakeCheckoutCreator{}
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(settlement, checkoutCreator, stripebilling.CheckoutConfig{}, nil, nil),
	)
	session := testAdminSession()
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_paid/install?versionID=ver_1&provider=alipay", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusNotImplemented {
		t.Fatalf("expected unconfigured paid install provider to return 501, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != payment.CodeProviderNotConfigured {
		t.Fatalf("expected provider_not_configured response, got %+v", response.Error)
	}
	if settlement.createCalls != 0 {
		t.Fatalf("settlement must not be called before provider is configured, got %d calls", settlement.createCalls)
	}
	if checkoutCreator.request.PaymentIntentID != "" {
		t.Fatalf("checkout creator must not be called for unconfigured provider, got %+v", checkoutCreator.request)
	}
}

func TestMarketplaceHandlerUsesConfiguredPaidInstallProviderCheckoutCreator(t *testing.T) {
	store := &fakeMarketplaceStore{
		agent: &marketplace.PublishedAgent{
			ID:             "agent_paid",
			OrganizationID: "org_publisher",
			OwnerID:        "publisher_1",
			Name:           "Paid Agent",
			Status:         "approved",
			Visibility:     "public",
			PricingType:    "one_time",
			PricingAmount:  25,
		},
	}
	settlement := &fakeMarketplaceSettlementService{
		order: &marketplace.MarketplaceOrder{
			ID:                      "order_alipay",
			BuyerOrganizationID:     "org_buyer",
			BuyerUserID:             "user_admin",
			PublisherOrganizationID: "org_publisher",
			PublisherUserID:         "publisher_1",
			AgentID:                 "agent_paid",
			VersionID:               "ver_1",
			PaymentIntentID:         "pi_alipay",
			GrossAmount:             25,
			Currency:                "cny",
		},
	}
	stripeCreator := &fakeCheckoutCreator{}
	alipayCreator := &fakeCheckoutCreator{
		sessionID:  "alipay_marketplace_session",
		sessionURL: "https://checkout.alipay.test/marketplace/alipay_marketplace_session",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true, Currency: "cny"})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(
			settlement,
			stripeCreator,
			stripebilling.CheckoutConfig{},
			providerRegistry,
			map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		),
	)
	session := testAdminSession()
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_paid/install?versionID=ver_1&provider=alipay", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected configured paid install provider to return 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if settlement.createCalls != 1 {
		t.Fatalf("expected one settlement checkout call, got %d", settlement.createCalls)
	}
	if settlement.request.Provider != "alipay" || settlement.request.Currency != "cny" {
		t.Fatalf("expected settlement provider alipay/cny, got %+v", settlement.request)
	}
	if stripeCreator.request.PaymentIntentID != "" {
		t.Fatalf("stripe checkout creator must not be called for alipay, got %+v", stripeCreator.request)
	}
	if alipayCreator.request.PaymentIntentID != "pi_alipay" || alipayCreator.request.CheckoutKind != "marketplace_install" ||
		alipayCreator.request.Currency != "cny" || alipayCreator.request.AgentID != "agent_paid" {
		t.Fatalf("alipay checkout creator saw wrong marketplace request: %+v", alipayCreator.request)
	}
	if settlement.sessionID != "alipay_marketplace_session" || settlement.sessionPaymentIntentID != "pi_alipay" {
		t.Fatalf("expected alipay checkout session to be recorded, got session=%q intent=%q", settlement.sessionID, settlement.sessionPaymentIntentID)
	}
}

func TestMarketplaceGovernanceTakedownAppealAndReinstate(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "marketplace-governance-admin@example.com")
	if _, err := database.Exec(`UPDATE users SET role = 'admin' WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("mark admin user: %v", err)
	}
	publisherCookie, publisherCSRF, publisherUserID := registerHTTPUser(t, router, "marketplace-governance-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)
	buyerCookie, buyerCSRF, buyerUserID := registerHTTPUser(t, router, "marketplace-governance-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)

	insertHTTPMarketplaceAgent(t, database, "agent_governance_http", publisherUserID, publisherOrganizationID, "free", 0)

	installRecorder := httptest.NewRecorder()
	installRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_governance_http/install?versionID=version_agent_governance_http", nil)
	installRequest.AddCookie(buyerCookie)
	addCSRF(installRequest, buyerCSRF)
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("pre-takedown install expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}

	takedownRecorder := httptest.NewRecorder()
	takedownRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_governance_http/takedown", strings.NewReader(`{"reason":"policy violation"}`))
	takedownRequest.Header.Set("Content-Type", "application/json")
	takedownRequest.AddCookie(adminCookie)
	addCSRF(takedownRequest, adminCSRF)
	router.ServeHTTP(takedownRecorder, takedownRequest)
	if takedownRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("takedown expected 200, got %d with body %s", takedownRecorder.Code, takedownRecorder.Body.String())
	}

	blockedRecorder := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_governance_http/install?versionID=version_agent_governance_http", nil)
	blockedRequest.AddCookie(buyerCookie)
	addCSRF(blockedRequest, buyerCSRF)
	router.ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("post-takedown install expected 400, got %d with body %s", blockedRecorder.Code, blockedRecorder.Body.String())
	}

	appealRecorder := httptest.NewRecorder()
	appealRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_governance_http/appeal", strings.NewReader(`{"reason":"fixed the issue"}`))
	appealRequest.Header.Set("Content-Type", "application/json")
	appealRequest.AddCookie(publisherCookie)
	addCSRF(appealRequest, publisherCSRF)
	router.ServeHTTP(appealRecorder, appealRequest)
	if appealRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("appeal expected 200, got %d with body %s", appealRecorder.Code, appealRecorder.Body.String())
	}

	reinstateRecorder := httptest.NewRecorder()
	reinstateRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_governance_http/reinstate", strings.NewReader(`{"reason":"appeal accepted"}`))
	reinstateRequest.Header.Set("Content-Type", "application/json")
	reinstateRequest.AddCookie(adminCookie)
	addCSRF(reinstateRequest, adminCSRF)
	router.ServeHTTP(reinstateRecorder, reinstateRequest)
	if reinstateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("reinstate expected 200, got %d with body %s", reinstateRecorder.Code, reinstateRecorder.Body.String())
	}

	var status string
	var installCount, takedownCount, appealCount, reinstateCount int
	if err := database.QueryRow(`SELECT status FROM published_agents WHERE id = 'agent_governance_http'`).Scan(&status); err != nil {
		t.Fatalf("query governance agent status: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_governance_http' AND organization_id = $1`, buyerOrganizationID).Scan(&installCount); err != nil {
		t.Fatalf("count preserved installs: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_governance_http' AND action = 'takedown'`).Scan(&takedownCount); err != nil {
		t.Fatalf("count takedown events: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_governance_http' AND action = 'appeal'`).Scan(&appealCount); err != nil {
		t.Fatalf("count appeal events: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_governance_http' AND action = 'reinstate'`).Scan(&reinstateCount); err != nil {
		t.Fatalf("count reinstate events: %v", err)
	}
	if status != "approved" || installCount != 1 || takedownCount != 1 || appealCount != 1 || reinstateCount != 1 {
		t.Fatalf("expected approved agent with preserved install and governance events, got status=%s installs=%d takedown=%d appeal=%d reinstate=%d", status, installCount, takedownCount, appealCount, reinstateCount)
	}
}

func TestMarketplaceAbuseReportLifecycle(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "marketplace-abuse-admin@example.com")
	if _, err := database.Exec(`UPDATE users SET role = 'admin' WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("mark admin user: %v", err)
	}
	reporterCookie, reporterCSRF, _ := registerHTTPUser(t, router, "marketplace-abuse-reporter@example.com")
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-abuse-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	insertHTTPMarketplaceAgent(t, database, "agent_abuse_http", publisherUserID, publisherOrganizationID, "free", 0)

	reportRecorder := httptest.NewRecorder()
	reportRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_abuse_http/abuse-reports", strings.NewReader(`{"reason":"malware","details":"attempted credential exfiltration"}`))
	reportRequest.Header.Set("Content-Type", "application/json")
	reportRequest.AddCookie(reporterCookie)
	addCSRF(reportRequest, reporterCSRF)
	router.ServeHTTP(reportRecorder, reportRequest)
	if reportRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("abuse report expected 201, got %d with body %s", reportRecorder.Code, reportRecorder.Body.String())
	}
	var reportResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reportRecorder.Body.Bytes(), &reportResponse); err != nil {
		t.Fatalf("decode report response: %v", err)
	}
	if reportResponse.Data.ID == "" {
		t.Fatal("expected abuse report id")
	}

	resolveRecorder := httptest.NewRecorder()
	resolveRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/"+reportResponse.Data.ID+"/resolve", strings.NewReader(`{"resolution":"agent removed"}`))
	resolveRequest.Header.Set("Content-Type", "application/json")
	resolveRequest.AddCookie(adminCookie)
	addCSRF(resolveRequest, adminCSRF)
	router.ServeHTTP(resolveRecorder, resolveRequest)
	if resolveRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("resolve abuse report expected 200, got %d with body %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	var status, resolution string
	var reportEvents, resolveEvents int
	if err := database.QueryRow(`SELECT status, COALESCE(resolution, '') FROM marketplace_abuse_reports WHERE id = $1`, reportResponse.Data.ID).Scan(&status, &resolution); err != nil {
		t.Fatalf("query abuse report: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_abuse_http' AND action = 'abuse_report'`).Scan(&reportEvents); err != nil {
		t.Fatalf("count abuse report events: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_abuse_http' AND action = 'abuse_resolve'`).Scan(&resolveEvents); err != nil {
		t.Fatalf("count abuse resolve events: %v", err)
	}
	if status != "resolved" || resolution != "agent removed" || reportEvents != 1 || resolveEvents != 1 {
		t.Fatalf("expected resolved abuse report with events, got status=%s resolution=%q reportEvents=%d resolveEvents=%d", status, resolution, reportEvents, resolveEvents)
	}
}

func TestAdminMarketplaceListsOpenAbuseReports(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, _, adminUserID := registerHTTPUser(t, router, "marketplace-abuse-list-admin@example.com")
	if _, err := database.Exec(`UPDATE users SET role = 'admin' WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("mark admin user: %v", err)
	}
	reporterCookie, reporterCSRF, reporterUserID := registerHTTPUser(t, router, "marketplace-abuse-list-reporter@example.com")
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-abuse-list-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	insertHTTPMarketplaceAgent(t, database, "agent_abuse_list_http", publisherUserID, publisherOrganizationID, "free", 0)

	createReport := func(reason, details string) string {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_abuse_list_http/abuse-reports", strings.NewReader(`{"reason":"`+reason+`","details":"`+details+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(reporterCookie)
		addCSRF(request, reporterCSRF)
		router.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusCreated {
			t.Fatalf("create abuse report expected 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode created report: %v", err)
		}
		return response.Data.ID
	}

	oldReportID := createReport("spam", "old report")
	if _, err := database.Exec(`UPDATE marketplace_abuse_reports SET created_at = NOW() - INTERVAL '2 hours', updated_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, oldReportID); err != nil {
		t.Fatalf("age old report: %v", err)
	}
	resolvedReportID := createReport("phishing", "resolved report")
	if _, err := database.Exec(`UPDATE marketplace_abuse_reports SET status = 'resolved', resolution = 'handled', reviewer_user_id = $2, updated_at = NOW(), resolved_at = NOW() WHERE id = $1`, resolvedReportID, adminUserID); err != nil {
		t.Fatalf("resolve report directly: %v", err)
	}
	newReportID := createReport("malware", "new report")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/marketplace/abuse-reports?status=open&limit=1&offset=1", nil)
	request.AddCookie(adminCookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("list abuse reports expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Reports []marketplace.AbuseReport `json:"reports"`
			Total   int                       `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if response.Data.Total != 1 || len(response.Data.Reports) != 1 {
		t.Fatalf("expected one paginated open report, got total=%d reports=%d", response.Data.Total, len(response.Data.Reports))
	}
	report := response.Data.Reports[0]
	if report.ID != oldReportID || report.AgentID != "agent_abuse_list_http" || report.ReporterUserID != reporterUserID || report.Reason != "spam" || report.Details != "old report" || report.Status != "open" {
		t.Fatalf("expected old open report with public triage fields, got %+v; newest was %s", report, newReportID)
	}
	if report.CreatedAt.IsZero() || report.UpdatedAt.IsZero() {
		t.Fatalf("expected createdAt and updatedAt to be populated, got %+v", report)
	}
}

func TestMarketplacePublisherStatsIncludesSettlementAmounts(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	publisherCookie, _, publisherUserID := registerHTTPUser(t, router, "marketplace-stats-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)
	_, _, buyerUserID := registerHTTPUser(t, router, "marketplace-stats-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)

	insertHTTPMarketplaceAgent(t, database, "agent_stats_http", publisherUserID, publisherOrganizationID, "one_time", 100)
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ('pi_stats_http', 'stripe', $1, $2, 'marketplace_install', 100, 'usd', 'completed', '{}', NOW(), NOW())
	`, buyerOrganizationID, buyerUserID); err != nil {
		t.Fatalf("insert stats payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_orders (
			id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
			agent_id, version_id, payment_intent_id, gross_amount, platform_fee_amount,
			publisher_net_amount, refunded_amount, currency, status, created_at, updated_at, paid_at
		)
		VALUES ('order_stats_http', $1, $2, $3, $4, 'agent_stats_http', 'version_agent_stats_http',
		        'pi_stats_http', 100, 20, 80, 10, 'usd', 'partially_refunded', NOW(), NOW(), NOW())
	`, buyerOrganizationID, buyerUserID, publisherOrganizationID, publisherUserID); err != nil {
		t.Fatalf("insert stats marketplace order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_settlements (
			id, order_id, publisher_organization_id, publisher_user_id, agent_id,
			gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			status, hold_until, created_at, updated_at
		)
		VALUES ('settlement_stats_http', 'order_stats_http', $1, $2, 'agent_stats_http',
		        100, 20, 80, 10, 'available', NOW(), NOW(), NOW())
	`, publisherOrganizationID, publisherUserID); err != nil {
		t.Fatalf("insert stats marketplace settlement: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/publisher/stats", nil)
	request.AddCookie(publisherCookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("publisher stats expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			GrossRevenue            float64 `json:"grossRevenue"`
			PlatformFees            float64 `json:"platformFees"`
			NetRevenue              float64 `json:"netRevenue"`
			RefundedAmount          float64 `json:"refundedAmount"`
			AvailableAmount         float64 `json:"availableAmount"`
			PendingSettlementAmount float64 `json:"pendingSettlementAmount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode publisher stats: %v", err)
	}
	if response.Data.GrossRevenue != 100 || response.Data.PlatformFees != 20 || response.Data.NetRevenue != 80 || response.Data.RefundedAmount != 10 || response.Data.AvailableAmount != 70 || response.Data.PendingSettlementAmount != 0 {
		t.Fatalf("unexpected settlement-backed stats: %+v", response.Data)
	}
}

func insertHTTPMarketplaceAgent(t *testing.T, database *sql.DB, agentID, publisherUserID, publisherOrganizationID, pricingType string, pricingAmount float64) {
	t.Helper()
	if _, err := database.Exec(`
			INSERT INTO published_agents (
				id, owner_id, organization_id, name, description, category_id, tools, example_conversations,
				visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'Phase 19 HTTP test agent.',
			        'cat_productivity', '{"tools":[{"name":"governance"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
			        $5, $6, 0, 0, 0, NOW(), NOW())
	`, agentID, publisherUserID, publisherOrganizationID, agentID, pricingType, pricingAmount); err != nil {
		t.Fatalf("insert marketplace agent %s: %v", agentID, err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ($1, $2, $3, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, "version_"+agentID, agentID, publisherOrganizationID); err != nil {
		t.Fatalf("insert marketplace version for %s: %v", agentID, err)
	}
}

func newTestAuthMiddleware(session auth.Session) authMiddleware {
	return newAuthMiddleware(config.Config{
		SessionCookieName:   "oblivious_session",
		SessionCookieSecure: false,
		SessionSecret:       "test-secret",
	}, auth.NewService(stubAuthStore{session: session}))
}

func addSignedSessionCookie(t *testing.T, middleware authMiddleware, request *stdhttp.Request, session auth.Session) {
	t.Helper()

	recorder := httptest.NewRecorder()
	middleware.setSessionCookie(recorder, session)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one signed session cookie, got %d", len(cookies))
	}
	request.AddCookie(cookies[0])
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testAdminSession() auth.Session {
	return auth.Session{
		ID: "session_admin",
		User: auth.User{
			ID:    "user_admin",
			Email: "admin@example.com",
			Name:  "Admin",
			Role:  "admin",
		},
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		ExpiresAt:      time.Now().Add(time.Hour),
	}
}

type fakeAdminStore struct {
	channelFilter               admin.ChannelFilter
	usageLogFilter              admin.UsageLogFilter
	usageAnalyticsFilter        admin.UsageAnalyticsFilter
	apiTokenFilter              admin.APITokenFilter
	modelInventoryFilter        admin.ModelInventoryFilter
	billingFilter               admin.BillingInspectionFilter
	pendingReviews              []*marketplace.PublishedAgent
	batchAction                 string
	routeUpdate                 admin.RouteUpdateRequest
	approvedAgentID             string
	claimedAgentID              string
	claimedReviewerID           string
	needsChangesAgentID         string
	needsChangesReason          string
	revokedAPITokenID           string
	relayPricingSettings        admin.RelayPricingSettings
	channelTestResult           *admin.ChannelTestResult
	currentChannelModels        []string
	updatedChannelModels        []string
	channelDiagnostics          *admin.ChannelDiagnosticsUpdate
	createdPlan                 admin.PlanCreateRequest
	recordedTopupRefundID       string
	recordedTopupRefund         admin.TopupRefundRequest
	topupRefundErr              error
	marketplacePayouts          []*admin.MarketplacePayoutInspection
	marketplacePayoutsSet       bool
	marketplacePayoutsTotal     int
	marketplaceSettlements      []*admin.MarketplaceSettlementInspection
	marketplaceSettlementsSet   bool
	marketplaceSettlementsTotal int
	updatedQuotaUserID          string
	updatedQuotaBalance         float64
	missingUserID               string
	createdChannelAPIKey        string
	updatedChannelAPIKey        *string
	auditEntries                []*admin.AuditEntry
}

func (s *fakeAdminStore) GetSystemStats(ctx context.Context) (*admin.SystemStats, error) {
	return &admin.SystemStats{}, nil
}

func (s *fakeAdminStore) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	s.updatedQuotaUserID = userID
	s.updatedQuotaBalance = balance
	return nil
}

func (s *fakeAdminStore) DeleteUser(ctx context.Context, userID string) error {
	return nil
}

func (s *fakeAdminStore) ListPendingReviews(ctx context.Context, status string) ([]*marketplace.PublishedAgent, error) {
	if s.pendingReviews != nil {
		return s.pendingReviews, nil
	}
	return []*marketplace.PublishedAgent{{ID: "agent_1", Name: "Review me", Status: "pending_review"}}, nil
}

func (s *fakeAdminStore) ApproveAgent(ctx context.Context, id string) error {
	s.approvedAgentID = id
	return nil
}

func (s *fakeAdminStore) ClaimReview(ctx context.Context, id string, reviewerID string) error {
	s.claimedAgentID = id
	s.claimedReviewerID = reviewerID
	return nil
}

func (s *fakeAdminStore) RejectAgent(ctx context.Context, id string, reason string) error {
	return nil
}

func (s *fakeAdminStore) RequestAgentChanges(ctx context.Context, id string, reason string) error {
	s.needsChangesAgentID = id
	s.needsChangesReason = reason
	return nil
}

type fakeReviewSLAEnforcer struct {
	options marketplace.ReviewSLAEnforcementOptions
	result  marketplace.ReviewSLAEnforcementResult
}

func (s *fakeReviewSLAEnforcer) EnforceReviewSLAs(ctx context.Context, options marketplace.ReviewSLAEnforcementOptions) (marketplace.ReviewSLAEnforcementResult, error) {
	s.options = options
	return s.result, nil
}

type captureReviewSLAAlertSink struct {
	events []observability.AlertEvent
}

func (s *captureReviewSLAAlertSink) Notify(_ context.Context, event observability.AlertEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *fakeAdminStore) GetRelayPricingSettings(ctx context.Context) (*admin.RelayPricingSettings, error) {
	settings := s.relayPricingSettings
	if settings.ModelMultipliers == nil {
		settings.ModelMultipliers = map[string]float64{}
	}
	if settings.GroupMultipliers == nil {
		settings.GroupMultipliers = map[string]float64{}
	}
	return &settings, nil
}

func (s *fakeAdminStore) UpdateRelayPricingSettings(ctx context.Context, settings admin.RelayPricingSettings) (*admin.RelayPricingSettings, error) {
	s.relayPricingSettings = settings
	return &settings, nil
}

type fakeAdminQuotaSettingsService struct {
	settings           []quota.UsageLimitSettings
	listOrganizationID string
	saved              quota.UsageLimitSettings
}

func (s *fakeAdminQuotaSettingsService) ListUsageLimitSettings(ctx context.Context, organizationID string) ([]quota.UsageLimitSettings, error) {
	s.listOrganizationID = organizationID
	return s.settings, nil
}

func (s *fakeAdminQuotaSettingsService) SaveUsageLimitSettings(ctx context.Context, settings quota.UsageLimitSettings) (*quota.UsageLimitSettings, error) {
	s.saved = settings
	return &settings, nil
}

func (s *fakeAdminStore) ListChannels(ctx context.Context, filter admin.ChannelFilter) ([]*admin.ChannelInfo, error) {
	s.channelFilter = filter
	return []*admin.ChannelInfo{{ID: "ch_1", OrganizationID: filter.OrganizationID, Name: "OpenAI", Provider: "openai"}}, nil
}

func (s *fakeAdminStore) GetChannel(ctx context.Context, organizationID, id string) (*admin.ChannelInfo, error) {
	return &admin.ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.currentChannelModels...)}, nil
}

func (s *fakeAdminStore) CreateChannel(ctx context.Context, input admin.ChannelCreateRequest) (*admin.ChannelInfo, error) {
	s.createdChannelAPIKey = input.APIKey
	return &admin.ChannelInfo{ID: "ch_1", Name: input.Name, Provider: input.Provider}, nil
}

func (s *fakeAdminStore) UpdateChannel(ctx context.Context, organizationID, id string, input admin.ChannelUpdateRequest) (*admin.ChannelInfo, error) {
	if input.Models != nil {
		s.updatedChannelModels = append([]string{}, (*input.Models)...)
	}
	s.updatedChannelAPIKey = input.APIKey
	return &admin.ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.updatedChannelModels...)}, nil
}

func (s *fakeAdminStore) UpdateChannelDiagnostics(ctx context.Context, organizationID, id string, input admin.ChannelDiagnosticsUpdate) (*admin.ChannelHealth, error) {
	s.channelDiagnostics = &input
	return &admin.ChannelHealth{
		ID:           id,
		Status:       input.Status,
		Latency:      input.Latency,
		Balance:      input.Balance,
		BalanceError: input.BalanceError,
		Health:       input.Health,
		Error:        input.Error,
		CheckedAt:    input.CheckedAt,
	}, nil
}

func (s *fakeAdminStore) DeleteChannel(ctx context.Context, organizationID, id string) error {
	return nil
}

func (s *fakeAdminStore) TestChannel(ctx context.Context, organizationID, id string) (*admin.ChannelTestResult, error) {
	if s.channelTestResult != nil {
		return s.channelTestResult, nil
	}
	return &admin.ChannelTestResult{Success: true, Latency: 12}, nil
}

func (s *fakeAdminStore) BatchUpdateChannels(ctx context.Context, organizationID string, ids []string, action string) error {
	s.batchAction = action
	return nil
}

type fakeRuntimeStatsProvider struct {
	stats map[string]*relaytypes.ChannelStats
}

func (p fakeRuntimeStatsProvider) GetAllStats() map[string]*relaytypes.ChannelStats {
	return p.stats
}

func (s *fakeAdminStore) ListRoutes(ctx context.Context) ([]*admin.RouteInfo, error) {
	return []*admin.RouteInfo{{ID: "route_1", Model: "gpt-4o-mini"}}, nil
}

func (s *fakeAdminStore) GetRoute(ctx context.Context, id string) (*admin.RouteInfo, error) {
	return &admin.RouteInfo{ID: id, Model: "gpt-4o-mini"}, nil
}

func (s *fakeAdminStore) CreateRoute(ctx context.Context, input admin.RouteCreateRequest) (*admin.RouteInfo, error) {
	return &admin.RouteInfo{ID: "route_1", Model: input.Model}, nil
}

func (s *fakeAdminStore) UpdateRoute(ctx context.Context, id string, input admin.RouteUpdateRequest) (*admin.RouteInfo, error) {
	s.routeUpdate = input
	model := "gpt-4o-mini"
	if input.Model != nil {
		model = *input.Model
	}
	return &admin.RouteInfo{ID: id, Model: model}, nil
}

func (s *fakeAdminStore) DeleteRoute(ctx context.Context, id string) error {
	return nil
}

func (s *fakeAdminStore) ListPlans(ctx context.Context, filter admin.PlanFilter) ([]*admin.PlanInfo, error) {
	return []*admin.PlanInfo{{ID: "plan_1", Name: "Pro", MaxTokensPerRequest: 32000, IsActive: true}}, nil
}

func (s *fakeAdminStore) GetPlan(ctx context.Context, id string) (*admin.PlanInfo, error) {
	return &admin.PlanInfo{ID: id, Name: "Pro", MaxTokensPerRequest: 32000, IsActive: true}, nil
}

func (s *fakeAdminStore) CreatePlan(ctx context.Context, input admin.PlanCreateRequest) (*admin.PlanInfo, error) {
	s.createdPlan = input
	return &admin.PlanInfo{ID: "plan_1", Name: input.Name, MaxTokensPerRequest: input.MaxTokensPerRequest, IsActive: true}, nil
}

func (s *fakeAdminStore) UpdatePlan(ctx context.Context, id string, input admin.PlanUpdateRequest) (*admin.PlanInfo, error) {
	return &admin.PlanInfo{ID: id, Name: "Pro", IsActive: true}, nil
}

func (s *fakeAdminStore) DeactivatePlan(ctx context.Context, id string) error {
	return nil
}

func (s *fakeAdminStore) ListUsers(ctx context.Context, filter admin.UserListFilter) ([]*admin.UserDetail, int, error) {
	return []*admin.UserDetail{{ID: "user_1", Email: "user@example.com", Role: "user", Status: "active"}}, 1, nil
}

func (s *fakeAdminStore) GetUserByID(ctx context.Context, id string) (*admin.UserDetail, error) {
	if s.missingUserID != "" && id == s.missingUserID {
		return nil, sql.ErrNoRows
	}
	return &admin.UserDetail{ID: id, Email: "user@example.com", Role: "user", Status: "active", QuotaBalance: s.updatedQuotaBalance}, nil
}

func (s *fakeAdminStore) UpdateUser(ctx context.Context, id string, input admin.UserUpdateRequest) (*admin.UserDetail, error) {
	return &admin.UserDetail{ID: id, Email: "user@example.com", Role: "user", Status: "active"}, nil
}

func (s *fakeAdminStore) DisableUser(ctx context.Context, id string) error {
	return nil
}

func (s *fakeAdminStore) EnableUser(ctx context.Context, id string) error {
	return nil
}

func (s *fakeAdminStore) CountUsers(ctx context.Context, filter admin.UserListFilter) (int, error) {
	return 1, nil
}

func (s *fakeAdminStore) CreateAuditEntry(ctx context.Context, entry *admin.AuditEntry) error {
	s.auditEntries = append(s.auditEntries, entry)
	return nil
}

func (s *fakeAdminStore) ListAuditEntries(ctx context.Context, filter admin.AuditFilter) ([]*admin.AuditEntry, int, error) {
	if s.auditEntries != nil {
		var entries []*admin.AuditEntry
		for _, entry := range s.auditEntries {
			if filter.Action != "" && entry.Action != filter.Action {
				continue
			}
			if filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
				continue
			}
			if filter.ResourceID != "" && entry.ResourceID != filter.ResourceID {
				continue
			}
			entries = append(entries, entry)
		}
		return entries, len(entries), nil
	}
	return []*admin.AuditEntry{{ID: "aud_1", Action: "channel.create"}}, 1, nil
}

func (s *fakeAdminStore) ListUsageLogs(ctx context.Context, filter admin.UsageLogFilter) ([]*admin.UsageLogEntry, int, error) {
	s.usageLogFilter = filter
	return []*admin.UsageLogEntry{{
		ID:               "usage_1",
		OrganizationID:   "org_1",
		UserID:           "user_1",
		APITokenID:       "tok_1",
		RequestID:        "req_1",
		APIType:          "chat",
		FeatureType:      "workspace_chat",
		QuotaMode:        "relay_billing",
		Model:            "gpt-4o",
		ChannelID:        "ch_1",
		Provider:         "openai",
		Status:           "success",
		StatusCode:       200,
		LatencyMS:        42,
		Cost:             0.42,
		ChannelCost:      0.21,
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
		CreatedAt:        time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}}, 1, nil
}

func (s *fakeAdminStore) GetUsageAnalytics(ctx context.Context, filter admin.UsageAnalyticsFilter) (admin.UsageAnalytics, error) {
	s.usageAnalyticsFilter = filter
	return admin.UsageAnalytics{
		ByModel: []admin.UsageAnalyticsBucket{{
			Dimension:    "model",
			Key:          "gpt-4o",
			RequestCount: 3,
			TotalTokens:  1200,
			TotalCost:    0.42,
		}},
		ByFeature: []admin.UsageAnalyticsBucket{{
			Dimension:    "feature",
			Key:          "chat",
			RequestCount: 3,
			TotalTokens:  1200,
			TotalCost:    0.42,
		}},
		ByUser: []admin.UsageAnalyticsBucket{{
			Dimension:    "user",
			Key:          "user_1",
			RequestCount: 3,
			TotalTokens:  1200,
			TotalCost:    0.42,
		}},
		ByTime: []admin.UsageAnalyticsBucket{{
			Dimension:    "time",
			Key:          "2026-06-01",
			RequestCount: 3,
			TotalTokens:  1200,
			TotalCost:    0.42,
			StartedAt:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		}},
		CrossDimensions: []admin.UsageAnalyticsBucket{{
			Dimension:    "model_time",
			Key:          "gpt-4o|2026-06-01",
			Primary:      "gpt-4o",
			Secondary:    "2026-06-01",
			RequestCount: 3,
			TotalTokens:  1200,
			TotalCost:    0.42,
			StartedAt:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		}},
	}, nil
}

func (s *fakeAdminStore) GetRelayUsagePriceReconciliation(ctx context.Context, filter admin.RelayUsagePriceReconciliationFilter) (*admin.RelayUsagePriceReconciliationSummary, error) {
	return &admin.RelayUsagePriceReconciliationSummary{
		CheckedRecords: 0,
		MatchedRecords: 0,
		Issues:         []admin.RelayUsagePriceReconciliationIssue{},
		Limit:          filter.Limit,
		Offset:         filter.Offset,
	}, nil
}

func (s *fakeAdminStore) ListAPITokens(ctx context.Context, filter admin.APITokenFilter) ([]*admin.APITokenEntry, int, error) {
	s.apiTokenFilter = filter
	quotaLimit := 50.0
	return []*admin.APITokenEntry{{
		ID:                 "tok_1",
		OrganizationID:     "org_1",
		UserID:             "user_1",
		UserEmail:          "user@example.com",
		Name:               "Production key",
		TokenPrefix:        "sk-oblv",
		Status:             "active",
		UserGroup:          "vip",
		ModelLimitsEnabled: true,
		ModelLimits:        []string{"gpt-4o"},
		QuotaLimit:         &quotaLimit,
		UsedQuota:          12.5,
		RequestCount:       12,
		TotalCost:          1.23,
		CreatedAt:          time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}}, 1, nil
}

func (s *fakeAdminStore) RevokeAPIToken(ctx context.Context, tokenID string) error {
	s.revokedAPITokenID = tokenID
	return nil
}

func (s *fakeAdminStore) ListModelInventory(ctx context.Context, filter admin.ModelInventoryFilter) ([]*admin.ModelInventoryEntry, int, error) {
	s.modelInventoryFilter = filter
	return []*admin.ModelInventoryEntry{{
		Model:                 "gpt-4o",
		Providers:             []string{"openai"},
		Groups:                []string{"default", "vip"},
		ChannelCount:          1,
		EnabledChannelCount:   1,
		DisabledChannelCount:  0,
		MinEstimatedCostPer1K: 0.02,
		MaxEstimatedCostPer1K: 0.02,
		AvgCostMultiplier:     1.1,
		RequestCount:          30,
		TotalCost:             1.23,
		TotalChannelCost:      0.61,
		Channels: []admin.ModelInventoryChannel{{
			ID:                 "ch_1",
			Name:               "OpenAI primary",
			Provider:           "openai",
			Groups:             []string{"default", "vip"},
			Enabled:            true,
			Priority:           10,
			EstimatedCostPer1K: 0.02,
			CostMultiplier:     1.1,
		}},
	}}, 1, nil
}

type fakeMarketplaceStore struct {
	installedAgentID   string
	installedOrgID     string
	installedUserID    string
	installedVersionID string
	agent              *marketplace.PublishedAgent
}

type fakeMarketplaceSettlementService struct {
	order                  *marketplace.MarketplaceOrder
	request                marketplace.PaidInstallCheckoutRequest
	sessionID              string
	sessionPaymentIntentID string
	failedOrderID          string
	failedPaymentIntentID  string
	failureReason          string
	createCalls            int
	setSessionCalls        int
	failCalls              int
}

func (s *fakeMarketplaceSettlementService) CreatePaidInstallCheckout(ctx context.Context, input marketplace.PaidInstallCheckoutRequest) (*marketplace.MarketplaceOrder, error) {
	s.createCalls++
	s.request = input
	if s.order != nil {
		return s.order, nil
	}
	return &marketplace.MarketplaceOrder{
		ID:                      "order_1",
		BuyerOrganizationID:     input.BuyerOrganizationID,
		BuyerUserID:             input.BuyerUserID,
		PublisherOrganizationID: "org_publisher",
		PublisherUserID:         "publisher_1",
		AgentID:                 input.AgentID,
		VersionID:               input.VersionID,
		PaymentIntentID:         "pi_marketplace",
		GrossAmount:             25,
		Currency:                firstNonEmptyString(input.Currency, "usd"),
	}, nil
}

func (s *fakeMarketplaceSettlementService) SetPaidInstallCheckoutSession(ctx context.Context, orderID, paymentIntentID, providerCheckoutSessionID string) error {
	s.setSessionCalls++
	s.sessionID = providerCheckoutSessionID
	s.sessionPaymentIntentID = paymentIntentID
	return nil
}

func (s *fakeMarketplaceSettlementService) MarkPaidInstallCheckoutFailed(ctx context.Context, orderID, paymentIntentID, reason string) error {
	s.failCalls++
	s.failedOrderID = orderID
	s.failedPaymentIntentID = paymentIntentID
	s.failureReason = reason
	return nil
}

func (s *fakeMarketplaceStore) CreateAgent(ctx context.Context, ownerID, organizationID string, input marketplace.AgentPublishRequest) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: "agent_1", OrganizationID: organizationID, OwnerID: ownerID, Name: input.Name, Status: "pending_review", Visibility: input.Visibility}, nil
}

func (s *fakeMarketplaceStore) GetAgent(ctx context.Context, id string) (*marketplace.PublishedAgent, error) {
	if s.agent != nil {
		agent := *s.agent
		if agent.ID == "" {
			agent.ID = id
		}
		return &agent, nil
	}
	return &marketplace.PublishedAgent{ID: id, OrganizationID: "org_1", OwnerID: "owner_1", Name: "Agent", Status: "approved", Visibility: "public"}, nil
}

func (s *fakeMarketplaceStore) UpdateAgent(ctx context.Context, id, organizationID string, input marketplace.AgentPublishRequest) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: id, OrganizationID: organizationID, OwnerID: "user_admin", Name: input.Name, Status: "pending_review", Visibility: input.Visibility}, nil
}

func (s *fakeMarketplaceStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	return nil
}

func (s *fakeMarketplaceStore) ListUserAgents(ctx context.Context, ownerID, organizationID string, limit, offset int) ([]*marketplace.PublishedAgent, error) {
	return []*marketplace.PublishedAgent{{ID: "agent_1", OrganizationID: organizationID, OwnerID: ownerID, Name: "Agent"}}, nil
}

func (s *fakeMarketplaceStore) ListPendingReviews(ctx context.Context, limit, offset int) ([]*marketplace.PublishedAgent, error) {
	return []*marketplace.PublishedAgent{{ID: "agent_1", Name: "Agent", Status: "pending_review"}}, nil
}

func (s *fakeMarketplaceStore) ListReviewQueue(ctx context.Context, status string, limit, offset int) ([]*marketplace.PublishedAgent, error) {
	return s.ListPendingReviews(ctx, limit, offset)
}

func (s *fakeMarketplaceStore) ApproveAgent(ctx context.Context, id, reviewerID string) error {
	return nil
}

func (s *fakeMarketplaceStore) RejectAgent(ctx context.Context, id, reviewerID, reason string) error {
	return nil
}

func (s *fakeMarketplaceStore) CreateVersion(ctx context.Context, agentID, organizationID string, version, changelog string, metadata string) (*marketplace.AgentVersion, error) {
	return &marketplace.AgentVersion{ID: "ver_1", AgentID: agentID, OrganizationID: organizationID, Version: version}, nil
}

func (s *fakeMarketplaceStore) ListVersions(ctx context.Context, agentID string) ([]*marketplace.AgentVersion, error) {
	return []*marketplace.AgentVersion{{ID: "ver_1", AgentID: agentID, Version: "1.0.0"}}, nil
}

func (s *fakeMarketplaceStore) GetVersion(ctx context.Context, agentID, version string) (*marketplace.AgentVersion, error) {
	return &marketplace.AgentVersion{ID: "ver_1", AgentID: agentID, Version: version}, nil
}

func (s *fakeMarketplaceStore) InstallAgent(ctx context.Context, agentID, userID, organizationID, versionID string) (*marketplace.AgentInstall, error) {
	s.installedAgentID = agentID
	s.installedOrgID = organizationID
	s.installedUserID = userID
	s.installedVersionID = versionID
	return &marketplace.AgentInstall{ID: "install_1", AgentID: agentID, OrganizationID: organizationID, UserID: userID}, nil
}

func (s *fakeMarketplaceStore) UninstallAgent(ctx context.Context, agentID, userID, organizationID string) error {
	return nil
}

func (s *fakeMarketplaceStore) ListUserInstalls(ctx context.Context, userID, organizationID string) ([]*marketplace.AgentInstall, error) {
	return []*marketplace.AgentInstall{{ID: "install_1", AgentID: "agent_1", OrganizationID: organizationID, UserID: userID}}, nil
}

func (s *fakeMarketplaceStore) IsInstalled(ctx context.Context, agentID, userID, organizationID string) (bool, error) {
	return true, nil
}

func (s *fakeMarketplaceStore) RecordAgentRankingSignal(ctx context.Context, agentID string, event marketplace.AgentRankingSignalEvent) error {
	return nil
}

func (s *fakeMarketplaceStore) CreateReview(ctx context.Context, userID, organizationID string, input marketplace.ReviewInput) (*marketplace.AgentReview, error) {
	return &marketplace.AgentReview{ID: "review_1", AgentID: input.AgentID, OrganizationID: organizationID, UserID: userID, Rating: input.Rating}, nil
}

func (s *fakeMarketplaceStore) UpdateReview(ctx context.Context, userID, organizationID string, input marketplace.ReviewInput) (*marketplace.AgentReview, error) {
	return &marketplace.AgentReview{ID: "review_1", AgentID: input.AgentID, OrganizationID: organizationID, UserID: userID, Rating: input.Rating}, nil
}

func (s *fakeMarketplaceStore) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*marketplace.AgentReview, error) {
	return []*marketplace.AgentReview{{ID: "review_1", AgentID: agentID, Rating: 5}}, nil
}

func (s *fakeMarketplaceStore) GetUserReview(ctx context.Context, agentID, userID, organizationID string) (*marketplace.AgentReview, error) {
	return nil, nil
}

func (s *fakeMarketplaceStore) ListCategories(ctx context.Context) ([]*marketplace.Category, error) {
	return []*marketplace.Category{{ID: "cat_1", Name: "Productivity", Slug: "productivity"}}, nil
}

func (s *fakeMarketplaceStore) GetCategoryByID(ctx context.Context, id string) (*marketplace.Category, error) {
	if id == "cat_1" {
		return &marketplace.Category{ID: "cat_1", Name: "Productivity", Slug: "productivity"}, nil
	}
	return nil, nil
}

func (s *fakeMarketplaceStore) GetCategoryBySlug(ctx context.Context, slug string) (*marketplace.Category, error) {
	return &marketplace.Category{ID: "cat_1", Name: "Productivity", Slug: slug}, nil
}

func (s *fakeMarketplaceStore) CreateTemplate(ctx context.Context, organizationID string, input marketplace.TemplateCreateRequest) (*marketplace.MarketplaceTemplate, error) {
	return &marketplace.MarketplaceTemplate{ID: "tpl_1", OrganizationID: organizationID, Type: input.Type, Name: input.Name, TemplateData: input.TemplateData, Tags: input.Tags}, nil
}

func (s *fakeMarketplaceStore) ListTemplates(ctx context.Context, filter marketplace.TemplateFilter) ([]*marketplace.MarketplaceTemplate, int, error) {
	templates := []*marketplace.MarketplaceTemplate{{ID: "tpl_1", Type: "workflow", Name: "Lead Intake", TemplateData: []byte(`{"nodes":[]}`)}}
	return templates, len(templates), nil
}

func (s *fakeMarketplaceStore) GetTemplate(ctx context.Context, id string) (*marketplace.MarketplaceTemplate, error) {
	return &marketplace.MarketplaceTemplate{ID: id, Type: "workflow", Name: "Lead Intake", TemplateData: []byte(`{"nodes":[]}`)}, nil
}

func (s *fakeMarketplaceStore) InstallTemplate(ctx context.Context, templateID, userID, organizationID string) (*marketplace.TemplateInstall, error) {
	return &marketplace.TemplateInstall{ID: "tpl_install_1", TemplateID: templateID, UserID: userID, OrganizationID: organizationID, Type: "workflow", Name: "Lead Intake", TemplateData: []byte(`{"nodes":[]}`)}, nil
}

func (s *fakeMarketplaceStore) GetPublisherSettlementPreferences(ctx context.Context, organizationID string) (*marketplace.MarketplaceSettlementPreferences, error) {
	return &marketplace.MarketplaceSettlementPreferences{Cycle: "monthly"}, nil
}

func (s *fakeMarketplaceStore) UpdatePublisherSettlementPreferences(ctx context.Context, organizationID string, cycle string) (*marketplace.MarketplaceSettlementPreferences, error) {
	return &marketplace.MarketplaceSettlementPreferences{Cycle: cycle}, nil
}

func (s *fakeMarketplaceStore) SetAgentTags(ctx context.Context, agentID string, tags []string) error {
	return nil
}

func (s *fakeMarketplaceStore) GetDB() *sql.DB {
	return nil
}
