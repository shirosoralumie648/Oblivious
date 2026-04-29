package admin

import (
	"context"
	"database/sql"
	"fmt"
)

// ListUsers returns users with validation and pagination clamping.
// Per D-12: supports search (name/email partial), filter (role/plan/status), sort.
func (s *Service) ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, int, error) {
	// Validate and clamp Limit
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Validate Sort — fall back to default for unrecognized values
	switch filter.Sort {
	case "", "created_at_desc", "created_at_asc", "name_asc", "name_desc", "tokens_desc", "last_login_desc":
		// valid
	default:
		filter.Sort = "created_at_desc"
	}

	return s.store.ListUsers(ctx, filter)
}

// GetUserDetail returns a user with usage statistics per D-13.
// Returns a wrapped error on not-found or empty id.
func (s *Service) GetUserDetail(ctx context.Context, id string) (*UserDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}

	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		// GetUserByID returns fmt.Errorf("user not found: %s", id) for not found
		return nil, err
	}

	return user, nil
}

// UpdateUser updates a user's role, plan, or status with validation and audit.
// Per D-16: role must be one of {admin, moderator, user} — predefined RBAC.
// Self-role-change is blocked to prevent self-demotion.
// Plan changes are validated against active plans in the store.
// Every mutation is audit-logged per D-08.
func (s *Service) UpdateUser(ctx context.Context, actorID, actorEmail, id string, input UserUpdateRequest, ip string) (*UserDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}

	// Validate role per D-16: predefined set only
	if input.Role != nil && *input.Role != "" {
		switch *input.Role {
		case "admin", "moderator", "user":
			// valid
		default:
			return nil, fmt.Errorf("invalid role: %s (must be admin, moderator, or user)", *input.Role)
		}
	}

	// Cannot change own role (prevent self-demotion)
	if input.Role != nil && *input.Role != "" && actorID == id {
		return nil, fmt.Errorf("cannot change your own role")
	}

	// Validate plan exists and is active
	if input.PlanID != nil && *input.PlanID != "" {
		plan, err := s.store.GetPlan(ctx, *input.PlanID)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid plan: plan not found")
		}
		if err != nil {
			return nil, fmt.Errorf("validate plan: %w", err)
		}
		if !plan.IsActive {
			return nil, fmt.Errorf("invalid plan: plan %s is not active", plan.Name)
		}
	}

	result, err := s.store.UpdateUser(ctx, id, input)
	if err != nil {
		return nil, err
	}

	// Audit log per D-08: actor, timestamp, IP, changes
	_ = s.LogAction(ctx, actorID, actorEmail, "user.update", "user", id, toJSON(input), ip)

	return result, nil
}

// DisableUser disables a user and revokes all their active sessions.
// Per D-12: full lifecycle — status change + session deletion in a transaction.
// Self-disable is blocked for safety.
// Audit logged per D-08.
func (s *Service) DisableUser(ctx context.Context, actorID, actorEmail, id string, ip string) error {
	if id == "" {
		return fmt.Errorf("user id is required")
	}

	// Cannot disable self
	if actorID == id {
		return fmt.Errorf("cannot disable your own account")
	}

	if err := s.store.DisableUser(ctx, id); err != nil {
		return err
	}

	// Audit log per D-08: actor, timestamp, IP
	_ = s.LogAction(ctx, actorID, actorEmail, "user.disable", "user", id, `{"reason":"admin action"}`, ip)

	return nil
}

// EnableUser re-enables a previously disabled user.
// Per D-12: user must log in again (sessions are not restored).
// Audit logged per D-08.
func (s *Service) EnableUser(ctx context.Context, actorID, actorEmail, id string, ip string) error {
	if id == "" {
		return fmt.Errorf("user id is required")
	}

	if err := s.store.EnableUser(ctx, id); err != nil {
		return err
	}

	// Audit log per D-08
	_ = s.LogAction(ctx, actorID, actorEmail, "user.enable", "user", id, "", ip)

	return nil
}
