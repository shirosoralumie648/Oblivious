package tenant

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

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
	creatorID := "user_admin"
	if _, err := database.Exec(`
INSERT INTO users (id, email, password_hash, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
	email = EXCLUDED.email,
	password_hash = EXCLUDED.password_hash,
	role = EXCLUDED.role
`, creatorID, "admin@example.com", "hash", "admin"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	created, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:            "Acme",
		Slug:            "acme",
		CreatedByUserID: &creatorID,
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

	return database
}

func resetTenantTestTables(t *testing.T, database *sql.DB) {
	t.Helper()

	statements := []string{
		`DROP TABLE IF EXISTS organizations`,
		`CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user')`,
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
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare tenant test database: %v", err)
		}
	}
}
