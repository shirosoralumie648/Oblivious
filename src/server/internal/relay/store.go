package relay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/types"
)

// RelayStore 持久化渠道和模型路由配置
type RelayStore struct {
	db *sql.DB
}

// NewRelayStore 创建 RelayStore
func NewRelayStore(db *sql.DB) *RelayStore {
	return &RelayStore{db: db}
}

func normalizeCostMultiplier(multiplier float64) float64 {
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func normalizeChannelWeight(weight int) int {
	if weight <= 0 {
		return 100
	}
	return weight
}

// ListChannels 列出所有渠道
func (s *RelayStore) ListChannels() ([]*types.Channel, error) {
	rows, err := s.db.Query(`
		SELECT id, name, provider, base_url, api_key_encrypted, models, groups,
		       rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		       health_check_strategy, probe_model, probe_prompt,
		       strategy, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled
		FROM channels
		WHERE enabled = true
		ORDER BY priority DESC, name
	`)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	defer rows.Close()

	var channels []*types.Channel
	for rows.Next() {
		ch := &types.Channel{}
		var models []string
		var groups []string
		var probeModel, probePrompt, healthCheckStrategy sql.NullString

		err := rows.Scan(
			&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey,
			pq.Array(&models), pq.Array(&groups),
			&ch.RPMLimit, &ch.TPMLimit, &ch.CBThreshold, &ch.CBTimeout,
			&healthCheckStrategy, &probeModel, &probePrompt,
			&ch.Strategy, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}

		ch.Models = models
		ch.Groups = groups
		ch.HealthCheckStrategy = healthCheckStrategy.String
		ch.ProbeModel = probeModel.String
		ch.ProbePrompt = probePrompt.String

		channels = append(channels, ch)
	}

	return channels, rows.Err()
}

// GetChannel 根据 ID 获取渠道
func (s *RelayStore) GetChannel(id string) (*types.Channel, error) {
	ch := &types.Channel{}
	var models []string
	var groups []string
	var probeModel, probePrompt, healthCheckStrategy sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, provider, base_url, api_key_encrypted, models, groups,
		       rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		       health_check_strategy, probe_model, probe_prompt,
		       strategy, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled
		FROM channels
		WHERE id = $1
	`, id).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey,
		pq.Array(&models), pq.Array(&groups),
		&ch.RPMLimit, &ch.TPMLimit, &ch.CBThreshold, &ch.CBTimeout,
		&healthCheckStrategy, &probeModel, &probePrompt,
		&ch.Strategy, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}

	ch.Models = models
	ch.Groups = groups
	ch.HealthCheckStrategy = healthCheckStrategy.String
	ch.ProbeModel = probeModel.String
	ch.ProbePrompt = probePrompt.String

	return ch, nil
}

// CreateChannel 创建渠道
func (s *RelayStore) CreateChannel(ch *types.Channel) error {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO channels (id, name, provider, base_url, api_key_encrypted, models, groups,
		                      rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		                      health_check_strategy, probe_model, probe_prompt,
		                      strategy, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`, ch.ID, ch.Name, ch.Provider, ch.BaseURL, ch.APIKey, pq.Array(ch.Models), pq.Array(ch.Groups),
		ch.RPMLimit, ch.TPMLimit, ch.CBThreshold, ch.CBTimeout,
		ch.HealthCheckStrategy, ch.ProbeModel, ch.ProbePrompt,
		ch.Strategy, ch.Priority, normalizeChannelWeight(ch.Weight), ch.EstimatedCostPer1K, normalizeCostMultiplier(ch.CostMultiplier), ch.Enabled, now, now)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

// UpdateChannel 更新渠道
func (s *RelayStore) UpdateChannel(ch *types.Channel) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE channels SET
			name = $2, provider = $3, base_url = $4, api_key_encrypted = $5, models = $6, groups = $7,
			rpm_limit = $8, tpm_limit = $9, cb_threshold = $10, cb_timeout = $11,
			health_check_strategy = $12, probe_model = $13, probe_prompt = $14,
			strategy = $15, priority = $16, weight = $17, estimated_cost_per_1k = $18, cost_multiplier = $19, enabled = $20, updated_at = $21
		WHERE id = $1
	`, ch.ID, ch.Name, ch.Provider, ch.BaseURL, ch.APIKey, pq.Array(ch.Models), pq.Array(ch.Groups),
		ch.RPMLimit, ch.TPMLimit, ch.CBThreshold, ch.CBTimeout,
		ch.HealthCheckStrategy, ch.ProbeModel, ch.ProbePrompt,
		ch.Strategy, ch.Priority, normalizeChannelWeight(ch.Weight), ch.EstimatedCostPer1K, normalizeCostMultiplier(ch.CostMultiplier), ch.Enabled, now)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	return nil
}

// DeleteChannel 删除渠道
func (s *RelayStore) DeleteChannel(id string) error {
	_, err := s.db.Exec(`DELETE FROM channels WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}

