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
	installedUserID    string
	installedVersionID string
}

func (s *fakeMarketplaceStore) CreateAgent(ctx context.Context, ownerID string, input marketplace.AgentPublishRequest) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: "agent_1", OwnerID: ownerID, Name: input.Name, Status: "pending_review", Visibility: input.Visibility}, nil
}

func (s *fakeMarketplaceStore) GetAgent(ctx context.Context, id string) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: id, OwnerID: "owner_1", Name: "Agent", Status: "approved", Visibility: "public"}, nil
}

func (s *fakeMarketplaceStore) UpdateAgent(ctx context.Context, id string, input marketplace.AgentPublishRequest) (*marketplace.PublishedAgent, error) {
	return &marketplace.PublishedAgent{ID: id, OwnerID: "user_admin", Name: input.Name, Status: "pending_review", Visibility: input.Visibility}, nil
}

func (s *fakeMarketplaceStore) DeleteAgent(ctx context.Context, id string) error {
	return nil
}

func (s *fakeMarketplaceStore) ListUserAgents(ctx context.Context, ownerID string, limit, offset int) ([]*marketplace.PublishedAgent, error) {
	return []*marketplace.PublishedAgent{{ID: "agent_1", OwnerID: ownerID, Name: "Agent"}}, nil
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

func (s *fakeMarketplaceStore) CreateVersion(ctx context.Context, agentID string, version, changelog string, metadata string) (*marketplace.AgentVersion, error) {
	return &marketplace.AgentVersion{ID: "ver_1", AgentID: agentID, Version: version}, nil
}

func (s *fakeMarketplaceStore) ListVersions(ctx context.Context, agentID string) ([]*marketplace.AgentVersion, error) {
	return []*marketplace.AgentVersion{{ID: "ver_1", AgentID: agentID, Version: "1.0.0"}}, nil
}

func (s *fakeMarketplaceStore) GetVersion(ctx context.Context, agentID, version string) (*marketplace.AgentVersion, error) {
	return &marketplace.AgentVersion{ID: "ver_1", AgentID: agentID, Version: version}, nil
}

func (s *fakeMarketplaceStore) InstallAgent(ctx context.Context, agentID, userID, versionID string) (*marketplace.AgentInstall, error) {
	s.installedAgentID = agentID
	s.installedUserID = userID
	s.installedVersionID = versionID
	return &marketplace.AgentInstall{ID: "install_1", AgentID: agentID, UserID: userID}, nil
}

func (s *fakeMarketplaceStore) UninstallAgent(ctx context.Context, agentID, userID string) error {
	return nil
}

func (s *fakeMarketplaceStore) ListUserInstalls(ctx context.Context, userID string) ([]*marketplace.AgentInstall, error) {
	return []*marketplace.AgentInstall{{ID: "install_1", AgentID: "agent_1", UserID: userID}}, nil
}

func (s *fakeMarketplaceStore) IsInstalled(ctx context.Context, agentID, userID string) (bool, error) {
	return true, nil
}

func (s *fakeMarketplaceStore) CreateReview(ctx context.Context, userID string, input marketplace.ReviewInput) (*marketplace.AgentReview, error) {
	return &marketplace.AgentReview{ID: "review_1", AgentID: input.AgentID, UserID: userID, Rating: input.Rating}, nil
}

func (s *fakeMarketplaceStore) UpdateReview(ctx context.Context, userID string, input marketplace.ReviewInput) (*marketplace.AgentReview, error) {
	return &marketplace.AgentReview{ID: "review_1", AgentID: input.AgentID, UserID: userID, Rating: input.Rating}, nil
}

func (s *fakeMarketplaceStore) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*marketplace.AgentReview, error) {
	return []*marketplace.AgentReview{{ID: "review_1", AgentID: agentID, Rating: 5}}, nil
}

func (s *fakeMarketplaceStore) GetUserReview(ctx context.Context, agentID, userID string) (*marketplace.AgentReview, error) {
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
