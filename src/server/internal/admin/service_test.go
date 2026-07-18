package admin

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

func TestModelCatalogMutationContract(t *testing.T) {
	contract, profile := loadAdminModelReadinessAuthority(t)
	newService := func(t *testing.T, store Store, guard releasecontract.Guard, registrar releasecontract.EffectRegistrar) *Service {
		t.Helper()
		authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
		if err != nil {
			t.Fatalf("compile runtime authorities: %v", err)
		}
		service, err := NewReadinessService(store, ModelCatalogRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar})
		if err != nil {
			t.Fatalf("construct readiness Admin service: %v", err)
		}
		return service
	}
	actor := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "admin_1", Email: "admin@example.com"}}

	t.Run("constructor fails closed without startup authority", func(t *testing.T) {
		_, err := NewReadinessService(&syncChannelModelsStore{}, ModelCatalogRuntimeOptions{})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("expected readiness unavailable, got %v", err)
		}
	})

	t.Run("unknown upstream model is rejected atomically", func(t *testing.T) {
		store := &syncChannelModelsStore{testResult: &ChannelTestResult{Success: true, Models: []string{"caller-capability"}}}
		service := newService(t, store, &adminModelGuardSpy{}, &adminModelRegistrar{})
		_, err := service.SyncChannelModels(context.Background(), actor, "channel_1", httptest.NewRequest("POST", "/sync", nil))
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) || store.updateCalls != 0 {
			t.Fatalf("unknown model reached persistence: err=%v calls=%d", err, store.updateCalls)
		}
	})

	t.Run("ambiguous upstream models are rejected before normalization", func(t *testing.T) {
		store := &syncChannelModelsStore{testResult: &ChannelTestResult{Success: true, Models: []string{"gpt-4o-mini", " gpt-4o-mini "}}}
		service := newService(t, store, &adminModelGuardSpy{}, &adminModelRegistrar{})
		_, err := service.SyncChannelModels(context.Background(), actor, "channel_1", httptest.NewRequest("POST", "/sync", nil))
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) || store.updateCalls != 0 {
			t.Fatalf("ambiguous models reached persistence: err=%v calls=%d", err, store.updateCalls)
		}
	})

	t.Run("current catalog models are guarded and then persisted", func(t *testing.T) {
		store := &syncChannelModelsStore{testResult: &ChannelTestResult{Success: true, Models: []string{" gpt-4o-mini "}}}
		guard := &adminModelGuardSpy{}
		registrar := &adminModelRegistrar{}
		service := newService(t, store, guard, registrar)
		result, err := service.SyncChannelModels(context.Background(), actor, "channel_1", httptest.NewRequest("POST", "/sync", nil))
		if err != nil || store.updateCalls != 1 || len(store.updatedModels) != 1 || store.updatedModels[0] != "gpt-4o-mini" {
			t.Fatalf("valid model sync failed: err=%v calls=%d models=%#v", err, store.updateCalls, store.updatedModels)
		}
		if result == nil || result.TestResult == nil || len(result.TestResult.Models) != 1 || len(guard.calls) != 2 {
			t.Fatalf("valid model sync did not derive safe response/guards: result=%#v guard=%#v", result, guard.calls)
		}
		if len(registrar.descriptors) != 1 || registrar.descriptors[0].CapabilityID != "gateway.request_admission" {
			t.Fatalf("unexpected mutation descriptor: %#v", registrar.descriptors)
		}
	})

	t.Run("unknown persisted model blocks apply before a write", func(t *testing.T) {
		store := &syncChannelModelsStore{
			currentModels: []string{"stale-persisted-capability"},
			testResult:    &ChannelTestResult{Success: true, Models: []string{"gpt-4o-mini"}},
		}
		service := newService(t, store, &adminModelGuardSpy{}, &adminModelRegistrar{})
		_, err := service.ApplyChannelModelUpdates(context.Background(), actor, "channel_1", ChannelModelUpdateApplyRequest{Mode: "replace"}, httptest.NewRequest("POST", "/apply", nil))
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) || store.updateCalls != 0 {
			t.Fatalf("stale persisted model authorized apply: err=%v calls=%d", err, store.updateCalls)
		}
	})
}

type adminModelGuardCall struct {
	capabilityID string
	boundary     releasecontract.Boundary
}

type adminModelGuardSpy struct {
	calls []adminModelGuardCall
}

func (g *adminModelGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.calls = append(g.calls, adminModelGuardCall{capabilityID: capabilityID, boundary: boundary})
	return nil
}

