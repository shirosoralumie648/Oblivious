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
	// Embedded sub-interfaces for channel, route, audit, plan, user operations
	ChannelStore
	RouteStore
	AuditStore
	PlanStore
	UserStore
	BillingInspectionStore
	RelayPricingSettingsStore
	UsageLogStore
	APITokenStore
	ModelInventoryStore

	// System stats (D-07: admin dashboard)
	GetSystemStats(ctx context.Context) (*SystemStats, error)

	// User management (admin quota/account operations not in UserStore)
	UpdateUserQuota(ctx context.Context, userID string, balance float64) error
	DeleteUser(ctx context.Context, userID string) error

	// Review queue (D-17)
	ListPendingReviews(ctx context.Context, status string) ([]*marketplace.PublishedAgent, error)
	ApproveAgent(ctx context.Context, id string) error
	ClaimReview(ctx context.Context, id string, reviewerID string) error
	RejectAgent(ctx context.Context, id string, reason string) error
	RequestAgentChanges(ctx context.Context, id string, reason string) error
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

// --- User Management (quota/account — UserStore methods in user_store.go) ---

func (s *SQLStore) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quotas (id, organization_id, user_id, scope, balance, used, created_at, updated_at)
		SELECT 'quota_' || m.organization_id || '_' || $1, m.organization_id, $1, 'user', $2, 0, NOW(), NOW()
		FROM organization_memberships m
		WHERE m.user_id = $1 AND m.removed_at IS NULL
		ON CONFLICT (organization_id, user_id) WHERE scope = 'user'
		DO UPDATE SET balance = EXCLUDED.balance, updated_at = EXCLUDED.updated_at
	`, userID, balance)
	return err
}

func (s *SQLStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

// --- Review Queue ---

func (s *SQLStore) ListPendingReviews(ctx context.Context, status string) ([]*marketplace.PublishedAgent, error) {
	return marketplace.NewSQLStore(s.db).ListReviewQueue(ctx, status, 20, 0)
}

func (s *SQLStore) ApproveAgent(ctx context.Context, id string) error {
	return marketplace.NewSQLStore(s.db).ApproveAgent(ctx, id, "")
}

func (s *SQLStore) ClaimReview(ctx context.Context, id string, reviewerID string) error {
	return marketplace.NewGovernanceService(marketplace.NewSQLStore(s.db)).AssignReview(ctx, marketplace.GovernanceAction{
		ActorUserID: reviewerID,
		AgentID:     id,
		Reason:      "claimed for review",
	})
}

func (s *SQLStore) RejectAgent(ctx context.Context, id string, reason string) error {
	return marketplace.NewSQLStore(s.db).RejectAgent(ctx, id, "", reason)
}

func (s *SQLStore) RequestAgentChanges(ctx context.Context, id string, reason string) error {
	return marketplace.NewSQLStore(s.db).RequestAgentChanges(ctx, id, "", reason)
}

// Ensure SQLStore implements Store at compile time.
var _ Store = (*SQLStore)(nil)
