package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/auth"
)

// ListPlans returns plans with business rules applied.
// If IsPublic filter not set, defaults to showing all active plans for admin.
// Public plan listing (IsPublic=true) is available without authentication per D-14.
func (s *Service) ListPlans(ctx context.Context, filter PlanFilter) ([]*PlanInfo, error) {
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.store.ListPlans(ctx, filter)
}

// GetPlan returns a single plan by ID.
func (s *Service) GetPlan(ctx context.Context, id string) (*PlanInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("plan id is required")
	}
	plan, err := s.store.GetPlan(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plan not found: %s", id)
	}
	return plan, err
}

// CreatePlan creates a new subscription plan and records an audit entry.
// Required fields: Name, TokenQuota > 0, Price >= 0, AgentLimit > 0.
// If ModelAccess is empty, defaults to all available models.
func (s *Service) CreatePlan(ctx context.Context, actor auth.Session, input PlanCreateRequest, ip string) (*PlanInfo, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("plan name is required")
	}
	if input.TokenQuota <= 0 {
		return nil, fmt.Errorf("token quota must be > 0")
	}
	if input.Price < 0 {
		return nil, fmt.Errorf("price must be >= 0")
	}
	if input.AgentLimit <= 0 {
		return nil, fmt.Errorf("agent limit must be > 0")
	}

	result, err := s.store.CreatePlan(ctx, input)
	if err != nil {
		return nil, err
	}

	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "plan.create", "plan", result.ID, toJSON(input), ip)

	return result, nil
}

// UpdatePlan updates a plan and records an audit entry.
func (s *Service) UpdatePlan(ctx context.Context, actor auth.Session, id string, input PlanUpdateRequest, ip string) (*PlanInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("plan id is required")
	}

	result, err := s.store.UpdatePlan(ctx, id, input)
	if err != nil {
		return nil, err
	}

	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "plan.update", "plan", id, toJSON(input), ip)

	return result, nil
}

// DeactivatePlan deactivates a plan and records an audit entry.
// Does NOT cancel existing subscriptions — they continue until period end.
func (s *Service) DeactivatePlan(ctx context.Context, actor auth.Session, id string, ip string) error {
	if id == "" {
		return fmt.Errorf("plan id is required")
	}

	if err := s.store.DeactivatePlan(ctx, id); err != nil {
		return err
	}

	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "plan.deactivate", "plan", id, "", ip)

	return nil
}

// SubscribeUser registers a user subscription to a plan.
// Per D-14: only public + active plans are subscribable. For existing active subscriptions,
// the change takes effect at the next billing cycle (next_plan_id is set).
func (s *Service) SubscribeUser(ctx context.Context, actor auth.Session, planID string, ip string) error {
	if planID == "" {
		return fmt.Errorf("plan id is required")
	}

	// Validate plan exists, is active, and is public
	plan, err := s.store.GetPlan(ctx, planID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("plan not found: %s", planID)
	}
	if err != nil {
		return fmt.Errorf("subscribe user: %w", err)
	}
	if !plan.IsActive || !plan.IsPublic {
		return fmt.Errorf("plan %s is not available for subscription", plan.Name)
	}

	// Check for existing active subscription
	subID, err := s.findActiveSubscription(ctx, actor.User.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("subscribe user: %w", err)
	}

	if subID == "" {
		// No active subscription: create new one
		now := time.Now()
		periodEnd := now.Add(30 * 24 * time.Hour)

		newSubID := uuid.New().String()
		_, err := s.db().ExecContext(ctx, `
			INSERT INTO subscriptions (id, user_id, package_id, status, started_at, current_period_start, current_period_end, created_at)
			VALUES ($1, $2, $3, 'active', $4, $4, $5, $4)
		`, newSubID, actor.User.ID, planID, now, periodEnd)
		if err != nil {
			return fmt.Errorf("subscribe user: create subscription: %w", err)
		}

		// Update user's plan assignment
		_, err = s.db().ExecContext(ctx, `
			UPDATE users SET plan_id = $1 WHERE id = $2
		`, planID, actor.User.ID)
		if err != nil {
			return fmt.Errorf("subscribe user: update user plan: %w", err)
		}
	} else {
		// Active subscription exists: schedule change for next billing cycle per D-14
		_, err := s.db().ExecContext(ctx, `
			UPDATE subscriptions SET next_plan_id = $1 WHERE user_id = $2 AND status = 'active'
		`, planID, actor.User.ID)
		if err != nil {
			return fmt.Errorf("subscribe user: schedule plan change: %w", err)
		}

		// Update user's plan_id immediately (reflects intended plan)
		_, err = s.db().ExecContext(ctx, `
			UPDATE users SET plan_id = $1 WHERE id = $2
		`, planID, actor.User.ID)
		if err != nil {
			return fmt.Errorf("subscribe user: update user plan: %w", err)
		}
	}

	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "plan.subscribe", "plan", planID, toJSON(map[string]string{"planID": planID}), ip)

	return nil
}

// UserSubscription holds a subscription record with joined plan name.
type UserSubscription struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"userId"`
	PlanID             string     `json:"planId"`
	PlanName           string     `json:"planName"`
	Status             string     `json:"status"`
	StartedAt          time.Time  `json:"startedAt"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	NextPlanID         *string    `json:"nextPlanId,omitempty"`
	CurrentPeriodStart time.Time  `json:"currentPeriodStart"`
	CurrentPeriodEnd   *time.Time `json:"currentPeriodEnd,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}

// ListUserSubscriptions returns a user's subscriptions with joined plan names.
func (s *Service) ListUserSubscriptions(ctx context.Context, userID string) ([]*UserSubscription, error) {
	rows, err := s.db().QueryContext(ctx, `
		SELECT sub.id, sub.user_id, sub.package_id, COALESCE(p.name, ''),
		       sub.status, sub.started_at, sub.expires_at,
		       sub.next_plan_id, sub.current_period_start, sub.current_period_end,
		       sub.created_at
		FROM subscriptions sub
		LEFT JOIN packages p ON sub.package_id = p.id
		WHERE sub.user_id = $1
		ORDER BY sub.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*UserSubscription
	for rows.Next() {
		var sub UserSubscription
		var nextPlanID sql.NullString
		var expiresAt sql.NullTime
		var periodEnd sql.NullTime

		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.PlanID, &sub.PlanName,
			&sub.Status, &sub.StartedAt, &expiresAt,
			&nextPlanID, &sub.CurrentPeriodStart, &periodEnd,
			&sub.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		if nextPlanID.Valid {
			sub.NextPlanID = &nextPlanID.String
		}
		if expiresAt.Valid {
			sub.ExpiresAt = &expiresAt.Time
		}
		if periodEnd.Valid {
			sub.CurrentPeriodEnd = &periodEnd.Time
		}
		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// findActiveSubscription returns the ID of a user's active subscription, or empty string if none.
func (s *Service) findActiveSubscription(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db().QueryRowContext(ctx, `
		SELECT id FROM subscriptions
		WHERE user_id = $1 AND status = 'active'
		LIMIT 1
	`, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	return id, err
}

// db returns the underlying *sql.DB from the store.
func (s *Service) db() *sql.DB {
	return s.store.(*SQLStore).db
}
