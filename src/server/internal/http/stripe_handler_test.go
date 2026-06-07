package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
	"oblivious/server/internal/config"
	"oblivious/server/internal/payment"

	stripebilling "oblivious/server/internal/stripe"
)

type fakeCheckoutCreator struct {
	database   *sql.DB
	sessionID  string
	sessionURL string
	request    stripebilling.CheckoutSessionRequest
}

func (f *fakeCheckoutCreator) CreateCheckoutSession(_ context.Context, _ stripebilling.CheckoutConfig, req stripebilling.CheckoutSessionRequest) (*stripeapi.CheckoutSession, error) {
	f.request = req
	if f.database != nil {
		var exists bool
		if err := f.database.QueryRow(`SELECT EXISTS(SELECT 1 FROM payment_intents WHERE id = $1 AND status = 'pending')`, req.PaymentIntentID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("query precreated payment intent: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("payment intent %s was not precreated", req.PaymentIntentID)
		}
	}
	sessionID := f.sessionID
	if sessionID == "" {
		sessionID = "cs_test_phase17"
	}
	sessionURL := f.sessionURL
	if sessionURL == "" {
		sessionURL = "https://checkout.stripe.test/cs_test_phase17"
	}
	return &stripeapi.CheckoutSession{
		ID:  sessionID,
		URL: sessionURL,
	}, nil
}

func TestStripeWebhookRouteRejectsInvalidSignature(t *testing.T) {
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_phase17"
	router := NewRouter(cfg, testDatabase(t))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(`{"id":"evt_bad"}`))
	request.Header.Set("Stripe-Signature", "bad-signature")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid signature to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestStripeWebhookRouteRecordsSignedEventOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_phase17"
	router := NewRouter(cfg, database)
	cookie, _, userID := registerHTTPUser(t, router, "stripe-webhook@example.com")
	_ = cookie

	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_phase17', 'Phase 17 Pro', 'Phase 17 plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert package: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ('pi_http_phase17', 'stripe', $1, $2, 'pkg_phase17', 'subscription', 29, 'usd', 'pending', '{}', NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert payment intent: %v", err)
	}
	signed := signedHTTPCheckoutCompletedPayload(cfg.StripeWebhookSecret, "evt_http_phase17", organizationID, userID)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
		request.Header.Set("Stripe-Signature", signed.Header)

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var count int
	var status, storedOrganizationID, storedUserID string
	if err := database.QueryRow(`
		SELECT COUNT(*), MAX(status), MAX(organization_id), MAX(user_id)
		FROM stripe_webhook_events
		WHERE event_id = 'evt_http_phase17'
	`).Scan(&count, &status, &storedOrganizationID, &storedUserID); err != nil {
		t.Fatalf("query webhook event: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one ledger row for duplicate event, got %d", count)
	}
	if status != "processed" || storedOrganizationID != organizationID || storedUserID != userID {
		t.Fatalf("expected processed tenant webhook event, got status=%s org=%s user=%s", status, storedOrganizationID, storedUserID)
	}
}

func TestDomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents(t *testing.T) {
	for _, tc := range []struct {
		provider    string
		path        string
		secret      string
		eventID     string
		configure   func(*config.Config)
		signatureTo func(string, string, []byte) string
	}{
		{
			provider: "alipay",
			path:     "/api/v1/billing/alipay/webhook",
			secret:   "alipay_webhook_secret",
			eventID:  "evt_alipay_paid",
			configure: func(cfg *config.Config) {
				cfg.AlipayWebhookSecret = "alipay_webhook_secret"
			},
			signatureTo: domesticWebhookSignature,
		},
		{
			provider: "wechatpay",
			path:     "/api/v1/billing/wechatpay/webhook",
			secret:   "wechatpay_webhook_secret",
			eventID:  "evt_wechat_paid",
			configure: func(cfg *config.Config) {
				cfg.WeChatPayWebhookSecret = "wechatpay_webhook_secret"
			},
			signatureTo: domesticWebhookSignature,
		},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			database := testDatabase(t)
			cfg := testConfig()
			tc.configure(&cfg)
			router := NewRouter(cfg, database)
			_, _, userID := registerHTTPUser(t, router, tc.provider+"-webhook@example.com")
			_, organizationID := queryHTTPUserScope(t, database, userID)
			if _, err := database.Exec(`
				INSERT INTO payment_intents (id, provider, organization_id, user_id, kind, amount, currency, status, metadata, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 'topup', 25, 'cny', 'pending', '{}', NOW(), NOW())
			`, "pi_"+tc.provider+"_webhook", tc.provider, organizationID, userID); err != nil {
				t.Fatalf("insert domestic payment intent: %v", err)
			}
			payload := []byte(fmt.Sprintf(`{
				"id": %q,
				"type": "checkout.paid",
				"organization_id": %q,
				"user_id": %q,
				"payment_intent_id": %q,
				"provider_payment_intent_id": %q
			}`, tc.eventID, organizationID, userID, "pi_"+tc.provider+"_webhook", "provider_"+tc.provider+"_paid"))

			badRecorder := httptest.NewRecorder()
			badRequest := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(payload)))
			badRequest.Header.Set("Oblivious-Payment-Timestamp", "1760000000")
			badRequest.Header.Set("Oblivious-Payment-Signature", "bad-signature")
			router.ServeHTTP(badRecorder, badRequest)
			if badRecorder.Code != http.StatusBadRequest {
				t.Fatalf("expected invalid %s signature to return 400, got %d with body %s", tc.provider, badRecorder.Code, badRecorder.Body.String())
			}

			timestamp := "1760000000"
			for i := 0; i < 2; i++ {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(payload)))
				request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
				request.Header.Set("Oblivious-Payment-Signature", tc.signatureTo(tc.secret, timestamp, payload))
				router.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					t.Fatalf("attempt %d expected signed %s webhook 200, got %d with body %s", i+1, tc.provider, recorder.Code, recorder.Body.String())
				}
			}

			var count int
			var storedProvider, status, storedOrganizationID, storedUserID, paymentIntentID string
			if err := database.QueryRow(`
				SELECT COUNT(*), MAX(provider), MAX(status), MAX(organization_id), MAX(user_id), MAX(payment_intent_id)
				FROM stripe_webhook_events
				WHERE event_id = $1
			`, tc.eventID).Scan(&count, &storedProvider, &status, &storedOrganizationID, &storedUserID, &paymentIntentID); err != nil {
				t.Fatalf("query domestic webhook event: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected one domestic webhook ledger row for duplicate event, got %d", count)
			}
			if storedProvider != tc.provider || status != "processed" || storedOrganizationID != organizationID || storedUserID != userID || paymentIntentID != "pi_"+tc.provider+"_webhook" {
				t.Fatalf("unexpected domestic webhook event provider=%s status=%s org=%s user=%s intent=%s", storedProvider, status, storedOrganizationID, storedUserID, paymentIntentID)
			}
		})
	}
}

func TestDomesticPaymentWebhookRouteAppliesTopupLifecycleOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_lifecycle_secret"
	router := NewRouter(cfg, database)
	_, _, userID := registerHTTPUser(t, router, "alipay-lifecycle@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ('pi_alipay_lifecycle', 'alipay', $1, $2, 'topup', 25, 'cny', 'pending', '{}', NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert domestic topup payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, created_at)
		VALUES ('topup_alipay_lifecycle', $1, $2, 25, 25, 'pending', 'pi_alipay_lifecycle', NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert domestic topup order: %v", err)
	}

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_lifecycle",
		"type": "checkout.paid",
		"organization_id": %q,
		"user_id": %q,
		"payment_intent_id": "pi_alipay_lifecycle",
		"provider_payment_intent_id": "trade_alipay_lifecycle",
		"provider_checkout_session_id": "alipay_session_lifecycle",
		"kind": "topup",
		"amount": 25,
		"currency": "cny"
	}`, organizationID, userID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, payload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected alipay lifecycle webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var paymentStatus, providerPaymentIntentID, topupStatus string
	var quotaBalance float64
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_payment_intent_id, '')
		FROM payment_intents
		WHERE id = 'pi_alipay_lifecycle'
	`).Scan(&paymentStatus, &providerPaymentIntentID); err != nil {
		t.Fatalf("query domestic payment intent lifecycle: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM topup_orders WHERE id = 'topup_alipay_lifecycle'`).Scan(&topupStatus); err != nil {
		t.Fatalf("query domestic topup lifecycle: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'`, organizationID).Scan(&quotaBalance); err != nil {
		t.Fatalf("query domestic topup quota balance: %v", err)
	}
	if paymentStatus != "completed" || providerPaymentIntentID != "trade_alipay_lifecycle" || topupStatus != "paid" || quotaBalance != 25 {
		t.Fatalf("expected one applied domestic topup lifecycle, got payment=%s provider_pi=%s topup=%s balance=%.2f", paymentStatus, providerPaymentIntentID, topupStatus, quotaBalance)
	}

	var transitionCount int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM billing_lifecycle_events
		WHERE provider_event_id = 'evt_alipay_lifecycle'
	`).Scan(&transitionCount); err != nil {
		t.Fatalf("query domestic lifecycle transition count: %v", err)
	}
	if transitionCount != 1 {
		t.Fatalf("expected one domestic lifecycle transition, got %d", transitionCount)
	}
}

func TestBillingCheckoutRequiresSession(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"packageId":"pkg_phase17"}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected checkout without session to return 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func domesticWebhookSignature(secret string, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func signedHTTPCheckoutCompletedPayload(secret string, eventID string, organizationID string, userID string) *webhook.SignedPayload {
	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_http_phase17",
				"object": "checkout.session",
				"metadata": {
					"organization_id": "` + organizationID + `",
					"user_id": "` + userID + `",
					"payment_intent_id": "pi_http_phase17",
					"plan_id": "pkg_phase17",
					"checkout_kind": "subscription"
				}
			}
		}
	}`)

	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
}

