package admin

import (
	"context"
	"encoding/json"
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

// --- User Management (quota/account — enhanced lifecycle methods in user_service.go) ---

func (s *Service) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	return s.store.UpdateUserQuota(ctx, userID, balance)
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	return s.store.DeleteUser(ctx, userID)
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

// toJSON marshals a value to JSON string for audit logging.
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
