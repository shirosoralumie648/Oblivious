package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
FROM organizations
WHERE ($1 = '' OR status = $1)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR slug ILIKE '%' || $2 || '%')
ORDER BY created_at DESC
LIMIT $3 OFFSET $4
`, filter.Status, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()

	organizations := []*Organization{}
	for rows.Next() {
		org, err := scanOrganization(rows)
		if err != nil {
			return nil, 0, err
		}
		organizations = append(organizations, org)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("scan organizations: %w", err)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM organizations
WHERE ($1 = '' OR status = $1)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR slug ILIKE '%' || $2 || '%')
`, filter.Status, filter.Search).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count organizations: %w", err)
	}

	return organizations, total, nil
}

func (s *SQLStore) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	org, err := scanOrganization(s.db.QueryRowContext(ctx, `
SELECT id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
FROM organizations
WHERE id = $1
`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	return org, nil
}

func (s *SQLStore) CreateOrganization(ctx context.Context, organization *Organization) (*Organization, error) {
	metadataJSON, err := json.Marshal(organization.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal organization metadata: %w", err)
	}

	org, err := scanOrganization(s.db.QueryRowContext(ctx, `
INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
`, organization.ID, organization.Slug, organization.Name, organization.Status, string(metadataJSON), organization.CreatedByUserID))
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	return org, nil
}

func (s *SQLStore) UpdateOrganization(ctx context.Context, id string, input OrganizationUpdate) (*Organization, error) {
	var metadataJSON any
	if input.Metadata != nil {
		b, err := json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal organization metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	org, err := scanOrganization(s.db.QueryRowContext(ctx, `
UPDATE organizations
SET name = COALESCE($2, name),
    status = COALESCE($3, status),
    metadata = COALESCE($4, metadata),
    updated_at = NOW()
WHERE id = $1
RETURNING id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
`, id, input.Name, input.Status, metadataJSON))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

func (s *SQLStore) ArchiveOrganization(ctx context.Context, id string) (*Organization, error) {
	org, err := scanOrganization(s.db.QueryRowContext(ctx, `
UPDATE organizations
SET status = $2,
    archived_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
`, id, StatusArchived))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("archive organization: %w", err)
	}
	return org, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrganization(row scanner) (*Organization, error) {
	var org Organization
	var metadataRaw []byte
	var createdBy sql.NullString
	var archivedAt sql.NullTime
	if err := row.Scan(
		&org.ID,
		&org.Slug,
		&org.Name,
		&org.Status,
		&metadataRaw,
		&createdBy,
		&org.CreatedAt,
		&org.UpdatedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}

	org.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &org.Metadata); err != nil {
			return nil, fmt.Errorf("decode organization metadata: %w", err)
		}
	}
	if createdBy.Valid {
		org.CreatedByUserID = &createdBy.String
	}
	if archivedAt.Valid {
		value := archivedAt.Time.UTC()
		org.ArchivedAt = &value
	}
	org.CreatedAt = org.CreatedAt.UTC()
	org.UpdatedAt = org.UpdatedAt.UTC()

	return &org, nil
}

var _ Store = (*SQLStore)(nil)
var _ = time.Time{}
