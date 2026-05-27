package tenant

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestServiceRejectsInvalidCreateInput(t *testing.T) {
	service := NewService(&fakeStore{})

	tests := []struct {
		name string
		req  CreateOrganizationRequest
	}{
		{name: "empty name", req: CreateOrganizationRequest{Name: " ", Slug: "valid-slug"}},
		{name: "empty slug", req: CreateOrganizationRequest{Name: "Valid", Slug: " "}},
		{name: "uppercase slug", req: CreateOrganizationRequest{Name: "Valid", Slug: "Invalid"}},
		{name: "slug starts with dash", req: CreateOrganizationRequest{Name: "Valid", Slug: "-invalid"}},
		{name: "slug too short", req: CreateOrganizationRequest{Name: "Valid", Slug: "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.CreateOrganization(context.Background(), tt.req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestServiceCreatesOrganizationWithDefaults(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	creatorID := "user_admin"

	org, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:            "  Acme Team  ",
		Slug:            "acme-team",
		CreatedByUserID: &creatorID,
		Metadata:        map[string]any{"tier": "trial"},
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	if org.ID == "" {
		t.Fatal("expected generated organization id")
	}
	if org.Name != "Acme Team" {
		t.Fatalf("expected trimmed name, got %q", org.Name)
	}
	if org.Status != StatusActive {
		t.Fatalf("expected active status, got %q", org.Status)
	}
	if store.created == nil || store.created.Slug != "acme-team" {
		t.Fatalf("expected store to receive normalized organization, got %+v", store.created)
	}
}

func TestSQLStoreOrganizationLifecycle(t *testing.T) {
	database := testTenantDatabase(t)
	resetTenantTestTables(t, database)

	store := NewSQLStore(database)
	service := NewService(store)

	created, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name: "Acme",
		Slug: "acme",
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	listed, total, err := service.ListOrganizations(context.Background(), OrganizationListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list organizations: %v", err)
	}
	if total != 1 || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected created organization in list, total=%d listed=%+v", total, listed)
	}

	newName := "Acme Commercial"
	updated, err := service.UpdateOrganization(context.Background(), created.ID, OrganizationUpdateRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update organization: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected updated name %q, got %q", newName, updated.Name)
	}

	archived, err := service.ArchiveOrganization(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("archive organization: %v", err)
	}
	if archived.Status != StatusArchived || archived.ArchivedAt == nil {
		t.Fatalf("expected archived organization with archived_at, got %+v", archived)
	}

	fetched, err := service.GetOrganization(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get archived organization: %v", err)
	}
	if fetched == nil || fetched.ID != created.ID {
		t.Fatalf("expected archived row to remain fetchable, got %+v", fetched)
	}
}

func TestSQLStoreMembershipInvitationOwnershipLifecycle(t *testing.T) {
	database := testTenantDatabase(t)
	resetTenantSecurityTestTables(t, database)
	seedTenantTestUser(t, database, "user_owner", "owner@example.com")
	seedTenantTestUser(t, database, "user_admin", "admin@example.com")
	seedTenantTestUser(t, database, "user_member", "member@example.com")

	store := NewSQLStore(database)
	service := NewService(store)
	ownerActor := Actor{UserID: "user_owner", Email: "owner@example.com", IPAddress: "127.0.0.1"}
	ownerID := "user_owner"

	org, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:            "Acme",
		Slug:            "acme",
		CreatedByUserID: &ownerID,
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	ownerMemberships, err := service.ListMembershipsForUser(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("list owner memberships: %v", err)
	}
	if len(ownerMemberships) != 1 || ownerMemberships[0].Role != RoleOwner || ownerMemberships[0].OrganizationID != org.ID {
		t.Fatalf("expected creator owner membership, got %+v", ownerMemberships)
	}

	adminInvite, err := service.InviteMember(context.Background(), ownerActor, org.ID, InviteMemberRequest{
		Email: "ADMIN@example.com",
		Role:  RoleAdmin,
	})
	if err != nil {
		t.Fatalf("invite admin: %v", err)
	}
	if adminInvite.Token == "" {
		t.Fatal("expected raw invitation token to be returned once")
	}

	var rawTokenMatches int
	if err := database.QueryRow(`SELECT COUNT(*) FROM organization_invitations WHERE token_hash = $1`, adminInvite.Token).Scan(&rawTokenMatches); err != nil {
		t.Fatalf("check invitation token storage: %v", err)
	}
	if rawTokenMatches != 0 {
		t.Fatal("raw invitation token must not be stored in token_hash")
	}

	adminMembership, err := service.AcceptInvitation(context.Background(), Actor{
		UserID:    "user_admin",
		Email:     "admin@example.com",
		IPAddress: "127.0.0.1",
	}, adminInvite.Token)
	if err != nil {
		t.Fatalf("accept admin invitation: %v", err)
	}
	if adminMembership.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %q", adminMembership.Role)
	}

	memberInvite, err := service.InviteMember(context.Background(), Actor{UserID: "user_admin", Email: "admin@example.com"}, org.ID, InviteMemberRequest{
		Email: "member@example.com",
		Role:  RoleMember,
	})
	if err != nil {
		t.Fatalf("admin invite member: %v", err)
	}
	if _, err := service.AcceptInvitation(context.Background(), Actor{UserID: "user_member", Email: "member@example.com"}, memberInvite.Token); err != nil {
		t.Fatalf("accept member invitation: %v", err)
	}

	if _, err := service.InviteMember(context.Background(), Actor{UserID: "user_member", Email: "member@example.com"}, org.ID, InviteMemberRequest{
		Email: "other@example.com",
		Role:  RoleMember,
	}); err == nil {
		t.Fatal("expected member invite to be forbidden")
	}

	if _, err := service.UpdateMemberRole(context.Background(), ownerActor, org.ID, "user_member", UpdateMemberRoleRequest{Role: RoleAdmin}); err != nil {
		t.Fatalf("owner promotes member: %v", err)
	}

	if err := service.TransferOwnership(context.Background(), ownerActor, org.ID, TransferOwnershipRequest{NewOwnerUserID: "user_admin"}); err != nil {
		t.Fatalf("transfer ownership: %v", err)
	}

	var ownerCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM organization_memberships
		WHERE organization_id = $1 AND role = 'owner' AND removed_at IS NULL
	`, org.ID).Scan(&ownerCount); err != nil {
		t.Fatalf("count active owners: %v", err)
	}
	if ownerCount != 1 {
		t.Fatalf("expected exactly one active owner, got %d", ownerCount)
	}

	var auditCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'organization' AND resource_id = $1`, org.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount < 5 {
		t.Fatalf("expected membership mutations to be audited, got %d audit rows", auditCount)
	}
}

type fakeStore struct {
	created *Organization
}

func (s *fakeStore) ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error) {
	return nil, 0, nil
}

