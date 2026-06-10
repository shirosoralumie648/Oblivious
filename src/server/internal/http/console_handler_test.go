package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/console"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/userprefs"
)

func TestConsoleHandlerGetUsageReturnsTypedUsageSummary(t *testing.T) {
	createdAt := time.Date(2026, 6, 9, 8, 30, 0, 0, time.UTC)
	store := &consoleHandlerFakeStore{
		summary: console.UsageSummary{
			Period:   "30d",
			Requests: 3,
			ByModel: []console.UsageDimensionSummary{{
				Key:          "gpt-4o",
				RequestCount: 2,
				TotalTokens:  1200,
				TotalCost:    0.42,
			}},
			ByFeature: []console.UsageDimensionSummary{{
				Key:          "chat",
				RequestCount: 3,
				TotalTokens:  1800,
				TotalCost:    0.55,
			}},
			ByUser: []console.UsageDimensionSummary{{
				Key:          "user_1",
				RequestCount: 3,
				TotalTokens:  1800,
				TotalCost:    0.55,
			}},
			TimeSeries: []console.UsageTimeSeriesSummary{{
				Bucket:       "2026-06-09T08:00:00Z",
				RequestCount: 3,
				TotalTokens:  1800,
				TotalCost:    0.55,
			}},
			Recent: []relay.RelayAPITokenUsageItem{{
				ID:               "usage_1",
				APITokenID:       "tok_1",
				RequestID:        "req_1",
				APIType:          "chat",
				Model:            "gpt-4o",
				ChannelID:        "ch_1",
				Provider:         "openai",
				Status:           "success",
				StatusCode:       200,
				LatencyMS:        123,
				Cost:             0.21,
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CreatedAt:        createdAt,
			}},
		},
	}
	handler := newConsoleHandler(console.NewService(store), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/usage", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.getUsage(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data console.UsageSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode usage response: %v", err)
	}
	if store.organizationID != "org_1" || store.userID != "user_1" {
		t.Fatalf("expected scoped usage lookup, got org=%s user=%s", store.organizationID, store.userID)
	}
	if response.Data.Period != "30d" || response.Data.Requests != 3 ||
		len(response.Data.ByModel) != 1 || response.Data.ByModel[0].Key != "gpt-4o" ||
		len(response.Data.TimeSeries) != 1 || response.Data.TimeSeries[0].Bucket != "2026-06-09T08:00:00Z" ||
		len(response.Data.Recent) != 1 || response.Data.Recent[0].APITokenID != "tok_1" ||
		response.Data.Recent[0].PromptTokens != 100 || !response.Data.Recent[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected usage summary: %+v", response.Data)
	}
}

func TestConsoleHandlerGetAccessReturnsTypedAccessSummary(t *testing.T) {
	preferencesStore := &consoleHandlerPreferencesStore{
		preferences: userprefs.Preferences{
			DefaultMode:         "agent",
			ModelStrategy:       "quality",
			NetworkEnabledHint:  true,
			OnboardingCompleted: true,
		},
	}
	expiresAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	handler := newConsoleHandler(console.NewService(&consoleHandlerFakeStore{}), userprefs.NewService(preferencesStore))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/access", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		ID:             "sess_1",
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1", Email: "user@example.com"},
		ExpiresAt:      expiresAt,
	}))
	recorder := httptest.NewRecorder()

	handler.getAccess(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data console.AccessSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode access response: %v", err)
	}
	if preferencesStore.userID != "user_1" {
		t.Fatalf("expected preferences lookup for user_1, got %s", preferencesStore.userID)
	}
	if response.Data.DefaultMode != "agent" || response.Data.ModelStrategy != "quality" ||
		!response.Data.NetworkEnabledHint || !response.Data.OnboardingCompleted ||
		response.Data.SessionExpiresAt != "2026-06-10T12:00:00Z" ||
		response.Data.SessionID != "sess_1" || response.Data.UserEmail != "user@example.com" ||
		response.Data.UserID != "user_1" || response.Data.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected access summary: %+v", response.Data)
	}
}

func TestConsoleHandlerListInvoicesReturnsOrganizationInvoices(t *testing.T) {
	store := &consoleHandlerFakeStore{
		invoices: []console.BillingInvoiceSummary{{
			ID:        "inv_paid_1",
			Status:    "paid",
			AmountUSD: 29,
			DueAt:     "2026-05-31T00:00:00Z",
		}},
	}
	handler := newConsoleHandler(console.NewService(store), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/invoices", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.listBillingInvoices(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []console.BillingInvoiceSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode invoice response: %v", err)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "inv_paid_1" || response.Data[0].Status != "paid" {
		t.Fatalf("unexpected invoices: %+v", response.Data)
	}
}

