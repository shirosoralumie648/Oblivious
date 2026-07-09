package http

import (
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/admin"
	relaytypes "oblivious/server/internal/relay/types"
	"oblivious/server/internal/secretbox"
)

func TestAdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers(t *testing.T) {
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-admin-relay-channel-secret")
	database := testDatabase(t)
	applyCommercialJourneyMigrations(t, database)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "admin-relay-channel-redaction@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	var gotProbeAuth string
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		gotProbeAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
		case "/v1/dashboard/billing/credit_grants":
			_, _ = w.Write([]byte(`{"total_available":42}`))
		default:
			t.Fatalf("unexpected admin relay channel probe path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)
	channelBaseURL := "https://api.openai.example.com"

	secrets := struct {
		create  string
		updated string
	}{
		create:  "sk-admin-relay-create-secret",
		updated: "sk-admin-relay-rotate-secret",
	}

	createBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/channels", `{
		"name": "Secret OpenAI",
		"provider": "openai",
		"baseURL": "`+channelBaseURL+`",
		"apiKey": "`+secrets.create+`",
		"models": ["gpt-4o-mini"],
		"groups": ["default"],
		"rpmLimit": 120,
		"tpmLimit": 120000,
		"priority": 10,
		"weight": 80,
		"estimatedCostPer1K": 0.02,
		"costMultiplier": 1.25
	}`, cookie, csrfToken, stdhttp.StatusCreated)
	assertAdminRelayChannelResponseRedactsSecrets(t, string(createBody), secrets.create)

	var createResponse struct {
		Data admin.ChannelInfo `json:"data"`
	}
	if err := json.Unmarshal(createBody, &createResponse); err != nil {
		t.Fatalf("decode admin relay channel create response: %v", err)
	}
	channelID := createResponse.Data.ID
	if channelID == "" || createResponse.Data.Name != "Secret OpenAI" || createResponse.Data.Provider != "openai" {
		t.Fatalf("expected created admin relay channel identity, got %+v", createResponse.Data)
	}
	assertAdminRelayChannelStoredAPIKey(t, database, channelID, secrets.create)

	listBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/channels", "", cookie, "", stdhttp.StatusOK)
	assertAdminRelayChannelResponseRedactsSecrets(t, string(listBody), secrets.create)
	var listResponse struct {
		Data struct {
			Channels []admin.ChannelInfo `json:"channels"`
			Total    int                 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &listResponse); err != nil {
		t.Fatalf("decode admin relay channel list response: %v", err)
	}
	if listResponse.Data.Total != 1 || len(listResponse.Data.Channels) != 1 || listResponse.Data.Channels[0].ID != channelID {
		t.Fatalf("expected channel list to contain created channel only, got %+v", listResponse.Data)
	}
	if listResponse.Data.Channels[0].BaseURL != channelBaseURL || listResponse.Data.Channels[0].Weight != 80 {
		t.Fatalf("expected channel list to preserve non-secret config, got %+v", listResponse.Data.Channels[0])
	}

	getBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/channels/"+channelID, "", cookie, "", stdhttp.StatusOK)
	assertAdminRelayChannelResponseRedactsSecrets(t, string(getBody), secrets.create)
	var getResponse struct {
		Data admin.ChannelInfo `json:"data"`
	}
	if err := json.Unmarshal(getBody, &getResponse); err != nil {
		t.Fatalf("decode admin relay channel get response: %v", err)
	}
	if getResponse.Data.ID != channelID || getResponse.Data.BaseURL != channelBaseURL {
		t.Fatalf("expected channel get to preserve non-secret config, got %+v", getResponse.Data)
	}

	updateBody := commercialDoJSON(t, router, stdhttp.MethodPut, "/api/v1/admin/channels/"+channelID, `{
		"apiKey": "`+secrets.updated+`",
		"models": ["gpt-4o-mini", "gpt-4.1"]
	}`, cookie, csrfToken, stdhttp.StatusOK)
	assertAdminRelayChannelResponseRedactsSecrets(t, string(updateBody), secrets.create, secrets.updated)
	var updateResponse struct {
		Data admin.ChannelInfo `json:"data"`
	}
	if err := json.Unmarshal(updateBody, &updateResponse); err != nil {
		t.Fatalf("decode admin relay channel update response: %v", err)
	}
	if updateResponse.Data.ID != channelID || updateResponse.Data.BaseURL != channelBaseURL {
		t.Fatalf("expected channel update to preserve non-secret config, got %+v", updateResponse.Data)
	}
	if len(updateResponse.Data.Models) != 2 || updateResponse.Data.Models[0] != "gpt-4o-mini" || updateResponse.Data.Models[1] != "gpt-4.1" {
		t.Fatalf("expected channel update to persist models, got %+v", updateResponse.Data.Models)
	}
	assertAdminRelayChannelStoredAPIKey(t, database, channelID, secrets.updated)
	assertAdminRelayChannelStoredBaseURL(t, database, channelID, channelBaseURL)

	if _, err := database.Exec(`UPDATE channels SET base_url = $1 WHERE id = $2`, upstream.URL, channelID); err != nil {
		t.Fatalf("point admin relay channel probe fixture at local upstream: %v", err)
	}

	testBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/channels/"+channelID+"/test", "", cookie, csrfToken, stdhttp.StatusOK)
	if gotProbeAuth != "Bearer "+secrets.updated {
		t.Fatalf("expected admin relay probe to use decrypted updated API key, got %q body=%s", gotProbeAuth, testBody)
	}

	auditBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/audit-logs?resourceType=channel&resourceID="+channelID, "", cookie, "", stdhttp.StatusOK)
	assertAdminRelayChannelAuditResponseRedactsSecrets(t, string(auditBody), secrets.create, secrets.updated)
	var auditResponse struct {
		Data struct {
			Entries []admin.AuditEntry `json:"entries"`
			Total   int                `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(auditBody, &auditResponse); err != nil {
		t.Fatalf("decode admin relay channel audit response: %v", err)
	}
	if auditResponse.Data.Total != 2 || len(auditResponse.Data.Entries) != 2 {
		t.Fatalf("expected two admin relay channel audit entries, got %+v", auditResponse.Data)
	}
	redactedEntries := 0
	for _, entry := range auditResponse.Data.Entries {
		if entry.ResourceType != "channel" || entry.ResourceID != channelID {
			t.Fatalf("expected channel audit entry for %s, got %+v", channelID, entry)
		}
		if adminRelayChannelAuditChangesRedactAPIKey(t, entry.Changes) {
			redactedEntries++
		}
	}
	if redactedEntries != 2 {
		t.Fatalf("expected both admin relay channel audit entries to redact apiKey, got %+v", auditResponse.Data.Entries)
	}
}

func TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation(t *testing.T) {
	database := testDatabase(t)
	applyCommercialJourneyMigrations(t, database)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "admin-relay-channel-isolation@example.com")
	_, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Relay Org", "other-relay-org")

	if _, err := database.Exec(`
		INSERT INTO channels (
			id, organization_id, name, provider, base_url, api_key_encrypted, models, groups,
			rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier,
			enabled, last_health_status, last_latency_ms, created_at, updated_at
		)
		VALUES
			('channel_active_primary', $1, 'Active primary relay', 'openai', 'https://active-primary.example.test', 'sk-active-primary', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 100, 100000, 10, 100, 0.02, 1.1, true, 'online', 42, NOW(), NOW()),
			('channel_active_batch', $1, 'Active batch relay', 'openai', 'https://active-batch.example.test', 'sk-active-batch', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 100, 100000, 20, 100, 0.02, 1.1, true, 'online', 43, NOW(), NOW()),
			('channel_other_org', $2, 'Other org relay', 'openai', 'https://other-org.example.test', 'sk-other-org', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 100, 100000, 30, 100, 0.02, 1.1, true, 'online', 44, NOW(), NOW())
	`, activeOrganizationID, otherOrganizationID); err != nil {
		t.Fatalf("insert admin relay channel fixtures: %v", err)
	}

	listBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/channels", "", cookie, "", stdhttp.StatusOK)
	var listResponse struct {
		Data struct {
			Channels []admin.ChannelInfo `json:"channels"`
			Total    int                 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &listResponse); err != nil {
		t.Fatalf("decode admin relay channel list response: %v", err)
	}
	if listResponse.Data.Total != 2 || len(listResponse.Data.Channels) != 2 {
		t.Fatalf("expected only active organization channels in list, got %+v", listResponse.Data)
	}
	for _, channel := range listResponse.Data.Channels {
		if channel.ID == "channel_other_org" || channel.OrganizationID == otherOrganizationID {
			t.Fatalf("active organization %s must not list other organization channel %+v", activeOrganizationID, channel)
		}
		if channel.OrganizationID != activeOrganizationID {
			t.Fatalf("expected listed admin relay channel to carry active organization %s, got %+v", activeOrganizationID, channel)
		}
	}

	crossTenantRequests := []struct {
		name       string
		method     string
		path       string
		body       string
		csrf       bool
		wantStatus int
	}{
		{
			name:       "get channel",
			method:     stdhttp.MethodGet,
			path:       "/api/v1/admin/channels/channel_other_org",
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "update channel",
			method:     stdhttp.MethodPut,
			path:       "/api/v1/admin/channels/channel_other_org",
			body:       `{"name":"Mutated other org relay","enabled":false}`,
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "delete channel",
			method:     stdhttp.MethodDelete,
			path:       "/api/v1/admin/channels/channel_other_org",
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "test channel",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/channel_other_org/test",
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "get health",
			method:     stdhttp.MethodGet,
			path:       "/api/v1/admin/channels/channel_other_org/health",
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "sync models",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/channel_other_org/sync-models",
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "detect model updates",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/channel_other_org/model-updates/detect",
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "apply model updates",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/channel_other_org/model-updates/apply",
			body:       `{"mode":"replace"}`,
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "refresh balance",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/channel_other_org/refresh-balance",
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
		{
			name:       "mixed organization batch",
			method:     stdhttp.MethodPost,
			path:       "/api/v1/admin/channels/batch",
			body:       `{"ids":["channel_active_batch","channel_other_org"],"action":"disable"}`,
			csrf:       true,
			wantStatus: stdhttp.StatusNotFound,
		},
	}
	for _, requestCase := range crossTenantRequests {
		token := ""
		if requestCase.csrf {
			token = csrfToken
		}
		body := commercialDoJSON(t, router, requestCase.method, requestCase.path, requestCase.body, cookie, token, requestCase.wantStatus)
		if !strings.Contains(string(body), "not_found") {
			t.Fatalf("%s expected fail-closed not_found response, got %s", requestCase.name, body)
		}
	}

	var otherName string
	var otherEnabled bool
	if err := database.QueryRow(`SELECT name, enabled FROM channels WHERE id = 'channel_other_org'`).Scan(&otherName, &otherEnabled); err != nil {
		t.Fatalf("query other organization admin relay channel after denied mutations: %v", err)
	}
	if otherName != "Other org relay" || !otherEnabled {
		t.Fatalf("expected denied mutations to preserve other organization relay channel, got name=%q enabled=%v", otherName, otherEnabled)
	}

	var activeBatchEnabled bool
	if err := database.QueryRow(`SELECT enabled FROM channels WHERE id = 'channel_active_batch'`).Scan(&activeBatchEnabled); err != nil {
		t.Fatalf("query active batch relay channel after denied mixed batch: %v", err)
	}
	if !activeBatchEnabled {
		t.Fatalf("expected denied mixed-organization batch not to mutate active channel")
	}
}

func TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization(t *testing.T) {
	database := testDatabase(t)
	applyCommercialJourneyMigrations(t, database)
	router := NewRouterWithOptions(testConfig(), database, RouterOptions{
		ChannelRuntimeStatsProvider: fakeRuntimeStatsProvider{
			stats: map[string]*relaytypes.ChannelStats{
				"channel_active_read": {
					ChannelID:     "channel_active_read",
					RPMCurrent:    5,
					TPMCurrent:    150,
					TotalRequests: 12,
					SuccessCount:  11,
				},
				"channel_other_read": {
					ChannelID:     "channel_other_read",
					RPMCurrent:    99,
					TPMCurrent:    900,
					TotalRequests: 999,
					FailureCount:  999,
				},
			},
		},
	})
	cookie, csrfToken, userID := registerHTTPUser(t, router, "admin-relay-read-scope@example.com")
	workspaceID, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Relay Read Org", "other-relay-read-org")

	if _, err := database.Exec(`
		INSERT INTO channels (
			id, organization_id, name, provider, base_url, api_key_encrypted, models, groups,
			rpm_limit, tpm_limit, priority, weight, estimated_cost_per_1k, cost_multiplier,
			enabled, last_health_status, last_latency_ms, created_at, updated_at
		)
		VALUES
			('channel_active_read', $1, 'Active read relay', 'openai', 'https://active-read.example.test', 'sk-active-read', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 120, 120000, 10, 100, 0.02, 1.1, true, 'online', 41, NOW(), NOW()),
			('channel_other_read', $2, 'Other read relay', 'openai', 'https://other-read.example.test', 'sk-other-read', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 120, 120000, 20, 100, 0.02, 1.1, true, 'online', 42, NOW(), NOW())
	`, activeOrganizationID, otherOrganizationID); err != nil {
		t.Fatalf("insert admin relay read fixtures: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO usage_records (
			id, user_id, workspace_id, organization_id, model_id, request_count, input_tokens, output_tokens,
			channel_id, provider, cost, channel_cost, total_tokens, created_at
		)
		VALUES
			('usage_active_read', $1, $2, $3, 'gpt-4o-mini', 3, 30, 12, 'channel_active_read', 'openai', 0.30, 0.12, 42, NOW()),
			('usage_other_read', $1, NULL, $4, 'gpt-4o-mini', 99, 99, 99, 'channel_other_read', 'openai', 9.90, 4.95, 198, NOW())
	`, userID, workspaceID, activeOrganizationID, otherOrganizationID); err != nil {
		t.Fatalf("insert admin relay read usage fixtures: %v", err)
	}

	statsBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/channels/stats", "", cookie, "", stdhttp.StatusOK)
	if strings.Contains(string(statsBody), "channel_other_read") {
		t.Fatalf("expected runtime stats to hide other organization channel, got %s", statsBody)
	}
	var statsResponse struct {
		Data struct {
			Stats []admin.ChannelRuntimeStats `json:"stats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(statsBody, &statsResponse); err != nil {
		t.Fatalf("decode admin relay runtime stats response: %v", err)
	}
	if len(statsResponse.Data.Stats) != 1 || statsResponse.Data.Stats[0].ChannelID != "channel_active_read" {
		t.Fatalf("expected only active organization runtime stats, got %+v", statsResponse.Data.Stats)
	}
	if statsResponse.Data.Stats[0].RPMCurrent != 5 || statsResponse.Data.Stats[0].TotalRequests != 12 {
		t.Fatalf("expected active runtime stats to be preserved, got %+v", statsResponse.Data.Stats[0])
	}

	modelBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/models?provider=openai&sort=model:asc", "", cookie, "", stdhttp.StatusOK)
	if strings.Contains(string(modelBody), "channel_other_read") || strings.Contains(string(modelBody), "other-read.example.test") {
		t.Fatalf("expected model inventory to hide other organization channels, got %s", modelBody)
	}
	var modelResponse struct {
		Data struct {
			Models []admin.ModelInventoryEntry `json:"models"`
			Total  int                         `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelBody, &modelResponse); err != nil {
		t.Fatalf("decode admin relay model inventory response: %v", err)
	}
	if modelResponse.Data.Total != 1 || len(modelResponse.Data.Models) != 1 {
		t.Fatalf("expected one active organization model, got %+v", modelResponse.Data)
	}
	model := modelResponse.Data.Models[0]
	if model.Model != "gpt-4o-mini" || model.RequestCount != 3 || model.TotalCost != 0.3 || model.TotalChannelCost != 0.12 {
		t.Fatalf("expected active organization model inventory to ignore other org usage, got %+v", model)
	}
	if len(model.Channels) != 1 || model.Channels[0].ID != "channel_active_read" {
		t.Fatalf("expected only active organization channels in model inventory, got %+v", model.Channels)
	}
}

func adminRelayChannelAuditChangesRedactAPIKey(t *testing.T, changes string) bool {
	t.Helper()

	var fields map[string]any
	if err := json.Unmarshal([]byte(changes), &fields); err != nil {
		t.Fatalf("decode admin relay channel audit changes %q: %v", changes, err)
	}
	value, ok := fields["apiKey"].(string)
	return ok && value == "********"
}

func assertAdminRelayChannelResponseRedactsSecrets(t *testing.T, body string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(body, secret) {
			t.Fatalf("expected admin relay channel response to redact %q, got %s", secret, body)
		}
	}
	if strings.Contains(body, "api_key_encrypted") {
		t.Fatalf("expected admin relay channel response to omit api_key_encrypted, got %s", body)
	}
	if strings.Contains(body, `"apiKey"`) {
		t.Fatalf("expected admin relay channel response to omit apiKey, got %s", body)
	}
}

func assertAdminRelayChannelAuditResponseRedactsSecrets(t *testing.T, body string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(body, secret) {
			t.Fatalf("expected admin relay channel audit response to redact %q, got %s", secret, body)
		}
	}
	if strings.Contains(body, "api_key_encrypted") {
		t.Fatalf("expected admin relay channel audit response to omit api_key_encrypted, got %s", body)
	}
}

func assertAdminRelayChannelStoredAPIKey(t *testing.T, database *sql.DB, channelID, want string) {
	t.Helper()

	var got string
	if err := database.QueryRow(`SELECT api_key_encrypted FROM channels WHERE id = $1`, channelID).Scan(&got); err != nil {
		t.Fatalf("query stored admin relay channel api key: %v", err)
	}
	if got == "" {
		t.Fatalf("expected stored admin relay channel api key to be non-empty")
	}
	if got == want || strings.Contains(got, want) {
		t.Fatalf("stored admin relay channel api key contains plaintext %q: %q", want, got)
	}
	if !secretbox.IsProtected(got) {
		t.Fatalf("expected stored admin relay channel api key to use protected prefix, got %q", got)
	}
	opened, err := secretbox.Open(secretbox.DomainRelayChannelAPIKey, got)
	if err != nil {
		t.Fatalf("open stored admin relay channel api key: %v", err)
	}
	if opened != want {
		t.Fatalf("opened admin relay channel api key = %q, want %q", opened, want)
	}
}

func assertAdminRelayChannelStoredBaseURL(t *testing.T, database *sql.DB, channelID, want string) {
	t.Helper()

	var got string
	if err := database.QueryRow(`SELECT base_url FROM channels WHERE id = $1`, channelID).Scan(&got); err != nil {
		t.Fatalf("query stored admin relay channel base URL: %v", err)
	}
	if got != want {
		t.Fatalf("expected stored admin relay channel base URL %q, got %q", want, got)
	}
}
