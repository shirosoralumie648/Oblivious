package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create organization: %w", err)
	}
	defer tx.Rollback()

	org, err := scanOrganization(tx.QueryRowContext(ctx, `
INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, slug, name, status, metadata, created_by_user_id, created_at, updated_at, archived_at
`, organization.ID, organization.Slug, organization.Name, organization.Status, string(metadataJSON), organization.CreatedByUserID))
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	if organization.CreatedByUserID != nil && *organization.CreatedByUserID != "" {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
VALUES ($1, $2, $3, $4, $5)
`, newID(), org.ID, *organization.CreatedByUserID, RoleOwner, *organization.CreatedByUserID); err != nil {
			return nil, fmt.Errorf("create owner membership: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create organization: %w", err)
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

func (s *SQLStore) ListMembershipsForUser(ctx context.Context, userID string) ([]*Membership, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT m.id, m.organization_id, COALESCE(o.slug, ''), COALESCE(o.name, ''),
       m.user_id, COALESCE(u.email, ''), m.role, m.created_by_user_id,
       m.created_at, m.updated_at, m.removed_at
FROM organization_memberships m
JOIN organizations o ON o.id = m.organization_id
JOIN users u ON u.id = m.user_id
WHERE m.user_id = $1 AND m.removed_at IS NULL
ORDER BY o.name ASC, m.created_at ASC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships for user: %w", err)
	}
	defer rows.Close()

	return scanMembershipRows(rows)
}

func (s *SQLStore) ListOrganizationMembers(ctx context.Context, organizationID string) ([]*Membership, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT m.id, m.organization_id, COALESCE(o.slug, ''), COALESCE(o.name, ''),
       m.user_id, COALESCE(u.email, ''), m.role, m.created_by_user_id,
       m.created_at, m.updated_at, m.removed_at
FROM organization_memberships m
JOIN organizations o ON o.id = m.organization_id
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1 AND m.removed_at IS NULL
ORDER BY
  CASE m.role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 ELSE 3 END,
  u.email ASC
`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list organization members: %w", err)
	}
	defer rows.Close()

	return scanMembershipRows(rows)
}

func (s *SQLStore) GetActiveMembership(ctx context.Context, organizationID, userID string) (*Membership, error) {
	membership, err := scanMembership(s.db.QueryRowContext(ctx, `
SELECT m.id, m.organization_id, COALESCE(o.slug, ''), COALESCE(o.name, ''),
       m.user_id, COALESCE(u.email, ''), m.role, m.created_by_user_id,
       m.created_at, m.updated_at, m.removed_at
FROM organization_memberships m
JOIN organizations o ON o.id = m.organization_id
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1 AND m.user_id = $2 AND m.removed_at IS NULL
`, organizationID, userID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active membership: %w", err)
	}
	return membership, nil
}

func (s *SQLStore) CreateInvitation(ctx context.Context, invitation *Invitation, audit AuditRecord) (*Invitation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create invitation: %w", err)
	}
	defer tx.Rollback()

	created, err := scanInvitation(tx.QueryRowContext(ctx, `
INSERT INTO organization_invitations (
  id, organization_id, email, role, token_hash, status, invited_by_user_id, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, organization_id, email, role, token_hash, status, invited_by_user_id,
          accepted_by_user_id, expires_at, accepted_at, revoked_at, created_at, updated_at
`, invitation.ID, invitation.OrganizationID, invitation.Email, invitation.Role, invitation.TokenHash,
		invitation.Status, invitation.InvitedByUserID, invitation.ExpiresAt))
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create invitation: %w", err)
	}
	return created, nil
}

func (s *SQLStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*Invitation, error) {
	invitation, err := scanInvitation(s.db.QueryRowContext(ctx, `
SELECT id, organization_id, email, role, token_hash, status, invited_by_user_id,
       accepted_by_user_id, expires_at, accepted_at, revoked_at, created_at, updated_at
FROM organization_invitations
WHERE token_hash = $1
`, tokenHash))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation by token: %w", err)
	}
	return invitation, nil
}

