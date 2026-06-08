package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay/types"
)

// ChannelStore defines operations on relay channels.
type ChannelStore interface {
	ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error)
	GetChannel(ctx context.Context, id string) (*ChannelInfo, error)
	CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error)
	UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error)
	UpdateChannelDiagnostics(ctx context.Context, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error)
	DeleteChannel(ctx context.Context, id string) error
	TestChannel(ctx context.Context, id string) (*ChannelTestResult, error)
	BatchUpdateChannels(ctx context.Context, ids []string, action string) error
}

// ChannelFilter contains filter parameters for listing channels.
type ChannelFilter struct {
	Provider string
	Status   string
	Search   string
	Limit    int
	Offset   int
}

// ListChannels returns channels with optional filters.
func (s *SQLStore) ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error) {
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

	if filter.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, filter.Provider)
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
			SELECT id, name, provider, base_url, models, groups,
			       rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
			       COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
			       created_at, updated_at
		FROM channels
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []*ChannelInfo
	for rows.Next() {
		var ch ChannelInfo
		var models []string
		var groups []string
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
			&ch.RPM, &ch.TPM, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
			&ch.Status, &ch.Latency,
			&ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		ch.Models = models
		ch.Groups = groups
		channels = append(channels, &ch)
	}
	return channels, rows.Err()
}

// GetChannel returns a single channel by ID.
func (s *SQLStore) GetChannel(ctx context.Context, id string) (*ChannelInfo, error) {
	var ch ChannelInfo
	var models []string
	var groups []string

	err := s.db.QueryRowContext(ctx, `
			SELECT id, name, provider, base_url, models, groups,
			       rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		       COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		       created_at, updated_at
		FROM channels
		WHERE id = $1
	`, id).Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		&ch.Status, &ch.Latency,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	ch.Models = models
	ch.Groups = groups
	return &ch, nil
}

