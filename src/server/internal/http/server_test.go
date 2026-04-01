package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	router := NewRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestAuthPlaceholderRoutes(t *testing.T) {
	router := NewRouter()

	cases := []struct {
		method string
		path   string
		route  string
	}{
		{method: stdhttp.MethodPost, path: "/api/v1/auth/login", route: "login"},
		{method: stdhttp.MethodPost, path: "/api/v1/auth/register", route: "register"},
		{method: stdhttp.MethodGet, path: "/api/v1/auth/me", route: "me"},
		{method: stdhttp.MethodPost, path: "/api/v1/auth/logout", route: "logout"},
	}

	for _, tc := range cases {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)

		router.ServeHTTP(recorder, request)

		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("%s %s expected 200, got %d", tc.method, tc.path, recorder.Code)
		}
		if recorder.Header().Get("X-Request-Id") == "" {
			t.Fatalf("%s %s expected request id header", tc.method, tc.path)
		}

		var body struct {
			OK   bool `json:"ok"`
			Data struct {
				Placeholder bool   `json:"placeholder"`
				Route       string `json:"route"`
			} `json:"data"`
			Error any `json:"error"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s invalid json: %v", tc.method, tc.path, err)
		}
		if !body.OK {
			t.Fatalf("%s %s expected ok response", tc.method, tc.path)
		}
		if !body.Data.Placeholder {
			t.Fatalf("%s %s expected placeholder flag", tc.method, tc.path)
		}
		if body.Data.Route != tc.route {
			t.Fatalf("%s %s expected route %q, got %q", tc.method, tc.path, tc.route, body.Data.Route)
		}
		if body.Error != nil {
			t.Fatalf("%s %s expected nil error, got %v", tc.method, tc.path, body.Error)
		}
	}
}