type adminModelRegistrar struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *adminModelRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

func loadAdminModelReadinessAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Admin readiness test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}

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

func TestUpdateUserQuotaValidatesTargetAndAudits(t *testing.T) {
	store := &userQuotaAdminStore{
		user: &UserDetail{ID: "user_1", Email: "user@example.com", Role: "user", Status: "active"},
	}
	service := NewService(store)

	err := service.UpdateUserQuota(context.Background(), "admin_1", "admin@example.com", " user_1 ", 42.5, "203.0.113.9")
	if err != nil {
		t.Fatalf("UpdateUserQuota returned error: %v", err)
	}

	if store.quotaUserID != "user_1" || store.quotaBalance != 42.5 {
		t.Fatalf("expected user quota update to be forwarded, got user=%q balance=%f", store.quotaUserID, store.quotaBalance)
	}
	if len(store.auditEntries) != 1 {
		t.Fatalf("expected one audit entry, got %#v", store.auditEntries)
	}
	entry := store.auditEntries[0]
	if entry.ActorID != "admin_1" || entry.ActorEmail != "admin@example.com" ||
		entry.Action != "user.quota.update" || entry.ResourceType != "user" || entry.ResourceID != "user_1" ||
		entry.IPAddress != "203.0.113.9" || !strings.Contains(entry.Changes, `"balance":42.5`) {
		t.Fatalf("unexpected quota audit entry: %#v", entry)
	}
}

func TestUpdateUserQuotaRejectsInvalidOrMissingTarget(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		balance   float64
		userError error
		want      string
	}{
		{
			name:    "blank user",
			userID:  " ",
			balance: 10,
			want:    "user id is required",
		},
		{
			name:    "negative balance",
			userID:  "user_1",
			balance: -1,
			want:    "balance must be a non-negative finite number",
		},
		{
			name:      "missing user",
			userID:    "user_missing",
			balance:   10,
			userError: errors.New("user not found: user_missing"),
			want:      "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &userQuotaAdminStore{
				user:        &UserDetail{ID: "user_1", Email: "user@example.com", Role: "user", Status: "active"},
				userErr:     tt.userError,
				quotaUserID: "",
			}
			service := NewService(store)

			err := service.UpdateUserQuota(context.Background(), "admin_1", "admin@example.com", tt.userID, tt.balance, "203.0.113.9")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
			if store.quotaUserID != "" {
				t.Fatalf("invalid quota update should not reach store, got user=%q balance=%f", store.quotaUserID, store.quotaBalance)
			}
			if len(store.auditEntries) != 0 {
				t.Fatalf("invalid quota update should not audit, got %#v", store.auditEntries)
			}
		})
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

	for _, provider := range []string{"azure-openai", "perplexity", "cohere"} {
		err := validateChannelProvider(provider)
		if err == nil || !strings.Contains(err.Error(), "not configurable") {
			t.Fatalf("planned provider %q should not be configurable, got %v", provider, err)
		}
	}
}

func TestListChannelProvidersMarksPlannedProvidersNotConfigurable(t *testing.T) {
	service := NewService(nil)

	providers, err := service.ListChannelProviders(context.Background())
	if err != nil {
		t.Fatalf("ListChannelProviders returned error: %v", err)
	}

	byID := map[string]ChannelProviderInfo{}
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	for _, provider := range []string{"openai", "deepseek", "claude", "gemini"} {
		if !byID[provider].Configurable || !byID[provider].Installable || !byID[provider].RuntimeReady {
			t.Fatalf("supported provider %q should be configurable, installable, and runtime-ready: %+v", provider, byID[provider])
		}
	}
	for _, provider := range []string{"azure-openai", "perplexity", "cohere"} {
		if byID[provider].Configurable || byID[provider].Installable || byID[provider].RuntimeReady {
			t.Fatalf("planned provider %q must not be configurable, installable, or runtime-ready: %+v", provider, byID[provider])
		}
	}
}

