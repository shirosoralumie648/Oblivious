package admin

import (
	"context"
	"database/sql"
	"fmt"

	"oblivious/server/internal/marketplace"
)

// Store defines all admin CRUD operations for Phase 3 channel, route,
// plan, user, audit, and review-queue management.
type Store interface {
	// Embedded sub-interfaces for channel, route, audit operations
	ChannelStore
	RouteStore
	AuditStore

	// System stats (D-07: admin dashboard)
	GetSystemStats(ctx context.Context) (*SystemStats, error)

	// User management (D-12, D-13)
	ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, error)
	GetUserByID(ctx context.Context, id string) (*UserDetail, error)
	UpdateUser(ctx context.Context, id string, input UserUpdateRequest) (*UserDetail, error)
	UpdateUserQuota(ctx context.Context, userID string, balance float64) error
	DeleteUser(ctx context.Context, userID string) error
	DisableUser(ctx context.Context, id string) error
	EnableUser(ctx context.Context, id string) error

	// Plan CRUD (D-10, D-11)
	ListPlans(ctx context.Context) ([]*PlanInfo, error)
	GetPlan(ctx context.Context, id string) (*PlanInfo, error)
	CreatePlan(ctx context.Context, input PlanCreateRequest) (*PlanInfo, error)
	UpdatePlan(ctx context.Context, id string, input PlanUpdateRequest) (*PlanInfo, error)
	DeactivatePlan(ctx context.Context, id string) error

	// Review queue (D-17)
	ListPendingReviews(ctx context.Context) ([]*marketplace.PublishedAgent, error)
	ApproveAgent(ctx context.Context, id string) error
	RejectAgent(ctx context.Context, id string, reason string) error
}

// SQLStore implements Store using database/sql.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new SQLStore.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// --- System Stats ---

func (s *SQLStore) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{}

	// User stats
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE last_login_at > NOW() - INTERVAL '7 days') as active,
			COUNT(*) FILTER (WHERE created_at > CURRENT_DATE) as today,
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '7 days') as week
		FROM users
	`).Scan(&stats.Users.TotalUsers, &stats.Users.ActiveUsers, &stats.Users.NewUsersToday, &stats.Users.NewUsersWeek)
	if err != nil {
		return nil, fmt.Errorf("get user stats: %w", err)
	}

	// Quota stats
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(balance), 0), COALESCE(SUM(used), 0)
		FROM quotas
	`).Scan(&stats.Quotas.TotalBalance, &stats.Quotas.TotalUsed)
	if err != nil {
		return nil, fmt.Errorf("get quota stats: %w", err)
	}

	// Active topups
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM topup_orders WHERE status = 'paid'
	`).Scan(&stats.Quotas.ActiveTopups)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get topup stats: %w", err)
	}

	// Conversations
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversations`).Scan(&stats.Conversations)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get conversation stats: %w", err)
	}

	// Agents
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents`).Scan(&stats.Agents)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get agent stats: %w", err)
	}

	// Tasks
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&stats.Tasks)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get task stats: %w", err)
	}

	// MCP Servers
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_servers`).Scan(&stats.MCPServers)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get mcp stats: %w", err)
	}

	// Channels stats (D-07 extension)
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE enabled = true)
		FROM channels
	`).Scan(&stats.ChannelsTotal, &stats.ChannelsOnline)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get channel stats: %w", err)
	}

	// Active agents (D-07 extension — agents with recent activity)
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM published_agents WHERE status = 'approved'
	`).Scan(&stats.ActiveAgents)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get active agents: %w", err)
	}

	// API calls in last 24h (D-07 extension)
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM billing_sessions
		WHERE created_at > NOW() - INTERVAL '24 hours'
	`).Scan(&stats.APICalls24h)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get api calls: %w", err)
	}

	return stats, nil
}

// --- User Management ---

func (s *SQLStore) ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.name, u.role, u.plan_id,
		       COALESCE(p.name, ''),
		       u.status, u.created_at, u.last_login_at,
		       COALESCE(q.balance, 0), COALESCE(q.used, 0),
		       (SELECT COUNT(*) FROM agents WHERE user_id = u.id),
		       (SELECT COUNT(*) FROM tasks WHERE user_id = u.id)
		FROM users u
		LEFT JOIN quotas q ON u.id = q.user_id
		LEFT JOIN packages p ON u.plan_id = p.id
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
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
			&usage.TotalCost, &usage.TotalCost,
			&usage.TotalAPICalls, &usage.TotalTokens); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
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
		ou := u
		users = append(users, &ou)
	}

	return users, rows.Err()
}

func (s *SQLStore) GetUserByID(ctx context.Context, id string) (*UserDetail, error) {
	var u UserDetail
	var planID, planName sql.NullString
	var lastLogin sql.NullTime
	var usage UserUsageStats

	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.role, u.plan_id,
		       COALESCE(p.name, ''),
		       u.status, u.created_at, u.last_login_at,
		       COALESCE(q.balance, 0), COALESCE(q.used, 0),
		       (SELECT COUNT(*) FROM agents WHERE user_id = u.id),
		       (SELECT COUNT(*) FROM tasks WHERE user_id = u.id)
		FROM users u
		LEFT JOIN quotas q ON u.id = q.user_id
		LEFT JOIN packages p ON u.plan_id = p.id
		WHERE u.id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role,
		&planID, &planName,
		&u.Status, &u.CreatedAt, &lastLogin,
		&usage.TotalCost, &usage.TotalCost,
		&usage.TotalAPICalls, &usage.TotalTokens)

	if err == sql.ErrNoRows {
		return nil, nil
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

	return &u, nil
}

func (s *SQLStore) UpdateUser(ctx context.Context, id string, input UserUpdateRequest) (*UserDetail, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quotas (id, user_id, balance, used, created_at, updated_at)
		SELECT 'quota_' || $1, $1, $2, 0, NOW(), NOW()
		ON CONFLICT (user_id) DO UPDATE SET balance = $2, updated_at = NOW()
	`, userID, balance)
	return err
}

func (s *SQLStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (s *SQLStore) DisableUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, id)
	return err
}

func (s *SQLStore) EnableUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, id)
	return err
}

// --- Plan CRUD (stubs — implemented in Plan 03) ---

func (s *SQLStore) ListPlans(ctx context.Context) ([]*PlanInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) GetPlan(ctx context.Context, id string) (*PlanInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) CreatePlan(ctx context.Context, input PlanCreateRequest) (*PlanInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) UpdatePlan(ctx context.Context, id string, input PlanUpdateRequest) (*PlanInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) DeactivatePlan(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

// --- Review Queue (stubs — implemented in Plan 05) ---

func (s *SQLStore) ListPendingReviews(ctx context.Context) ([]*marketplace.PublishedAgent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLStore) ApproveAgent(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (s *SQLStore) RejectAgent(ctx context.Context, id string, reason string) error {
	return fmt.Errorf("not implemented")
}

// Ensure SQLStore implements Store at compile time.
var _ Store = (*SQLStore)(nil)