func signedHTTPMarketplaceCheckoutCompletedPayload(secret string, eventID string, metadata map[string]string, fields map[string]string) *webhook.SignedPayload {
	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "` + fields["id"] + `",
				"object": "checkout.session",
				"client_reference_id": "` + metadata["payment_intent_id"] + `",
				"payment_intent": "` + fields["payment_intent"] + `",
				"amount_total": ` + fields["amount_total"] + `,
				"currency": "` + fields["currency"] + `",
				"metadata": {
					"organization_id": "` + metadata["organization_id"] + `",
					"user_id": "` + metadata["user_id"] + `",
					"payment_intent_id": "` + metadata["payment_intent_id"] + `",
					"checkout_kind": "` + metadata["checkout_kind"] + `",
					"marketplace_order_id": "` + metadata["marketplace_order_id"] + `",
					"agent_id": "` + metadata["agent_id"] + `",
					"version_id": "` + metadata["version_id"] + `",
					"publisher_user_id": "` + metadata["publisher_user_id"] + `",
					"publisher_organization_id": "` + metadata["publisher_organization_id"] + `"
				}
			}
		}
	}`)

	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
}

func signedHTTPMarketplaceRefundPayload(secret string, eventID string, metadata map[string]string, fields map[string]string) *webhook.SignedPayload {
	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "refund.created",
		"data": {
			"object": {
				"id": "` + fields["id"] + `",
				"object": "refund",
				"payment_intent": "` + fields["payment_intent"] + `",
				"charge": "` + fields["charge"] + `",
				"amount": ` + fields["amount"] + `,
				"currency": "` + fields["currency"] + `",
				"status": "` + fields["status"] + `",
				"reason": "` + fields["reason"] + `",
				"metadata": {
					"organization_id": "` + metadata["organization_id"] + `",
					"user_id": "` + metadata["user_id"] + `",
					"payment_intent_id": "` + metadata["payment_intent_id"] + `",
					"checkout_kind": "` + metadata["checkout_kind"] + `"
				}
			}
		}
	}`)

	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
}

func TestBillingCheckoutPersistsTenantPaymentIntent(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_phase17', 'Phase 17 Pro', 'Phase 17 plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert package: %v", err)
	}

	cookie, csrfToken, userID := registerHTTPUser(t, router, "stripe-checkout@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"packageId":"pkg_phase17","kind":"subscription"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected checkout to return 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			CheckoutSessionID string `json:"checkoutSessionId"`
			URL               string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode checkout response: %v", err)
	}
	if response.Data.CheckoutSessionID != "cs_test_phase17" {
		t.Fatalf("expected checkout session id cs_test_phase17, got %s", response.Data.CheckoutSessionID)
	}
	if fakeCreator.request.OrganizationID != organizationID || fakeCreator.request.UserID != userID || fakeCreator.request.PlanID != "pkg_phase17" {
		t.Fatalf("checkout creator saw wrong metadata: %+v", fakeCreator.request)
	}

	var storedID, storedOrganizationID, storedUserID, storedPackageID, providerSessionID, status string
	var amount float64
	if err := database.QueryRow(`
		SELECT id, organization_id, user_id, package_id, provider_checkout_session_id, amount, status
		FROM payment_intents
		WHERE provider_checkout_session_id = 'cs_test_phase17'
	`).Scan(&storedID, &storedOrganizationID, &storedUserID, &storedPackageID, &providerSessionID, &amount, &status); err != nil {
		t.Fatalf("query payment intent: %v", err)
	}
	if storedID != fakeCreator.request.PaymentIntentID {
		t.Fatalf("stored payment intent id %s does not match checkout metadata %s", storedID, fakeCreator.request.PaymentIntentID)
	}
	if storedOrganizationID != organizationID || storedUserID != userID || storedPackageID != "pkg_phase17" {
		t.Fatalf("stored wrong tenant payment intent: org=%s user=%s package=%s", storedOrganizationID, storedUserID, storedPackageID)
	}
	if providerSessionID != "cs_test_phase17" || amount != 29 || status != "pending" {
		t.Fatalf("stored wrong checkout state: session=%s amount=%.2f status=%s", providerSessionID, amount, status)
	}
}

