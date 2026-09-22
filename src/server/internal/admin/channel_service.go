package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	relaychannel "oblivious/server/internal/relay/channel"
)

// ListChannels returns channels for admin display, applying limit bounds.
func (s *Service) ListChannels(ctx context.Context, actor auth.Session, filter ChannelFilter) ([]*ChannelInfo, error) {
	return s.listChannelsForOrganization(ctx, actor.OrganizationID, filter)
}

func (s *Service) listChannelsForOrganization(ctx context.Context, organizationID string, filter ChannelFilter) ([]*ChannelInfo, error) {
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	filter.OrganizationID = organizationID
	return s.store.ListChannels(ctx, filter)
}

// GetChannel returns a single channel by ID.
func (s *Service) GetChannel(ctx context.Context, organizationID, id string) (*ChannelInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	return s.store.GetChannel(ctx, organizationID, id)
}

// CreateChannel creates a new channel and records an audit entry.
func (s *Service) CreateChannel(ctx context.Context, actor auth.Session, input ChannelCreateRequest, r *http.Request) (*ChannelInfo, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("channel name is required")
	}
	if input.Provider == "" {
		return nil, fmt.Errorf("channel provider is required")
	}
	if err := validateChannelProvider(input.Provider); err != nil {
		return nil, err
	}
	if err := validateChannelBaseURL(input.BaseURL); err != nil {
		return nil, err
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	input.OrganizationID = actor.OrganizationID

	result, err := s.store.CreateChannel(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: result.ID}); err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(redactedChannelCreateAuditChanges(input))
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.create", "channel", result.ID, string(changes), ip)

	return result, nil
}

func validateChannelProvider(provider string) error {
	spec, ok := relaychannel.LookupProvider(provider)
	if !ok {
		return fmt.Errorf("unsupported channel provider: %s", provider)
	}
	if spec.Status != relaychannel.ProviderStatusSupported {
		return fmt.Errorf("channel provider %s is not configurable", spec.ID)
	}
	return nil
}

func validateChannelBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid channel base URL: %w", err)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("channel base URL must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("channel base URL must not include credentials")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("channel base URL must include a host")
	}
	if ip := net.ParseIP(host); ip != nil && isUnsafeChannelBaseIP(ip) {
		return fmt.Errorf("channel base URL must not target local or private network addresses")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("channel base URL must use https")
	}
	return nil
}

func isUnsafeChannelBaseIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// ListChannelProviders returns the canonical provider catalog from the Relay
// gateway so admin clients do not hardcode a different provider surface.
func (s *Service) ListChannelProviders(ctx context.Context) ([]ChannelProviderInfo, error) {
	_ = ctx
	specs := relaychannel.SupportedProviders()
	providers := make([]ChannelProviderInfo, 0, len(specs))
	for _, spec := range specs {
		runtimeReady := spec.Status == relaychannel.ProviderStatusSupported
		providers = append(providers, ChannelProviderInfo{
			ID:             spec.ID,
			DisplayName:    spec.DisplayName,
			Kind:           string(spec.Kind),
			Status:         string(spec.Status),
			DefaultBaseURL: spec.DefaultBaseURL,
			Configurable:   runtimeReady,
			Installable:    runtimeReady,
			RuntimeReady:   runtimeReady,
		})
	}
	return providers, nil
}

// UpdateChannel updates a channel and records an audit entry.
func (s *Service) UpdateChannel(ctx context.Context, actor auth.Session, id string, input ChannelUpdateRequest, r *http.Request) (*ChannelInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	if input.BaseURL != nil {
		if err := validateChannelBaseURL(*input.BaseURL); err != nil {
			return nil, err
		}
	}

	result, err := s.store.UpdateChannel(ctx, actor.OrganizationID, id, input)
	if err != nil {
		return nil, err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: result.ID}); err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(redactedChannelUpdateAuditChanges(input))
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.update", "channel", id, string(changes), ip)

	return result, nil
}

// DeleteChannel deletes a channel and records an audit entry.
func (s *Service) DeleteChannel(ctx context.Context, actor auth.Session, id string, r *http.Request) error {
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return fmt.Errorf("organization id is required")
	}
	if err := s.store.DeleteChannel(ctx, actor.OrganizationID, id); err != nil {
		return err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeChannel, Action: RelayConfigActionDelete, ID: id}); err != nil {
		return err
	}

	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.delete", "channel", id, "", ip)
	return nil
}

func redactedChannelCreateAuditChanges(input ChannelCreateRequest) ChannelCreateRequest {
	input.APIKey = redactChannelAuditSecret(input.APIKey)
	return input
}

func redactedChannelUpdateAuditChanges(input ChannelUpdateRequest) ChannelUpdateRequest {
	if input.APIKey != nil {
		redacted := redactChannelAuditSecret(*input.APIKey)
		input.APIKey = &redacted
	}
	return input
}

func redactChannelAuditSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return "********"
}

