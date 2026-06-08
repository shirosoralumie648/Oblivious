package http

import (
	"database/sql"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarketplaceRouterRegistersTemplateAndPublisherPreferenceRoutes(t *testing.T) {
	router := NewRouter(testConfig(), (*sql.DB)(nil))

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{
			name:   "list templates public route dispatches instead of falling through",
			method: stdhttp.MethodPut,
			path:   "/api/v1/marketplace/templates",
			want:   stdhttp.StatusMethodNotAllowed,
		},
		{
			name:   "template detail public route dispatches instead of falling through",
			method: stdhttp.MethodPut,
			path:   "/api/v1/marketplace/templates/tpl_route",
			want:   stdhttp.StatusMethodNotAllowed,
		},
		{
			name:   "template install route requires a session",
			method: stdhttp.MethodPost,
			path:   "/api/v1/marketplace/templates/tpl_route/install",
			want:   stdhttp.StatusUnauthorized,
		},
		{
			name:   "publisher settlement preferences GET route requires a session",
			method: stdhttp.MethodGet,
			path:   "/api/v1/marketplace/publisher/settlement-preferences",
			want:   stdhttp.StatusUnauthorized,
		},
		{
			name:   "publisher settlement preferences PUT route requires a session",
			method: stdhttp.MethodPut,
			path:   "/api/v1/marketplace/publisher/settlement-preferences",
			want:   stdhttp.StatusUnauthorized,
		},
		{
			name:   "admin abuse report list route requires admin auth without trailing slash",
			method: stdhttp.MethodGet,
			path:   "/api/v1/admin/marketplace/abuse-reports",
			want:   stdhttp.StatusUnauthorized,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != tt.want {
				t.Fatalf("expected %d for %s %s, got %d with body %s", tt.want, tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
			if recorder.Code == stdhttp.StatusNotFound && strings.Contains(recorder.Body.String(), "404 page not found") {
				t.Fatalf("%s %s fell through ServeMux instead of marketplace route registration", tt.method, tt.path)
			}
		})
	}
}
