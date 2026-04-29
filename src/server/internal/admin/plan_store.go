package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
)

// PlanFilter contains filter parameters for listing plans.
type PlanFilter struct {
	IsActive *bool
	IsPublic *bool
	Search   string
	Limit    int
	Offset   int
}

// PlanStore defines CRUD operations on subscription plans (packages table).
type PlanStore interface {
	ListPlans(ctx context.Context, filter PlanFilter) ([]*PlanInfo, error)
	GetPlan(ctx context.Context, id string) (*PlanInfo, error)
	CreatePlan(ctx context.Context, input PlanCreateRequest) (*PlanInfo, error)
	UpdatePlan(ctx context.Context, id string, input PlanUpdateRequest) (*PlanInfo, error)
	DeactivatePlan(ctx context.Context, id string) error
}

// ListPlans returns plans with optional filters.
// Supports: IsActive, IsPublic, Search (name ILIKE), Limit, Offset.
// Sorted by sort_order ASC, then created_at DESC.
func (s *SQLStore) ListPlans(ctx context.Context, filter PlanFilter) ([]*PlanInfo, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *filter.IsActive)
		argIdx++
	}
	if filter.IsPublic != nil {
		conditions = append(conditions, fmt.Sprintf("is_public = $%d", argIdx))
		args = append(args, *filter.IsPublic)
		argIdx++
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, name, description, quota_amount, token_quota, price,
		       model_access, agent_limit, duration_days, is_active, is_public,
		       sort_order, created_at, updated_at
		FROM packages
		%s
		ORDER BY sort_order ASC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	var plans []*PlanInfo
	for rows.Next() {
		var p PlanInfo
		var description sql.NullString
		var models []string
		var durationDays sql.NullInt64

		if err := rows.Scan(
			&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price,
			pq.Array(&models), &p.AgentLimit, &durationDays,
			&p.IsActive, &p.IsPublic, &p.SortOrder,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}

		p.Description = description.String
		p.ModelAccess = models
		if p.ModelAccess == nil {
			p.ModelAccess = []string{}
		}
		if durationDays.Valid {
			days := int(durationDays.Int64)
			p.DurationDays = &days
		}
		plans = append(plans, &p)
	}

	return plans, rows.Err()
}

// GetPlan returns a single plan by ID.
func (s *SQLStore) GetPlan(ctx context.Context, id string) (*PlanInfo, error) {
	var p PlanInfo
	var description sql.NullString
	var models []string
	var durationDays sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, quota_amount, token_quota, price,
		       model_access, agent_limit, duration_days, is_active, is_public,
		       sort_order, created_at, updated_at
		FROM packages
		WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price,
		pq.Array(&models), &p.AgentLimit, &durationDays,
		&p.IsActive, &p.IsPublic, &p.SortOrder,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}

	p.Description = description.String
	p.ModelAccess = models
	if p.ModelAccess == nil {
		p.ModelAccess = []string{}
	}
	if durationDays.Valid {
		days := int(durationDays.Int64)
		p.DurationDays = &days
	}

	return &p, nil
}

// CreatePlan inserts a new plan into the packages table.
// Uses auth.NewID("pkg") for ID generation. Stores model_access as PostgreSQL TEXT[] via pq.Array.
func (s *SQLStore) CreatePlan(ctx context.Context, input PlanCreateRequest) (*PlanInfo, error) {
	id, err := auth.NewID("pkg")
	if err != nil {
		return nil, fmt.Errorf("generate plan id: %w", err)
	}

	models := input.ModelAccess
	if models == nil {
		models = []string{}
	}

	var p PlanInfo
	var description sql.NullString
	var scannedModels []string
	var durationDays sql.NullInt64

	err = s.db.QueryRowContext(ctx, `
		INSERT INTO packages (id, name, description, quota_amount, token_quota, price,
		                      model_access, agent_limit, duration_days, is_active, is_public,
		                      sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6,
		        $7, $8, $9, true, $10,
		        $11, NOW(), NOW())
		RETURNING id, name, description, quota_amount, token_quota, price,
		          model_access, agent_limit, duration_days, is_active, is_public,
		          sort_order, created_at, updated_at
	`, id, input.Name, nullString(input.Description), input.QuotaAmount, input.TokenQuota, input.Price,
		pq.Array(models), input.AgentLimit, input.DurationDays, input.IsPublic,
		input.SortOrder).Scan(
		&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price,
		pq.Array(&scannedModels), &p.AgentLimit, &durationDays,
		&p.IsActive, &p.IsPublic, &p.SortOrder,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}

	p.Description = description.String
	p.ModelAccess = scannedModels
	if p.ModelAccess == nil {
		p.ModelAccess = []string{}
	}
	if durationDays.Valid {
		days := int(durationDays.Int64)
		p.DurationDays = &days
	}

	return &p, nil
}

// UpdatePlan updates an existing plan using COALESCE for nullable pointer fields.
// Only non-nil fields in the input are applied; nil fields retain existing values.
// Always sets updated_at = NOW().
func (s *SQLStore) UpdatePlan(ctx context.Context, id string, input PlanUpdateRequest) (*PlanInfo, error) {
	var p PlanInfo
	var description sql.NullString
	var scannedModels []string
	var durationDays sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		UPDATE packages SET
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			quota_amount = COALESCE($3, quota_amount),
			token_quota = COALESCE($4, token_quota),
			price = COALESCE($5, price),
			model_access = COALESCE($6, model_access),
			agent_limit = COALESCE($7, agent_limit),
			is_active = COALESCE($8, is_active),
			is_public = COALESCE($9, is_public),
			updated_at = NOW()
		WHERE id = $10
		RETURNING id, name, description, quota_amount, token_quota, price,
		          model_access, agent_limit, duration_days, is_active, is_public,
		          sort_order, created_at, updated_at
	`,
		input.Name,
		input.Description,
		input.QuotaAmount,
		input.TokenQuota,
		input.Price,
		pq.Array(coalesceModels(input.ModelAccess)),
		input.AgentLimit,
		input.IsActive,
		input.IsPublic,
		id,
	).Scan(
		&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price,
		pq.Array(&scannedModels), &p.AgentLimit, &durationDays,
		&p.IsActive, &p.IsPublic, &p.SortOrder,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update plan: %w", err)
	}

	p.Description = description.String
	p.ModelAccess = scannedModels
	if p.ModelAccess == nil {
		p.ModelAccess = []string{}
	}
	if durationDays.Valid {
		days := int(durationDays.Int64)
		p.DurationDays = &days
	}

	return &p, nil
}

// DeactivatePlan sets a plan's is_active to false. Does NOT delete — subscriptions may
// reference this plan for historical billing. Returns error if no rows were affected.
func (s *SQLStore) DeactivatePlan(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE packages SET is_active = false, updated_at = $1 WHERE id = $2
	`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("deactivate plan: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("deactivate plan rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("deactivate plan: plan %s not found", id)
	}
	return nil
}

// nullString returns nil if the string is empty, otherwise returns the string.
// Used to store NULL instead of empty string in the description column.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
