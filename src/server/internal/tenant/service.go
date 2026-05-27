package tenant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type Store interface {
	ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error)
	GetOrganization(ctx context.Context, id string) (*Organization, error)
	CreateOrganization(ctx context.Context, organization *Organization) (*Organization, error)
	UpdateOrganization(ctx context.Context, id string, input OrganizationUpdate) (*Organization, error)
	ArchiveOrganization(ctx context.Context, id string) (*Organization, error)
	ListMembershipsForUser(ctx context.Context, userID string) ([]*Membership, error)
	ListOrganizationMembers(ctx context.Context, organizationID string) ([]*Membership, error)
	GetActiveMembership(ctx context.Context, organizationID, userID string) (*Membership, error)
	CreateInvitation(ctx context.Context, invitation *Invitation, audit AuditRecord) (*Invitation, error)
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*Invitation, error)
	AcceptInvitation(ctx context.Context, invitation *Invitation, userID string, audit AuditRecord) (*Membership, error)
	UpdateMemberRole(ctx context.Context, organizationID, userID, role string, audit AuditRecord) (*Membership, error)
	RemoveMember(ctx context.Context, organizationID, userID string, audit AuditRecord) error
	TransferOwnership(ctx context.Context, organizationID, currentOwnerUserID, newOwnerUserID string, audit AuditRecord) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListOrganizations(ctx, filter)
}

func (s *Service) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}
	return s.store.GetOrganization(ctx, id)
}

func (s *Service) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (*Organization, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}
	slug, err := normalizeSlug(req.Slug)
	if err != nil {
		return nil, err
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	return s.store.CreateOrganization(ctx, &Organization{
		ID:              uuid.New().String(),
		Slug:            slug,
		Name:            name,
		Status:          StatusActive,
		Metadata:        metadata,
		CreatedByUserID: req.CreatedByUserID,
	})
}

func (s *Service) UpdateOrganization(ctx context.Context, id string, req OrganizationUpdateRequest) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}

	input := OrganizationUpdate{Metadata: req.Metadata}
	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return nil, err
		}
		input.Name = &name
	}
	if req.Status != nil {
		status, err := normalizeStatus(*req.Status)
		if err != nil {
			return nil, err
		}
		input.Status = &status
	}

	return s.store.UpdateOrganization(ctx, id, input)
}

func (s *Service) ArchiveOrganization(ctx context.Context, id string) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}
	return s.store.ArchiveOrganization(ctx, id)
}

func (s *Service) ListMembershipsForUser(ctx context.Context, userID string) ([]*Membership, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user id is required")
	}
	return s.store.ListMembershipsForUser(ctx, userID)
}

func (s *Service) ListOrganizationMembers(ctx context.Context, actor Actor, organizationID string) ([]*Membership, error) {
	if err := s.requireAnyMembership(ctx, organizationID, actor.UserID); err != nil {
		return nil, err
	}
	return s.store.ListOrganizationMembers(ctx, organizationID)
}

func (s *Service) InviteMember(ctx context.Context, actor Actor, organizationID string, req InviteMemberRequest) (*Invitation, error) {
	if err := s.requireOrganizationRole(ctx, organizationID, actor.UserID, RoleOwner, RoleAdmin); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	role, err := normalizeInvitationRole(req.Role)
	if err != nil {
		return nil, err
	}
	if active, err := s.findMembershipByEmail(ctx, organizationID, email); err != nil {
		return nil, err
	} else if active != nil {
		return nil, errors.New("user is already an active organization member")
	}
	rawToken, tokenHash, err := newInvitationToken()
	if err != nil {
		return nil, err
	}

	invitation := &Invitation{
		ID:              uuid.New().String(),
		OrganizationID:  organizationID,
		Email:           email,
		Role:            role,
		Token:           rawToken,
		TokenHash:       tokenHash,
		Status:          InvitationPending,
		InvitedByUserID: actor.UserID,
		ExpiresAt:       time.Now().UTC().Add(7 * 24 * time.Hour),
	}
	created, err := s.store.CreateInvitation(ctx, invitation, auditRecord(actor, "organization.member.invite", organizationID, map[string]any{
		"email": email,
		"role":  role,
	}))
	if err != nil {
		return nil, err
	}
	created.Token = rawToken
	return created, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, actor Actor, rawToken string) (*Membership, error) {
	tokenHash := hashToken(strings.TrimSpace(rawToken))
	if tokenHash == "" {
		return nil, errors.New("invitation token is required")
	}
	invitation, err := s.store.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, errors.New("invitation not found")
	}
	if invitation.Status != InvitationPending {
		return nil, errors.New("invitation is not pending")
	}
	if time.Now().UTC().After(invitation.ExpiresAt) {
		return nil, errors.New("invitation expired")
	}
	email, err := normalizeEmail(actor.Email)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(invitation.Email, email) {
		return nil, errors.New("invitation email does not match authenticated user")
	}
	return s.store.AcceptInvitation(ctx, invitation, actor.UserID, auditRecord(actor, "organization.member.accept", invitation.OrganizationID, map[string]any{
		"email": invitation.Email,
		"role":  invitation.Role,
	}))
}