func TestConsoleHandlerListAPITokensReturnsTypedTokensWithoutSecrets(t *testing.T) {
	quotaLimit := 100.0
	createdAt := time.Date(2026, 6, 9, 9, 0, 0, 0, time.UTC)
	apiTokenStore := &consoleHandlerAPITokenStore{
		tokens: []relay.RelayAPITokenListItem{{
			ID:                 "tok_1",
			Name:               "Production key",
			TokenPrefix:        "obl_live",
			Status:             relay.RelayAPITokenStatusActive,
			UserGroup:          "default",
			ModelLimitsEnabled: true,
			ModelLimits:        []string{"gpt-4o"},
			QuotaLimit:         &quotaLimit,
			UsedQuota:          12.5,
			CreatedAt:          createdAt,
		}},
	}
	handler := newConsoleHandler(console.NewServiceWithAPITokens(&consoleHandlerFakeStore{}, apiTokenStore), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/api-tokens", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.listAPITokens(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	assertNoTokenSecretKeys(t, recorder.Body.String(), "list api tokens")
	var response struct {
		Data []relay.RelayAPITokenListItem `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode api token response: %v", err)
	}
	if apiTokenStore.listOrganizationID != "org_1" || apiTokenStore.listUserID != "user_1" {
		t.Fatalf("expected scoped api token list, got org=%s user=%s", apiTokenStore.listOrganizationID, apiTokenStore.listUserID)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "tok_1" || response.Data[0].TokenPrefix != "obl_live" ||
		response.Data[0].QuotaLimit == nil || *response.Data[0].QuotaLimit != quotaLimit ||
		!response.Data[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected api tokens: %+v", response.Data)
	}
}

func TestConsoleHandlerListAPITokenUsageReturnsTypedUsageItems(t *testing.T) {
	createdAt := time.Date(2026, 6, 9, 9, 30, 0, 0, time.UTC)
	apiTokenStore := &consoleHandlerAPITokenStore{
		tokens: []relay.RelayAPITokenListItem{{ID: "tok_1", Name: "Production key"}},
		usage: []relay.RelayAPITokenUsageItem{{
			ID:               "usage_1",
			APITokenID:       "tok_1",
			RequestID:        "req_1",
			APIType:          "chat",
			Model:            "gpt-4o",
			ChannelID:        "ch_1",
			Provider:         "openai",
			Status:           "success",
			StatusCode:       200,
			ErrorCode:        "",
			LatencyMS:        320,
			Cost:             0.15,
			PromptTokens:     120,
			CompletionTokens: 80,
			TotalTokens:      200,
			CreatedAt:        createdAt,
		}},
	}
	handler := newConsoleHandler(console.NewServiceWithAPITokens(&consoleHandlerFakeStore{}, apiTokenStore), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/api-tokens/tok_1/usage", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.listAPITokenUsage(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	assertNoTokenSecretKeys(t, recorder.Body.String(), "api token usage")
	var response struct {
		Data []relay.RelayAPITokenUsageItem `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode api token usage response: %v", err)
	}
	if apiTokenStore.listOrganizationID != "org_1" || apiTokenStore.listUserID != "user_1" ||
		apiTokenStore.usageOrganizationID != "org_1" || apiTokenStore.usageUserID != "user_1" ||
		apiTokenStore.usageTokenID != "tok_1" {
		t.Fatalf("expected scoped api token usage, got list org=%s user=%s usage org=%s user=%s token=%s",
			apiTokenStore.listOrganizationID, apiTokenStore.listUserID, apiTokenStore.usageOrganizationID, apiTokenStore.usageUserID, apiTokenStore.usageTokenID)
	}
	if len(response.Data) != 1 || response.Data[0].APITokenID != "tok_1" || response.Data[0].RequestID != "req_1" ||
		response.Data[0].StatusCode != 200 || response.Data[0].LatencyMS != 320 ||
		response.Data[0].PromptTokens != 120 || response.Data[0].CompletionTokens != 80 ||
		response.Data[0].TotalTokens != 200 || !response.Data[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected api token usage: %+v", response.Data)
	}
}

func TestConsoleHandlerRevokeAPITokenReturnsTypedRevokedStatus(t *testing.T) {
	apiTokenStore := &consoleHandlerAPITokenStore{}
	handler := newConsoleHandler(console.NewServiceWithAPITokens(&consoleHandlerFakeStore{}, apiTokenStore), nil)
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/console/api-tokens/tok_1", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.revokeAPIToken(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode revoke response: %v", err)
	}
	if response.Data.Status != "revoked" {
		t.Fatalf("expected revoked status, got %+v", response.Data)
	}
	if apiTokenStore.revokeOrganizationID != "org_1" || apiTokenStore.revokeUserID != "user_1" || apiTokenStore.revokedTokenID != "tok_1" {
		t.Fatalf("expected scoped revoke, got org=%s user=%s token=%s",
			apiTokenStore.revokeOrganizationID, apiTokenStore.revokeUserID, apiTokenStore.revokedTokenID)
	}
}

