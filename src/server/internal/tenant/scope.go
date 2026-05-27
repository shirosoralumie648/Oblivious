package tenant

import (
	"context"
	"errors"
	"strings"

	"oblivious/server/internal/auth"
)

type Scope struct {
	MembershipID   string
	OrganizationID string
	Role           string
	UserID         string
}

func (s *Service) ResolveSessionScope(ctx context.Context, session auth.Session) (Scope, error) {
	return s.ResolveOrganizationScope(ctx, session.User.ID, session.OrganizationID)
}

func (s *Service) ResolveOrganizationScope(ctx context.Context, userID, organizationID string) (Scope, error) {
	if strings.TrimSpace(userID) == "" {
		return Scope{}, errors.New("user id is required")
	}
	if strings.TrimSpace(organizationID) == "" {
		return Scope{}, errors.New("organization membership required")
	}
	membership, err := s.store.GetActiveMembership(ctx, organizationID, userID)
	if err != nil {
		return Scope{}, err
	}
	if membership == nil {
		return Scope{}, errors.New("organization membership required")
	}
	return Scope{
		MembershipID:   membership.ID,
		OrganizationID: membership.OrganizationID,
		Role:           membership.Role,
		UserID:         membership.UserID,
	}, nil
}
