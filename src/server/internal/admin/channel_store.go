package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/secretbox"
)

var ErrChannelNotFound = errors.New("channel not found")

// ChannelStore defines operations on relay channels.
type ChannelStore interface {
	ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error)
	GetChannel(ctx context.Context, organizationID, id string) (*ChannelInfo, error)
	CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error)
	UpdateChannel(ctx context.Context, organizationID, id string, input ChannelUpdateRequest) (*ChannelInfo, error)
	UpdateChannelDiagnostics(ctx context.Context, organizationID, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error)
	DeleteChannel(ctx context.Context, organizationID, id string) error
	TestChannel(ctx context.Context, organizationID, id string) (*ChannelTestResult, error)
	BatchUpdateChannels(ctx context.Context, organizationID string, ids []string, action string) error
}

// ChannelFilter contains filter parameters for listing channels.
type ChannelFilter struct {
	OrganizationID string
	Provider       string
	Status         string
	Search         string
	Limit          int
	Offset         int
}

// ListChannels returns channels with optional filters.
func (s *SQLStore) ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error) {
	if strings.TrimSpace(filter.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	conditions := []string{"organization_id = $1"}
	args := []interface{}{filter.OrganizationID}
	argIdx := 2

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
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(last_health_status, 'offline') = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	where := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
			SELECT id, organization_id, name, provider, base_url, models, groups,
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
		if err := rows.Scan(&ch.ID, &ch.OrganizationID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
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
func (s *SQLStore) GetChannel(ctx context.Context, organizationID, id string) (*ChannelInfo, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	var ch ChannelInfo
	var models []string
	var groups []string

	err := s.db.QueryRowContext(ctx, `
			SELECT id, organization_id, name, provider, base_url, models, groups,
			       rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		       COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		       created_at, updated_at
		FROM channels
		WHERE organization_id = $1 AND id = $2
	`, organizationID, id).Scan(&ch.ID, &ch.OrganizationID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
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
	if strings.TrimSpace(input.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

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
	protectedAPIKey, err := secretbox.Protect(secretbox.DomainRelayChannelAPIKey, input.APIKey)
	if err != nil {
		return nil, fmt.Errorf("protect channel api key: %w", err)
	}

	var ch ChannelInfo
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO channels (id, organization_id, name, provider, base_url, api_key_encrypted, models, groups,
		                      rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, true, NOW(), NOW())
		RETURNING id, organization_id, name, provider, base_url, models, groups,
		          rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		          COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		          created_at, updated_at
	`, id, input.OrganizationID, input.Name, input.Provider, input.BaseURL, protectedAPIKey, pq.Array(models), pq.Array(groups),
		input.RpmLimit, input.TpmLimit, input.Priority, normalizeAdminChannelWeight(input.Weight), input.EstimatedCostPer1K, normalizeAdminCostMultiplier(input.CostMultiplier)).Scan(
		&ch.ID, &ch.OrganizationID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
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
func (s *SQLStore) UpdateChannel(ctx context.Context, organizationID, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	// Build dynamic UPDATE with COALESCE/NULLIF for optional pointer fields
	var ch ChannelInfo
	var models []string
	var groups []string
	apiKey := coalesceString(input.APIKey)
	protectedAPIKey := apiKey
	if apiKey != "" {
		var err error
		protectedAPIKey, err = secretbox.Protect(secretbox.DomainRelayChannelAPIKey, apiKey)
		if err != nil {
			return nil, fmt.Errorf("protect channel api key: %w", err)
		}
	}

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
		WHERE organization_id = $13 AND id = $14
		RETURNING id, organization_id, name, provider, base_url, models, groups,
		          rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled,
		          COALESCE(last_health_status, 'offline'), COALESCE(last_latency_ms, 0),
		          created_at, updated_at
	`,
		coalesceString(input.Name),
		coalesceString(input.BaseURL),
		protectedAPIKey,
		pq.Array(coalesceModels(input.Models)),
		pq.Array(coalesceModels(input.Groups)),
		coalesceInt(input.RpmLimit),
		coalesceInt(input.TpmLimit),
		coalesceInt(input.Priority),
		coalesceChannelWeight(input.Weight),
		coalesceFloat(input.EstimatedCostPer1K),
		coalesceCostMultiplier(input.CostMultiplier),
		input.Enabled,
		organizationID,
		id,
	).Scan(&ch.ID, &ch.OrganizationID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models), pq.Array(&groups),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
		&ch.Status, &ch.Latency,
		&ch.CreatedAt, &ch.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	ch.Models = models
	ch.Groups = groups
	return &ch, nil
}

// UpdateChannelDiagnostics persists the latest provider probe diagnostics.
func (s *SQLStore) UpdateChannelDiagnostics(ctx context.Context, organizationID, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

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
		WHERE id = $1 AND organization_id = $11
	`, id, balanceAmount, balanceCurrency, balanceSource, nullableString(input.BalanceError),
		status, nullableString(message), checkedAt, input.Latency, nullableString(input.Error), organizationID)
	if err != nil {
		return nil, fmt.Errorf("update channel diagnostics: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return nil, ErrChannelNotFound
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
func (s *SQLStore) DeleteChannel(ctx context.Context, organizationID, id string) error {
	if strings.TrimSpace(organizationID) == "" {
		return fmt.Errorf("organization id is required")
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE organization_id = $1 AND id = $2`, organizationID, id)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// TestChannel performs a provider-aware connectivity test using the stored
// channel credentials and returns the upstream model list when available.
func (s *SQLStore) TestChannel(ctx context.Context, organizationID, id string) (*ChannelTestResult, error) {
	ch, err := s.getRelayChannelForProbe(ctx, organizationID, id)
	if err != nil {
		return nil, fmt.Errorf("test channel: %w", err)
	}
	if ch == nil {
		return nil, ErrChannelNotFound
	}

	return testRelayChannel(ctx, ch), nil
}

func (s *SQLStore) getRelayChannelForProbe(ctx context.Context, organizationID, id string) (*types.Channel, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	ch := &types.Channel{}
	var models []string
	var groups []string
	err := s.db.QueryRowContext(ctx, `
			SELECT id, name, provider, base_url, api_key_encrypted, models, groups,
			       rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier, enabled
		FROM channels
		WHERE organization_id = $1 AND id = $2
	`, organizationID, id).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, &ch.APIKey, pq.Array(&models), pq.Array(&groups),
		&ch.RPMLimit, &ch.TPMLimit, &ch.Priority, &ch.Weight, &ch.EstimatedCostPer1K, &ch.CostMultiplier, &ch.Enabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ch.APIKey, err = secretbox.Open(secretbox.DomainRelayChannelAPIKey, ch.APIKey)
	if err != nil {
		return nil, fmt.Errorf("open channel api key: %w", err)
	}
	ch.Models = models
	ch.Groups = groups
	return ch, nil
}

// BatchUpdateChannels enables or disables multiple channels at once.
func (s *SQLStore) BatchUpdateChannels(ctx context.Context, organizationID string, ids []string, action string) error {
	if strings.TrimSpace(organizationID) == "" {
		return fmt.Errorf("organization id is required")
	}
	uniqueIDs := uniqueChannelIDs(ids)
	if len(uniqueIDs) == 0 {
		return fmt.Errorf("channel ids are required")
	}

	var enabled bool
	switch action {
	case "enable":
		enabled = true
	case "disable":
		enabled = false
	default:
		return fmt.Errorf("invalid batch action: %s", action)
	}

	var matched int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM channels
		WHERE organization_id = $1 AND id = ANY($2)
	`, organizationID, pq.Array(uniqueIDs)).Scan(&matched); err != nil {
		return fmt.Errorf("batch check channels: %w", err)
	}
	if matched != len(uniqueIDs) {
		return ErrChannelNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE channels SET enabled = $1, updated_at = NOW()
		WHERE organization_id = $2 AND id = ANY($3)
	`, enabled, organizationID, pq.Array(uniqueIDs))
	if err != nil {
		return fmt.Errorf("batch update channels: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows != int64(len(uniqueIDs)) {
		return ErrChannelNotFound
	}
	return nil
}

func uniqueChannelIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
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