func TestConsoleHandlerGetBillingReturnsConfiguredPaymentProviders(t *testing.T) {
	store := &consoleHandlerFakeStore{
		billing: console.BillingSummary{
			Period:           "30d",
			Requests:         5,
			InputTokens:      120,
			OutputTokens:     80,
			EstimatedCostUSD: 0.0004,
		},
	}
	handler := newConsoleHandler(console.NewService(store, console.WithBillingPaymentProviders([]console.BillingPaymentProviderSummary{
		{Name: "stripe"},
		{Name: "alipay"},
	})), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/billing", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.getBilling(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data console.BillingSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode billing response: %v", err)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if len(response.Data.PaymentProviders) != 2 || response.Data.PaymentProviders[0].Name != "stripe" || response.Data.PaymentProviders[1].Name != "alipay" {
		t.Fatalf("expected configured payment providers, got %+v", response.Data.PaymentProviders)
	}
}

func TestConsoleHandlerGetModelsReturnsTypedModelSummaries(t *testing.T) {
	store := &consoleHandlerFakeStore{
		models: []console.ModelSummary{
			{ID: "gpt-4o", Label: "GPT-4o", Requests: 7},
			{ID: "gpt-4o-mini", Label: "GPT-4o mini", Requests: 3},
		},
	}
	handler := newConsoleHandler(console.NewService(store), nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/models", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.getModels(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []console.ModelSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "gpt-4o" || response.Data[0].Label != "GPT-4o" || response.Data[0].Requests != 7 {
		t.Fatalf("unexpected model summaries: %+v", response.Data)
	}
}

type consoleHandlerFakeStore struct {
	billing        console.BillingSummary
	invoices       []console.BillingInvoiceSummary
	models         []console.ModelSummary
	organizationID string
	summary        console.UsageSummary
	userID         string
}

func (s *consoleHandlerFakeStore) GetBillingSummary(_ context.Context, organizationID string) (console.BillingSummary, error) {
	s.organizationID = organizationID
	return s.billing, nil
}

func (s *consoleHandlerFakeStore) ListBillingInvoices(_ context.Context, organizationID string) ([]console.BillingInvoiceSummary, error) {
	s.organizationID = organizationID
	return s.invoices, nil
}

func (s *consoleHandlerFakeStore) GetModelSummaries(_ context.Context, organizationID string) ([]console.ModelSummary, error) {
	s.organizationID = organizationID
	return s.models, nil
}

func (s *consoleHandlerFakeStore) GetUsageSummary(_ context.Context, organizationID, userID string) (console.UsageSummary, error) {
	s.organizationID = organizationID
	s.userID = userID
	return s.summary, nil
}

type consoleHandlerPreferencesStore struct {
	preferences userprefs.Preferences
	userID      string
}

func (s *consoleHandlerPreferencesStore) GetByUserID(_ context.Context, userID string) (userprefs.Preferences, error) {
	s.userID = userID
	return s.preferences, nil
}

func (s *consoleHandlerPreferencesStore) UpsertByUserID(_ context.Context, userID string, preferences userprefs.Preferences) (userprefs.Preferences, error) {
	s.userID = userID
	s.preferences = preferences
	return preferences, nil
}

type consoleHandlerAPITokenStore struct {
	created              relay.CreatedRelayAPIToken
	input                relay.CreateRelayAPITokenInput
	listOrganizationID   string
	listUserID           string
	revokeOrganizationID string
	revokeUserID         string
	revokedTokenID       string
	tokens               []relay.RelayAPITokenListItem
	usage                []relay.RelayAPITokenUsageItem
	usageOrganizationID  string
	usageUserID          string
	usageTokenID         string
}

func (s *consoleHandlerAPITokenStore) CreateRelayAPIToken(_ context.Context, input relay.CreateRelayAPITokenInput) (relay.CreatedRelayAPIToken, error) {
	s.input = input
	return s.created, nil
}

func (s *consoleHandlerAPITokenStore) ListRelayAPITokens(_ context.Context, organizationID, userID string) ([]relay.RelayAPITokenListItem, error) {
	s.listOrganizationID = organizationID
	s.listUserID = userID
	return s.tokens, nil
}

func (s *consoleHandlerAPITokenStore) ListRelayAPITokenUsage(_ context.Context, organizationID, userID, tokenID string) ([]relay.RelayAPITokenUsageItem, error) {
	s.usageOrganizationID = organizationID
	s.usageUserID = userID
	s.usageTokenID = tokenID
	return s.usage, nil
}

func (s *consoleHandlerAPITokenStore) RevokeRelayAPIToken(_ context.Context, organizationID, userID, tokenID string) error {
	s.revokeOrganizationID = organizationID
	s.revokeUserID = userID
	s.revokedTokenID = tokenID
	return nil
}

func assertNoTokenSecretKeys(t *testing.T, body string, surface string) {
	t.Helper()
	for _, key := range []string{"rawToken", "tokenHash", "token_hash"} {
		if jsonContainsKey(body, key) {
			t.Fatalf("%s must not expose %s: %s", surface, key, body)
		}
	}
}

func jsonContainsKey(body string, key string) bool {
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return false
	}
	return jsonObjectContainsKey(payload, key)
}

func jsonObjectContainsKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for candidate, child := range typed {
			if candidate == key || jsonObjectContainsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonObjectContainsKey(child, key) {
				return true
			}
		}
	}
	return false
}
