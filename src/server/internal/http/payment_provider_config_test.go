package http

import (
	"context"
	"net/url"
	"strings"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"oblivious/server/internal/config"
	"oblivious/server/internal/payment"
	stripebilling "oblivious/server/internal/stripe"
)

func TestBuildPaymentCheckoutProvidersEnablesDomesticHostedProviders(t *testing.T) {
	stripeCreator := stripebilling.CheckoutCreatorFunc(func(context.Context, stripebilling.CheckoutConfig, stripebilling.CheckoutSessionRequest) (*stripeapi.CheckoutSession, error) {
		return nil, nil
	})

	registry, creators := buildPaymentCheckoutProviders(config.Config{
		AlipayCheckoutBaseURL:    " https://checkout.alipay.test/session ",
		WeChatPayCheckoutBaseURL: " https://checkout.wechatpay.test/session ",
	}, stripeCreator, nil, nil)

	for _, providerName := range []string{"alipay", "wechatpay"} {
		provider, err := registry.Resolve(providerName)
		if err != nil {
			t.Fatalf("expected %s to resolve when checkout URL is configured: %v", providerName, err)
		}
		if provider.Currency != "cny" {
			t.Fatalf("expected %s currency cny, got %q", providerName, provider.Currency)
		}
		if creators[providerName] == nil {
			t.Fatalf("expected %s checkout creator to be configured", providerName)
		}
	}

	session, err := creators["alipay"].CreateCheckoutSession(context.Background(), stripebilling.CheckoutConfig{
		SuccessURL: "https://app.oblivious.test/billing/success",
		CancelURL:  "https://app.oblivious.test/billing/cancel",
	}, stripebilling.CheckoutSessionRequest{
		OrganizationID:          "org_domestic",
		UserID:                  "user_domestic",
		PaymentIntentID:         "pi_domestic",
		PlanName:                "Quota top-up",
		PlanPrice:               25,
		Currency:                "cny",
		CheckoutKind:            "marketplace_install",
		MarketplaceOrderID:      "order_domestic",
		AgentID:                 "agent_paid",
		VersionID:               "version_paid_1",
		PublisherUserID:         "publisher_1",
		PublisherOrganizationID: "org_publisher",
	})
	if err != nil {
		t.Fatalf("create alipay hosted checkout session: %v", err)
	}
	if !strings.HasPrefix(session.ID, "alipay_pi_domestic") {
		t.Fatalf("expected alipay session id to include provider and payment intent, got %q", session.ID)
	}
	parsedURL, err := url.Parse(session.URL)
	if err != nil {
		t.Fatalf("parse alipay checkout URL: %v", err)
	}
	query := parsedURL.Query()
	if parsedURL.Scheme != "https" || parsedURL.Host != "checkout.alipay.test" || parsedURL.Path != "/session" {
		t.Fatalf("expected configured alipay checkout base URL, got %s", session.URL)
	}
	if query.Get("provider") != "alipay" || query.Get("currency") != "cny" || query.Get("amount") != "25.00" || query.Get("payment_intent_id") != "pi_domestic" {
		t.Fatalf("expected alipay checkout query to carry provider/currency/amount/intent, got %s", session.URL)
	}
	for key, want := range map[string]string{
		"checkout_kind":             "marketplace_install",
		"marketplace_order_id":      "order_domestic",
		"agent_id":                  "agent_paid",
		"version_id":                "version_paid_1",
		"publisher_user_id":         "publisher_1",
		"publisher_organization_id": "org_publisher",
	} {
		if got := query.Get(key); got != want {
			t.Fatalf("expected alipay checkout query %s=%q, got %q in %s", key, want, got, session.URL)
		}
	}
}

func TestConsoleBillingPaymentProvidersExposeConfiguredCheckoutProviders(t *testing.T) {
	registry := payment.NewRegistry("stripe")
	registry.Register(payment.Provider{Name: "stripe", Configured: true, Currency: "usd"})
	registry.Register(payment.Provider{Name: "alipay", Configured: true, Currency: "cny"})
	registry.Register(payment.Provider{Name: "wechatpay", Configured: false, Currency: "cny"})

	providers := consoleBillingPaymentProviders(registry, map[string]stripebilling.CheckoutCreator{
		"stripe": hostedCheckoutCreator{provider: "stripe", baseURL: "https://checkout.stripe.test/session"},
	})

	if len(providers) != 1 || providers[0].Name != "stripe" {
		t.Fatalf("expected only configured providers with checkout creators, got %+v", providers)
	}
}
