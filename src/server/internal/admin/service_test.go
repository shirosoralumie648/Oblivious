package admin

import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
)

func TestSystemStatsStruct(t *testing.T) {
	stats := &SystemStats{
		Users: UserStats{
			TotalUsers:    100,
			ActiveUsers:   50,
			NewUsersToday: 5,
			NewUsersWeek:  20,
		},
		Quotas: QuotaStats{
			TotalBalance: 1000.50,
			TotalUsed:    500.25,
			ActiveTopups: 10,
		},
		Conversations: 500,
		Agents:        25,
		Tasks:         100,
		MCPServers:    15,
	}

	if stats.Users.TotalUsers != 100 {
		t.Errorf("expected 100 total users, got %d", stats.Users.TotalUsers)
	}
	if stats.Quotas.TotalBalance != 1000.50 {
		t.Errorf("expected 1000.50 total balance, got %f", stats.Quotas.TotalBalance)
	}
}

func TestUserInfoStruct(t *testing.T) {
	now := time.Now().UTC()
	user := &UserInfo{
		ID:          "user-1",
		Email:       "test@example.com",
		Name:        "Test User",
		CreatedAt:   now,
		LastLoginAt: &now,
		Balance:     100.0,
		Used:        50.0,
		AgentCount:  5,
		TaskCount:   10,
	}

	if user.ID != "user-1" {
		t.Errorf("expected user-1, got %s", user.ID)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", user.Email)
	}
}

func TestListUsersPagination(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},
		{-1, 20},
		{50, 50},
		{150, 100},
	}

	for _, tt := range tests {
		limit := tt.input
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		if limit != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, limit)
		}
	}
}

func TestValidateChannelProviderUsesRelayProviderCatalog(t *testing.T) {
	if err := validateChannelProvider("deepseek"); err != nil {
		t.Fatalf("supported OpenAI-compatible provider should be valid: %v", err)
	}

	err := validateChannelProvider("not-a-provider")
	if err == nil || !strings.Contains(err.Error(), "unsupported channel provider") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}

	for _, provider := range []string{"claude", "gemini", "vertex", "bedrock"} {
		if err := validateChannelProvider(provider); err != nil {
			t.Fatalf("supported native provider %q should be valid: %v", provider, err)
		}
	}
}

func TestListChannelRuntimeStatsFromRelayPool(t *testing.T) {
	until := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	service := NewService(nil, WithChannelRuntimeStatsProvider(fakeChannelRuntimeStatsProvider{
		stats: map[string]*relaytypes.ChannelStats{
			"ch_2": {
				ChannelID:                 "ch_2",
				RPMCurrent:                3,
				TPMCurrent:                120,
				TotalRequests:             10,
				SuccessCount:              8,
				FailureCount:              2,
				LatencySumUs:              250_000,
				LatencyCount:              2,
				RateLimitedUntil:          until,
				AffinityConversationCount: 4,
			},
			"ch_1": {
				ChannelID:     "ch_1",
				TotalRequests: 1,
				SuccessCount:  1,
			},
		},
	}))

	stats, err := service.ListChannelRuntimeStats(context.Background())
	if err != nil {
		t.Fatalf("ListChannelRuntimeStats returned error: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected two channel stats, got %#v", stats)
	}
	if stats[0].ChannelID != "ch_1" || stats[1].ChannelID != "ch_2" {
		t.Fatalf("expected stats sorted by channel id, got %#v", stats)
	}
	runtime := stats[1]
	if runtime.RPMCurrent != 3 || runtime.TPMCurrent != 120 || runtime.SuccessCount != 8 || runtime.FailureCount != 2 {
		t.Fatalf("runtime counters not mapped: %#v", runtime)
	}
	if runtime.AvgLatencyMS != 125 {
		t.Fatalf("avg latency = %f, want 125", runtime.AvgLatencyMS)
	}
	if runtime.RateLimitedUntil == nil || !runtime.RateLimitedUntil.Equal(until) {
		t.Fatalf("rate limited until not mapped: %#v", runtime.RateLimitedUntil)
	}
	if runtime.AffinityConversationCount != 4 {
		t.Fatalf("affinity conversation count = %d, want 4", runtime.AffinityConversationCount)
	}
}

