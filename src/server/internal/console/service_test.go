package console

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/userprefs"
)

type fakeStore struct {
	billing        BillingSummary
	invoices       []BillingInvoiceSummary
	models         []ModelSummary
	organizationID string
	summary        UsageSummary
	userID         string
}

func (f *fakeStore) GetUsageSummary(ctx context.Context, organizationID, userID string) (UsageSummary, error) {
	f.organizationID = organizationID
	f.userID = userID
	return f.summary, nil
}

func (f *fakeStore) GetModelSummaries(ctx context.Context, organizationID string) ([]ModelSummary, error) {
	f.organizationID = organizationID
	return f.models, nil
}

func (f *fakeStore) GetBillingSummary(ctx context.Context, organizationID string) (BillingSummary, error) {
	f.organizationID = organizationID
	return f.billing, nil
}

func (f *fakeStore) ListBillingInvoices(ctx context.Context, organizationID string) ([]BillingInvoiceSummary, error) {
	f.organizationID = organizationID
	return f.invoices, nil
}

func TestGetUsageReturnsOrganizationSummary(t *testing.T) {
	store := &fakeStore{
		summary: UsageSummary{
			Period:   "7d",
			Requests: 4,
			Recent: []relay.RelayAPITokenUsageItem{{
				ID:          "usage_1",
				APITokenID:  "tok_1",
				RequestID:   "req_1",
				Model:       "gpt-4o",
				Status:      "success",
				TotalTokens: 120,
			}},
		},
	}
	service := NewService(store)

	summary, err := service.GetUsage(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
		WorkspaceID:    "workspace_1",
	})
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if store.userID != "user_1" {
		t.Fatalf("expected user id user_1, got %s", store.userID)
	}
	if summary.Period != "7d" {
		t.Fatalf("expected period 7d, got %s", summary.Period)
	}
	if summary.Requests != 4 {
		t.Fatalf("expected requests 4, got %d", summary.Requests)
	}
	if len(summary.Recent) != 1 || summary.Recent[0].RequestID != "req_1" {
		t.Fatalf("expected recent usage request req_1, got %+v", summary.Recent)
	}
}

func TestGetModelsReturnsOrganizationModelSummaries(t *testing.T) {
	store := &fakeStore{
		models: []ModelSummary{
			{ID: "balanced-chat", Label: "balanced-chat", Requests: 2},
			{ID: "quality-chat", Label: "quality-chat", Requests: 1},
		},
	}
	service := NewService(store)

	models, err := service.GetModels(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get models: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 model summaries, got %d", len(models))
	}
	if models[0].ID != "balanced-chat" {
		t.Fatalf("expected first model balanced-chat, got %s", models[0].ID)
	}
}