func (s *fakeStore) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	return nil, nil
}

func (s *fakeStore) CreateOrganization(ctx context.Context, organization *Organization) (*Organization, error) {
	s.created = organization
	return organization, nil
}

func (s *fakeStore) UpdateOrganization(ctx context.Context, id string, input OrganizationUpdate) (*Organization, error) {
	return nil, nil
}

func (s *fakeStore) ArchiveOrganization(ctx context.Context, id string) (*Organization, error) {
	return nil, nil
}

func (s *fakeStore) ListMembershipsForUser(ctx context.Context, userID string) ([]*Membership, error) {
	return nil, nil
}

func (s *fakeStore) ListOrganizationMembers(ctx context.Context, organizationID string) ([]*Membership, error) {
	return nil, nil
}

func (s *fakeStore) GetActiveMembership(ctx context.Context, organizationID, userID string) (*Membership, error) {
	return nil, nil
}

func (s *fakeStore) CreateInvitation(ctx context.Context, invitation *Invitation, audit AuditRecord) (*Invitation, error) {
	return invitation, nil
}

func (s *fakeStore) GetInvitation(ctx context.Context, organizationID, invitationID string) (*Invitation, error) {
	return nil, nil
}

func (s *fakeStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*Invitation, error) {
	return nil, nil
}

