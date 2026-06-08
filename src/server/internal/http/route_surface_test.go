package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

const routeSurfaceStrongPassword = "StrongerPass1!"

func TestRouteSurfaceRequiresSessionForAppRoutes(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"agents", stdhttp.MethodGet, "/api/v1/app/agents"},
		{"agent memories", stdhttp.MethodGet, "/api/v1/agent/memories"},
		{"memory documents", stdhttp.MethodGet, "/api/v1/app/memory/documents"},
		{"mcp servers", stdhttp.MethodGet, "/api/v1/app/mcp-servers"},
		{"notifications", stdhttp.MethodGet, "/api/v1/app/notifications"},
		{"quota", stdhttp.MethodGet, "/api/v1/app/quota"},
		{"console usage", stdhttp.MethodGet, "/api/v1/console/usage"},
		{"preferences", stdhttp.MethodGet, "/api/v1/app/me/preferences"},
		{"knowledge bases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases"},
		{"knowledge document upload", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"tasks", stdhttp.MethodGet, "/api/v1/app/tasks"},
		{"organizations", stdhttp.MethodGet, "/api/v1/app/organizations"},
		{"organization members", stdhttp.MethodGet, "/api/v1/app/organizations/org_1/members"},
		{"websocket", stdhttp.MethodGet, "/api/v1/ws"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceRegistersCanonicalKnowledgeRoutesThroughRegistrar(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	if !strings.Contains(string(source), "registerKnowledgeRoutes(mux, authMiddleware, knowledgeHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical app knowledge routes through registerKnowledgeRoutes")
	}
}

func TestRouteSurfaceRegistersConsoleInvoiceRoute(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	routesSource, err := os.ReadFile("routes_console.go")
	if err != nil {
		t.Fatalf("read routes_console.go: %v", err)
	}

	if !strings.Contains(string(routerSource), "registerConsoleRoutes(mux, authMiddleware, consoleHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRoutes")
	}
	if !strings.Contains(string(routesSource), `mux.Handle("/api/v1/console/invoices"`) {
		t.Fatal("expected NewRouterWithOptions to expose /api/v1/console/invoices")
	}
}

func TestRouteSurfaceRegistersConsoleAPITokenRoutes(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	routesSource, err := os.ReadFile("routes_console.go")
	if err != nil {
		t.Fatalf("read routes_console.go: %v", err)
	}
	if !strings.Contains(string(routerSource), "registerConsoleRoutes(mux, authMiddleware, consoleHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRoutes")
	}
	text := string(routesSource)

	for _, expected := range []string{
		`mux.Handle("/api/v1/console/api-tokens"`,
		`mux.Handle("/api/v1/console/api-tokens/"`,
		"consoleHandler.createAPIToken",
		"consoleHandler.listAPITokenUsage",
		"consoleHandler.revokeAPIToken",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected NewRouterWithOptions to expose Console API token route surface containing %q", expected)
		}
	}
}