// GetModelRoute 获取模型路由
func (s *RelayStore) GetModelRoute(model string) (*types.ModelRoute, error) {
	route := &types.ModelRoute{Model: model}
	err := s.db.QueryRow(`
		SELECT id, strategy FROM model_routes WHERE model = $1
	`, model).Scan(&route.ID, &route.Strategy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get model route: %w", err)
	}

	// Load channel weights
	rows, err := s.db.Query(`
		SELECT mcw.channel_id, mcw.weight, mcw.priority, mcw.enabled,
		       c.name, c.provider, c.base_url, c.models, c.groups,
		       c.estimated_cost_per_1k, c.cost_multiplier, c.enabled
		FROM model_channel_weights mcw
		JOIN channels c ON c.id = mcw.channel_id
		WHERE mcw.route_id = $1
		ORDER BY mcw.priority DESC, mcw.weight DESC
	`, route.ID)
	if err != nil {
		return nil, fmt.Errorf("get channel weights: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rc types.RouteChannel
		var ch types.Channel
		var chModels []string
		var chGroups []string

		err := rows.Scan(
			&rc.ChannelID, &rc.Weight, &rc.Priority, &rc.Enabled,
			&ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&chModels), pq.Array(&chGroups),
			&ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel weight: %w", err)
		}

		ch.ID = rc.ChannelID
		ch.Models = chModels
		ch.Groups = chGroups
		rc.Channel = &ch
		rc.Healthy = ch.Enabled
		rc.EstimatedCostPer1K = ch.EstimatedCostPer1K
		rc.CostMultiplier = ch.CostMultiplier

		route.Channels = append(route.Channels, rc)
	}

	return route, rows.Err()
}

// SetModelRoute 设置模型路由
func (s *RelayStore) SetModelRoute(route *types.ModelRoute) error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Upsert model route
	var routeID string
	err = tx.QueryRow(`
		INSERT INTO model_routes (id, model, strategy, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (model) DO UPDATE SET strategy = $3
		RETURNING id
	`, route.ID, route.Model, route.Strategy).Scan(&routeID)
	if err != nil {
		return fmt.Errorf("upsert model route: %w", err)
	}

	// Delete existing weights
	_, err = tx.Exec(`DELETE FROM model_channel_weights WHERE route_id = $1`, routeID)
	if err != nil {
		return fmt.Errorf("delete weights: %w", err)
	}

	// Insert new weights
	for _, rc := range route.Channels {
		_, err = tx.Exec(`
			INSERT INTO model_channel_weights (id, route_id, channel_id, weight, priority, enabled)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, rc.ChannelID+"_"+routeID, routeID, rc.ChannelID, rc.Weight, rc.Priority, rc.Enabled)
		if err != nil {
			return fmt.Errorf("insert weight: %w", err)
		}
	}

	return tx.Commit()
}

func (s *RelayStore) SaveFileMapping(ctx context.Context, record handler.FileMappingRecord) error {
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO relay_file_mappings (
			local_file_id,
			openai_file_id,
			local_path,
			size_bytes,
			user_id,
			organization_id,
			request_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, record.LocalFileID, record.OpenAIFileID, record.LocalPath, record.SizeBytes,
		record.UserID, record.OrganizationID, record.RequestID, createdAt)
	if err != nil {
		return fmt.Errorf("save relay file mapping: %w", err)
	}
	return nil
}

func (s *RelayStore) GetFileMapping(ctx context.Context, localFileID, userID, organizationID string) (handler.FileMappingRecord, error) {
	var record handler.FileMappingRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT local_file_id, openai_file_id, local_path, size_bytes,
		       user_id, organization_id, request_id, created_at
		FROM relay_file_mappings
		WHERE local_file_id = $1
		  AND user_id = $2
		  AND organization_id = $3
	`, localFileID, userID, organizationID).Scan(
		&record.LocalFileID,
		&record.OpenAIFileID,
		&record.LocalPath,
		&record.SizeBytes,
		&record.UserID,
		&record.OrganizationID,
		&record.RequestID,
		&record.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return handler.FileMappingRecord{}, handler.ErrFileMappingNotFound
	}
	if err != nil {
		return handler.FileMappingRecord{}, fmt.Errorf("get relay file mapping: %w", err)
	}
	return record, nil
}

