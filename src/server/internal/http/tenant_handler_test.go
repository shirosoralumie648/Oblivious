package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/tenant"
)

func TestTenantHandlerCreateListUpdateAndArchive(t *testing.T) {
	store := newFakeTenantStore()
	handler := newTenantHandler(tenant.NewService(store))

	createRequest := adminJSONRequest(stdhttp.MethodPost, "/api/v1/admin/organizations", `{"name":"Acme","slug":"acme"}`)
	createRecorder := httptest.NewRecorder()
	handler.createOrganization(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create expected 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if store.createdByUserID == nil || *store.createdByUserID != "user_admin" {
		t.Fatalf("expected created_by_user_id from session, got %v", store.createdByUserID)
	}

	listRecorder := httptest.NewRecorder()
	handler.listOrganizations(listRecorder, adminJSONRequest(stdhttp.MethodGet, "/api/v1/admin/organizations", ""))
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list expected 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	handler.updateOrganization(updateRecorder, adminJSONRequest(stdhttp.MethodPut, "/api/v1/admin/organizations/org_1", `{"name":"Acme Commercial"}`), "org_1")
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update expected 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	archiveRecorder := httptest.NewRecorder()
	handler.archiveOrganization(archiveRecorder, adminJSONRequest(stdhttp.MethodPost, "/api/v1/admin/organizations/org_1/archive", ""), "org_1")
	if archiveRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("archive expected 200, got %d: %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}

	var archived struct {
		Data tenant.Organization `json:"data"`
	}
	if err := json.Unmarshal(archiveRecorder.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archive response: %v", err)
	}
	if archived.Data.Status != tenant.StatusArchived {
		t.Fatalf("expected archived status, got %q", archived.Data.Status)
	}
}

func TestTenantHandlerRequiresSessionForCreate(t *testing.T) {
	handler := newTenantHandler(tenant.NewService(newFakeTenantStore()))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/organizations", strings.NewReader(`{"name":"Acme","slug":"acme"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createOrganization(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func adminJSONRequest(method, path, body string) *stdhttp.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		ID: "session_admin",
		User: auth.User{
			ID:    "user_admin",
			Email: "admin@example.com",
			Role:  "admin",
		},
		ExpiresAt: time.Now().Add(time.Hour),
	}))
}

type fakeTenantStore struct {
	organizations   map[string]*tenant.Organization
	createdByUserID *string
}

func newFakeTenantStore() *fakeTenantStore {
	now := time.Now().UTC()
	return &fakeTenantStore{
		organizations: map[string]*tenant.Organization{
			"org_1": {
				ID:        "org_1",
				Slug:      "acme",
				Name:      "Acme",
				Status:    tenant.StatusActive,
				Metadata:  map[string]any{},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
}

func (s *fakeTenantStore) ListOrganizations(ctx context.Context, filter tenant.OrganizationListFilter) ([]*tenant.Organization, int, error) {
	items := make([]*tenant.Organization, 0, len(s.organizations))
	for _, org := range s.organizations {
		items = append(items, org)
	}
	return items, len(items), nil
}

func (s *fakeTenantStore) GetOrganization(ctx context.Context, id string) (*tenant.Organization, error) {
	return s.organizations[id], nil
}

func (s *fakeTenantStore) CreateOrganization(ctx context.Context, organization *tenant.Organization) (*tenant.Organization, error) {
	organization.ID = "org_1"
	s.createdByUserID = organization.CreatedByUserID
	s.organizations[organization.ID] = organization
	return organization, nil
}

func (s *fakeTenantStore) UpdateOrganization(ctx context.Context, id string, input tenant.OrganizationUpdate) (*tenant.Organization, error) {
	org := s.organizations[id]
	if input.Name != nil {
		org.Name = *input.Name
	}
	if input.Status != nil {
		org.Status = *input.Status
	}
	return org, nil
}

func (s *fakeTenantStore) ArchiveOrganization(ctx context.Context, id string) (*tenant.Organization, error) {
	org := s.organizations[id]
	now := time.Now().UTC()
	org.Status = tenant.StatusArchived
	org.ArchivedAt = &now
	return org, nil
}