func TestGetBillingReturnsOrganizationSummary(t *testing.T) {
	store := &fakeStore{
		billing: BillingSummary{
			Period:           "30d",
			Requests:         5,
			InputTokens:      120,
			OutputTokens:     80,
			EstimatedCostUSD: 0.0004,
			BalanceUSD:       42.50,
			CreditLimitUSD:   100.00,
			CurrentSpendUSD:  0.0004,
			NextInvoice: &BillingInvoiceSummary{
				ID:        "draft-2026-06",
				Status:    "draft",
				AmountUSD: 0.0004,
				DueAt:     "2026-06-30T00:00:00Z",
			},
		},
	}
	service := NewService(store)

	summary, err := service.GetBilling(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if summary.Requests != 5 {
		t.Fatalf("expected requests 5, got %d", summary.Requests)
	}
	if summary.EstimatedCostUSD != 0.0004 {
		t.Fatalf("expected estimated cost 0.0004, got %f", summary.EstimatedCostUSD)
	}
	if summary.BalanceUSD != 42.50 {
		t.Fatalf("expected balance 42.50, got %f", summary.BalanceUSD)
	}
	if summary.CreditLimitUSD != 100.00 {
		t.Fatalf("expected credit limit 100.00, got %f", summary.CreditLimitUSD)
	}
	if summary.CurrentSpendUSD != summary.EstimatedCostUSD {
		t.Fatalf("expected current spend to mirror estimated cost, got %f", summary.CurrentSpendUSD)
	}
	if summary.NextInvoice == nil || summary.NextInvoice.ID != "draft-2026-06" || summary.NextInvoice.Status != "draft" {
		t.Fatalf("expected draft next invoice, got %+v", summary.NextInvoice)
	}
}

func TestGetBillingAddsConfiguredPaymentProviders(t *testing.T) {
	store := &fakeStore{
		billing: BillingSummary{
			Period:           "30d",
			Requests:         5,
			InputTokens:      120,
			OutputTokens:     80,
			EstimatedCostUSD: 0.0004,
		},
	}
	service := NewService(store, WithBillingPaymentProviders([]BillingPaymentProviderSummary{
		{Name: " stripe "},
		{Name: "ALIPAY"},
		{Name: ""},
	}))

	summary, err := service.GetBilling(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}
	if len(summary.PaymentProviders) != 2 || summary.PaymentProviders[0].Name != "stripe" || summary.PaymentProviders[1].Name != "alipay" {
		t.Fatalf("expected normalized configured providers, got %+v", summary.PaymentProviders)
	}

	summary.PaymentProviders[0].Name = "mutated"
	reloaded, err := service.GetBilling(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("reload billing: %v", err)
	}
	if reloaded.PaymentProviders[0].Name != "stripe" {
		t.Fatalf("expected provider list to be cloned, got %+v", reloaded.PaymentProviders)
	}
}

func TestListBillingInvoicesReturnsOrganizationInvoices(t *testing.T) {
	store := &fakeStore{
		invoices: []BillingInvoiceSummary{{
			ID:               "inv_paid_1",
			Status:           "paid",
			AmountUSD:        29,
			DueAt:            "2026-05-31T00:00:00Z",
			HostedInvoiceURL: "https://billing.stripe.test/invoices/inv_paid_1",
			InvoicePDF:       "https://billing.stripe.test/invoices/inv_paid_1.pdf",
		}},
	}
	service := NewService(store)

	invoices, err := service.ListBillingInvoices(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("list billing invoices: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization id org_1, got %s", store.organizationID)
	}
	if len(invoices) != 1 || invoices[0].ID != "inv_paid_1" || invoices[0].Status != "paid" || invoices[0].AmountUSD != 29 {
		t.Fatalf("unexpected invoice summaries: %+v", invoices)
	}
	if invoices[0].HostedInvoiceURL != "https://billing.stripe.test/invoices/inv_paid_1" || invoices[0].InvoicePDF != "https://billing.stripe.test/invoices/inv_paid_1.pdf" {
		t.Fatalf("expected invoice document links to be preserved, got %+v", invoices[0])
	}
}

func TestGetAccessReturnsSessionAndPreferenceSummary(t *testing.T) {
	service := NewService(&fakeStore{})

	summary := service.GetAccess(
		auth.Session{
			ExpiresAt:   mustTime(t, "2026-04-03T00:00:00Z"),
			ID:          "session_1",
			WorkspaceID: "workspace_1",
			User: auth.User{
				Email: "user@example.com",
				ID:    "user_1",
			},
		},
		userprefs.Preferences{
			DefaultMode:         "chat",
			ModelStrategy:       "balanced",
			NetworkEnabledHint:  true,
			OnboardingCompleted: true,
		},
	)

	if summary.UserEmail != "user@example.com" {
		t.Fatalf("expected user email user@example.com, got %s", summary.UserEmail)
	}
	if summary.WorkspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", summary.WorkspaceID)
	}
	if summary.DefaultMode != "chat" {
		t.Fatalf("expected default mode chat, got %s", summary.DefaultMode)
	}
	if summary.SessionExpiresAt != "2026-04-03T00:00:00Z" {
		t.Fatalf("expected session expiry 2026-04-03T00:00:00Z, got %s", summary.SessionExpiresAt)
	}
}

func TestCreateAPITokenScopesTokenToSessionUserAndOrganization(t *testing.T) {
	tokenStore := &fakeAPITokenStore{
		created: relay.CreatedRelayAPIToken{
			RawToken: "obv_secret",
			Token: relay.RelayAPITokenListItem{
				ID:          "tok_1",
				Name:        "CI key",
				TokenPrefix: "obv_secr",
				Status:      relay.RelayAPITokenStatusActive,
			},
		},
	}
	service := NewServiceWithAPITokens(&fakeStore{}, tokenStore)

	created, err := service.CreateAPIToken(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}, CreateAPITokenRequest{
		Name:               "CI key",
		ModelLimitsEnabled: true,
		ModelLimits:        []string{"gpt-4o"},
		UserGroup:          " vip ",
	})
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}

	if tokenStore.input.UserID != "user_1" || tokenStore.input.OrganizationID != "org_1" {
		t.Fatalf("expected token scoped to session user/org, got %+v", tokenStore.input)
	}
	if tokenStore.input.Name != "CI key" || !tokenStore.input.ModelLimitsEnabled || len(tokenStore.input.ModelLimits) != 1 || tokenStore.input.ModelLimits[0] != "gpt-4o" {
		t.Fatalf("unexpected create input: %+v", tokenStore.input)
	}
	if tokenStore.input.UserGroup != "vip" {
		t.Fatalf("expected normalized token user group vip, got %q", tokenStore.input.UserGroup)
	}
	if created.RawToken != "obv_secret" || created.Token.ID != "tok_1" {
		t.Fatalf("unexpected created token: %+v", created)
	}
}