func TestListChannelRuntimeStatsFromRelayPool(t *testing.T) {
	until := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	store := &runtimeStatsChannelStore{
		channels: []*ChannelInfo{
			{ID: "ch_2", OrganizationID: "org_1"},
			{ID: "ch_1", OrganizationID: "org_1"},
		},
	}
	service := NewService(store, WithChannelRuntimeStatsProvider(fakeChannelRuntimeStatsProvider{
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
			"ch_other": {
				ChannelID:     "ch_other",
				TotalRequests: 99,
				FailureCount:  99,
			},
		},
	}))

	stats, err := service.ListChannelRuntimeStats(context.Background(), " org_1 ")
	if err != nil {
		t.Fatalf("ListChannelRuntimeStats returned error: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected two channel stats, got %#v", stats)
	}
	if stats[0].ChannelID != "ch_1" || stats[1].ChannelID != "ch_2" {
		t.Fatalf("expected stats sorted by channel id, got %#v", stats)
	}
	if store.filter.OrganizationID != "org_1" || store.filter.Limit != 100 {
		t.Fatalf("expected runtime stats to load current organization channel ids, got %#v", store.filter)
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
	actor := testAdminActor()
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

func testAdminActor() auth.Session {
	return auth.Session{
		OrganizationID: "org_1",
		User: auth.User{
			ID:    "user_admin",
			Email: "admin@example.com",
			Role:  "admin",
		},
	}
}

type fakeChannelRuntimeStatsProvider struct {
	stats map[string]*relaytypes.ChannelStats
}

func (p fakeChannelRuntimeStatsProvider) GetAllStats() map[string]*relaytypes.ChannelStats {
	return p.stats
}

type runtimeStatsChannelStore struct {
	Store
	filter   ChannelFilter
	channels []*ChannelInfo
}

func (s *runtimeStatsChannelStore) ListChannels(_ context.Context, filter ChannelFilter) ([]*ChannelInfo, error) {
	s.filter = filter
	return s.channels, nil
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
	actor := testAdminActor()
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

	result, err := service.DetectChannelModelUpdates(context.Background(), "org_1", "ch_1")
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
	actor := testAdminActor()
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
	actor := testAdminActor()
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

func TestServiceRedactsChannelAPIKeyFromAuditChanges(t *testing.T) {
	store := &relayConfigApplyStore{}
	service := NewService(store)
	actor := testAdminActor()
	request := httptest.NewRequest("POST", "/api/v1/admin/channels", nil)

	_, err := service.CreateChannel(context.Background(), actor, ChannelCreateRequest{
		Name:     "Secret OpenAI",
		Provider: "openai",
		APIKey:   "sk-create-secret",
	}, request)
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if store.createdAPIKey != "sk-create-secret" {
		t.Fatalf("expected store to receive raw create API key, got %q", store.createdAPIKey)
	}
	if len(store.auditEntries) != 1 || store.auditEntries[0].Action != "channel.create" {
		t.Fatalf("expected channel.create audit entry, got %#v", store.auditEntries)
	}
	if strings.Contains(store.auditEntries[0].Changes, "sk-create-secret") || !strings.Contains(store.auditEntries[0].Changes, `"apiKey":"********"`) {
		t.Fatalf("expected create audit changes to redact apiKey, got %s", store.auditEntries[0].Changes)
	}

	updateKey := "sk-update-secret"
	_, err = service.UpdateChannel(context.Background(), actor, "ch_1", ChannelUpdateRequest{
		APIKey: &updateKey,
	}, request)
	if err != nil {
		t.Fatalf("UpdateChannel failed: %v", err)
	}
	if store.updatedAPIKey == nil || *store.updatedAPIKey != "sk-update-secret" {
		t.Fatalf("expected store to receive raw update API key, got %#v", store.updatedAPIKey)
	}
	if len(store.auditEntries) != 2 || store.auditEntries[1].Action != "channel.update" {
		t.Fatalf("expected channel.update audit entry, got %#v", store.auditEntries)
	}
	if strings.Contains(store.auditEntries[1].Changes, "sk-update-secret") || !strings.Contains(store.auditEntries[1].Changes, `"apiKey":"********"`) {
		t.Fatalf("expected update audit changes to redact apiKey, got %s", store.auditEntries[1].Changes)
	}
}

func TestServiceRejectsUnsafeChannelBaseURLBeforePersisting(t *testing.T) {
	unsafeUpdateURL := "http://169.254.169.254/latest/meta-data"
	tests := []struct {
		name        string
		createURL   string
		updateURL   *string
		wantSnippet string
	}{
		{
			name:        "loopback create",
			createURL:   "http://127.0.0.1:8080/v1",
			wantSnippet: "channel base URL must not target local or private network addresses",
		},
		{
			name:        "private create",
			createURL:   "http://10.0.0.8:8080/v1",
			wantSnippet: "channel base URL must not target local or private network addresses",
		},
		{
			name:        "metadata update",
			updateURL:   &unsafeUpdateURL,
			wantSnippet: "channel base URL must not target local or private network addresses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &relayConfigApplyStore{}
			var applied []RelayConfigChange
			service := NewService(store, WithRelayConfigApplier(func(ctx context.Context, change RelayConfigChange) error {
				applied = append(applied, change)
				return nil
			}))
			actor := testAdminActor()
			request := httptest.NewRequest("POST", "/api/v1/admin/channels", nil)

			var err error
			if tt.updateURL != nil {
				_, err = service.UpdateChannel(context.Background(), actor, "ch_1", ChannelUpdateRequest{BaseURL: tt.updateURL}, request)
			} else {
				_, err = service.CreateChannel(context.Background(), actor, ChannelCreateRequest{
					Name:     "Unsafe OpenAI",
					Provider: "openai",
					BaseURL:  tt.createURL,
				}, request)
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantSnippet) {
				t.Fatalf("expected unsafe base URL error containing %q, got %v", tt.wantSnippet, err)
			}
			if store.createdChannelCalled || store.updatedChannelCalled {
				t.Fatalf("unsafe base URL should not reach store, create=%v update=%v", store.createdChannelCalled, store.updatedChannelCalled)
			}
			if len(applied) != 0 {
				t.Fatalf("unsafe base URL should not apply relay config, got %#v", applied)
			}
			if len(store.auditEntries) != 0 {
				t.Fatalf("unsafe base URL should not audit, got %#v", store.auditEntries)
			}
		})
	}
}

func TestServicePassesChannelWeightThroughCreateAndUpdate(t *testing.T) {
	store := &relayConfigApplyStore{}
	service := NewService(store)
	actor := testAdminActor()
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
	updateCalls   int
}

func (s *syncChannelModelsStore) GetChannel(ctx context.Context, organizationID, id string) (*ChannelInfo, error) {
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.currentModels...)}, nil
}