func (s *Service) UpdateMemberRole(ctx context.Context, actor Actor, organizationID, userID string, req UpdateMemberRoleRequest) (*Membership, error) {
	role, err := normalizeMembershipRole(req.Role)
	if err != nil {
		return nil, err
	}
	if role == RoleOwner {
		return nil, errors.New("use ownership transfer to assign owner role")
	}
	actorMembership, err := s.requireOrganizationRoleWithMembership(ctx, organizationID, actor.UserID, RoleOwner, RoleAdmin)
	if err != nil {
		return nil, err
	}
	target, err := s.store.GetActiveMembership(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, errors.New("organization member not found")
	}
	if target.Role == RoleOwner && actorMembership.Role != RoleOwner {
		return nil, errors.New("only owners can change owner memberships")
	}
	if target.Role == RoleOwner {
		return nil, errors.New("use ownership transfer before changing owner role")
	}
	return s.store.UpdateMemberRole(ctx, organizationID, userID, role, auditRecord(actor, "organization.member.role_update", organizationID, map[string]any{
		"userID": userID,
		"role":   role,
	}))
}

func (s *Service) RemoveMember(ctx context.Context, actor Actor, organizationID, userID string) error {
	actorMembership, err := s.requireOrganizationRoleWithMembership(ctx, organizationID, actor.UserID, RoleOwner, RoleAdmin)
	if err != nil {
		return err
	}
	target, err := s.store.GetActiveMembership(ctx, organizationID, userID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("organization member not found")
	}
	if target.Role == RoleOwner {
		if actorMembership.Role != RoleOwner {
			return errors.New("only owners can remove owners")
		}
		return errors.New("cannot remove the active owner; transfer ownership first")
	}
	return s.store.RemoveMember(ctx, organizationID, userID, auditRecord(actor, "organization.member.remove", organizationID, map[string]any{
		"userID": userID,
		"role":   target.Role,
	}))
}

func (s *Service) TransferOwnership(ctx context.Context, actor Actor, organizationID string, req TransferOwnershipRequest) error {
	if err := s.requireOrganizationRole(ctx, organizationID, actor.UserID, RoleOwner); err != nil {
		return err
	}
	newOwnerUserID := strings.TrimSpace(req.NewOwnerUserID)
	if newOwnerUserID == "" {
		return errors.New("new owner user id is required")
	}
	if newOwnerUserID == actor.UserID {
		return errors.New("new owner must be a different user")
	}
	target, err := s.store.GetActiveMembership(ctx, organizationID, newOwnerUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("new owner must be an active organization member")
	}
	return s.store.TransferOwnership(ctx, organizationID, actor.UserID, newOwnerUserID, auditRecord(actor, "organization.owner.transfer", organizationID, map[string]any{
		"fromUserID": actor.UserID,
		"toUserID":   newOwnerUserID,
	}))
}

func (s *Service) requireAnyMembership(ctx context.Context, organizationID, userID string) error {
	_, err := s.requireOrganizationRoleWithMembership(ctx, organizationID, userID, RoleOwner, RoleAdmin, RoleMember)
	return err
}

func (s *Service) requireOrganizationRole(ctx context.Context, organizationID, userID string, roles ...string) error {
	_, err := s.requireOrganizationRoleWithMembership(ctx, organizationID, userID, roles...)
	return err
}

func (s *Service) requireOrganizationRoleWithMembership(ctx context.Context, organizationID, userID string, roles ...string) (*Membership, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.New("organization id is required")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("actor user id is required")
	}
	membership, err := s.store.GetActiveMembership(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, errors.New("organization membership required")
	}
	for _, role := range roles {
		if membership.Role == role {
			return membership, nil
		}
	}
	return nil, errors.New("insufficient organization role")
}

func (s *Service) findMembershipByEmail(ctx context.Context, organizationID, email string) (*Membership, error) {
	members, err := s.store.ListOrganizationMembers(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if strings.EqualFold(member.UserEmail, email) && member.RemovedAt == nil {
			return member, nil
		}
	}
	return nil, nil
}

func normalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("organization name is required")
	}
	return name, nil
}

func normalizeSlug(value string) (string, error) {
	slug := strings.TrimSpace(value)
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("organization slug must match %s", slugPattern.String())
	}
	return slug, nil
}

func normalizeStatus(value string) (string, error) {
	status := strings.TrimSpace(value)
	switch status {
	case StatusActive, StatusDisabled, StatusArchived:
		return status, nil
	default:
		return "", errors.New("organization status must be active, disabled, or archived")
	}
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" || !strings.Contains(email, "@") {
		return "", errors.New("valid email is required")
	}
	return email, nil
}

func normalizeMembershipRole(value string) (string, error) {
	role := strings.TrimSpace(value)
	switch role {
	case RoleOwner, RoleAdmin, RoleMember:
		return role, nil
	default:
		return "", errors.New("organization role must be owner, admin, or member")
	}
}

func normalizeInvitationRole(value string) (string, error) {
	role := strings.TrimSpace(value)
	switch role {
	case RoleAdmin, RoleMember:
		return role, nil
	default:
		return "", errors.New("invitation role must be admin or member")
	}
}

func newInvitationToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(buffer)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func auditRecord(actor Actor, action, organizationID string, changes map[string]any) AuditRecord {
	changesJSON, _ := json.Marshal(changes)
	return AuditRecord{
		ActorID:      actor.UserID,
		ActorEmail:   actor.Email,
		Action:       action,
		ResourceType: "organization",
		ResourceID:   organizationID,
		Changes:      string(changesJSON),
		IPAddress:    actor.IPAddress,
	}
}