func TestCreateAPITokenRejectsInvalidQuotaLimit(t *testing.T) {
	service := NewServiceWithAPITokens(&fakeStore{}, &fakeAPITokenStore{})
	zeroQuota := 0.0

	_, err := service.CreateAPIToken(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}, CreateAPITokenRequest{
		Name:       "CI key",
		QuotaLimit: &zeroQuota,
	})

	if err == nil || err.Error() != "api token quota limit must be greater than zero" {
		t.Fatalf("expected quota limit validation error, got %v", err)
	}
}

func TestCreateAPITokenRejectsPastExpiry(t *testing.T) {
	service := NewServiceWithAPITokens(&fakeStore{}, &fakeAPITokenStore{})
	expiresAt := time.Now().Add(-time.Minute)

	_, err := service.CreateAPIToken(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}, CreateAPITokenRequest{
		Name:      "CI key",
		ExpiresAt: &expiresAt,
	})

	if err == nil || err.Error() != "api token expiration must be in the future" {
		t.Fatalf("expected expiration validation error, got %v", err)
	}
}

func TestListAPITokenUsageScopesToSessionUserAndOrganization(t *testing.T) {
	tokenStore := &fakeAPITokenStore{
		tokens: []relay.RelayAPITokenListItem{{
			ID: "tok_1",
		}},
		usage: []relay.RelayAPITokenUsageItem{{
			ID:          "usage_1",
			APITokenID:  "tok_1",
			RequestID:   "req_1",
			APIType:     "chat",
			Model:       "gpt-4o",
			ChannelID:   "ch_1",
			Provider:    "openai",
			Status:      "success",
			StatusCode:  200,
			LatencyMS:   42,
			Cost:        0.004,
			TotalTokens: 1100,
			CreatedAt:   mustTime(t, "2026-05-30T00:00:00Z"),
		}},
	}
	service := NewServiceWithAPITokens(&fakeStore{}, tokenStore)

	usage, err := service.ListAPITokenUsage(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}, "tok_1")
	if err != nil {
		t.Fatalf("list api token usage: %v", err)
	}

	if tokenStore.usageOrganizationID != "org_1" || tokenStore.usageUserID != "user_1" || tokenStore.usageTokenID != "tok_1" {
		t.Fatalf("usage query was not scoped to session user/org/token: %+v", tokenStore)
	}
	if len(usage) != 1 || usage[0].RequestID != "req_1" || usage[0].TotalTokens != 1100 {
		t.Fatalf("unexpected usage items: %+v", usage)
	}
}

func TestListAPITokenUsageRejectsTokensOutsideSessionScope(t *testing.T) {
	tokenStore := &fakeAPITokenStore{
		tokens: []relay.RelayAPITokenListItem{{
			ID: "tok_owned",
		}},
		usage: []relay.RelayAPITokenUsageItem{{
			ID:         "usage_external",
			APITokenID: "tok_external",
			RequestID:  "req_external",
		}},
	}
	service := NewServiceWithAPITokens(&fakeStore{}, tokenStore)

	_, err := service.ListAPITokenUsage(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}, "tok_external")

	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for token outside session scope, got %v", err)
	}
	if tokenStore.usageTokenID != "" {
		t.Fatalf("expected usage query to be skipped for external token, got token id %q", tokenStore.usageTokenID)
	}
}

type fakeAPITokenStore struct {
	input               relay.CreateRelayAPITokenInput
	created             relay.CreatedRelayAPIToken
	tokens              []relay.RelayAPITokenListItem
	listOrganizationID  string
	listUserID          string
	usage               []relay.RelayAPITokenUsageItem
	usageOrganizationID string
	usageUserID         string
	usageTokenID        string
}

func (s *fakeAPITokenStore) CreateRelayAPIToken(_ context.Context, input relay.CreateRelayAPITokenInput) (relay.CreatedRelayAPIToken, error) {
	s.input = input
	return s.created, nil
}

func (s *fakeAPITokenStore) ListRelayAPITokens(_ context.Context, organizationID, userID string) ([]relay.RelayAPITokenListItem, error) {
	s.listOrganizationID = organizationID
	s.listUserID = userID
	return s.tokens, nil
}

func (s *fakeAPITokenStore) RevokeRelayAPIToken(_ context.Context, _, _, _ string) error {
	return nil
}

func (s *fakeAPITokenStore) ListRelayAPITokenUsage(_ context.Context, organizationID, userID, tokenID string) ([]relay.RelayAPITokenUsageItem, error) {
	s.usageOrganizationID = organizationID
	s.usageUserID = userID
	s.usageTokenID = tokenID
	return s.usage, nil
}

func mustTime(t *testing.T, raw string) (parsedTime time.Time) {
	t.Helper()

	parsedTime, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}

	return parsedTime
}
