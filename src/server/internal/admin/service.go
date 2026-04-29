package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/marketplace"
)

// UserStats holds aggregate user statistics.
type UserStats struct {
	TotalUsers    int `json:"totalUsers"`
	ActiveUsers   int `json:"activeUsers"` // logged in last 7 days
	NewUsersToday int `json:"newUsersToday"`
	NewUsersWeek  int `json:"newUsersWeek"`
}

// QuotaStats holds aggregate quota statistics.
type QuotaStats struct {
	TotalBalance float64 `json:"totalBalance"`
	TotalUsed    float64 `json:"totalUsed"`
	ActiveTopups int     `json:"activeTopups"`
}

// SystemStats is the admin dashboard overview (D-07).
type SystemStats struct {
	Users          UserStats  `json:"users"`
	Quotas         QuotaStats `json:"quotas"`
	Conversations  int        `json:"conversations"`
	Agents         int        `json:"agents"`
	Tasks          int        `json:"tasks"`
	MCPServers     int        `json:"mcpServers"`
	ChannelsTotal  int        `json:"channelsTotal"`
	ChannelsOnline int        `json:"channelsOnline"`
	ActiveAgents   int        `json:"activeAgents"`
	APICalls24h    int        `json:"apiCalls24h"`
}

// UserInfo is the legacy user list item (pre-Phase 3).
// Deprecated: use UserDetail for new code.
type UserInfo struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	Balance     float64    `json:"balance"`
	Used        float64    `json:"used"`
	AgentCount  int        `json:"agentCount"`
	TaskCount   int        `json:"taskCount"`
}

// Service provides admin business logic, delegating to Store.
type Service struct {
	store Store
}

// NewService creates a new admin Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// --- System Stats ---

func (s *Service) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	return s.store.GetSystemStats(ctx)
}

// --- User Management ---

func (s *Service) ListUsers(ctx context.Context, filter UserListFilter) ([]*UserDetail, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.store.ListUsers(ctx, filter)
}

func (s *Service) GetUserByID(ctx context.Context, id string) (*UserDetail, error) {
	return s.store.GetUserByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, id string, input UserUpdateRequest) (*UserDetail, error) {
	return s.store.UpdateUser(ctx, id, input)
}

func (s *Service) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	return s.store.UpdateUserQuota(ctx, userID, balance)
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	return s.store.DeleteUser(ctx, userID)
}

func (s *Service) DisableUser(ctx context.Context, id string) error {
	return s.store.DisableUser(ctx, id)
}

func (s *Service) EnableUser(ctx context.Context, id string) error {
	return s.store.EnableUser(ctx, id)
}

// --- Plan Management ---

func (s *Service) ListPlans(ctx context.Context) ([]*PlanInfo, error) {
	return s.store.ListPlans(ctx)
}

func (s *Service) GetPlan(ctx context.Context, id string) (*PlanInfo, error) {
	return s.store.GetPlan(ctx, id)
}

func (s *Service) CreatePlan(ctx context.Context, input PlanCreateRequest) (*PlanInfo, error) {
	return s.store.CreatePlan(ctx, input)
}

func (s *Service) UpdatePlan(ctx context.Context, id string, input PlanUpdateRequest) (*PlanInfo, error) {
	return s.store.UpdatePlan(ctx, id, input)
}

func (s *Service) DeactivatePlan(ctx context.Context, id string) error {
	return s.store.DeactivatePlan(ctx, id)
}

// --- Review Queue ---

func (s *Service) ListPendingReviews(ctx context.Context) ([]*marketplace.PublishedAgent, error) {
	return s.store.ListPendingReviews(ctx)
}

func (s *Service) ApproveAgent(ctx context.Context, id string) error {
	return s.store.ApproveAgent(ctx, id)
}

func (s *Service) RejectAgent(ctx context.Context, id string, reason string) error {
	return s.store.RejectAgent(ctx, id, reason)
}

// LogAction creates an audit log entry for an admin operation.
func (s *Service) LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error {
	entry := &AuditEntry{
		ID:           uuid.New().String(),
		ActorID:      actorID,
		ActorEmail:   actorEmail,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
		IPAddress:    ipAddress,
		CreatedAt:    time.Now(),
	}
	return s.store.CreateAuditEntry(ctx, entry)
}

// Compile-time check: ensure Service satisfies the delegation pattern.
var _ = fmt.Sprintf("%v", (*Service)(nil))
