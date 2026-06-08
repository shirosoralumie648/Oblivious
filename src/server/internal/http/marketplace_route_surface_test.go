package http

import (
	"database/sql"
	"encoding/json"
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

func TestMarketplacePublishRunsAutomatedReviewGovernance(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	publisherCookie, publisherCSRF, publisherUserID := registerHTTPUser(t, router, "marketplace-auto-review@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	agentID := commercialPostDataID(t, router, publisherCookie, publisherCSRF, "/api/v1/marketplace/agents", `{
		"name":"Automated Review Clean Agent",
		"description":"Clean marketplace agent for automated review governance evidence.",
		"tags":["review"],
		"tools":"[{\"name\":\"datetime\",\"type\":\"builtin\"}]",
		"exampleConversations":"[]",
		"systemPrompt":"Help operators summarize safe operational context.",
		"visibility":"public",
		"pricingType":"free",
		"pricingAmount":0,
		"version":"1.0.0",
		"changelog":"Initial clean version"
	}`, stdhttp.StatusCreated)

	var status string
	var organizationID string
	if err := database.QueryRow(`
		SELECT status, organization_id
		FROM published_agents
		WHERE id = $1
	`, agentID).Scan(&status, &organizationID); err != nil {
		t.Fatalf("query published agent: %v", err)
	}
	if status != "pending_review" {
		t.Fatalf("expected published agent to remain pending_review, got %q", status)
	}
	if organizationID != publisherOrganizationID {
		t.Fatalf("expected publisher organization %q, got %q", publisherOrganizationID, organizationID)
	}

	var action string
	var fromStatus string
	var toStatus string
	var reason string
	var metadata []byte
	if err := database.QueryRow(`
		SELECT action, COALESCE(from_status, ''), COALESCE(to_status, ''), COALESCE(reason, ''), metadata
		FROM marketplace_governance_events
		WHERE agent_id = $1
	`, agentID).Scan(&action, &fromStatus, &toStatus, &reason, &metadata); err != nil {
		t.Fatalf("query automated review governance event: %v", err)
	}
	if action != "automated_review_pass" || fromStatus != "pending_review" || toStatus != "pending_review" {
		t.Fatalf("expected automated review pass event, got action=%q from=%q to=%q", action, fromStatus, toStatus)
	}
	if reason != "automated review passed; awaiting manual review" {
		t.Fatalf("expected automated review pass reason, got %q", reason)
	}

	var eventMetadata struct {
		Decision string `json:"decision"`
		Scanner  string `json:"scanner"`
	}
	if err := json.Unmarshal(metadata, &eventMetadata); err != nil {
		t.Fatalf("decode automated review metadata: %v", err)
	}
	if eventMetadata.Decision != "pending_manual_review" || eventMetadata.Scanner == "" {
		t.Fatalf("expected pending manual review metadata with scanner, got %+v", eventMetadata)
	}
}
