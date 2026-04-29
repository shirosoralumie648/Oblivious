package admin

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
)

// ChannelStore defines operations on relay channels.
type ChannelStore interface {
	ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error)
	GetChannel(ctx context.Context, id string) (*ChannelInfo, error)
	CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error)
	UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error)
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
		SELECT id, name, provider, base_url, models,
		       rpm_limit, tpm_limit, priority, enabled,
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
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models),
			&ch.RPM, &ch.TPM, &ch.Priority, &ch.Enabled,
			&ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		ch.Models = models
		ch.Status = "offline" // default; health checker populates this
		channels = append(channels, &ch)
	}
	return channels, rows.Err()
}

// GetChannel returns a single channel by ID.
func (s *SQLStore) GetChannel(ctx context.Context, id string) (*ChannelInfo, error) {
	var ch ChannelInfo
	var models []string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, provider, base_url, models,
		       rpm_limit, tpm_limit, priority, enabled,
		       created_at, updated_at
		FROM channels
		WHERE id = $1
	`, id).Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Enabled,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	ch.Models = models
	ch.Status = "offline"
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

	var ch ChannelInfo
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO channels (id, name, provider, base_url, api_key_encrypted, models,
		                      rpm_limit, tpm_limit, priority, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, NOW(), NOW())
		RETURNING id, name, provider, base_url, models,
		          rpm_limit, tpm_limit, priority, enabled,
		          created_at, updated_at
	`, id, input.Name, input.Provider, input.BaseURL, input.APIKey, pq.Array(models),
		input.RpmLimit, input.TpmLimit, input.Priority).Scan(
		&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Enabled,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	ch.Models = models
	ch.Status = "offline"
	return &ch, nil
}

// UpdateChannel updates an existing channel with optional fields.
func (s *SQLStore) UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	// Build dynamic UPDATE with COALESCE/NULLIF for optional pointer fields
	var ch ChannelInfo
	var models []string

	err := s.db.QueryRowContext(ctx, `
		UPDATE channels SET
			name = COALESCE(NULLIF($1::text,''), name),
			base_url = COALESCE(NULLIF($2::text,''), base_url),
			api_key_encrypted = COALESCE(NULLIF($3::text,''), api_key_encrypted),
			models = COALESCE(NULLIF($4::text[], '{}'), models),
			rpm_limit = COALESCE(NULLIF($5::int, 0), rpm_limit),
			tpm_limit = COALESCE(NULLIF($6::int, 0), tpm_limit),
			priority = COALESCE(NULLIF($7::int, 0), priority),
			enabled = COALESCE($8::boolean, enabled),
			updated_at = NOW()
		WHERE id = $9
		RETURNING id, name, provider, base_url, models,
		          rpm_limit, tpm_limit, priority, enabled,
		          created_at, updated_at
	`,
		coalesceString(input.Name),
		coalesceString(input.BaseURL),
		coalesceString(input.APIKey),
		pq.Array(coalesceModels(input.Models)),
		coalesceInt(input.RpmLimit),
		coalesceInt(input.TpmLimit),
		coalesceInt(input.Priority),
		input.Enabled,
		id,
	).Scan(&ch.ID, &ch.Name, &ch.Provider, &ch.BaseURL, pq.Array(&models),
		&ch.RPM, &ch.TPM, &ch.Priority, &ch.Enabled,
		&ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	ch.Models = models
	ch.Status = "offline"
	return &ch, nil
}

// DeleteChannel deletes a channel by ID.
func (s *SQLStore) DeleteChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE id = $1`, id)
	return err
}

// TestChannel performs a lightweight connectivity test to the channel's base URL.
func (s *SQLStore) TestChannel(ctx context.Context, id string) (*ChannelTestResult, error) {
	ch, err := s.GetChannel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("test channel: %w", err)
	}
	if ch == nil {
		return &ChannelTestResult{Success: false, Error: "channel not found"}, nil
	}

	start := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(ch.BaseURL, "/")+"/models", nil)
	if err != nil {
		return &ChannelTestResult{Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Bearer <redacted>")
	// Note: API key stored encrypted — actual key retrieval requires decryption at the relay layer.
	// For admin test, we use a simple connectivity check without actual auth.

	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &ChannelTestResult{Success: false, Latency: latency, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	// Read a limited portion to confirm response body
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return &ChannelTestResult{Success: true, Latency: latency}, nil
	}
	return &ChannelTestResult{
		Success: false,
		Latency: latency,
		Error:   fmt.Sprintf("unexpected status: %d", resp.StatusCode),
	}, nil
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
