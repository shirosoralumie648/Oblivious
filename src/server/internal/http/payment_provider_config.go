package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	stripeapi "github.com/stripe/stripe-go/v83"

	"oblivious/server/internal/config"
	"oblivious/server/internal/console"
	"oblivious/server/internal/payment"
	stripebilling "oblivious/server/internal/stripe"
)

type hostedCheckoutCreator struct {
	provider string
	baseURL  string
}

var (
	hostedCheckoutSecretParameterNamePattern  = regexp.MustCompile(`(?i)^(?:.*[_-])?(?:token|secret|password|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)$`)
	hostedCheckoutSecretParameterValuePattern = regexp.MustCompile(`(?i)(?:\b|[_-])(?:token|secret|password|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)(?:\b|[_-])`)
)

func (c hostedCheckoutCreator) CreateCheckoutSession(_ context.Context, _ stripebilling.CheckoutConfig, req stripebilling.CheckoutSessionRequest) (*stripeapi.CheckoutSession, error) {
	base, err := url.Parse(strings.TrimSpace(c.baseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid %s checkout base url", c.provider)
	}
	provider := strings.ToLower(strings.TrimSpace(c.provider))
	if err := validateHostedCheckoutBaseURL(provider, base); err != nil {
		return nil, err
	}
	sessionID := hostedCheckoutSessionID(provider, req.PaymentIntentID)
	currency := strings.ToLower(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "cny"
	}
	query := base.Query()
	query.Set("provider", provider)
	query.Set("checkout_kind", firstNonEmptyString(req.CheckoutKind, "subscription"))
	query.Set("payment_intent_id", req.PaymentIntentID)
	query.Set("organization_id", req.OrganizationID)
	query.Set("user_id", req.UserID)
	query.Set("plan_id", req.PlanID)
	query.Set("plan_name", req.PlanName)
	query.Set("amount", fmt.Sprintf("%.2f", req.PlanPrice))
	query.Set("currency", currency)
	query.Set("session_id", sessionID)
	addHostedCheckoutMarketplaceMetadata(query, req)
	base.RawQuery = query.Encode()
	return &stripeapi.CheckoutSession{
		ID:  sessionID,
		URL: base.String(),
	}, nil
}

func validateHostedCheckoutBaseURL(provider string, base *url.URL) error {
	if !strings.EqualFold(base.Scheme, "https") {
		return fmt.Errorf("%s checkout base url must use https", provider)
	}
	if strings.TrimSpace(base.Host) == "" {
		return fmt.Errorf("%s checkout base url must include a host", provider)
	}
	if base.User != nil {
		return fmt.Errorf("%s checkout base url must not embed credentials", provider)
	}
	if hostedCheckoutRawURIValuesCarrySecret(base.RawQuery) || hostedCheckoutRawURIValuesCarrySecret(base.Fragment) {
		return fmt.Errorf("%s checkout base url must not embed secret-like query or fragment parameters", provider)
	}
	return nil
}

func hostedCheckoutRawURIValuesCarrySecret(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	values, err := url.ParseQuery(raw)
	if err == nil {
		for key, entries := range values {
			if hostedCheckoutDecodedParameterNameLooksSecret(key) {
				return true
			}
			for _, value := range entries {
				if hostedCheckoutDecodedParameterValueLooksSecret(value) {
					return true
				}
			}
		}
	}
	decodedRaw := hostedCheckoutRepeatedlyDecode(raw)
	return hostedCheckoutSecretParameterValuePattern.MatchString(decodedRaw)
}

func hostedCheckoutDecodedParameterNameLooksSecret(value string) bool {
	return hostedCheckoutSecretParameterNamePattern.MatchString(hostedCheckoutRepeatedlyDecode(value))
}

func hostedCheckoutDecodedParameterValueLooksSecret(value string) bool {
	return hostedCheckoutSecretParameterValuePattern.MatchString(hostedCheckoutRepeatedlyDecode(value))
}

func hostedCheckoutRepeatedlyDecode(value string) string {
	decoded := strings.TrimSpace(value)
	for range 3 {
		next, err := url.QueryUnescape(decoded)
		if err != nil || next == decoded {
			return decoded
		}
		decoded = next
	}
	return decoded
}

func addHostedCheckoutMarketplaceMetadata(query url.Values, req stripebilling.CheckoutSessionRequest) {
	for key, value := range map[string]string{
		"marketplace_order_id":      req.MarketplaceOrderID,
		"agent_id":                  req.AgentID,
		"version_id":                req.VersionID,
		"publisher_user_id":         req.PublisherUserID,
		"publisher_organization_id": req.PublisherOrganizationID,
	} {
		if strings.TrimSpace(value) != "" {
			query.Set(key, strings.TrimSpace(value))
		}
	}
}

func hostedCheckoutSessionID(provider string, paymentIntentID string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	paymentIntentID = strings.TrimSpace(paymentIntentID)
	if paymentIntentID == "" {
		return provider + "_checkout"
	}
	safeIntent := sanitizeCheckoutIDPart(paymentIntentID)
	if len(safeIntent) <= 48 {
		return provider + "_" + safeIntent
	}
	sum := sha256.Sum256([]byte(paymentIntentID))
	return provider + "_" + safeIntent[:32] + "_" + hex.EncodeToString(sum[:])[:12]
}

func sanitizeCheckoutIDPart(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "checkout"
	}
	return builder.String()
}

func buildPaymentCheckoutProviders(cfg config.Config, stripeCreator stripebilling.CheckoutCreator, providerRegistry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator) (*payment.Registry, map[string]stripebilling.CheckoutCreator) {
	if providerRegistry == nil {
		providerRegistry = payment.DefaultRegistry()
	}
	creators := map[string]stripebilling.CheckoutCreator{}
	if stripeCreator != nil {
		creators["stripe"] = stripeCreator
	}
	for providerName, creator := range checkoutCreators {
		normalized := strings.ToLower(strings.TrimSpace(providerName))
		if normalized == "" || creator == nil {
			continue
		}
		creators[normalized] = creator
	}
	registerHostedDomesticCheckout(providerRegistry, creators, "alipay", cfg.AlipayCheckoutBaseURL)
	registerHostedDomesticCheckout(providerRegistry, creators, "wechatpay", cfg.WeChatPayCheckoutBaseURL)
	return providerRegistry, creators
}

func consoleBillingPaymentProviders(registry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator) []console.BillingPaymentProviderSummary {
	if registry == nil {
		registry = payment.DefaultRegistry()
	}
	if len(checkoutCreators) == 0 {
		return []console.BillingPaymentProviderSummary{}
	}
	providers := registry.AvailableProviders()
	available := make([]console.BillingPaymentProviderSummary, 0, len(providers))
	for _, provider := range providers {
		if checkoutCreators[provider.Name] != nil {
			available = append(available, console.BillingPaymentProviderSummary{Name: provider.Name})
		}
	}
	return available
}

func registerHostedDomesticCheckout(registry *payment.Registry, creators map[string]stripebilling.CheckoutCreator, providerName string, baseURL string) {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	baseURL = strings.TrimSpace(baseURL)
	if providerName == "" || baseURL == "" {
		return
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return
	}
	if err := validateHostedCheckoutBaseURL(providerName, base); err != nil {
		return
	}
	if _, exists := creators[providerName]; !exists {
		creators[providerName] = hostedCheckoutCreator{provider: providerName, baseURL: baseURL}
	}
	registry.Register(payment.Provider{Name: providerName, Configured: true, Currency: "cny"})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
