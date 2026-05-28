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
)

func TestAdminHandlerExposesPhase31Operations(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channels?limit=200&offset=3", nil)
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
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			tt.call(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("%s expected 200, got %d: %s", tt.path, recorder.Code, recorder.Body.String())
			}
		})
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
			name: "search",
			path: "/api/v1/marketplace/search?query=agent",
			call: handler.searchAgents,
		},
		{
			name: "agents",
			path: "/api/v1/marketplace/agents?query=agent",
			call: handler.listAgents,
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
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'Phase 19 HTTP test agent.',
		        '{"tools":[{"name":"governance"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
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

func testAdminSession() auth.Session {
	return auth.Session{
		ID: "session_admin",
		User: auth.User{
			ID:    "user_admin",
			Email: "admin@example.com",
			Name:  "Admin",
			Role:  "admin",
		},
		WorkspaceID: "workspace_1",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
}

type fakeAdminStore struct {
	channelFilter   admin.ChannelFilter
	batchAction     string
	routeUpdate     admin.RouteUpdateRequest
	approvedAgentID string
}

func (s *fakeAdminStore) GetSystemStats(ctx context.Context) (*admin.SystemStats, error) {
	return &admin.SystemStats{}, nil
}

func (s *fakeAdminStore) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	return nil
}

func (s *fakeAdminStore) DeleteUser(ctx context.Context, userID string) error {
	return nil
}

func (s *fakeAdminStore) ListPendingReviews(ctx context.Context) ([]*marketplace.PublishedAgent, error) {
	return []*marketplace.PublishedAgent{{ID: "agent_1", Name: "Review me", Status: "pending_review"}}, nil
}

func (s *fakeAdminStore) ApproveAgent(ctx context.Context, id string) error {
	s.approvedAgentID = id
	return nil
}

func (s *fakeAdminStore) RejectAgent(ctx context.Context, id string, reason string) error {
	return nil
}

func (s *fakeAdminStore) ListChannels(ctx context.Context, filter admin.ChannelFilter) ([]*admin.ChannelInfo, error) {
	s.channelFilter = filter
	return []*admin.ChannelInfo{{ID: "ch_1", Name: "OpenAI", Provider: "openai"}}, nil
}

func (s *fakeAdminStore) GetChannel(ctx context.Context, id string) (*admin.ChannelInfo, error) {
	return &admin.ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai"}, nil
}

func (s *fakeAdminStore) CreateChannel(ctx context.Context, input admin.ChannelCreateRequest) (*admin.ChannelInfo, error) {
	return &admin.ChannelInfo{ID: "ch_1", Name: input.Name, Provider: input.Provider}, nil
}

func (s *fakeAdminStore) UpdateChannel(ctx context.Context, id string, input admin.ChannelUpdateRequest) (*admin.ChannelInfo, error) {
	return &admin.ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai"}, nil
}

func (s *fakeAdminStore) DeleteChannel(ctx context.Context, id string) error {
	return nil
}

func (s *fakeAdminStore) TestChannel(ctx context.Context, id string) (*admin.ChannelTestResult, error) {
	return &admin.ChannelTestResult{Success: true, Latency: 12}, nil
}

func (s *fakeAdminStore) BatchUpdateChannels(ctx context.Context, ids []string, action string) error {
	s.batchAction = action
	return nil
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
	return []*admin.PlanInfo{{ID: "plan_1", Name: "Pro", IsActive: true}}, nil
}

func (s *fakeAdminStore) GetPlan(ctx context.Context, id string) (*admin.PlanInfo, error) {
	return &admin.PlanInfo{ID: id, Name: "Pro", IsActive: true}, nil
}

func (s *fakeAdminStore) CreatePlan(ctx context.Context, input admin.PlanCreateRequest) (*admin.PlanInfo, error) {
	return &admin.PlanInfo{ID: "plan_1", Name: input.Name, IsActive: true}, nil
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
	return &admin.UserDetail{ID: id, Email: "user@example.com", Role: "user", Status: "active"}, nil
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
	return nil
}

func (s *fakeAdminStore) ListAuditEntries(ctx context.Context, filter admin.AuditFilter) ([]*admin.AuditEntry, int, error) {
	return []*admin.AuditEntry{{ID: "aud_1", Action: "channel.create"}}, 1, nil
}

type fakeMarketplaceStore struct {
	installedAgentID   string
	installedOrgID     string
	installedUserID    string
	installedVersionID string
}

func (s *fakeMarketplaceStore) CreateAgent(ctx context.Context, ownerID, organizationID string, input marketplace.AgentPublishRequest) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: "agent_1", OrganizationID: organizationID, OwnerID: ownerID, Name: input.Name, Status: "pending_review", Visibility: input.Visibility}, nil
}

func (s *fakeMarketplaceStore) GetAgent(ctx context.Context, id string) (*marketplace.PublishedAgent, error) {
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

func (s *fakeMarketplaceStore) GetCategoryBySlug(ctx context.Context, slug string) (*marketplace.Category, error) {
	return &marketplace.Category{ID: "cat_1", Name: "Productivity", Slug: slug}, nil
}

func (s *fakeMarketplaceStore) SetAgentTags(ctx context.Context, agentID string, tags []string) error {
	return nil
}

func (s *fakeMarketplaceStore) GetDB() *sql.DB {
	return nil
}
