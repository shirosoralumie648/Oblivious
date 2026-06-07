package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/payment"
	stripebilling "oblivious/server/internal/stripe"
)

func TestMarketplaceAgentDetailExposesOnlyConfiguredPaymentProviders(t *testing.T) {
	store := &fakeMarketplaceStore{
		agent: &marketplace.PublishedAgent{
			ID:             "agent_paid",
			OrganizationID: "org_publisher",
			OwnerID:        "publisher_1",
			Name:           "Paid Agent",
			Status:         "approved",
			Visibility:     "public",
			PricingType:    "one_time",
			PricingAmount:  25,
		},
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: false})
	providerRegistry.Register(payment.Provider{Name: "wechatpay", Configured: false})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(&fakeMarketplaceSettlementService{}, &fakeCheckoutCreator{}, stripebilling.CheckoutConfig{}, providerRegistry, nil),
	)

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/agents/agent_paid", nil).
		WithContext(context.Background())
	recorder := httptest.NewRecorder()

	handler.getAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected agent detail 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			PaymentProviders []struct {
				Name string `json:"name"`
			} `json:"paymentProviders"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.PaymentProviders) != 1 || response.Data.PaymentProviders[0].Name != "stripe" {
		t.Fatalf("expected only configured stripe provider, got %+v", response.Data.PaymentProviders)
	}
}