// TestChannel performs a connectivity test on the channel.
func (s *Service) TestChannel(ctx context.Context, organizationID, id string) (*ChannelTestResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	return s.store.TestChannel(ctx, organizationID, id)
}

// SyncChannelModels probes a channel and persists the upstream model list.
func (s *Service) SyncChannelModels(ctx context.Context, actor auth.Session, id string, r *http.Request) (*ChannelModelSyncResult, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	if err := s.modelReadiness.requireMutation(ctx); err != nil {
		return nil, err
	}

	result, err := s.store.TestChannel(ctx, actor.OrganizationID, id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("channel probe returned no result")
	}
	if !result.Success {
		if result.Error != "" {
			return nil, fmt.Errorf("channel probe failed: %s", result.Error)
		}
		return nil, fmt.Errorf("channel probe failed")
	}
	if err := s.modelReadiness.requireModels(ctx, result.Models); err != nil {
		return nil, err
	}

	models := normalizeProbeModels(result.Models)
	if len(models) == 0 {
		return nil, fmt.Errorf("channel probe returned no models")
	}

	channel, err := s.store.UpdateChannel(ctx, actor.OrganizationID, id, ChannelUpdateRequest{Models: &models})
	if err != nil {
		return nil, err
	}

	normalizedResult := *result
	normalizedResult.Models = append([]string{}, models...)
	changes, _ := json.Marshal(map[string]any{"models": models})
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.sync_models", "channel", id, string(changes), ip)

	return &ChannelModelSyncResult{
		Channel:    channel,
		TestResult: &normalizedResult,
	}, nil
}

// DetectChannelModelUpdates probes upstream and returns the model delta without
// changing the persisted channel configuration.
func (s *Service) DetectChannelModelUpdates(ctx context.Context, organizationID, id string) (*ChannelModelUpdatePreview, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	if err := s.modelReadiness.requireMutation(ctx); err != nil {
		return nil, err
	}
	return s.detectChannelModelUpdatesChecked(ctx, organizationID, id)
}

func (s *Service) detectChannelModelUpdatesChecked(ctx context.Context, organizationID, id string) (*ChannelModelUpdatePreview, error) {
	channel, err := s.store.GetChannel(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, ErrChannelNotFound
	}

	result, err := s.store.TestChannel(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("channel probe returned no result")
	}
	if !result.Success {
		if result.Error != "" {
			return nil, fmt.Errorf("channel probe failed: %s", result.Error)
		}
		return nil, fmt.Errorf("channel probe failed")
	}

	if err := s.modelReadiness.requireModels(ctx, channel.Models); err != nil {
		return nil, err
	}
	if err := s.modelReadiness.requireModels(ctx, result.Models); err != nil {
		return nil, err
	}
	currentModels := normalizeProbeModels(channel.Models)
	upstreamModels := normalizeProbeModels(result.Models)
	if len(upstreamModels) == 0 {
		return nil, fmt.Errorf("channel probe returned no models")
	}

	added, removed, unchanged := diffModelSets(currentModels, upstreamModels)
	normalizedResult := *result
	normalizedResult.Models = append([]string{}, upstreamModels...)

	return &ChannelModelUpdatePreview{
		ID:             id,
		CurrentModels:  currentModels,
		UpstreamModels: upstreamModels,
		Added:          added,
		Removed:        removed,
		Unchanged:      unchanged,
		TestResult:     &normalizedResult,
	}, nil
}

// ApplyChannelModelUpdates persists a detected upstream model delta. "merge"
// keeps configured models and appends newly discovered upstream models, while
// "replace" makes the channel model list match upstream exactly.
func (s *Service) ApplyChannelModelUpdates(ctx context.Context, actor auth.Session, id string, input ChannelModelUpdateApplyRequest, r *http.Request) (*ChannelModelUpdateApplyResult, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	if err := s.modelReadiness.requireMutation(ctx); err != nil {
		return nil, err
	}
	preview, err := s.detectChannelModelUpdatesChecked(ctx, actor.OrganizationID, id)
	if err != nil {
		return nil, err
	}

	mode := strings.TrimSpace(strings.ToLower(input.Mode))
	if mode == "" {
		mode = "merge"
	}

	var appliedModels []string
	switch mode {
	case "merge":
		appliedModels = normalizeProbeModels(append(append([]string{}, preview.CurrentModels...), preview.Added...))
	case "replace":
		appliedModels = append([]string{}, preview.UpstreamModels...)
	default:
		return nil, fmt.Errorf("model update mode must be 'merge' or 'replace'")
	}
	if err := s.modelReadiness.requireModels(ctx, appliedModels); err != nil {
		return nil, err
	}

	channel, err := s.store.UpdateChannel(ctx, actor.OrganizationID, id, ChannelUpdateRequest{Models: &appliedModels})
	if err != nil {
		return nil, err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: channel.ID}); err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(map[string]any{
		"mode":          mode,
		"added":         preview.Added,
		"removed":       preview.Removed,
		"unchanged":     preview.Unchanged,
		"appliedModels": appliedModels,
	})
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.apply_model_updates", "channel", id, string(changes), ip)

	return &ChannelModelUpdateApplyResult{
		Channel:       channel,
		Preview:       preview,
		Mode:          mode,
		AppliedModels: appliedModels,
	}, nil
}