func (s *fakeStore) AcceptInvitation(ctx context.Context, invitation *Invitation, userID string, audit AuditRecord) (*Membership, error) {
	return nil, nil
}

func (s *fakeStore) RevokeInvitation(ctx context.Context, organizationID, invitationID string, audit AuditRecord) (*Invitation, error) {
	return nil, nil
}

func (s *fakeStore) UpdateMemberRole(ctx context.Context, organizationID, userID, role string, audit AuditRecord) (*Membership, error) {
	return nil, nil
}

func (s *fakeStore) RemoveMember(ctx context.Context, organizationID, userID string, audit AuditRecord) error {
	return nil
}

func (s *fakeStore) TransferOwnership(ctx context.Context, organizationID, currentOwnerUserID, newOwnerUserID string, audit AuditRecord) error {
	return nil
}

func testTenantDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for tenant integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	lockIntegrationTestDatabase(t, database)

	return database
}

func lockIntegrationTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock integration test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock integration test database: %v", err)
		}
	})
}

func resetTenantTestTables(t *testing.T, database *sql.DB) {
	t.Helper()

	statements := []string{
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (
			id TEXT PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			metadata JSONB NOT NULL DEFAULT '{}',
			created_by_user_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			archived_at TIMESTAMPTZ,
			CHECK (status IN ('active', 'disabled', 'archived'))
		)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare tenant test database: %v", err)
		}
	}
}

func resetTenantSecurityTestTables(t *testing.T, database *sql.DB) {
	t.Helper()

	statements := []string{
		`DROP TABLE IF EXISTS password_reset_tokens CASCADE`,
		`DROP TABLE IF EXISTS auth_rate_limits CASCADE`,
		`DROP TABLE IF EXISTS sessions CASCADE`,
		`DROP TABLE IF EXISTS workspaces CASCADE`,
		`DROP TABLE IF EXISTS organization_invitations CASCADE`,
		`DROP TABLE IF EXISTS organization_memberships CASCADE`,
		`DROP TABLE IF EXISTS audit_logs CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ
		)`,
		`CREATE TABLE organizations (
			id TEXT PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			metadata JSONB NOT NULL DEFAULT '{}',
			created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			archived_at TIMESTAMPTZ,
			CHECK (status IN ('active', 'disabled', 'archived'))
		)`,
		`CREATE TABLE organization_memberships (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			removed_at TIMESTAMPTZ,
			CHECK (role IN ('owner', 'admin', 'member'))
		)`,
		`CREATE UNIQUE INDEX idx_org_memberships_active_user_test ON organization_memberships(organization_id, user_id) WHERE removed_at IS NULL`,
		`CREATE UNIQUE INDEX idx_org_memberships_single_owner_test ON organization_memberships(organization_id) WHERE role = 'owner' AND removed_at IS NULL`,
		`CREATE TABLE organization_invitations (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			email TEXT NOT NULL,
			role TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			invited_by_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			accepted_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			accepted_at TIMESTAMPTZ,
			revoked_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (role IN ('admin', 'member')),
			CHECK (status IN ('pending', 'accepted', 'revoked', 'expired'))
		)`,
		`CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			actor_id TEXT NOT NULL REFERENCES users(id),
			actor_email TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT,
			changes JSONB,
			ip_address TEXT,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare tenant security test database: %v", err)
		}
	}
}

func seedTenantTestUser(t *testing.T, database *sql.DB, id, email string) {
	t.Helper()

	if _, err := database.Exec(`
		INSERT INTO users (id, email, password_hash, role, name, created_at)
		VALUES ($1, $2, 'hash', 'user', $2, $3)
	`, id, email, time.Now().UTC()); err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}
