package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/userprefs"
)

type authSessionEnvelope struct {
	Data struct {
		Preferences userprefs.Preferences `json:"preferences"`
		User        struct {
			Email string `json:"email"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Role  string `json:"role"`
		} `json:"user"`
	} `json:"data"`
}

func TestAuthResponsesExposeStableUserAndPreferenceContracts(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	email := "auth-contract@example.com"

	registerResponse, cookie := authResponseRequest(t, router, stdhttp.MethodPost, "/api/v1/auth/register", `{"email":"`+email+`","password":"StrongerPass1!"}`, nil)
	assertAuthSessionPayload(t, registerResponse, email)

	meResponse, _ := authResponseRequest(t, router, stdhttp.MethodGet, "/api/v1/auth/me", "", cookie)
	assertAuthSessionPayload(t, meResponse, email)

	loginResponse, _ := authResponseRequest(t, router, stdhttp.MethodPost, "/api/v1/auth/login", `{"email":"`+email+`","password":"StrongerPass1!"}`, nil)
	assertAuthSessionPayload(t, loginResponse, email)
}

func authResponseRequest(t *testing.T, router stdhttp.Handler, method, path, body string, cookie *stdhttp.Cookie) (authSessionEnvelope, *stdhttp.Cookie) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("%s %s expected 200, got %d with body %s", method, path, recorder.Code, recorder.Body.String())
	}

	var response authSessionEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode %s %s response: %v", method, path, err)
	}
	var sessionCookie *stdhttp.Cookie
	if cookies := recorder.Result().Cookies(); len(cookies) > 0 {
		sessionCookie = cookies[0]
	}
	return response, sessionCookie
}

func assertAuthSessionPayload(t *testing.T, response authSessionEnvelope, email string) {
	t.Helper()

	if response.Data.User.ID == "" {
		t.Fatal("expected user id")
	}
	if response.Data.User.Email != email {
		t.Fatalf("expected user email %q, got %q", email, response.Data.User.Email)
	}
	if response.Data.User.Name != email {
		t.Fatalf("expected user name to default to email %q, got %q", email, response.Data.User.Name)
	}
	if response.Data.User.Role != "user" {
		t.Fatalf("expected default role user, got %q", response.Data.User.Role)
	}
	if response.Data.Preferences.DefaultMode != "chat" {
		t.Fatalf("expected defaultMode chat, got %q", response.Data.Preferences.DefaultMode)
	}
	if response.Data.Preferences.ModelStrategy != "balanced" {
		t.Fatalf("expected modelStrategy balanced, got %q", response.Data.Preferences.ModelStrategy)
	}
	if response.Data.Preferences.DefaultAgentModel != "gpt-4o-mini" {
		t.Fatalf("expected defaultAgentModel gpt-4o-mini, got %q", response.Data.Preferences.DefaultAgentModel)
	}
	if response.Data.Preferences.SidebarCollapsed {
		t.Fatal("expected sidebarCollapsed default false")
	}
	if response.Data.Preferences.Notifications == nil || len(response.Data.Preferences.Notifications) != 0 {
		t.Fatalf("expected empty notifications map, got %#v", response.Data.Preferences.Notifications)
	}
}