func TestSyncChannelModelsProbesAndPersistsReturnedModels(t *testing.T) {
	store := &syncChannelModelsStore{
		testResult: &ChannelTestResult{
			Success: true,
			Models:  []string{"gpt-4o", "", " gpt-4o-mini ", "gpt-4o"},
		},
	}
	service := NewService(store)
	actor := auth.Session{User: auth.User{ID: "user_admin", Email: "admin@example.com"}}
	request := httptest.NewRequest("POST", "/api/v1/admin/channels/ch_1/sync-models", nil)

	result, err := service.SyncChannelModels(context.Background(), actor, "ch_1", request)
	if err != nil {
		t.Fatalf("sync channel models should succeed: %v", err)
	}

	wantModels := []string{"gpt-4o", "gpt-4o-mini"}
	if !equalStringSlices(store.updatedModels, wantModels) {
		t.Fatalf("updated models = %#v, want %#v", store.updatedModels, wantModels)
	}
	if result.Channel == nil || !equalStringSlices(result.Channel.Models, wantModels) {
		t.Fatalf("response channel models = %#v, want %#v", result.Channel, wantModels)
	}
	if result.TestResult == nil || !equalStringSlices(result.TestResult.Models, wantModels) {
		t.Fatalf("response test result models = %#v, want %#v", result.TestResult, wantModels)
	}
	if store.auditEntry == nil || store.auditEntry.Action != "channel.sync_models" || store.auditEntry.ResourceID != "ch_1" {
		t.Fatalf("expected sync audit entry for ch_1, got %#v", store.auditEntry)
	}
}

type fakeChannelRuntimeStatsProvider struct {
	stats map[string]*relaytypes.ChannelStats
}

func (p fakeChannelRuntimeStatsProvider) GetAllStats() map[string]*relaytypes.ChannelStats {
	return p.stats
}

func TestRefreshChannelBalanceProbesPersistsDiagnosticsAndAudits(t *testing.T) {
	checkedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	store := &syncChannelModelsStore{
		testResult: &ChannelTestResult{
			Success:  true,
			Latency:  87,
			Provider: "openai",
			Balance: &ChannelBalance{
				Amount:   42.75,
				Currency: "USD",
				Source:   "provider_balance",
			},
			Health: &ChannelHealthDetail{
				Status:    "online",
				Message:   "models endpoint ok",
				CheckedAt: checkedAt,
			},
		},
	}
	service := NewService(store)
	actor := auth.Session{User: auth.User{ID: "user_admin", Email: "admin@example.com"}}
	request := httptest.NewRequest("POST", "/api/v1/admin/channels/ch_1/refresh-balance", nil)

	result, err := service.RefreshChannelBalance(context.Background(), actor, "ch_1", request)
	if err != nil {
		t.Fatalf("refresh channel balance should succeed: %v", err)
	}

	if store.diagnostics == nil {
		t.Fatal("expected channel diagnostics to be persisted")
	}
	if store.diagnostics.Status != "online" || store.diagnostics.Latency != 87 {
		t.Fatalf("unexpected diagnostics status/latency: %#v", store.diagnostics)
	}
	if store.diagnostics.Balance == nil || store.diagnostics.Balance.Amount != 42.75 || store.diagnostics.Balance.Currency != "USD" {
		t.Fatalf("unexpected persisted balance: %#v", store.diagnostics.Balance)
	}
	if result.Balance == nil || result.Balance.Amount != 42.75 || result.ChannelHealth == nil || result.ChannelHealth.Status != "online" {
		t.Fatalf("unexpected refresh result: %#v", result)
	}
	if store.auditEntry == nil || store.auditEntry.Action != "channel.refresh_balance" || store.auditEntry.ResourceID != "ch_1" {
		t.Fatalf("expected refresh audit entry for ch_1, got %#v", store.auditEntry)
	}
}

func TestDetectChannelModelUpdatesReturnsDiffWithoutPersisting(t *testing.T) {
	store := &syncChannelModelsStore{
		currentModels: []string{"gpt-4o", "legacy-model"},
		testResult: &ChannelTestResult{
			Success: true,
			Models:  []string{"gpt-4o", "gpt-4.1", "gpt-4.1", ""},
		},
	}
	service := NewService(store)

	result, err := service.DetectChannelModelUpdates(context.Background(), "ch_1")
	if err != nil {
		t.Fatalf("detect channel model updates should succeed: %v", err)
	}

	if !equalStringSlices(result.Added, []string{"gpt-4.1"}) {
		t.Fatalf("added = %#v, want gpt-4.1", result.Added)
	}
	if !equalStringSlices(result.Removed, []string{"legacy-model"}) {
		t.Fatalf("removed = %#v, want legacy-model", result.Removed)
	}
	if !equalStringSlices(result.Unchanged, []string{"gpt-4o"}) {
		t.Fatalf("unchanged = %#v, want gpt-4o", result.Unchanged)
	}
	if store.updatedModels != nil {
		t.Fatalf("detect must not persist channel models, got %#v", store.updatedModels)
	}
}