// CreateChannel inserts a new channel and returns it.
func (s *SQLStore) CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error) {
	id, err := auth.NewID("ch")
	if err != nil {
		return nil, fmt.Errorf("generate channel id: %w", err)
	}

	models := input.Models
	if models == nil {
		models = []string{}
	}
	groups := input.Groups
	if groups == nil {
		groups = []string{}
	}

	var ch ChannelInfo
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO channels (id, name, provider, base_url, api_key_encrypted, models, groups,
		                      rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, true, NOW(), NOW())
		RETURNING id, name, provider, base_url, models, groups,
		          rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		          COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		          created_at, updated_at
	`, id, input.Name, input.Provider, input.BaseURL, input.APIKey, pq.Array(models), pq.Array(groups),
		input.RpmLimit, input.TpmLimit, input.Priority, normalizeAdminChannelWeight(input.Weight), input.EstimatedCostPer1K, normalizeAdminCostMultiplier(input.CostMultiplier)).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		&ch.Status, &ch.Latency,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	ch.Models = models
	ch.Groups = groups
	return &ch, nil
}

// UpdateChannel updates an existing channel with optional fields.
func (s *SQLStore) UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	// Build dynamic UPDATE with COALESCE/NULLIF for optional pointer fields
	var ch ChannelInfo
	var models []string
	var groups []string

	err := s.db.QueryRowContext(ctx, `
		UPDATE channels SET
			name = COALESCE(NULLIF($1::text,''), name),
			base_url = COALESCE(NULLIF($2::text,''), base_url),
			api_key_encrypted = COALESCE(NULLIF($3::text,''), api_key_encrypted),
			models = COALESCE(NULLIF($4::text[], '{}'), models),
			groups = COALESCE(NULLIF($5::text[], '{}'), groups),
			rpm_limit = COALESCE(NULLIF($6::int, 0), rpm_limit),
			tpm_limit = COALESCE(NULLIF($7::int, 0), tpm_limit),
			priority = COALESCE(NULLIF($8::int, 0), priority),
			weight = COALESCE($9::int, weight),
			estimated_cost_per_1k = COALESCE($10::double precision, estimated_cost_per_1k),
			cost_multiplier = COALESCE($11::double precision, cost_multiplier),
			enabled = COALESCE($12::boolean, enabled),
			updated_at = NOW()
		WHERE id = $13
		RETURNING id, name, provider, base_url, models, groups,
		          rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		          COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		          created_at, updated_at
	`,
		coalesceString(input.Name),
		coalesceString(input.BaseURL),
		coalesceString(input.APIKey),
		pq.Array(coalesceModels(input.Models)),
		pq.Array(coalesceModels(input.Groups)),
		coalesceInt(input.RpmLimit),
		coalesceInt(input.TpmLimit),
		coalesceInt(input.Priority),
		coalesceChannelWeight(input.Weight),
		coalesceFloat(input.EstimatedCostPer1K),
		coalesceCostMultiplier(input.CostMultiplier),
		input.Enabled,
		id,
	).Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		&ch.Status, &ch.Latency,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	ch.Models = models
	ch.Groups = groups
	return &ch, nil
}

// UpdateChannelDiagnostics persists the latest provider probe diagnostics.
func (s *SQLStore) UpdateChannelDiagnostics(ctx context.Context, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error) {
	checkedAt := input.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	status := firstNonEmpty(input.Status, "offline")
	message := ""
	if input.Health != nil {
		status = firstNonEmpty(input.Health.Status, status)
		message = input.Health.Message
		if !input.Health.CheckedAt.IsZero() {
			checkedAt = input.Health.CheckedAt
		}
	}

	var balanceAmount interface{}
	var balanceCurrency interface{}
	var balanceSource interface{}
	if input.Balance != nil {
		balanceAmount = input.Balance.Amount
		balanceCurrency = firstNonEmpty(input.Balance.Currency, "USD")
		balanceSource = nullableString(input.Balance.Source)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE channels SET
			last_balance_amount = $2,
			last_balance_currency = $3,
			last_balance_source = $4,
			last_balance_error = $5,
			last_health_status = $6,
			last_health_message = $7,
			last_health_checked_at = $8,
			last_latency_ms = $9,
			last_probe_error = $10,
			updated_at = NOW()
		WHERE id = $1
	`, id, balanceAmount, balanceCurrency, balanceSource, nullableString(input.BalanceError),
		status, nullableString(message), checkedAt, input.Latency, nullableString(input.Error))
	if err != nil {
		return nil, fmt.Errorf("update channel diagnostics: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	health := input.Health
	if health == nil {
		health = &ChannelHealthDetail{Status: status, CheckedAt: checkedAt}
	} else {
		copied := *health
		copied.Status = status
		copied.CheckedAt = checkedAt
		health = &copied
	}

	return &ChannelHealth{
		ID:           id,
		Status:       status,
		Latency:      input.Latency,
		Balance:      input.Balance,
		BalanceError: input.BalanceError,
		Health:       health,
		Error:        input.Error,
		CheckedAt:    checkedAt,
	}, nil
}

// DeleteChannel deletes a channel by ID.
func (s *SQLStore) DeleteChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE id = $1`, id)
	return err
}

// TestChannel performs a provider-aware connectivity test using the stored
// channel credentials and returns the upstream model list when available.
func (s *SQLStore) TestChannel(ctx context.Context, id string) (*ChannelTestResult, error) {
	ch, err := s.getRelayChannelForProbe(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("test channel: %w", err)
	}
	if ch == nil {
		return &ChannelTestResult{Success: false, Error: "channel not found"}, nil
	}

	return testRelayChannel(ctx, ch), nil
}

func (s *SQLStore) getRelayChannelForProbe(ctx context.Context, id string) (*types.Channel, error) {
	ch := &types.Channel{}
	var models []string
	var groups []string
	err := s.db.QueryRowContext(ctx, `
			SELECT id, name, provider, base_url, api_key_encrypted, models, groups,
			       rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled
		FROM channels
		WHERE id = $1
	`, id).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey, pq.Array(&models), pq.Array(&groups),
		&ch.RPMLimit, &ch.TPMLimit, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ch.Models = models
	ch.Groups = groups
	return ch, nil
}

// BatchUpdateChannels enables or disables multiple channels at once.
func (s *SQLStore) BatchUpdateChannels(ctx context.Context, ids []string, action string) error {
	var enabled bool
	switch action {
	case "enable":
		enabled = true
	case "disable":
		enabled = false
	default:
		return fmt.Errorf("invalid batch action: %s", action)
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE channels SET enabled = $1, updated_at = NOW()
		WHERE id = ANY($2)
	`, enabled, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("batch update channels: %w", err)
	}
	return nil
}

// --- pointer-to-scalar helpers for UPDATE ---

func coalesceString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func coalesceInt(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func coalesceModels(ptr *[]string) []string {
	if ptr == nil {
		return nil
	}
	return *ptr
}

func coalesceFloat(ptr *float64) interface{} {
	if ptr == nil {
		return nil
	}
	return *ptr
}

func coalesceCostMultiplier(ptr *float64) interface{} {
	if ptr == nil {
		return nil
	}
	return normalizeAdminCostMultiplier(*ptr)
}

func coalesceChannelWeight(ptr *int) interface{} {
	if ptr == nil {
		return nil
	}
	return normalizeAdminChannelWeight(*ptr)
}

func normalizeAdminCostMultiplier(multiplier float64) float64 {
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func normalizeAdminChannelWeight(weight int) int {
	if weight <= 0 {
		return 100
	}
	return weight
}

func nullableString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