func TestRouteSurfaceRequiresSessionForConsoleAPITokenRoutesWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []struct {
		method string
		path   string
	}{
		{stdhttp.MethodGet, "/api/v1/console/api-tokens"},
		{stdhttp.MethodPost, "/api/v1/console/api-tokens"},
		{stdhttp.MethodGet, "/api/v1/console/api-tokens/tok_1/usage"},
		{stdhttp.MethodDelete, "/api/v1/console/api-tokens/tok_1"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceRefreshesWorkflowHealthBeforeMetricsScrape(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	text := string(source)
	if !strings.Contains(text, `mux.Handle("/metrics", workflowMetricsHandler(workflowService))`) {
		t.Fatal("expected /metrics to use workflowMetricsHandler")
	}
	if !strings.Contains(text, "RefreshExecutionHealthMetrics") {
		t.Fatal("expected workflowMetricsHandler to refresh workflow execution health metrics before scrape")
	}
}

func TestRouteSurfaceWiresConfiguredWorkflowSystemLimits(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	serviceSource, err := os.ReadFile("workflow_service.go")
	if err != nil {
		t.Fatalf("read workflow_service.go: %v", err)
	}

	if !strings.Contains(string(routerSource), "workflowService = newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected default router workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
	}
	if !strings.Contains(string(serverSource), "workflowService := newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected server workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
	}
	serviceText := string(serviceSource)
	if !strings.Contains(serviceText, "workflow.WithSystemWorkflowLimits") ||
		!strings.Contains(serviceText, "cfg.WorkflowSystemMaxConcurrent") ||
		!strings.Contains(serviceText, "cfg.WorkflowGlobalMaxExecutionsPerMinute") ||
		!strings.Contains(serviceText, "workflow.WithFailurePauseNotificationSink") ||
		!strings.Contains(serviceText, "workflowFailurePauseAlertEvent") {
		t.Fatal("expected workflow service helper to pass configured system workflow limits, failure notification sink, and alert event adapter")
	}
}

func TestRouteSurfaceAdminRoutesRequireAdmin(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	cookie := routeSurfaceRegisterUser(t, router, "route-user@example.com")

	adminRoutes := []string{
		"/api/v1/admin/stats",
		"/api/v1/admin/channels",
		"/api/v1/admin/routes",
		"/api/v1/admin/plans",
		"/api/v1/admin/users",
		"/api/v1/admin/organizations",
		"/api/v1/admin/audit-logs",
		"/api/v1/admin/reviews",
	}

	for _, path := range adminRoutes {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, path, nil)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected 403 for normal user on %s, got %d with body %s", path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSubRoutesRequireAdminWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	for _, tt := range routeSurfaceAdminSubRouteCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for anonymous %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSubRoutesRejectNonAdminWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	for _, tt := range routeSurfaceAdminSubRouteCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected 403 for non-admin %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKeepsAuthRoutesPublic(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"public-auth@example.com","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public register to return 200, got %d with body %s", registerRecorder.Code, registerRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"public-auth@example.com","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public login to return 200, got %d with body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestRouteSurfaceRejectsCookieMutationWithoutCSRF(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	cookie, _ := routeSurfaceRegisterUserWithCSRF(t, router, "csrf-user@example.com")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(cookie)
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfacePublishingChannelMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create channel", stdhttp.MethodPost, "/api/v1/channels"},
		{"update channel", stdhttp.MethodPut, "/api/v1/channels/channel_1"},
		{"delete channel", stdhttp.MethodDelete, "/api/v1/channels/channel_1"},
		{"update channel status", stdhttp.MethodPatch, "/api/v1/channels/channel_1/status"},
		{"test channel", stdhttp.MethodPost, "/api/v1/channels/channel_1/test"},
		{"send channel message", stdhttp.MethodPost, "/api/v1/channels/channel_1/send"},
		{"retry failed messages", stdhttp.MethodPost, "/api/v1/channels/channel_1/retry-failed-messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func routeSurfaceRegisterUser(t *testing.T, router stdhttp.Handler, email string) *stdhttp.Cookie {
	t.Helper()

	cookie, _ := routeSurfaceRegisterUserWithCSRF(t, router, email)
	return cookie
}

func routeSurfaceRegisterUserWithCSRF(t *testing.T, router stdhttp.Handler, email string) (*stdhttp.Cookie, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("register test user: expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	var response struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	return cookies[0], response.Data.CSRFToken
}

type routeSurfaceCase struct {
	name   string
	method string
	path   string
}

func routeSurfaceAdminSubRouteCases() []routeSurfaceCase {
	return []routeSurfaceCase{
		{"billing summary", stdhttp.MethodGet, "/api/v1/admin/billing/summary"},
		{"billing sessions", stdhttp.MethodGet, "/api/v1/admin/billing/sessions"},
		{"billing payment intents", stdhttp.MethodGet, "/api/v1/admin/billing/payment-intents"},
		{"billing webhook events", stdhttp.MethodGet, "/api/v1/admin/billing/webhook-events"},
		{"billing subscriptions", stdhttp.MethodGet, "/api/v1/admin/billing/subscriptions"},
		{"billing topups", stdhttp.MethodGet, "/api/v1/admin/billing/topups"},
		{"billing invoices", stdhttp.MethodGet, "/api/v1/admin/billing/invoices"},
		{"billing refunds", stdhttp.MethodGet, "/api/v1/admin/billing/refunds"},
		{"billing settlements", stdhttp.MethodGet, "/api/v1/admin/billing/settlements"},
		{"billing payouts", stdhttp.MethodGet, "/api/v1/admin/billing/payouts"},
		{"billing topup refund", stdhttp.MethodPost, "/api/v1/admin/billing/topups/topup_1/refund"},
		{"billing payout paid", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/paid"},
		{"core stats", stdhttp.MethodGet, "/api/v1/admin/stats"},
		{"core routes list", stdhttp.MethodGet, "/api/v1/admin/routes"},
		{"core routes create", stdhttp.MethodPost, "/api/v1/admin/routes"},
		{"core route get", stdhttp.MethodGet, "/api/v1/admin/routes/route_1"},
		{"core route update", stdhttp.MethodPut, "/api/v1/admin/routes/route_1"},
		{"core route delete", stdhttp.MethodDelete, "/api/v1/admin/routes/route_1"},
		{"core plans list", stdhttp.MethodGet, "/api/v1/admin/plans"},
		{"core plans create", stdhttp.MethodPost, "/api/v1/admin/plans"},
		{"core plan get", stdhttp.MethodGet, "/api/v1/admin/plans/plan_1"},
		{"core plan update", stdhttp.MethodPut, "/api/v1/admin/plans/plan_1"},
		{"core plan deactivate", stdhttp.MethodDelete, "/api/v1/admin/plans/plan_1"},
		{"core users list", stdhttp.MethodGet, "/api/v1/admin/users"},
		{"core user get", stdhttp.MethodGet, "/api/v1/admin/users/user_1"},
		{"core user update", stdhttp.MethodPut, "/api/v1/admin/users/user_1"},
		{"core user quota update", stdhttp.MethodPatch, "/api/v1/admin/users/user_1"},
		{"core user delete", stdhttp.MethodDelete, "/api/v1/admin/users/user_1"},
		{"core user disable", stdhttp.MethodPost, "/api/v1/admin/users/user_1/disable"},
		{"core user enable", stdhttp.MethodPost, "/api/v1/admin/users/user_1/enable"},
		{"core audit logs", stdhttp.MethodGet, "/api/v1/admin/audit-logs"},
		{"channel provider catalog", stdhttp.MethodGet, "/api/v1/admin/channel-providers"},
		{"observability alert routing get", stdhttp.MethodGet, "/api/v1/admin/observability/alert-routing"},
		{"observability alert routing update", stdhttp.MethodPut, "/api/v1/admin/observability/alert-routing"},
		{"observability providers list", stdhttp.MethodGet, "/api/v1/admin/observability/alert-providers"},
		{"observability providers create", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers"},
		{"observability provider update", stdhttp.MethodPut, "/api/v1/admin/observability/alert-providers/provider_1"},
		{"observability provider test", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers/provider_1/test"},
		{"observability alerts list", stdhttp.MethodGet, "/api/v1/admin/observability/alerts"},
		{"observability alert get", stdhttp.MethodGet, "/api/v1/admin/observability/alerts/relay-backlog"},
		{"observability alert acknowledge", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/acknowledge"},
		{"observability alert resolve", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/resolve"},
		{"observability alert deliveries", stdhttp.MethodGet, "/api/v1/admin/observability/alerts/relay-backlog/deliveries"},
		{"observability recovery actions", stdhttp.MethodGet, "/api/v1/admin/observability/recovery-actions"},
		{"marketplace takedown", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/takedown"},
		{"marketplace reinstate", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/reinstate"},
		{"marketplace abuse reports", stdhttp.MethodGet, "/api/v1/admin/marketplace/abuse-reports"},
		{"marketplace abuse resolve", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/resolve"},
		{"marketplace abuse dismiss", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/dismiss"},
		{"marketplace review sla enforce", stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce"},
	}
}

func routeSurfaceUserSession() auth.Session {
	return auth.Session{
		ID:             "session_route_surface_user",
		OrganizationID: "org_route_surface",
		WorkspaceID:    "workspace_route_surface",
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
		User: auth.User{
			ID:    "user_route_surface",
			Email: "route-surface-user@example.com",
			Name:  "Route Surface User",
			Role:  "user",
		},
	}
}

func routeSurfaceSignedSessionCookie(t *testing.T, session auth.Session) *stdhttp.Cookie {
	t.Helper()

	middleware := newAuthMiddleware(testConfig(), auth.NewService(stubAuthStore{session: session}))
	recorder := httptest.NewRecorder()
	middleware.setSessionCookie(recorder, session)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one signed session cookie, got %d", len(cookies))
	}
	return cookies[0]
}