func TestApplyChannelModelUpdatesMergesAndAudits(t *testing.T) {
	store := &syncChannelModelsStore{
		currentModels: []string{"gpt-4o", "legacy-model"},
		testResult: &ChannelTestResult{
			Success: true,
			Models:  []string{"gpt-4o", "gpt-4.1"},
		},
	}
	service := NewService(store)
	actor := auth.Session{User: auth.User{ID: "user_admin", Email: "admin@example.com"}}
	request := httptest.NewRequest("POST", "/api/v1/admin/channels/ch_1/model-updates/apply", nil)

	result, err := service.ApplyChannelModelUpdates(context.Background(), actor, "ch_1", ChannelModelUpdateApplyRequest{Mode: "merge"}, request)
	if err != nil {
		t.Fatalf("apply channel model updates should succeed: %v", err)
	}

	wantModels := []string{"gpt-4o", "legacy-model", "gpt-4.1"}
	if !equalStringSlices(store.updatedModels, wantModels) {
		t.Fatalf("updated models = %#v, want %#v", store.updatedModels, wantModels)
	}
	if result.Channel == nil || !equalStringSlices(result.Channel.Models, wantModels) {
		t.Fatalf("response channel models = %#v, want %#v", result.Channel, wantModels)
	}
	if result.Preview == nil || !equalStringSlices(result.Preview.Added, []string{"gpt-4.1"}) {
		t.Fatalf("expected preview with added model, got %#v", result.Preview)
	}
	if store.auditEntry == nil || store.auditEntry.Action != "channel.apply_model_updates" || store.auditEntry.ResourceID != "ch_1" {
		t.Fatalf("expected apply audit entry for ch_1, got %#v", store.auditEntry)
	}
}

func TestRelayConfigApplierRunsAfterChannelAndRouteMutations(t *testing.T) {
	store := &relayConfigApplyStore{}
	var applied []RelayConfigChange
	service := NewService(store, WithRelayConfigApplier(func(ctx context.Context, change RelayConfigChange) error {
		applied = append(applied, change)
		return nil
	}))
	actor := auth.Session{User: auth.User{ID: "user_admin", Email: "admin@example.com"}}
	request := httptest.NewRequest("POST", "/api/v1/admin", nil)

	if _, err := service.CreateChannel(context.Background(), actor, ChannelCreateRequest{Name: "OpenAI", Provider: "openai"}, request); err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if _, err := service.UpdateChannel(context.Background(), actor, "ch_1", ChannelUpdateRequest{}, request); err != nil {
		t.Fatalf("UpdateChannel failed: %v", err)
	}
	if err := service.DeleteChannel(context.Background(), actor, "ch_1", request); err != nil {
		t.Fatalf("DeleteChannel failed: %v", err)
	}
	if _, err := service.CreateRoute(context.Background(), actor, RouteCreateRequest{
		Model: "gpt-4o",
		Channels: []RouteChannelInput{{
			ChannelID: "ch_1",
			Weight:    1,
			Enabled:   true,
		}},
	}, request); err != nil {
		t.Fatalf("CreateRoute failed: %v", err)
	}
	if _, err := service.UpdateRoute(context.Background(), actor, "route_1", RouteUpdateRequest{}, request); err != nil {
		t.Fatalf("UpdateRoute failed: %v", err)
	}
	if err := service.DeleteRoute(context.Background(), actor, "route_1", request); err != nil {
		t.Fatalf("DeleteRoute failed: %v", err)
	}

	want := []RelayConfigChange{
		{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: "ch_1"},
		{Kind: RelayConfigChangeChannel, Action: RelayConfigActionUpsert, ID: "ch_1"},
		{Kind: RelayConfigChangeChannel, Action: RelayConfigActionDelete, ID: "ch_1"},
		{Kind: RelayConfigChangeRoute, Action: RelayConfigActionUpsert, ID: "route_1"},
		{Kind: RelayConfigChangeRoute, Action: RelayConfigActionUpsert, ID: "route_1"},
		{Kind: RelayConfigChangeRoute, Action: RelayConfigActionDelete, ID: "route_1"},
	}
	if !reflect.DeepEqual(applied, want) {
		t.Fatalf("applied changes = %#v, want %#v", applied, want)
	}
}