func TestBillingCheckoutExplicitStripeUsesExistingCheckout(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_provider_stripe', 'Provider Stripe Pro', 'Provider regression plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert package: %v", err)
	}

	cookie, csrfToken, userID := registerHTTPUser(t, router, "stripe-explicit-provider@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"stripe","packageId":"pkg_provider_stripe","kind":"subscription"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected explicit stripe checkout to return 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if fakeCreator.request.OrganizationID != organizationID || fakeCreator.request.UserID != userID || fakeCreator.request.PlanID != "pkg_provider_stripe" {
		t.Fatalf("checkout creator saw wrong explicit stripe metadata: %+v", fakeCreator.request)
	}

	var provider string
	if err := database.QueryRow(`SELECT provider FROM payment_intents WHERE provider_checkout_session_id = 'cs_test_phase17'`).Scan(&provider); err != nil {
		t.Fatalf("query explicit stripe payment intent: %v", err)
	}
	if provider != "stripe" {
		t.Fatalf("expected explicit stripe payment intent provider stripe, got %q", provider)
	}
}

func TestBillingCheckoutUnconfiguredProvidersDoNotCreateArtifacts(t *testing.T) {
	for _, provider := range []string{"alipay", "wechatpay"} {
		t.Run(provider, func(t *testing.T) {
			database := testDatabase(t)
			fakeCreator := &fakeCheckoutCreator{database: database}
			cfg := testConfig()
			cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
			cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
			router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

			cookie, csrfToken, userID := registerHTTPUser(t, router, "checkout-"+provider+"@example.com")
			var organizationID string
			if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
				t.Fatalf("query session organization: %v", err)
			}

			recorder := httptest.NewRecorder()
			body := fmt.Sprintf(`{"provider":%q,"kind":"topup","amount":25}`, provider)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)
			addCSRF(request, csrfToken)

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNotImplemented {
				t.Fatalf("expected unconfigured %s checkout to return 501, got %d with body %s", provider, recorder.Code, recorder.Body.String())
			}
			var response Envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode unconfigured provider response: %v", err)
			}
			if response.Error == nil || response.Error.Code != "provider_not_configured" {
				t.Fatalf("expected provider_not_configured response, got %+v", response.Error)
			}
			if fakeCreator.request.PaymentIntentID != "" {
				t.Fatalf("checkout creator should not be called for %s, got %+v", provider, fakeCreator.request)
			}

			var intentCount, topupCount, marketplaceOrderCount int
			if err := database.QueryRow(`SELECT COUNT(*) FROM payment_intents WHERE organization_id = $1 AND user_id = $2`, organizationID, userID).Scan(&intentCount); err != nil {
				t.Fatalf("query payment intent count: %v", err)
			}
			if err := database.QueryRow(`SELECT COUNT(*) FROM topup_orders WHERE organization_id = $1 AND user_id = $2`, organizationID, userID).Scan(&topupCount); err != nil {
				t.Fatalf("query topup order count: %v", err)
			}
			if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_orders WHERE buyer_organization_id = $1 AND buyer_user_id = $2`, organizationID, userID).Scan(&marketplaceOrderCount); err != nil {
				t.Fatalf("query marketplace order count: %v", err)
			}
			if intentCount != 0 || topupCount != 0 || marketplaceOrderCount != 0 {
				t.Fatalf("expected no artifacts for %s, got intents=%d topups=%d marketplace_orders=%d", provider, intentCount, topupCount, marketplaceOrderCount)
			}
		})
	}
}

func TestBillingCheckoutUsesConfiguredProviderCheckoutCreator(t *testing.T) {
	database := testDatabase(t)
	stripeCreator := &fakeCheckoutCreator{database: database}
	alipayCreator := &fakeCheckoutCreator{
		database:   database,
		sessionID:  "alipay_session_phase20",
		sessionURL: "https://checkout.alipay.test/alipay_session_phase20",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true})

	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{
		CheckoutCreator:         stripeCreator,
		CheckoutCreators:        map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		PaymentProviderRegistry: providerRegistry,
	})

	cookie, csrfToken, userID := registerHTTPUser(t, router, "alipay-checkout-provider@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"alipay","kind":"topup","amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected configured alipay checkout to return 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if stripeCreator.request.PaymentIntentID != "" {
		t.Fatalf("stripe checkout creator must not be used for alipay, got %+v", stripeCreator.request)
	}
	if alipayCreator.request.PaymentIntentID == "" || alipayCreator.request.CheckoutKind != "topup" || alipayCreator.request.PlanPrice != 25 {
		t.Fatalf("alipay checkout creator saw wrong request: %+v", alipayCreator.request)
	}

	var storedProvider, storedOrganizationID, storedUserID string
	if err := database.QueryRow(`
		SELECT provider, organization_id, user_id
		FROM payment_intents
		WHERE provider_checkout_session_id = 'alipay_session_phase20'
	`).Scan(&storedProvider, &storedOrganizationID, &storedUserID); err != nil {
		t.Fatalf("query alipay payment intent: %v", err)
	}
	if storedProvider != "alipay" || storedOrganizationID != organizationID || storedUserID != userID {
		t.Fatalf("expected alipay tenant payment intent, got provider=%s org=%s user=%s", storedProvider, storedOrganizationID, storedUserID)
	}
}

func TestBillingCheckoutUsesConfiguredDomesticProviderFromRouterConfig(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	cfg.AlipayCheckoutBaseURL = "https://checkout.alipay.test/session"
	router := NewRouter(cfg, database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "alipay-router-config@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"alipay","kind":"topup","amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected configured alipay checkout to return 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			CheckoutSessionID string `json:"checkoutSessionId"`
			URL               string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode alipay router checkout response: %v", err)
	}
	if response.Data.CheckoutSessionID == "" || !strings.HasPrefix(response.Data.CheckoutSessionID, "alipay_") {
		t.Fatalf("expected alipay checkout session id, got %q", response.Data.CheckoutSessionID)
	}
	if !strings.HasPrefix(response.Data.URL, "https://checkout.alipay.test/session?") {
		t.Fatalf("expected configured alipay checkout URL, got %q", response.Data.URL)
	}

	var storedProvider, storedCurrency, providerSessionID string
	if err := database.QueryRow(`
		SELECT provider, currency, provider_checkout_session_id
		FROM payment_intents
		WHERE organization_id = $1 AND user_id = $2 AND kind = 'topup'
	`, organizationID, userID).Scan(&storedProvider, &storedCurrency, &providerSessionID); err != nil {
		t.Fatalf("query configured alipay payment intent: %v", err)
	}
	if storedProvider != "alipay" || storedCurrency != "cny" || providerSessionID != response.Data.CheckoutSessionID {
		t.Fatalf("expected alipay/cny payment intent with checkout session, got provider=%s currency=%s session=%s response=%s", storedProvider, storedCurrency, providerSessionID, response.Data.CheckoutSessionID)
	}
}

func TestMarketplacePaidInstallDoesNotInstallBeforeWebhook(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-paid-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	publisherCookie, publisherCSRF, publisherUserID := registerHTTPUser(t, router, "marketplace-paid-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)
	_ = publisherCookie
	_ = publisherCSRF

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_http', $1, $2, 'Paid HTTP Agent', 'Paid marketplace route test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_http', 'agent_paid_http', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid marketplace version: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_http/install?versionID=version_agent_paid_http", nil)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("paid install expected checkout 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if fakeCreator.request.CheckoutKind != "marketplace_install" || fakeCreator.request.AgentID != "agent_paid_http" || fakeCreator.request.MarketplaceOrderID == "" {
		t.Fatalf("checkout creator saw wrong marketplace metadata: %+v", fakeCreator.request)
	}
	if fakeCreator.request.OrganizationID != buyerOrganizationID || fakeCreator.request.UserID != buyerUserID || fakeCreator.request.PlanPrice != 50 {
		t.Fatalf("checkout creator saw wrong buyer/amount: %+v buyerOrg=%s buyerUser=%s", fakeCreator.request, buyerOrganizationID, buyerUserID)
	}

	var installCount, orderCount, intentCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid_http'`).Scan(&installCount); err != nil {
		t.Fatalf("count paid installs: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_orders WHERE agent_id = 'agent_paid_http' AND status = 'pending_payment'`).Scan(&orderCount); err != nil {
		t.Fatalf("count marketplace orders: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM payment_intents WHERE kind = 'marketplace_install' AND organization_id = $1`, buyerOrganizationID).Scan(&intentCount); err != nil {
		t.Fatalf("count marketplace payment intents: %v", err)
	}
	if installCount != 0 || orderCount != 1 || intentCount != 1 {
		t.Fatalf("expected no install and one pending order/intent, got installs=%d orders=%d intents=%d", installCount, orderCount, intentCount)
	}
}

func TestMarketplacePaidInstallUsesConfiguredProviderCheckoutCreator(t *testing.T) {
	database := testDatabase(t)
	stripeCreator := &fakeCheckoutCreator{database: database}
	alipayCreator := &fakeCheckoutCreator{
		database:   database,
		sessionID:  "alipay_marketplace_session",
		sessionURL: "https://checkout.alipay.test/marketplace/alipay_marketplace_session",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true})

	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{
		CheckoutCreator:         stripeCreator,
		CheckoutCreators:        map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		PaymentProviderRegistry: providerRegistry,
	})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-alipay-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-alipay-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_alipay', $1, $2, 'Paid Alipay Agent', 'Paid marketplace provider routing test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_alipay', 'agent_paid_alipay', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay marketplace version: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_alipay/install?versionID=version_agent_paid_alipay&provider=alipay", nil)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("paid alipay install expected checkout 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if stripeCreator.request.PaymentIntentID != "" {
		t.Fatalf("stripe checkout creator must not be used for alipay marketplace install, got %+v", stripeCreator.request)
	}
	if alipayCreator.request.CheckoutKind != "marketplace_install" || alipayCreator.request.AgentID != "agent_paid_alipay" || alipayCreator.request.MarketplaceOrderID == "" {
		t.Fatalf("alipay checkout creator saw wrong marketplace metadata: %+v", alipayCreator.request)
	}

	var storedProvider string
	if err := database.QueryRow(`
		SELECT provider
		FROM payment_intents
		WHERE id = $1 AND kind = 'marketplace_install' AND organization_id = $2
	`, alipayCreator.request.PaymentIntentID, buyerOrganizationID).Scan(&storedProvider); err != nil {
		t.Fatalf("query alipay marketplace payment intent: %v", err)
	}
	if storedProvider != "alipay" {
		t.Fatalf("expected alipay marketplace payment intent, got provider %q", storedProvider)
	}
}

