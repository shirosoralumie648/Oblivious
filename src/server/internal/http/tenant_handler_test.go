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
	handler := newTenantHandler(tenant.NewService(store), nil, authMiddleware{})

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
	handler := newTenantHandler(tenant.NewService(newFakeTenantStore()), nil, authMiddleware{})
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/organizations", strings.NewReader(`{"name":"Acme","slug":"acme"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createOrganization(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminOrganizationRoutesPersistWithPostgres(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminUserID := registerHTTPUser(t, router, "admin-org-routes@example.com")
	promoteHTTPUserToAdmin(t, database, adminUserID)

	createBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/organizations", `{
		"name":"Admin Managed Org",
		"slug":"admin-managed-org",
		"metadata":{"tier":"enterprise","region":"us"}
	}`, adminCookie, adminCSRF, stdhttp.StatusCreated)
	var createResponse struct {
		Data tenant.Organization `json:"data"`
	}
	if err := json.Unmarshal(createBody, &createResponse); err != nil {
		t.Fatalf("decode create organization response: %v", err)
	}
	organizationID := createResponse.Data.ID
	if organizationID == "" {
		t.Fatal("expected created organization id")
	}
	if createResponse.Data.Slug != "admin-managed-org" ||
		createResponse.Data.Name != "Admin Managed Org" ||
		createResponse.Data.Status != tenant.StatusActive ||
		createResponse.Data.CreatedByUserID == nil ||
		*createResponse.Data.CreatedByUserID != adminUserID ||
		createResponse.Data.Metadata["tier"] != "enterprise" {
		t.Fatalf("unexpected created organization response: %+v", createResponse.Data)
	}

	var storedCreatedBy string
	var storedTier string
	var storedOwnerRole string
	var storedOwnerCreatedBy string
	if err := database.QueryRow(`
		SELECT o.created_by_user_id, o.metadata->>'tier', m.role, m.created_by_user_id
		FROM organizations o
		JOIN organization_memberships m ON m.organization_id = o.id AND m.user_id = $2 AND m.removed_at IS NULL
		WHERE o.id = $1
	`, organizationID, adminUserID).Scan(&storedCreatedBy, &storedTier, &storedOwnerRole, &storedOwnerCreatedBy); err != nil {
		t.Fatalf("query created organization and owner membership: %v", err)
	}
	if storedCreatedBy != adminUserID || storedTier != "enterprise" || storedOwnerRole != tenant.RoleOwner || storedOwnerCreatedBy != adminUserID {
		t.Fatalf("unexpected stored create evidence: createdBy=%q tier=%q ownerRole=%q ownerCreatedBy=%q", storedCreatedBy, storedTier, storedOwnerRole, storedOwnerCreatedBy)
	}

	listBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/organizations?status=active&search=managed&limit=10", "", adminCookie, "", stdhttp.StatusOK)
	var listResponse struct {
		Data struct {
			Organizations []tenant.Organization `json:"organizations"`
			Total         int                   `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &listResponse); err != nil {
		t.Fatalf("decode organization list response: %v", err)
	}
	foundCreated := false
	for _, org := range listResponse.Data.Organizations {
		if org.ID == organizationID {
			foundCreated = true
			break
		}
	}
	if !foundCreated || listResponse.Data.Total < 1 {
		t.Fatalf("expected created organization in filtered list, total=%d organizations=%+v", listResponse.Data.Total, listResponse.Data.Organizations)
	}

	detailBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/organizations/"+organizationID, "", adminCookie, "", stdhttp.StatusOK)
	var detailResponse struct {
		Data tenant.Organization `json:"data"`
	}
	if err := json.Unmarshal(detailBody, &detailResponse); err != nil {
		t.Fatalf("decode organization detail response: %v", err)
	}
	if detailResponse.Data.ID != organizationID || detailResponse.Data.CreatedByUserID == nil || *detailResponse.Data.CreatedByUserID != adminUserID {
		t.Fatalf("unexpected organization detail response: %+v", detailResponse.Data)
	}

	updateBody := commercialDoJSON(t, router, stdhttp.MethodPut, "/api/v1/admin/organizations/"+organizationID, `{
		"name":"Admin Managed Org Updated",
		"status":"disabled",
		"metadata":{"tier":"enterprise","region":"eu","audit":"updated"}
	}`, adminCookie, adminCSRF, stdhttp.StatusOK)
	var updateResponse struct {
		Data tenant.Organization `json:"data"`
	}
	if err := json.Unmarshal(updateBody, &updateResponse); err != nil {
		t.Fatalf("decode organization update response: %v", err)
	}
	if updateResponse.Data.Name != "Admin Managed Org Updated" ||
		updateResponse.Data.Status != tenant.StatusDisabled ||
		updateResponse.Data.Metadata["region"] != "eu" {
		t.Fatalf("unexpected organization update response: %+v", updateResponse.Data)
	}

	var storedName string
	var storedStatus string
	var storedRegion string
	if err := database.QueryRow(`SELECT name, status, metadata->>'region' FROM organizations WHERE id = $1`, organizationID).Scan(&storedName, &storedStatus, &storedRegion); err != nil {
		t.Fatalf("query updated organization: %v", err)
	}
	if storedName != "Admin Managed Org Updated" || storedStatus != tenant.StatusDisabled || storedRegion != "eu" {
		t.Fatalf("unexpected stored update evidence: name=%q status=%q region=%q", storedName, storedStatus, storedRegion)
	}

	membersBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/organizations/"+organizationID+"/members", "", adminCookie, "", stdhttp.StatusOK)
	var membersResponse struct {
		Data struct {
			Members []tenant.Membership `json:"members"`
		} `json:"data"`
	}
	if err := json.Unmarshal(membersBody, &membersResponse); err != nil {
		t.Fatalf("decode organization members response: %v", err)
	}
	if len(membersResponse.Data.Members) != 1 ||
		membersResponse.Data.Members[0].UserID != adminUserID ||
		membersResponse.Data.Members[0].Role != tenant.RoleOwner {
		t.Fatalf("expected admin owner membership in members response, got %+v", membersResponse.Data.Members)
	}

	archiveBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/organizations/"+organizationID+"/archive", "", adminCookie, adminCSRF, stdhttp.StatusOK)
	var archiveResponse struct {
		Data tenant.Organization `json:"data"`
	}
	if err := json.Unmarshal(archiveBody, &archiveResponse); err != nil {
		t.Fatalf("decode organization archive response: %v", err)
	}
	if archiveResponse.Data.Status != tenant.StatusArchived || archiveResponse.Data.ArchivedAt == nil {
		t.Fatalf("expected archived response with archivedAt, got %+v", archiveResponse.Data)
	}

	var archivedStatus string
	var archivedAt time.Time
	if err := database.QueryRow(`SELECT status, archived_at FROM organizations WHERE id = $1`, organizationID).Scan(&archivedStatus, &archivedAt); err != nil {
		t.Fatalf("query archived organization: %v", err)
	}
	if archivedStatus != tenant.StatusArchived || archivedAt.IsZero() {
		t.Fatalf("unexpected stored archive evidence: status=%q archivedAt=%v", archivedStatus, archivedAt)
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

func (s *fakeTenantStore) ListMembershipsForUser(ctx context.Context, userID string) ([]*tenant.Membership, error) {
	return nil, nil
}

func (s *fakeTenantStore) ListOrganizationMembers(ctx context.Context, organizationID string) ([]*tenant.Membership, error) {
	return nil, nil
}

func (s *fakeTenantStore) GetActiveMembership(ctx context.Context, organizationID, userID string) (*tenant.Membership, error) {
	return nil, nil
}

func (s *fakeTenantStore) CreateInvitation(ctx context.Context, invitation *tenant.Invitation, audit tenant.AuditRecord) (*tenant.Invitation, error) {
	return invitation, nil
}

func (s *fakeTenantStore) GetInvitation(ctx context.Context, organizationID, invitationID string) (*tenant.Invitation, error) {
	return nil, nil
}

func (s *fakeTenantStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*tenant.Invitation, error) {
	return nil, nil
}

func (s *fakeTenantStore) AcceptInvitation(ctx context.Context, invitation *tenant.Invitation, userID string, audit tenant.AuditRecord) (*tenant.Membership, error) {
	return nil, nil
}

func (s *fakeTenantStore) RevokeInvitation(ctx context.Context, organizationID, invitationID string, audit tenant.AuditRecord) (*tenant.Invitation, error) {
	return nil, nil
}

func (s *fakeTenantStore) UpdateMemberRole(ctx context.Context, organizationID, userID, role string, audit tenant.AuditRecord) (*tenant.Membership, error) {
	return nil, nil
}

func (s *fakeTenantStore) RemoveMember(ctx context.Context, organizationID, userID string, audit tenant.AuditRecord) error {
	return nil
}

func (s *fakeTenantStore) TransferOwnership(ctx context.Context, organizationID, currentOwnerUserID, newOwnerUserID string, audit tenant.AuditRecord) error {
	return nil
}
