package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouteSurfaceRequiresSessionForAppRoutes(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"agents", stdhttp.MethodGet, "/api/v1/app/agents"},
		{"memory documents", stdhttp.MethodGet, "/api/v1/app/memory/documents"},
		{"mcp servers", stdhttp.MethodGet, "/api/v1/app/mcp-servers"},
		{"notifications", stdhttp.MethodGet, "/api/v1/app/notifications"},
		{"quota", stdhttp.MethodGet, "/api/v1/app/quota"},
		{"console usage", stdhttp.MethodGet, "/api/v1/console/usage"},
		{"preferences", stdhttp.MethodGet, "/api/v1/app/me/preferences"},
		{"knowledge bases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases"},
		{"tasks", stdhttp.MethodGet, "/api/v1/app/tasks"},
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

func TestRouteSurfaceAdminRoutesRequireAdmin(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	cookie := routeSurfaceRegisterUser(t, router, "route-user@example.com")

	adminRoutes := []string{
		"/api/v1/admin/stats",
		"/api/v1/admin/channels",
		"/api/v1/admin/routes",
		"/api/v1/admin/plans",
		"/api/v1/admin/users",
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

func TestRouteSurfaceKeepsAuthRoutesPublic(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"public-auth@example.com","password":"secret"}`),
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
		strings.NewReader(`{"email":"public-auth@example.com","password":"secret"}`),
	)
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public login to return 200, got %d with body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func routeSurfaceRegisterUser(t *testing.T, router stdhttp.Handler, email string) *stdhttp.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"secret"}`),
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
	return cookies[0]
}