func (s *SQLStore) GetInvitation(ctx context.Context, organizationID, invitationID string) (*Invitation, error) {
	invitation, err := scanInvitation(s.db.QueryRowContext(ctx, `
SELECT id, organization_id, email, role, token_hash, status, invited_by_user_id,
       accepted_by_user_id, expires_at, accepted_at, revoked_at, created_at, updated_at
FROM organization_invitations
WHERE organization_id = $1 AND id = $2
`, organizationID, invitationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return invitation, nil
}

func (s *SQLStore) AcceptInvitation(ctx context.Context, invitation *Invitation, userID string, audit AuditRecord) (*Membership, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin accept invitation: %w", err)
	}
	defer tx.Rollback()

	var acceptedBy any = userID
	if _, err := tx.ExecContext(ctx, `
UPDATE organization_invitations
SET status = 'accepted', accepted_by_user_id = $2, accepted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'pending'
`, invitation.ID, acceptedBy); err != nil {
		return nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	membership, err := scanMembership(tx.QueryRowContext(ctx, `
INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, organization_id, '', '', user_id, '', role, created_by_user_id, created_at, updated_at, removed_at
`, newID(), invitation.OrganizationID, userID, invitation.Role, invitation.InvitedByUserID))
	if err != nil {
		return nil, fmt.Errorf("create invitation membership: %w", err)
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit accept invitation: %w", err)
	}
	return membership, nil
}

func (s *SQLStore) RevokeInvitation(ctx context.Context, organizationID, invitationID string, audit AuditRecord) (*Invitation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin revoke invitation: %w", err)
	}
	defer tx.Rollback()

	invitation, err := scanInvitation(tx.QueryRowContext(ctx, `
UPDATE organization_invitations
SET status = 'revoked', revoked_at = NOW(), updated_at = NOW()
WHERE organization_id = $1 AND id = $2 AND status = 'pending'
RETURNING id, organization_id, email, role, token_hash, status, invited_by_user_id,
          accepted_by_user_id, expires_at, accepted_at, revoked_at, created_at, updated_at
`, organizationID, invitationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("revoke invitation: %w", err)
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit revoke invitation: %w", err)
	}
	return invitation, nil
}

func (s *SQLStore) UpdateMemberRole(ctx context.Context, organizationID, userID, role string, audit AuditRecord) (*Membership, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update member role: %w", err)
	}
	defer tx.Rollback()

	membership, err := scanMembership(tx.QueryRowContext(ctx, `
UPDATE organization_memberships
SET role = $3, updated_at = NOW()
WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
RETURNING id, organization_id, '', '', user_id, '', role, created_by_user_id, created_at, updated_at, removed_at
`, organizationID, userID, role))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update member role: %w", err)
	}
	return membership, nil
}

func (s *SQLStore) RemoveMember(ctx context.Context, organizationID, userID string, audit AuditRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove member: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
UPDATE organization_memberships
SET removed_at = NOW(), updated_at = NOW()
WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
`, organizationID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove member: %w", err)
	}
	return nil
}

func (s *SQLStore) TransferOwnership(ctx context.Context, organizationID, currentOwnerUserID, newOwnerUserID string, audit AuditRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transfer ownership: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE organization_memberships
SET role = 'admin', updated_at = NOW()
WHERE organization_id = $1 AND user_id = $2 AND role = 'owner' AND removed_at IS NULL
`, organizationID, currentOwnerUserID); err != nil {
		return fmt.Errorf("demote current owner: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
UPDATE organization_memberships
SET role = 'owner', updated_at = NOW()
WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
`, organizationID, newOwnerUserID)
	if err != nil {
		return fmt.Errorf("promote new owner: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.New("new owner membership not found")
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transfer ownership: %w", err)
	}
	return nil
}

func scanMembershipRows(rows *sql.Rows) ([]*Membership, error) {
	memberships := []*Membership{}
	for rows.Next() {
		membership, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan memberships: %w", err)
	}
	return memberships, nil
}

func scanMembership(row scanner) (*Membership, error) {
	var membership Membership
	var createdBy sql.NullString
	var removedAt sql.NullTime
	if err := row.Scan(
		&membership.ID,
		&membership.OrganizationID,
		&membership.OrganizationSlug,
		&membership.OrganizationName,
		&membership.UserID,
		&membership.UserEmail,
		&membership.Role,
		&createdBy,
		&membership.CreatedAt,
		&membership.UpdatedAt,
		&removedAt,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		membership.CreatedByUserID = &createdBy.String
	}
	if removedAt.Valid {
		value := removedAt.Time.UTC()
		membership.RemovedAt = &value
	}
	membership.CreatedAt = membership.CreatedAt.UTC()
	membership.UpdatedAt = membership.UpdatedAt.UTC()
	return &membership, nil
}

func scanInvitation(row scanner) (*Invitation, error) {
	var invitation Invitation
	var acceptedBy sql.NullString
	var acceptedAt sql.NullTime
	var revokedAt sql.NullTime
	if err := row.Scan(
		&invitation.ID,
		&invitation.OrganizationID,
		&invitation.Email,
		&invitation.Role,
		&invitation.TokenHash,
		&invitation.Status,
		&invitation.InvitedByUserID,
		&acceptedBy,
		&invitation.ExpiresAt,
		&acceptedAt,
		&revokedAt,
		&invitation.CreatedAt,
		&invitation.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if acceptedBy.Valid {
		invitation.AcceptedByUserID = &acceptedBy.String
	}
	if acceptedAt.Valid {
		value := acceptedAt.Time.UTC()
		invitation.AcceptedAt = &value
	}
	if revokedAt.Valid {
		value := revokedAt.Time.UTC()
		invitation.RevokedAt = &value
	}
	invitation.ExpiresAt = invitation.ExpiresAt.UTC()
	invitation.CreatedAt = invitation.CreatedAt.UTC()
	invitation.UpdatedAt = invitation.UpdatedAt.UTC()
	return &invitation, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAudit(ctx context.Context, tx execer, audit AuditRecord) error {
	if audit.ActorID == "" {
		return errors.New("audit actor id is required")
	}
	_, err := tx.ExecContext(ctx, `
	INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id, organization_id, changes, ip_address, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`, newID(), audit.ActorID, audit.ActorEmail, audit.Action, audit.ResourceType, audit.ResourceID, nullString(audit.OrganizationID), nullString(audit.Changes), nullString(audit.IPAddress))
	if err != nil {
		return fmt.Errorf("create audit entry: %w", err)
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return sql.NullString{}
	}
	return value
}

func newID() string {
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}
