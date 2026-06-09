package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
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

func TestMarketplacePaidInstallCheckoutUsesSelectedProviderAndReturnsCheckoutSession(t *testing.T) {
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
	settlement := &fakeMarketplaceSettlementService{
		order: &marketplace.MarketplaceOrder{
			ID:                      "order_alipay",
			BuyerOrganizationID:     "org_buyer",
			BuyerUserID:             "buyer_1",
			PublisherOrganizationID: "org_publisher",
			PublisherUserID:         "publisher_1",
			AgentID:                 "agent_paid",
			VersionID:               "version_paid_1",
			PaymentIntentID:         "pi_alipay_marketplace",
			GrossAmount:             25,
			Currency:                "usd",
		},
	}
	alipayCreator := &fakeCheckoutCreator{
		sessionID:  "alipay_marketplace_session",
		sessionURL: "https://checkout.alipay.test/marketplace/alipay_marketplace_session",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(
			settlement,
			nil,
			stripebilling.CheckoutConfig{},
			providerRegistry,
			map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		),
	)
	session := testAdminSession()
	session.User.ID = "buyer_1"
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/marketplace/agents/agent_paid/install",
		strings.NewReader(`{"versionID":"version_paid_1","provider":"alipay"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected paid install checkout 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if settlement.createCalls != 1 {
		t.Fatalf("expected one settlement checkout call, got %d", settlement.createCalls)
	}
	if settlement.request.Provider != "alipay" || settlement.request.VersionID != "version_paid_1" ||
		settlement.request.AgentID != "agent_paid" || settlement.request.BuyerOrganizationID != "org_buyer" ||
		settlement.request.BuyerUserID != "buyer_1" {
		t.Fatalf("unexpected paid install settlement request: %+v", settlement.request)
	}
	if alipayCreator.request.PaymentIntentID != "pi_alipay_marketplace" ||
		alipayCreator.request.CheckoutKind != "marketplace_install" ||
		alipayCreator.request.MarketplaceOrderID != "order_alipay" ||
		alipayCreator.request.AgentID != "agent_paid" ||
		alipayCreator.request.VersionID != "version_paid_1" ||
		alipayCreator.request.PublisherUserID != "publisher_1" ||
		alipayCreator.request.PublisherOrganizationID != "org_publisher" {
		t.Fatalf("checkout creator saw wrong marketplace metadata request: %+v", alipayCreator.request)
	}
	if settlement.sessionID != "alipay_marketplace_session" || settlement.sessionPaymentIntentID != "pi_alipay_marketplace" {
		t.Fatalf("expected checkout session to be recorded, got session=%q intent=%q", settlement.sessionID, settlement.sessionPaymentIntentID)
	}

	var response struct {
		Data struct {
			CheckoutSessionID string `json:"checkoutSessionId"`
			URL               string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.CheckoutSessionID != "alipay_marketplace_session" ||
		response.Data.URL != "https://checkout.alipay.test/marketplace/alipay_marketplace_session" {
		t.Fatalf("expected checkout session payload, got %+v", response.Data)
	}
}

func TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailedWithoutDatabase(t *testing.T) {
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
	settlement := &fakeMarketplaceSettlementService{
		order: &marketplace.MarketplaceOrder{
			ID:                      "order_failed_checkout",
			BuyerOrganizationID:     "org_buyer",
			BuyerUserID:             "buyer_1",
			PublisherOrganizationID: "org_publisher",
			PublisherUserID:         "publisher_1",
			AgentID:                 "agent_paid",
			VersionID:               "version_paid_1",
			PaymentIntentID:         "pi_failed_marketplace",
			GrossAmount:             25,
			Currency:                "usd",
		},
	}
	checkoutCreator := &fakeCheckoutCreator{err: errors.New("provider checkout unavailable")}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(settlement, checkoutCreator, stripebilling.CheckoutConfig{}, providerRegistry, nil),
	)
	session := testAdminSession()
	session.User.ID = "buyer_1"
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/marketplace/agents/agent_paid/install",
		strings.NewReader(`{"versionID":"version_paid_1","provider":"stripe"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusBadGateway {
		t.Fatalf("expected checkout failure to return 502, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode checkout failure response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "checkout_create_failed" {
		t.Fatalf("expected checkout_create_failed response, got %+v", response.Error)
	}
	if settlement.createCalls != 1 || settlement.failCalls != 1 || settlement.setSessionCalls != 0 {
		t.Fatalf("expected create and fail without recording checkout session, create=%d fail=%d setSession=%d", settlement.createCalls, settlement.failCalls, settlement.setSessionCalls)
	}
	if settlement.failedOrderID != "order_failed_checkout" ||
		settlement.failedPaymentIntentID != "pi_failed_marketplace" ||
		!strings.Contains(settlement.failureReason, "provider checkout unavailable") {
		t.Fatalf("expected failed settlement state for order/payment intent, got order=%q intent=%q reason=%q", settlement.failedOrderID, settlement.failedPaymentIntentID, settlement.failureReason)
	}
	if checkoutCreator.request.PaymentIntentID != "pi_failed_marketplace" ||
		checkoutCreator.request.CheckoutKind != "marketplace_install" ||
		checkoutCreator.request.MarketplaceOrderID != "order_failed_checkout" ||
		checkoutCreator.request.AgentID != "agent_paid" ||
		checkoutCreator.request.VersionID != "version_paid_1" ||
		checkoutCreator.request.PublisherUserID != "publisher_1" ||
		checkoutCreator.request.PublisherOrganizationID != "org_publisher" {
		t.Fatalf("checkout creator saw wrong marketplace metadata request: %+v", checkoutCreator.request)
	}
}

func TestMarketplacePaidInstallCheckoutRejectsMissingProviderBeforeSettlement(t *testing.T) {
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
	settlement := &fakeMarketplaceSettlementService{}
	checkoutCreator := &fakeCheckoutCreator{}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(settlement, checkoutCreator, stripebilling.CheckoutConfig{}, providerRegistry, nil),
	)
	session := testAdminSession()
	session.User.ID = "buyer_1"
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/marketplace/agents/agent_paid/install",
		strings.NewReader(`{"versionID":"version_paid_1"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected missing provider to return 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "invalid_provider" || !strings.Contains(response.Error.Message, "payment provider is required") {
		t.Fatalf("expected invalid_provider response, got %+v", response.Error)
	}
	if settlement.createCalls != 0 {
		t.Fatalf("settlement must not be called before provider evidence is supplied, got %d calls", settlement.createCalls)
	}
	if checkoutCreator.request.PaymentIntentID != "" {
		t.Fatalf("checkout creator must not be called before provider evidence is supplied, got %+v", checkoutCreator.request)
	}
}

func TestMarketplacePaidInstallCheckoutRejectsUnsupportedProviderBeforeSettlement(t *testing.T) {
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
	settlement := &fakeMarketplaceSettlementService{}
	checkoutCreator := &fakeCheckoutCreator{}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	handler := newMarketplaceHandler(
		marketplace.NewService(store, nil),
		nil,
		withMarketplaceCheckout(settlement, checkoutCreator, stripebilling.CheckoutConfig{}, providerRegistry, nil),
	)
	session := testAdminSession()
	session.User.ID = "buyer_1"
	session.OrganizationID = "org_buyer"

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/marketplace/agents/agent_paid/install",
		strings.NewReader(`{"versionID":"version_paid_1","provider":"paypal"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	recorder := httptest.NewRecorder()

	handler.installAgent(recorder, request, "agent_paid")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected unsupported provider to return 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != payment.CodeUnsupportedProvider || !strings.Contains(response.Error.Message, "payment provider is not supported") {
		t.Fatalf("expected unsupported_provider response, got %+v", response.Error)
	}
	if settlement.createCalls != 0 {
		t.Fatalf("settlement must not be called for unsupported provider, got %d calls", settlement.createCalls)
	}
	if checkoutCreator.request.PaymentIntentID != "" {
		t.Fatalf("checkout creator must not be called for unsupported provider, got %+v", checkoutCreator.request)
	}
}
