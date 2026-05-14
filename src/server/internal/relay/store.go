package relay

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

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

// ListChannels 列出所有渠道
func (s *RelayStore) ListChannels() ([]*types.Channel, error) {
	rows, err := s.db.Query(`
		SELECT id, name, provider, base_url, api_key_encrypted, models,
		       rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		       health_check_strategy, probe_model, probe_prompt,
		       strategy, priority, enabled
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
		var probeModel, probePrompt, healthCheckStrategy sql.NullString

		err := rows.Scan(
			&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey,
			pq.Array(&models),
			&ch.RPMLimit, &ch.TPMLimit, &ch.CBThreshold, &ch.CBTimeout,
			&healthCheckStrategy, &probeModel, &probePrompt,
			&ch.Strategy, &ch.Priority, &ch.Enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}

		ch.Models = models
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
	var probeModel, probePrompt, healthCheckStrategy sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, provider, base_url, api_key_encrypted, models,
		       rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		       health_check_strategy, probe_model, probe_prompt,
		       strategy, priority, enabled
		FROM channels
		WHERE id = $1
	`, id).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey,
		pq.Array(&models),
		&ch.RPMLimit, &ch.TPMLimit, &ch.CBThreshold, &ch.CBTimeout,
		&healthCheckStrategy, &probeModel, &probePrompt,
		&ch.Strategy, &ch.Priority, &ch.Enabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}

	ch.Models = models
	ch.HealthCheckStrategy = healthCheckStrategy.String
	ch.ProbeModel = probeModel.String
	ch.ProbePrompt = probePrompt.String

	return ch, nil
}

// CreateChannel 创建渠道
func (s *RelayStore) CreateChannel(ch *types.Channel) error {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO channels (id, name, provider, base_url, api_key_encrypted, models,
		                      rpm_limit, tpm_limit, cb_threshold, cb_timeout,
		                      health_check_strategy, probe_model, probe_prompt,
		                      strategy, priority, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`, ch.ID, ch.Name, ch.Provider, ch.BaseURL, ch.APIKey, pq.Array(ch.Models),
		ch.RPMLimit, ch.TPMLimit, ch.CBThreshold, ch.CBTimeout,
		ch.HealthCheckStrategy, ch.ProbeModel, ch.ProbePrompt,
		ch.Strategy, ch.Priority, ch.Enabled, now, now)
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
			name = $2, provider = $3, base_url = $4, api_key_encrypted = $5, models = $6,
			rpm_limit = $7, tpm_limit = $8, cb_threshold = $9, cb_timeout = $10,
			health_check_strategy = $11, probe_model = $12, probe_prompt = $13,
			strategy = $14, priority = $15, enabled = $16, updated_at = $17
		WHERE id = $1
	`, ch.ID, ch.Name, ch.Provider, ch.BaseURL, ch.APIKey, pq.Array(ch.Models),
		ch.RPMLimit, ch.TPMLimit, ch.CBThreshold, ch.CBTimeout,
		ch.HealthCheckStrategy, ch.ProbeModel, ch.ProbePrompt,
		ch.Strategy, ch.Priority, ch.Enabled, now)
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
		       c.name, c.provider, c.base_url, c.models, c.enabled
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

		err := rows.Scan(
			&rc.ChannelID, &rc.Weight, &rc.Priority, &rc.Enabled,
			&ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&chModels), &ch.Enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel weight: %w", err)
		}

		ch.ID = rc.ChannelID
		ch.Models = chModels
		rc.Channel = &ch
		rc.Healthy = ch.Enabled

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

// LoadPoolFromStore 从数据库加载渠道池
func (s *RelayStore) LoadPoolFromStore(pool *ChannelPool) error {
	// Load channels
	channels, err := s.ListChannels()
	if err != nil {
		return err
	}

	for _, ch := range channels {
		pool.UpdateChannel(ch)
	}

	// Load model routes for each channel's models
	modelSet := make(map[string]bool)
	for _, ch := range channels {
		for _, model := range ch.Models {
			modelSet[model] = true
		}
	}

	for model := range modelSet {
		route, err := s.GetModelRoute(model)
		if err != nil {
			return err
		}
		if route != nil {
			pool.UpdateRoute(route)
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
							Channel:   ch,
							ChannelID: ch.ID,
							Weight:    100,
							Priority:  ch.Priority,
							Enabled:   ch.Enabled,
							Healthy:   ch.Enabled,
						})
					}
				}
			}
			if len(defaultRoute.Channels) > 0 {
				pool.UpdateRoute(defaultRoute)
			}
		}
	}

	return nil
}
