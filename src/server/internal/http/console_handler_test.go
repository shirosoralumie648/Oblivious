package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/console"
	"oblivious/server/internal/relay"
)

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

func (s *consoleHandlerFakeStore) ListRelayAPITokens(context.Context, string, string) ([]relay.RelayAPITokenListItem, error) {
	return nil, nil
}
