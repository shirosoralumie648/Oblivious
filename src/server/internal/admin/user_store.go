package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// UserStore defines operations on users for admin management (D-12, D-13, D-16).
// Enhanced beyond the basic CRUD — includes search, filter, sort, pagination,
// usage stats via JOINs, session revocation on disable, and audit-ready mutations.
type UserStore interface {
	ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, int, error)
	GetUserByID(ctx context.Context, id string) (*UserDetail, error)
	UpdateUser(ctx context.Context, id string, input UserUpdateRequest) (*UserDetail, error)
	DisableUser(ctx context.Context, id string) error
	EnableUser(ctx context.Context, id string) error
	CountUsers(ctx context.Context, filter UserListFilter) (int, error)
}

// --- ListUsers (enhanced: search, filter, sort, paginate, usage stats) ---

func (s *SQLStore) ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, int, error) {
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

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(u.name ILIKE $%d OR u.email ILIKE $%d)", argIdx, argIdx+1))
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%")
		argIdx += 2
	}
	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("u.role = $%d", argIdx))
		args = append(args, filter.Role)
		argIdx++
	}
	if filter.PlanID != "" {
		conditions = append(conditions, fmt.Sprintf("u.plan_id = $%d", argIdx))
		args = append(args, filter.PlanID)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("u.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Sort per D-12: multi-sort support with graceful fallback
	orderBy := "ORDER BY u.created_at DESC"
	switch filter.Sort {
	case "":
		// default
	case "created_at_desc":
		orderBy = "ORDER BY u.created_at DESC"
	case "created_at_asc":
		orderBy = "ORDER BY u.created_at ASC"
	case "name_asc":
		orderBy = "ORDER BY u.name ASC"
	case "name_desc":
		orderBy = "ORDER BY u.name DESC"
	case "tokens_desc":
		orderBy = "ORDER BY total_tokens DESC"
	case "last_login_desc":
		orderBy = "ORDER BY u.last_login_at DESC NULLS LAST"
	default:
		// unknown sort — fall back to default
		orderBy = "ORDER BY u.created_at DESC"
	}

	// Count total (without JOINs for performance)
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM users u %s`, where)
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Data query with usage stats JOINs
	dataQuery := fmt.Sprintf(`
		SELECT u.id, u.email, u.name, u.role, u.plan_id,
		       COALESCE(p.name, '') AS plan_name, u.status,
		       u.created_at, u.last_login_at,
		       COALESCE(SUM(ur.input_tokens + ur.output_tokens), 0) AS total_tokens,
		       COALESCE(SUM(ur.request_count), 0) AS total_api_calls,
		       COALESCE(q.used, 0) AS total_cost
		FROM users u
		LEFT JOIN packages p ON u.plan_id = p.id
		LEFT JOIN usage_records ur ON u.id = ur.user_id
		LEFT JOIN quotas q ON u.id = q.user_id
		%s
		GROUP BY u.id, p.name, q.used
		%s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*UserDetail
	for rows.Next() {
		var u UserDetail
		var planID, planName sql.NullString
		var lastLogin sql.NullTime
		var usage UserUsageStats

		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role,
			&planID, &planName,
			&u.Status, &u.CreatedAt, &lastLogin,
			&usage.TotalTokens, &usage.TotalAPICalls, &usage.TotalCost); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		if planID.Valid {
			u.PlanID = &planID.String
		}
		if planName.Valid && planName.String != "" {
			u.PlanName = &planName.String
		}
		if lastLogin.Valid {
			u.LastLoginAt = &lastLogin.Time
		}
		u.UsageStats = &usage
		users = append(users, &u)
	}

	return users, total, rows.Err()
}

// --- GetUserByID (enhanced: usage stats JOIN per D-13) ---

func (s *SQLStore) GetUserByID(ctx context.Context, id string) (*UserDetail, error) {
	var u UserDetail
	var planID, planName sql.NullString
	var lastLogin sql.NullTime
	var usage UserUsageStats

	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.role, u.plan_id,
		       COALESCE(p.name, '') AS plan_name, u.status,
		       u.created_at, u.last_login_at,
		       COALESCE(SUM(ur.input_tokens + ur.output_tokens), 0) AS total_tokens,
		       COALESCE(SUM(ur.request_count), 0) AS total_api_calls,
		       COALESCE(q.used, 0) AS total_cost
		FROM users u
		LEFT JOIN packages p ON u.plan_id = p.id
		LEFT JOIN usage_records ur ON u.id = ur.user_id
		LEFT JOIN quotas q ON u.id = q.user_id
		WHERE u.id = $1
		GROUP BY u.id, p.name, q.used
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role,
		&planID, &planName,
		&u.Status, &u.CreatedAt, &lastLogin,
		&usage.TotalTokens, &usage.TotalAPICalls, &usage.TotalCost)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if planID.Valid {
		u.PlanID = &planID.String
	}
	if planName.Valid && planName.String != "" {
		u.PlanName = &planName.String
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	u.UsageStats = &usage

	return &u, nil
}

// --- UpdateUser (D-16: role/plan assignment with validation — validation done in service layer) ---

func (s *SQLStore) UpdateUser(ctx context.Context, id string, input UserUpdateRequest) (*UserDetail, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("update user begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update user fields with COALESCE/NULLIF for optional pointer values.
	// $1: role, $2: plan_id, $3: status, $4: id
	_, err = tx.ExecContext(ctx, `
		UPDATE users SET
			role = COALESCE(NULLIF($1::text, ''), role),
			plan_id = COALESCE($2::text, plan_id),
			status = COALESCE(NULLIF($3::text, ''), status)
		WHERE id = $4
	`, coalesceUserRole(input.Role), coalesceString(input.PlanID), coalesceString(input.Status), id)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// If plan_id changed and user has an active subscription, update subscription too.
	if input.PlanID != nil && *input.PlanID != "" {
		_, err = tx.ExecContext(ctx, `
			UPDATE subscriptions SET package_id = $1
			WHERE user_id = $2 AND status = 'active'
		`, *input.PlanID, id)
		if err != nil {
			return nil, fmt.Errorf("update user subscription: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("update user commit: %w", err)
	}

	// Re-fetch the updated user with usage stats
	return s.GetUserByID(ctx, id)
}

// coalesceUserRole returns the string value or empty string for NULLIF usage.
func coalesceUserRole(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// --- DisableUser (D-12: full lifecycle — revokes ALL active sessions in a transaction) ---

func (s *SQLStore) DisableUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("disable user begin tx: %w", err)
	}
	defer tx.Rollback()

	// Set user status to disabled
	result, err := tx.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	// Revoke all active sessions (user immediately logged out everywhere)
	_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, id)
	if err != nil {
		return fmt.Errorf("disable user revoke sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("disable user commit: %w", err)
	}

	return nil
}

// --- EnableUser (D-12: sets status active; user must log in again) ---

func (s *SQLStore) EnableUser(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("enable user: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}
	return nil
}

// --- CountUsers (used for dashboard stats D-07, and internal health checks) ---

func (s *SQLStore) CountUsers(ctx context.Context, filter UserListFilter) (int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx+1))
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%")
		argIdx += 2
	}
	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, filter.Role)
		argIdx++
	}
	if filter.PlanID != "" {
		conditions = append(conditions, fmt.Sprintf("plan_id = $%d", argIdx))
		args = append(args, filter.PlanID)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM users %s`, where)
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}