func TestStripeWebhookRouteAppliesMarketplaceInstallSettlementOnce(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_marketplace"
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-webhook-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-webhook-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_webhook', $1, $2, 'Paid Webhook Agent', 'Paid marketplace webhook test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid webhook agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_webhook', 'agent_paid_webhook', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid webhook version: %v", err)
	}

	installRecorder := httptest.NewRecorder()
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_webhook/install?versionID=version_agent_paid_webhook", nil)
	installRequest.AddCookie(cookie)
	addCSRF(installRequest, csrfToken)
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != http.StatusCreated {
		t.Fatalf("paid install checkout expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}

	signed := signedHTTPMarketplaceCheckoutCompletedPayload(cfg.StripeWebhookSecret, "evt_marketplace_install", map[string]string{
		"organization_id":           buyerOrganizationID,
		"user_id":                   buyerUserID,
		"payment_intent_id":         fakeCreator.request.PaymentIntentID,
		"checkout_kind":             "marketplace_install",
		"marketplace_order_id":      fakeCreator.request.MarketplaceOrderID,
		"agent_id":                  fakeCreator.request.AgentID,
		"version_id":                fakeCreator.request.VersionID,
		"publisher_user_id":         publisherUserID,
		"publisher_organization_id": publisherOrganizationID,
	}, map[string]string{
		"id":             "cs_marketplace_install",
		"payment_intent": "pi_marketplace_install",
		"amount_total":   "5000",
		"currency":       "usd",
	})
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
		request.Header.Set("Stripe-Signature", signed.Header)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var orderStatus string
	var installCount, settlementCount, transitionCount int
	if err := database.QueryRow(`SELECT status FROM marketplace_orders WHERE id = $1`, fakeCreator.request.MarketplaceOrderID).Scan(&orderStatus); err != nil {
		t.Fatalf("query marketplace order: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid_webhook' AND organization_id = $1`, buyerOrganizationID).Scan(&installCount); err != nil {
		t.Fatalf("count marketplace install: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_settlements WHERE order_id = $1`, fakeCreator.request.MarketplaceOrderID).Scan(&settlementCount); err != nil {
		t.Fatalf("count marketplace settlements: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_lifecycle_events WHERE provider_event_id = 'evt_marketplace_install'`).Scan(&transitionCount); err != nil {
		t.Fatalf("count marketplace lifecycle transitions: %v", err)
	}
	if orderStatus != "paid" || installCount != 1 || settlementCount != 1 || transitionCount != 1 {
		t.Fatalf("expected paid order, one install, one settlement, one transition; got status=%s installs=%d settlements=%d transitions=%d", orderStatus, installCount, settlementCount, transitionCount)
	}
}

func TestStripeRefundUpdatesMarketplaceSettlementOnce(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_marketplace_refund"
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-refund-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-refund-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_refund', $1, $2, 'Paid Refund Agent', 'Paid marketplace refund test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid refund agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_refund', 'agent_paid_refund', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid refund version: %v", err)
	}

	installRecorder := httptest.NewRecorder()
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_refund/install?versionID=version_agent_paid_refund", nil)
	installRequest.AddCookie(cookie)
	addCSRF(installRequest, csrfToken)
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != http.StatusCreated {
		t.Fatalf("paid install checkout expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}

	checkout := signedHTTPMarketplaceCheckoutCompletedPayload(cfg.StripeWebhookSecret, "evt_marketplace_refund_checkout", map[string]string{
		"organization_id":           buyerOrganizationID,
		"user_id":                   buyerUserID,
		"payment_intent_id":         fakeCreator.request.PaymentIntentID,
		"checkout_kind":             "marketplace_install",
		"marketplace_order_id":      fakeCreator.request.MarketplaceOrderID,
		"agent_id":                  fakeCreator.request.AgentID,
		"version_id":                fakeCreator.request.VersionID,
		"publisher_user_id":         publisherUserID,
		"publisher_organization_id": publisherOrganizationID,
	}, map[string]string{
		"id":             "cs_marketplace_refund",
		"payment_intent": "pi_marketplace_refund",
		"amount_total":   "5000",
		"currency":       "usd",
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(checkout.Payload)))
	request.Header.Set("Stripe-Signature", checkout.Header)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("checkout webhook expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	refund := signedHTTPMarketplaceRefundPayload(cfg.StripeWebhookSecret, "evt_marketplace_refund", map[string]string{
		"organization_id":   buyerOrganizationID,
		"user_id":           buyerUserID,
		"payment_intent_id": fakeCreator.request.PaymentIntentID,
		"checkout_kind":     "marketplace_install",
	}, map[string]string{
		"id":             "re_marketplace_refund",
		"payment_intent": "pi_marketplace_refund",
		"charge":         "ch_marketplace_refund",
		"amount":         "1000",
		"currency":       "usd",
		"status":         "succeeded",
		"reason":         "requested_by_customer",
	})
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(refund.Payload)))
		request.Header.Set("Stripe-Signature", refund.Header)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("refund attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var orderStatus, settlementStatus string
	var orderRefunded, settlementRefunded float64
	var refundCount int
	if err := database.QueryRow(`
		SELECT status, refunded_amount FROM marketplace_orders WHERE id = $1
	`, fakeCreator.request.MarketplaceOrderID).Scan(&orderStatus, &orderRefunded); err != nil {
		t.Fatalf("query refunded marketplace order: %v", err)
	}
	if err := database.QueryRow(`
		SELECT status, refunded_amount FROM marketplace_settlements WHERE order_id = $1
	`, fakeCreator.request.MarketplaceOrderID).Scan(&settlementStatus, &settlementRefunded); err != nil {
		t.Fatalf("query refunded marketplace settlement: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_refunds WHERE provider_refund_id = 're_marketplace_refund'`).Scan(&refundCount); err != nil {
		t.Fatalf("query marketplace billing refunds: %v", err)
	}
	if orderStatus != "partially_refunded" || settlementStatus != "partially_refunded" || orderRefunded != 10 || settlementRefunded != 10 || refundCount != 1 {
		t.Fatalf("expected one partial marketplace refund, got order=%s %.2f settlement=%s %.2f refunds=%d", orderStatus, orderRefunded, settlementStatus, settlementRefunded, refundCount)
	}
}

func TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_phase18"
	router := NewRouter(cfg, database)

	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_phase18_sub', 'Phase 18 Pro', 'Phase 18 plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert package: %v", err)
	}
	_, _, userID := registerHTTPUser(t, router, "stripe-lifecycle-sub@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ('pi_http_lifecycle_sub', 'stripe', $1, $2, 'pkg_phase18_sub', 'subscription', 29, 'usd', 'pending', '{}', NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert payment intent: %v", err)
	}

	signed := signedHTTPCheckoutCompletedLifecyclePayload(cfg.StripeWebhookSecret, "evt_http_phase18_sub", organizationID, userID, "pi_http_lifecycle_sub", "pkg_phase18_sub", "subscription")
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
		request.Header.Set("Stripe-Signature", signed.Header)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var subscriptionCount, transitionCount int
	var paymentStatus string
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM subscriptions
		WHERE organization_id = $1 AND user_id = $2 AND package_id = 'pkg_phase18_sub' AND status = 'active'
	`, organizationID, userID).Scan(&subscriptionCount); err != nil {
		t.Fatalf("query subscription count: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM payment_intents WHERE id = 'pi_http_lifecycle_sub'`).Scan(&paymentStatus); err != nil {
		t.Fatalf("query payment status: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM billing_lifecycle_events
		WHERE transition_key = 'stripe:evt_http_phase18_sub:checkout:pi_http_lifecycle_sub'
	`).Scan(&transitionCount); err != nil {
		t.Fatalf("query transition count: %v", err)
	}
	if subscriptionCount != 1 || paymentStatus != "completed" || transitionCount != 1 {
		t.Fatalf("expected one applied subscription lifecycle, got subscriptions=%d payment=%s transitions=%d", subscriptionCount, paymentStatus, transitionCount)
	}
}

func TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.StripeWebhookSecret = "whsec_phase18_retry"
	router := NewRouter(cfg, database)

	_, _, userID := registerHTTPUser(t, router, "stripe-lifecycle-retry@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ('pi_http_lifecycle_retry', 'stripe', $1, $2, NULL, 'subscription', 29, 'usd', 'pending', '{}', NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert payment intent: %v", err)
	}

	signed := signedHTTPCheckoutCompletedLifecyclePayload(cfg.StripeWebhookSecret, "evt_http_phase18_retry", organizationID, userID, "pi_http_lifecycle_retry", "pkg_phase18_retry", "subscription")
	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
	firstRequest.Header.Set("Stripe-Signature", signed.Header)
	router.ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected first lifecycle attempt to fail after ledger insert, got %d with body %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	var ledgerCount, transitionCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM stripe_webhook_events WHERE event_id = 'evt_http_phase18_retry'`).Scan(&ledgerCount); err != nil {
		t.Fatalf("query retry ledger count: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_lifecycle_events WHERE provider_event_id = 'evt_http_phase18_retry'`).Scan(&transitionCount); err != nil {
		t.Fatalf("query retry transition count: %v", err)
	}
	if ledgerCount != 1 || transitionCount != 0 {
		t.Fatalf("expected recorded webhook without lifecycle transition after first failure, got ledger=%d transitions=%d", ledgerCount, transitionCount)
	}

	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_phase18_retry', 'Phase 18 Retry Pro', 'Retry plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert retry package: %v", err)
	}
	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
	secondRequest.Header.Set("Stripe-Signature", signed.Header)
	router.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate retry to apply lifecycle, got %d with body %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	var subscriptionCount int
	var paymentStatus string
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM subscriptions
		WHERE organization_id = $1 AND user_id = $2 AND package_id = 'pkg_phase18_retry' AND status = 'active'
	`, organizationID, userID).Scan(&subscriptionCount); err != nil {
		t.Fatalf("query retry subscription count: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM payment_intents WHERE id = 'pi_http_lifecycle_retry'`).Scan(&paymentStatus); err != nil {
		t.Fatalf("query retry payment status: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_lifecycle_events WHERE provider_event_id = 'evt_http_phase18_retry'`).Scan(&transitionCount); err != nil {
		t.Fatalf("query retry final transition count: %v", err)
	}
	if subscriptionCount != 1 || paymentStatus != "completed" || transitionCount != 1 {
		t.Fatalf("expected duplicate retry to complete lifecycle once, got subscriptions=%d payment=%s transitions=%d", subscriptionCount, paymentStatus, transitionCount)
	}
}

func TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, userID := registerHTTPUser(t, router, "stripe-topup-checkout@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"kind":"topup","amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected topup checkout to return 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var intentCount, topupCount int
	var balance float64
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM payment_intents
		WHERE organization_id = $1 AND user_id = $2 AND kind = 'topup' AND status = 'pending'
	`, organizationID, userID).Scan(&intentCount); err != nil {
		t.Fatalf("query topup intent count: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM topup_orders
		WHERE organization_id = $1 AND user_id = $2 AND status = 'pending' AND amount = 25
	`, organizationID, userID).Scan(&topupCount); err != nil {
		t.Fatalf("query topup order count: %v", err)
	}
	if err := database.QueryRow(`SELECT COALESCE((SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'), 0)`, organizationID).Scan(&balance); err != nil {
		t.Fatalf("query topup quota balance: %v", err)
	}
	if intentCount != 1 || topupCount != 1 || balance != 0 {
		t.Fatalf("expected pending paid topup artifacts without quota credit, got intents=%d topups=%d balance=%.2f", intentCount, topupCount, balance)
	}
}