func TestServicePassesChannelWeightThroughCreateAndUpdate(t *testing.T) {
	store := &relayConfigApplyStore{}
	service := NewService(store)
	actor := auth.Session{User: auth.User{ID: "user_admin", Email: "admin@example.com"}}
	request := httptest.NewRequest("POST", "/api/v1/admin", nil)

	created, err := service.CreateChannel(context.Background(), actor, ChannelCreateRequest{Name: "Weighted OpenAI", Provider: "openai", Weight: 25}, request)
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if store.createdWeight != 25 || created.Weight != 25 {
		t.Fatalf("expected create weight 25, store=%d result=%d", store.createdWeight, created.Weight)
	}

	updatedWeight := 40
	updated, err := service.UpdateChannel(context.Background(), actor, "ch_1", ChannelUpdateRequest{Weight: &updatedWeight}, request)
	if err != nil {
		t.Fatalf("UpdateChannel failed: %v", err)
	}
	if store.updatedWeight == nil || *store.updatedWeight != 40 || updated.Weight != 40 {
		t.Fatalf("expected update weight 40, store=%v result=%d", store.updatedWeight, updated.Weight)
	}
}

type syncChannelModelsStore struct {
	Store
	testResult    *ChannelTestResult
	currentModels []string
	updatedModels []string
	diagnostics   *ChannelDiagnosticsUpdate
	auditEntry    *AuditEntry
}

func (s *syncChannelModelsStore) GetChannel(ctx context.Context, id string) (*ChannelInfo, error) {
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.currentModels...)}, nil
}

func (s *syncChannelModelsStore) TestChannel(ctx context.Context, id string) (*ChannelTestResult, error) {
	return s.testResult, nil
}

func (s *syncChannelModelsStore) UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	if input.Models != nil {
		s.updatedModels = append([]string{}, (*input.Models)...)
	}
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.updatedModels...)}, nil
}

func (s *syncChannelModelsStore) UpdateChannelDiagnostics(ctx context.Context, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error) {
	s.diagnostics = &input
	return &ChannelHealth{
		ID:           id,
		Status:       input.Status,
		Latency:      input.Latency,
		Balance:      input.Balance,
		BalanceError: input.BalanceError,
		Health:       input.Health,
		Error:        input.Error,
		CheckedAt:    input.CheckedAt,
	}, nil
}

func (s *syncChannelModelsStore) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	s.auditEntry = entry
	return nil
}

type relayConfigApplyStore struct {
	Store
	createdWeight int
	updatedWeight *int
}

func (s *relayConfigApplyStore) CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error) {
	s.createdWeight = input.Weight
	return &ChannelInfo{ID: "ch_1", Name: input.Name, Provider: input.Provider, Weight: input.Weight}, nil
}

func (s *relayConfigApplyStore) UpdateChannel(ctx context.Context, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	s.updatedWeight = input.Weight
	weight := 0
	if input.Weight != nil {
		weight = *input.Weight
	}
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Weight: weight}, nil
}

func (s *relayConfigApplyStore) DeleteChannel(ctx context.Context, id string) error {
	return nil
}

func (s *relayConfigApplyStore) CreateRoute(ctx context.Context, input RouteCreateRequest) (*RouteInfo, error) {
	return &RouteInfo{ID: "route_1", Model: input.Model, Channels: []RouteChannel{{ChannelID: "ch_1"}}}, nil
}

func (s *relayConfigApplyStore) UpdateRoute(ctx context.Context, id string, input RouteUpdateRequest) (*RouteInfo, error) {
	return &RouteInfo{ID: id, Model: "gpt-4o", Channels: []RouteChannel{{ChannelID: "ch_1"}}}, nil
}

func (s *relayConfigApplyStore) DeleteRoute(ctx context.Context, id string) error {
	return nil
}

func (s *relayConfigApplyStore) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	return nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
