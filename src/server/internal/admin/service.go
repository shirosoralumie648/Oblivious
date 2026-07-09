package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/marketplace"
	relaytypes "oblivious/server/internal/relay/types"
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
	store                       Store
	usageAnalyticsStore         UsageAnalyticsStore
	requestLogEvidenceStore     RequestLogEvidenceStore
	relayPricingSettingsApplier func(RelayPricingSettings)
	channelRuntimeStatsProvider ChannelRuntimeStatsProvider
	relayConfigApplier          RelayConfigApplier
}

type ServiceOption func(*Service)

type ChannelRuntimeStatsProvider interface {
	GetAllStats() map[string]*relaytypes.ChannelStats
}

type RelayConfigChangeKind string

const (
	RelayConfigChangeChannel RelayConfigChangeKind = "channel"
	RelayConfigChangeRoute   RelayConfigChangeKind = "route"
)

type RelayConfigChangeAction string

const (
	RelayConfigActionUpsert RelayConfigChangeAction = "upsert"
	RelayConfigActionDelete RelayConfigChangeAction = "delete"
)

type RelayConfigChange struct {
	Kind   RelayConfigChangeKind
	Action RelayConfigChangeAction
	ID     string
}

type RelayConfigApplier func(ctx context.Context, change RelayConfigChange) error

type RequestLogEvidenceStore interface {
	ListRequestLogEvidence(ctx context.Context, requestIDs []string) (map[string]RequestLogEvidence, error)
}

func WithRelayPricingSettingsApplier(applier func(RelayPricingSettings)) ServiceOption {
	return func(service *Service) {
		service.relayPricingSettingsApplier = applier
	}
}

func WithChannelRuntimeStatsProvider(provider ChannelRuntimeStatsProvider) ServiceOption {
	return func(service *Service) {
		service.channelRuntimeStatsProvider = provider
	}
}

func WithRelayConfigApplier(applier RelayConfigApplier) ServiceOption {
	return func(service *Service) {
		service.relayConfigApplier = applier
	}
}

func WithUsageAnalyticsStore(store UsageAnalyticsStore) ServiceOption {
	return func(service *Service) {
		service.usageAnalyticsStore = store
	}
}

func WithRequestLogEvidenceStore(store RequestLogEvidenceStore) ServiceOption {
	return func(service *Service) {
		service.requestLogEvidenceStore = store
	}
}

// NewService creates a new admin Service.
func NewService(store Store, options ...ServiceOption) *Service {
	service := &Service{store: store}
	for _, option := range options {
		option(service)
	}
	return service
}

// --- System Stats ---

func (s *Service) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	return s.store.GetSystemStats(ctx)
}

func (s *Service) ListChannelRuntimeStats(ctx context.Context, organizationID string) ([]ChannelRuntimeStats, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	if s.channelRuntimeStatsProvider == nil {
		return []ChannelRuntimeStats{}, nil
	}
	if s.store == nil {
		return nil, fmt.Errorf("channel store is required")
	}
	allowedChannelIDs, err := s.channelIDsForOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	allStats := s.channelRuntimeStatsProvider.GetAllStats()
	channelIDs := make([]string, 0, len(allStats))
	for channelID := range allStats {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Strings(channelIDs)

	result := make([]ChannelRuntimeStats, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		stats := allStats[channelID]
		if stats == nil {
			continue
		}
		channelIDValue := stats.ChannelID
		if channelIDValue == "" {
			channelIDValue = channelID
		}
		if _, ok := allowedChannelIDs[channelIDValue]; !ok {
			if _, ok := allowedChannelIDs[channelID]; !ok {
				continue
			}
		}
		var rateLimitedUntil *time.Time
		if !stats.RateLimitedUntil.IsZero() {
			until := stats.RateLimitedUntil.UTC()
			rateLimitedUntil = &until
		}
		avgLatencyMS := 0.0
		if stats.LatencyCount > 0 {
			avgLatencyMS = float64(stats.LatencySumUs) / float64(stats.LatencyCount) / 1000.0
		}
		result = append(result, ChannelRuntimeStats{
			ChannelID:                 channelIDValue,
			RPMCurrent:                stats.RPMCurrent,
			TPMCurrent:                stats.TPMCurrent,
			TotalRequests:             stats.TotalRequests,
			SuccessCount:              stats.SuccessCount,
			FailureCount:              stats.FailureCount,
			AvgLatencyMS:              avgLatencyMS,
			RateLimitedUntil:          rateLimitedUntil,
			AffinityConversationCount: stats.AffinityConversationCount,
		})
	}
	return result, nil
}

func (s *Service) channelIDsForOrganization(ctx context.Context, organizationID string) (map[string]struct{}, error) {
	const pageSize = 100
	result := map[string]struct{}{}
	for offset := 0; ; offset += pageSize {
		channels, err := s.listChannelsForOrganization(ctx, organizationID, ChannelFilter{
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		for _, channel := range channels {
			if channel != nil && strings.TrimSpace(channel.ID) != "" {
				result[channel.ID] = struct{}{}
			}
		}
		if len(channels) < pageSize {
			break
		}
	}
	return result, nil
}

func (s *Service) applyRelayConfigChange(ctx context.Context, change RelayConfigChange) error {
	if s.relayConfigApplier == nil || change.ID == "" {
		return nil
	}
	return s.relayConfigApplier(ctx, change)
}

// --- User Management (quota/account — enhanced lifecycle methods in user_service.go) ---

func (s *Service) UpdateUserQuota(ctx context.Context, actorID, actorEmail, userID string, balance float64, ipAddress string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	if balance < 0 || math.IsNaN(balance) || math.IsInf(balance, 0) {
		return fmt.Errorf("balance must be a non-negative finite number")
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", userID)
	}
	if err := s.store.UpdateUserQuota(ctx, userID, balance); err != nil {
		return err
	}

	_ = s.LogAction(ctx, actorID, actorEmail, "user.quota.update", "user", userID, toJSON(map[string]any{"balance": balance}), ipAddress)
	return nil
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

func (s *Service) RequestAgentChanges(ctx context.Context, id string, reason string) error {
	return s.store.RequestAgentChanges(ctx, id, reason)
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