func TestQuotaTopupEndpointNoLongerCreditsWithoutPayment(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "direct-topup-disabled@example.com")
	var organizationID string
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&organizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/app/quota/topup", strings.NewReader(`{"amount":5}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("expected direct topup to require payment, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var balance float64
	if err := database.QueryRow(`SELECT COALESCE((SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'), 0)`, organizationID).Scan(&balance); err != nil {
		t.Fatalf("query direct topup balance: %v", err)
	}
	if balance != 0 {
		t.Fatalf("direct topup must not credit quota without payment, got balance %.2f", balance)
	}
}

func signedHTTPCheckoutCompletedLifecyclePayload(secret string, eventID string, organizationID string, userID string, paymentIntentID string, planID string, checkoutKind string) *webhook.SignedPayload {
	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_http_phase18",
				"object": "checkout.session",
				"payment_intent": "pi_provider_http_phase18",
				"subscription": "sub_provider_http_phase18",
				"customer": "cus_http_phase18",
				"amount_total": 2900,
				"currency": "usd",
				"metadata": {
					"organization_id": "` + organizationID + `",
					"user_id": "` + userID + `",
					"payment_intent_id": "` + paymentIntentID + `",
					"plan_id": "` + planID + `",
					"checkout_kind": "` + checkoutKind + `"
				}
			}
		}
	}`)

	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
}
