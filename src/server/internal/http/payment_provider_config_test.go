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

	for _, tc := range []struct {
		provider string
		host     string
		intentID string
	}{
		{provider: "alipay", host: "checkout.alipay.test", intentID: "pi_alipay_domestic"},
		{provider: "wechatpay", host: "checkout.wechatpay.test", intentID: "pi_wechatpay_domestic"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			session, err := creators[tc.provider].CreateCheckoutSession(context.Background(), stripebilling.CheckoutConfig{
				SuccessURL: "https://app.oblivious.test/billing/success",
				CancelURL:  "https://app.oblivious.test/billing/cancel",
			}, stripebilling.CheckoutSessionRequest{
				OrganizationID:          "org_domestic",
				UserID:                  "user_domestic",
				PaymentIntentID:         tc.intentID,
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
				t.Fatalf("create %s hosted checkout session: %v", tc.provider, err)
			}
			if !strings.HasPrefix(session.ID, tc.provider+"_"+tc.intentID) {
				t.Fatalf("expected %s session id to include provider and payment intent, got %q", tc.provider, session.ID)
			}
			parsedURL, err := url.Parse(session.URL)
			if err != nil {
				t.Fatalf("parse %s checkout URL: %v", tc.provider, err)
			}
			query := parsedURL.Query()
			if parsedURL.Scheme != "https" || parsedURL.Host != tc.host || parsedURL.Path != "/session" {
				t.Fatalf("expected configured %s checkout base URL, got %s", tc.provider, session.URL)
			}
			if query.Get("provider") != tc.provider || query.Get("currency") != "cny" || query.Get("amount") != "25.00" || query.Get("payment_intent_id") != tc.intentID {
				t.Fatalf("expected %s checkout query to carry provider/currency/amount/intent, got %s", tc.provider, session.URL)
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
					t.Fatalf("expected %s checkout query %s=%q, got %q in %s", tc.provider, key, want, got, session.URL)
				}
			}
		})
	}
}

func TestHostedCheckoutCreatorRejectsSecretBearingBaseURLs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		baseURL string
	}{
		{
			name:    "userinfo credentials",
			baseURL: "https://merchant:secret-password@checkout.alipay.test/session",
		},
		{
			name:    "secret query parameter",
			baseURL: "https://checkout.alipay.test/session?token=target-secret-token",
		},
		{
			name:    "secret fragment parameter",
			baseURL: "https://checkout.alipay.test/session#password=target-secret-password",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, err := hostedCheckoutCreator{provider: "alipay", baseURL: tc.baseURL}.CreateCheckoutSession(context.Background(), stripebilling.CheckoutConfig{}, stripebilling.CheckoutSessionRequest{
				OrganizationID:  "org_domestic",
				UserID:          "user_domestic",
				PaymentIntentID: "pi_domestic_secret_guard",
				PlanName:        "Quota top-up",
				PlanPrice:       25,
				Currency:        "cny",
			})
			if err == nil {
				t.Fatalf("expected secret-bearing hosted checkout base URL %q to fail closed, got session URL %q", tc.baseURL, session.URL)
			}
		})
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