func (s *syncChannelModelsStore) TestChannel(ctx context.Context, organizationID, id string) (*ChannelTestResult, error) {
	return s.testResult, nil
}

func (s *syncChannelModelsStore) UpdateChannel(ctx context.Context, organizationID, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	s.updateCalls++
	if input.Models != nil {
		s.updatedModels = append([]string{}, (*input.Models)...)
	}
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Models: append([]string{}, s.updatedModels...)}, nil
}

func (s *syncChannelModelsStore) UpdateChannelDiagnostics(ctx context.Context, organizationID, id string, input ChannelDiagnosticsUpdate) (*ChannelHealth, error) {
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
	createdChannelCalled bool
	updatedChannelCalled bool
	createdWeight        int
	updatedWeight        *int
	createdAPIKey        string
	updatedAPIKey        *string
	auditEntries         []*AuditEntry
}

type userQuotaAdminStore struct {
	Store
	user         *UserDetail
	userErr      error
	quotaUserID  string
	quotaBalance float64
	auditEntries []*AuditEntry
}

func (s *userQuotaAdminStore) GetUserByID(ctx context.Context, id string) (*UserDetail, error) {
	if s.userErr != nil {
		return nil, s.userErr
	}
	if s.user != nil && s.user.ID == id {
		return s.user, nil
	}
	return nil, errors.New("user not found: " + id)
}

func (s *userQuotaAdminStore) UpdateUserQuota(ctx context.Context, userID string, balance float64) error {
	s.quotaUserID = userID
	s.quotaBalance = balance
	return nil
}

func (s *userQuotaAdminStore) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	s.auditEntries = append(s.auditEntries, entry)
	return nil
}

func (s *relayConfigApplyStore) CreateChannel(ctx context.Context, input ChannelCreateRequest) (*ChannelInfo, error) {
	s.createdChannelCalled = true
	s.createdWeight = input.Weight
	s.createdAPIKey = input.APIKey
	return &ChannelInfo{ID: "ch_1", Name: input.Name, Provider: input.Provider, Weight: input.Weight}, nil
}

func (s *relayConfigApplyStore) UpdateChannel(ctx context.Context, organizationID, id string, input ChannelUpdateRequest) (*ChannelInfo, error) {
	s.updatedChannelCalled = true
	s.updatedWeight = input.Weight
	s.updatedAPIKey = input.APIKey
	weight := 0
	if input.Weight != nil {
		weight = *input.Weight
	}
	return &ChannelInfo{ID: id, Name: "OpenAI", Provider: "openai", Weight: weight}, nil
}

func (s *relayConfigApplyStore) DeleteChannel(ctx context.Context, organizationID, id string) error {
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
	s.auditEntries = append(s.auditEntries, entry)
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