func (s *RelayStore) SaveConversationAffinity(ctx context.Context, conversationID, channelID string) error {
	conversationID = strings.TrimSpace(conversationID)
	channelID = strings.TrimSpace(channelID)
	if conversationID == "" || channelID == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO relay_conversation_affinity (conversation_id, channel_id, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (conversation_id)
		DO UPDATE SET channel_id = EXCLUDED.channel_id, updated_at = NOW()
	`, conversationID, channelID)
	if err != nil {
		return fmt.Errorf("save relay conversation affinity: %w", err)
	}
	return nil
}

func (s *RelayStore) GetConversationAffinity(ctx context.Context, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", nil
	}

	var channelID string
	err := s.db.QueryRowContext(ctx, `
		SELECT channel_id
		FROM relay_conversation_affinity
		WHERE conversation_id = $1
	`, conversationID).Scan(&channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get relay conversation affinity: %w", err)
	}
	return strings.TrimSpace(channelID), nil
}

// LoadPoolFromStore 从数据库加载渠道池
func (s *RelayStore) LoadPoolFromStore(pool *ChannelPool) error {
	// Load channels
	channels, routes, err := s.poolConfigFromStore()
	if err != nil {
		return err
	}

	for _, ch := range channels {
		pool.UpdateChannel(ch)
	}

	for _, route := range routes {
		pool.UpdateRoute(route)
	}

	return nil
}

func (s *RelayStore) ReloadPoolFromStore(pool *ChannelPool) error {
	channels, routes, err := s.poolConfigFromStore()
	if err != nil {
		return err
	}
	pool.ReplaceConfig(channels, routes)
	return nil
}

func (s *RelayStore) poolConfigFromStore() ([]*types.Channel, []*types.ModelRoute, error) {
	channels, err := s.ListChannels()
	if err != nil {
		return nil, nil, err
	}

	// Load model routes for each channel's models
	modelSet := make(map[string]bool)
	for _, ch := range channels {
		for _, model := range ch.Models {
			modelSet[model] = true
		}
	}

	var routes []*types.ModelRoute
	for model := range modelSet {
		route, err := s.GetModelRoute(model)
		if err != nil {
			return nil, nil, err
		}
		if route != nil {
			routes = append(routes, route)
		} else {
			// Create default route for model
			defaultRoute := &types.ModelRoute{
				Model:    model,
				Strategy: "weighted",
			}
			for _, ch := range channels {
				for _, m := range ch.Models {
					if m == model {
						defaultRoute.Channels = append(defaultRoute.Channels, types.RouteChannel{
							Channel:            ch,
							ChannelID:          ch.ID,
							Weight:             normalizeChannelWeight(ch.Weight),
							Priority:           ch.Priority,
							Enabled:            ch.Enabled,
							Healthy:            ch.Enabled,
							EstimatedCostPer1K: ch.EstimatedCostPer1K,
							CostMultiplier:     ch.CostMultiplier,
						})
					}
				}
			}
			if len(defaultRoute.Channels) > 0 {
				routes = append(routes, defaultRoute)
			}
		}
	}

	return channels, routes, nil
}