// RefreshChannelBalance probes a channel, persists the latest balance/health
// diagnostics, and records an audit entry for the admin action.
func (s *Service) RefreshChannelBalance(ctx context.Context, actor auth.Session, id string, r *http.Request) (*ChannelBalanceRefreshResult, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	result, err := s.store.TestChannel(ctx, actor.OrganizationID, id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("channel probe returned no result")
	}

	status := "offline"
	if result.Success {
		status = "online"
	}
	if result.Health != nil && strings.TrimSpace(result.Health.Status) != "" {
		status = result.Health.Status
	}
	checkedAt := time.Now().UTC()
	if result.Health != nil && !result.Health.CheckedAt.IsZero() {
		checkedAt = result.Health.CheckedAt
	}

	health, persistErr := s.store.UpdateChannelDiagnostics(ctx, actor.OrganizationID, id, ChannelDiagnosticsUpdate{
		Status:       status,
		Latency:      result.Latency,
		Balance:      result.Balance,
		BalanceError: result.BalanceError,
		Health:       result.Health,
		Error:        result.Error,
		CheckedAt:    checkedAt,
	})
	if persistErr != nil {
		return nil, persistErr
	}

	if !result.Success {
		if result.Error != "" {
			return nil, fmt.Errorf("channel probe failed: %s", result.Error)
		}
		return nil, fmt.Errorf("channel probe failed")
	}
	if result.Balance == nil {
		if result.BalanceError != "" {
			return nil, fmt.Errorf("channel balance refresh failed: %s", result.BalanceError)
		}
		return nil, fmt.Errorf("channel probe returned no balance")
	}

	changes, _ := json.Marshal(map[string]any{
		"balance":   result.Balance,
		"latency":   result.Latency,
		"status":    status,
		"checkedAt": checkedAt,
	})
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.refresh_balance", "channel", id, string(changes), ip)

	return &ChannelBalanceRefreshResult{
		ID:            id,
		Status:        health.Status,
		Balance:       health.Balance,
		BalanceError:  health.BalanceError,
		ChannelHealth: health.Health,
		TestResult:    result,
		CheckedAt:     health.CheckedAt,
	}, nil
}

// GetChannelHealth returns a status-oriented health payload for one channel.
func (s *Service) GetChannelHealth(ctx context.Context, organizationID, id string) (*ChannelHealth, error) {
	result, err := s.TestChannel(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	status := "offline"
	if result.Success {
		status = "online"
	}

	return &ChannelHealth{
		ID:           id,
		Status:       status,
		Latency:      result.Latency,
		Models:       result.Models,
		Balance:      result.Balance,
		BalanceError: result.BalanceError,
		Health:       result.Health,
		Error:        result.Error,
		CheckedAt:    time.Now().UTC(),
	}, nil
}

// BatchUpdateChannels enables or disables multiple channels and records an audit entry.
func (s *Service) BatchUpdateChannels(ctx context.Context, actor auth.Session, ids []string, action string, r *http.Request) error {
	if action != "enable" && action != "disable" {
		return fmt.Errorf("action must be 'enable' or 'disable'")
	}
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return fmt.Errorf("organization id is required")
	}
	uniqueIDs := uniqueChannelIDs(ids)
	if len(uniqueIDs) == 0 {
		return fmt.Errorf("channel ids are required")
	}

	if err := s.store.BatchUpdateChannels(ctx, actor.OrganizationID, uniqueIDs, action); err != nil {
		return err
	}
	for _, id := range uniqueIDs {
		if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: id}); err != nil {
			return err
		}
	}

	changes, _ := json.Marshal(map[string]interface{}{"ids": uniqueIDs, "action": action})
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.batch_"+action, "channel", "", string(changes), ip)
	return nil
}

func normalizeProbeModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized
}

func diffModelSets(currentModels, upstreamModels []string) ([]string, []string, []string) {
	current := make(map[string]struct{}, len(currentModels))
	upstream := make(map[string]struct{}, len(upstreamModels))
	for _, model := range currentModels {
		current[model] = struct{}{}
	}
	for _, model := range upstreamModels {
		upstream[model] = struct{}{}
	}

	added := make([]string, 0)
	unchanged := make([]string, 0)
	for _, model := range upstreamModels {
		if _, ok := current[model]; ok {
			unchanged = append(unchanged, model)
		} else {
			added = append(added, model)
		}
	}

	removed := make([]string, 0)
	for _, model := range currentModels {
		if _, ok := upstream[model]; !ok {
			removed = append(removed, model)
		}
	}

	return added, removed, unchanged
}

// extractIP extracts the client IP address from an HTTP request.
func extractIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
