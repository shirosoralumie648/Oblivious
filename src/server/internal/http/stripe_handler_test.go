package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
	"oblivious/server/internal/config"
	"oblivious/server/internal/payment"
	"oblivious/server/internal/quota"

	stripebilling "oblivious/server/internal/stripe"
)

type fakeCheckoutCreator struct {
	database   *sql.DB
	sessionID  string
	sessionURL string
	request    stripebilling.CheckoutSessionRequest
	err        error
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
	if f.err != nil {
		return nil, f.err
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

type billingCheckoutPaymentIntentStore struct {
	created          stripebilling.PaymentIntent
	failedID         string
	failedReason     string
	checkoutIntentID string
}

func (s *billingCheckoutPaymentIntentStore) CreatePaymentIntent(_ context.Context, intent stripebilling.PaymentIntent) (stripebilling.PaymentIntent, error) {
	s.created = intent
	return intent, nil
}

func (s *billingCheckoutPaymentIntentStore) SetCheckoutSession(_ context.Context, id string, _ string, _ map[string]string) error {
	s.checkoutIntentID = id
	return nil
}

func (s *billingCheckoutPaymentIntentStore) MarkPaymentIntentFailed(_ context.Context, id string, reason string) error {
	s.failedID = id
	s.failedReason = reason
	return nil
}

type billingCheckoutQuotaStore struct {
	packages              map[string]*quota.Package
	topupCreated          bool
	createdTopup          *quota.TopupOrder
	topupFailedPaymentID  string
	topupCheckoutIntentID string
}

func (s *billingCheckoutQuotaStore) GetOrCreateQuota(_ context.Context, userID, organizationID string) (*quota.Quota, error) {
	return &quota.Quota{UserID: userID, OrganizationID: organizationID}, nil
}

func (s *billingCheckoutQuotaStore) UpdateQuotaBalance(context.Context, string, string, float64) error {
	return nil
}

func (s *billingCheckoutQuotaStore) CreateBillingSession(_ context.Context, session *quota.BillingSession) (*quota.BillingSession, error) {
	return session, nil
}

func (s *billingCheckoutQuotaStore) GetBillingSessionByIdempotencyKey(context.Context, string, string) (*quota.BillingSession, error) {
	return nil, sql.ErrNoRows
}

func (s *billingCheckoutQuotaStore) SettleBillingSession(context.Context, string, float64) error {
	return nil
}

func (s *billingCheckoutQuotaStore) RefundBillingSession(context.Context, string) error {
	return nil
}

func (s *billingCheckoutQuotaStore) ListPackages(context.Context, bool) ([]*quota.Package, error) {
	packages := make([]*quota.Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		packages = append(packages, pkg)
	}
	return packages, nil
}

func (s *billingCheckoutQuotaStore) GetPackage(_ context.Context, id string) (*quota.Package, error) {
	return s.packages[id], nil
}

func (s *billingCheckoutQuotaStore) CreateSubscription(_ context.Context, sub *quota.Subscription) (*quota.Subscription, error) {
	return sub, nil
}

func (s *billingCheckoutQuotaStore) ListActiveSubscriptions(context.Context, string, string) ([]*quota.Subscription, error) {
	return nil, nil
}

func (s *billingCheckoutQuotaStore) CreateTopupOrder(_ context.Context, order *quota.TopupOrder) (*quota.TopupOrder, error) {
	s.topupCreated = true
	copied := *order
	s.createdTopup = &copied
	return order, nil
}

func (s *billingCheckoutQuotaStore) UpdateTopupOrderCheckoutSession(_ context.Context, paymentIntentID string, _ string) error {
	s.topupCheckoutIntentID = paymentIntentID
	return nil
}

func (s *billingCheckoutQuotaStore) MarkTopupOrderFailedByPaymentIntent(_ context.Context, paymentIntentID string) error {
	s.topupFailedPaymentID = paymentIntentID
	return nil
}

func (s *billingCheckoutQuotaStore) UpdateTopupOrderStatus(context.Context, string, string, string) error {
	return nil
}

func (s *billingCheckoutQuotaStore) SaveUsageLimitSettings(_ context.Context, settings quota.UsageLimitSettings) (*quota.UsageLimitSettings, error) {
	return &settings, nil
}

func (s *billingCheckoutQuotaStore) ResolveUsageLimitSettings(context.Context, string, string) (quota.UsageLimitSettings, error) {
	return quota.UsageLimitSettings{}, nil
}

func (s *billingCheckoutQuotaStore) ListUsageLimitSettings(context.Context, string) ([]quota.UsageLimitSettings, error) {
	return nil, nil
}

func TestBillingCheckoutCreatorFailureMarksSubscriptionPaymentIntentFailed(t *testing.T) {
	durationDays := 30
	checkoutCreator := &fakeCheckoutCreator{err: errors.New("provider checkout unavailable")}
	paymentStore := &billingCheckoutPaymentIntentStore{}
	quotaStore := &billingCheckoutQuotaStore{packages: map[string]*quota.Package{
		"pkg_subscription_failed": {
			ID:           "pkg_subscription_failed",
			Name:         "Subscription Failure Plan",
			Price:        29,
			DurationDays: &durationDays,
			IsActive:     true,
		},
	}}
	handler := newBillingHandler(checkoutCreator, stripebilling.CheckoutConfig{}, paymentStore, quota.NewService(quotaStore), nil, nil)
	session := routeSurfaceUserSession()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"stripe","packageId":"pkg_subscription_failed","kind":"subscription"}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	request.Header.Set("Content-Type", "application/json")

	handler.checkout(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected subscription checkout failure to return 502, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode subscription checkout failure response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "checkout_create_failed" {
		t.Fatalf("expected checkout_create_failed response, got %+v", response.Error)
	}
	if paymentStore.created.ID == "" || paymentStore.created.Status != "pending" || paymentStore.created.Kind != "subscription" {
		t.Fatalf("expected pending subscription payment intent before checkout, got %+v", paymentStore.created)
	}
	if paymentStore.created.OrganizationID != session.OrganizationID || paymentStore.created.UserID != session.User.ID {
		t.Fatalf("expected tenant-scoped payment intent, got %+v", paymentStore.created)
	}
	if paymentStore.created.PackageID != "pkg_subscription_failed" || paymentStore.created.Amount != 29 {
		t.Fatalf("expected subscription package metadata in payment intent, got %+v", paymentStore.created)
	}
	if checkoutCreator.request.PaymentIntentID != paymentStore.created.ID || checkoutCreator.request.CheckoutKind != "subscription" {
		t.Fatalf("checkout creator should receive precreated subscription intent, got %+v", checkoutCreator.request)
	}
	if paymentStore.failedID != paymentStore.created.ID || !strings.Contains(paymentStore.failedReason, "provider checkout unavailable") {
		t.Fatalf("expected created subscription payment intent to be marked failed, failedID=%q reason=%q", paymentStore.failedID, paymentStore.failedReason)
	}
	if paymentStore.checkoutIntentID != "" {
		t.Fatalf("checkout session must not be recorded after provider failure, got intent %q", paymentStore.checkoutIntentID)
	}
	if quotaStore.topupCreated || quotaStore.topupFailedPaymentID != "" || quotaStore.topupCheckoutIntentID != "" {
		t.Fatalf("subscription checkout failure must not touch topup lifecycle, store=%+v", quotaStore)
	}
}

func TestBillingCheckoutCreatorFailureMarksTopupFailedWithoutDatabase(t *testing.T) {
	checkoutCreator := &fakeCheckoutCreator{err: errors.New("provider checkout unavailable")}
	paymentStore := &billingCheckoutPaymentIntentStore{}
	quotaStore := &billingCheckoutQuotaStore{}
	handler := newBillingHandler(checkoutCreator, stripebilling.CheckoutConfig{}, paymentStore, quota.NewService(quotaStore), nil, nil)
	session := routeSurfaceUserSession()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"stripe","kind":"topup","amount":25}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	request.Header.Set("Content-Type", "application/json")

	handler.checkout(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected topup checkout failure to return 502, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode topup checkout failure response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "checkout_create_failed" {
		t.Fatalf("expected checkout_create_failed response, got %+v", response.Error)
	}
	if paymentStore.created.ID == "" || paymentStore.created.Status != "pending" || paymentStore.created.Kind != "topup" {
		t.Fatalf("expected pending topup payment intent before checkout, got %+v", paymentStore.created)
	}
	if paymentStore.created.OrganizationID != session.OrganizationID || paymentStore.created.UserID != session.User.ID {
		t.Fatalf("expected tenant-scoped topup payment intent, got %+v", paymentStore.created)
	}
	if paymentStore.created.PackageID != "" || paymentStore.created.Amount != 25 || paymentStore.created.Metadata["topup_amount"] != "25.000000" {
		t.Fatalf("expected topup metadata in payment intent, got %+v", paymentStore.created)
	}
	if quotaStore.createdTopup == nil {
		t.Fatal("expected pending topup order to be created before checkout")
	}
	if quotaStore.createdTopup.PaymentIntentID != paymentStore.created.ID ||
		quotaStore.createdTopup.OrganizationID != session.OrganizationID ||
		quotaStore.createdTopup.UserID != session.User.ID ||
		quotaStore.createdTopup.Amount != 25 ||
		quotaStore.createdTopup.Status != "pending" {
		t.Fatalf("expected tenant-scoped pending topup order, got %+v", quotaStore.createdTopup)
	}
	if checkoutCreator.request.PaymentIntentID != paymentStore.created.ID ||
		checkoutCreator.request.CheckoutKind != "topup" ||
		checkoutCreator.request.PlanName != "Quota top-up" ||
		checkoutCreator.request.PlanPrice != 25 {
		t.Fatalf("checkout creator should receive precreated topup intent, got %+v", checkoutCreator.request)
	}
	if paymentStore.failedID != paymentStore.created.ID || !strings.Contains(paymentStore.failedReason, "provider checkout unavailable") {
		t.Fatalf("expected created topup payment intent to be marked failed, failedID=%q reason=%q", paymentStore.failedID, paymentStore.failedReason)
	}
	if quotaStore.topupFailedPaymentID != paymentStore.created.ID {
		t.Fatalf("expected topup order to be marked failed for payment intent %q, got %q", paymentStore.created.ID, quotaStore.topupFailedPaymentID)
	}
	if paymentStore.checkoutIntentID != "" || quotaStore.topupCheckoutIntentID != "" {
		t.Fatalf("checkout session must not be recorded after provider failure, paymentIntent=%q topupIntent=%q", paymentStore.checkoutIntentID, quotaStore.topupCheckoutIntentID)
	}
}

func TestBillingCheckoutRejectsUnconfiguredProviderBeforeArtifactsWithoutDatabase(t *testing.T) {
	for _, provider := range []string{"alipay", "wechatpay"} {
		t.Run(provider, func(t *testing.T) {
			checkoutCreator := &fakeCheckoutCreator{}
			paymentStore := &billingCheckoutPaymentIntentStore{}
			quotaStore := &billingCheckoutQuotaStore{}
			handler := newBillingHandler(checkoutCreator, stripebilling.CheckoutConfig{}, paymentStore, quota.NewService(quotaStore), nil, nil)
			session := routeSurfaceUserSession()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(fmt.Sprintf(`{"provider":%q,"kind":"topup","amount":25}`, provider))).
				WithContext(context.WithValue(context.Background(), sessionContextKey, session))
			request.Header.Set("Content-Type", "application/json")

			handler.checkout(recorder, request)

			if recorder.Code != http.StatusNotImplemented {
				t.Fatalf("expected unconfigured provider checkout to return 501, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			var response Envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode unconfigured provider response: %v", err)
			}
			if response.Error == nil || response.Error.Code != payment.CodeProviderNotConfigured {
				t.Fatalf("expected provider_not_configured response, got %+v", response.Error)
			}
			if paymentStore.created.ID != "" || paymentStore.failedID != "" || paymentStore.checkoutIntentID != "" {
				t.Fatalf("provider rejection must not create or update payment artifacts, store=%+v", paymentStore)
			}
			if quotaStore.topupCreated || quotaStore.topupFailedPaymentID != "" || quotaStore.topupCheckoutIntentID != "" {
				t.Fatalf("provider rejection must not create or update topup artifacts, store=%+v", quotaStore)
			}
			if checkoutCreator.request.PaymentIntentID != "" {
				t.Fatalf("checkout creator must not be called for unconfigured provider, got %+v", checkoutCreator.request)
			}
		})
	}
}

func TestBillingCheckoutRejectsUnsupportedProviderBeforeArtifactsWithoutDatabase(t *testing.T) {
	checkoutCreator := &fakeCheckoutCreator{}
	paymentStore := &billingCheckoutPaymentIntentStore{}
	quotaStore := &billingCheckoutQuotaStore{}
	handler := newBillingHandler(checkoutCreator, stripebilling.CheckoutConfig{}, paymentStore, quota.NewService(quotaStore), nil, nil)
	session := routeSurfaceUserSession()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"provider":"paypal","kind":"topup","amount":25}`)).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	request.Header.Set("Content-Type", "application/json")

	handler.checkout(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported provider checkout to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode unsupported provider response: %v", err)
	}
	if response.Error == nil || response.Error.Code != payment.CodeUnsupportedProvider {
		t.Fatalf("expected unsupported_provider response, got %+v", response.Error)
	}
	if paymentStore.created.ID != "" || paymentStore.failedID != "" || paymentStore.checkoutIntentID != "" {
		t.Fatalf("provider rejection must not create or update payment artifacts, store=%+v", paymentStore)
	}
	if quotaStore.topupCreated || quotaStore.topupFailedPaymentID != "" || quotaStore.topupCheckoutIntentID != "" {
		t.Fatalf("provider rejection must not create or update topup artifacts, store=%+v", quotaStore)
	}
	if checkoutCreator.request.PaymentIntentID != "" {
		t.Fatalf("checkout creator must not be called for unsupported provider, got %+v", checkoutCreator.request)
	}
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
			if _, err := database.Exec(`
				INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, created_at)
				VALUES ($1, $2, $3, 25, 25, 'pending', $4, NOW())
			`, "topup_"+tc.provider+"_webhook", organizationID, userID, "pi_"+tc.provider+"_webhook"); err != nil {
				t.Fatalf("insert domestic topup order: %v", err)
			}
			payload := []byte(fmt.Sprintf(`{
				"id": %q,
				"type": "checkout.paid",
				"organization_id": %q,
				"user_id": %q,
				"payment_intent_id": %q,
				"provider_payment_intent_id": %q,
				"kind": "topup",
				"amount": 25,
				"currency": "cny"
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

func TestDomesticPaymentWebhookRouteAppliesSubscriptionLifecycleOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_subscription_secret"
	router := NewRouter(cfg, database)
	_, _, userID := registerHTTPUser(t, router, "alipay-subscription@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ('pkg_alipay_subscription', 'Alipay Subscription', 'Domestic subscription plan', 100, 29, 30, true, 1, NOW())
	`); err != nil {
		t.Fatalf("insert domestic subscription package: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO subscriptions (
			id, organization_id, user_id, package_id, status, provider_subscription_id,
			provider_customer_id, current_period_start, current_period_end, started_at, created_at, updated_at
		)
		VALUES ('sub_alipay_local', $1, $2, 'pkg_alipay_subscription', 'active', 'sub_alipay_1',
		        'buyer_alipay_1', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert domestic subscription: %v", err)
	}

	updatedPayload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_subscription_updated",
		"type": "subscription.updated",
		"organization_id": %q,
		"user_id": %q,
		"provider_subscription_id": "sub_alipay_1",
		"provider_customer_id": "buyer_alipay_2",
		"status": "past_due",
		"cancel_at_period_end": true
	}`, organizationID, userID))
	deletedPayload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_subscription_deleted",
		"type": "subscription.deleted",
		"organization_id": %q,
		"user_id": %q,
		"provider_subscription_id": "sub_alipay_1",
		"provider_customer_id": "buyer_alipay_2",
		"status": "active"
	}`, organizationID, userID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(updatedPayload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, updatedPayload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("update attempt %d expected alipay subscription webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var status, providerCustomerID string
	var cancelAtPeriodEnd bool
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_customer_id, ''), cancel_at_period_end
		FROM subscriptions
		WHERE id = 'sub_alipay_local'
	`).Scan(&status, &providerCustomerID, &cancelAtPeriodEnd); err != nil {
		t.Fatalf("query updated domestic subscription: %v", err)
	}
	if status != "past_due" || providerCustomerID != "buyer_alipay_2" || !cancelAtPeriodEnd {
		t.Fatalf("expected past_due domestic subscription update, got status=%s customer=%s cancel=%v", status, providerCustomerID, cancelAtPeriodEnd)
	}

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(deletedPayload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, deletedPayload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("delete attempt %d expected alipay subscription webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var transitionCount, webhookCount int
	var transitionProvider, updateEventType, deleteEventType string
	if err := database.QueryRow(`SELECT status FROM subscriptions WHERE id = 'sub_alipay_local'`).Scan(&status); err != nil {
		t.Fatalf("query deleted domestic subscription: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(provider), '')
		FROM billing_lifecycle_events
		WHERE provider_event_id IN ('evt_alipay_subscription_updated', 'evt_alipay_subscription_deleted')
	`).Scan(&transitionCount, &transitionProvider); err != nil {
		t.Fatalf("query domestic subscription transitions: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COALESCE(MAX(event_type), '')
		FROM billing_lifecycle_events
		WHERE provider_event_id = 'evt_alipay_subscription_updated'
	`).Scan(&updateEventType); err != nil {
		t.Fatalf("query domestic subscription update event type: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COALESCE(MAX(event_type), '')
		FROM billing_lifecycle_events
		WHERE provider_event_id = 'evt_alipay_subscription_deleted'
	`).Scan(&deleteEventType); err != nil {
		t.Fatalf("query domestic subscription delete event type: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM stripe_webhook_events
		WHERE event_id IN ('evt_alipay_subscription_updated', 'evt_alipay_subscription_deleted')
	`).Scan(&webhookCount); err != nil {
		t.Fatalf("query domestic subscription webhook ledger count: %v", err)
	}
	if status != "cancelled" || transitionCount != 2 || transitionProvider != "alipay" ||
		updateEventType != "alipay.subscription.updated" || deleteEventType != "alipay.subscription.deleted" || webhookCount != 2 {
		t.Fatalf("expected one alipay update/delete lifecycle, got status=%s transitions=%d provider=%s update=%s delete=%s webhooks=%d",
			status, transitionCount, transitionProvider, updateEventType, deleteEventType, webhookCount)
	}
}

func TestDomesticPaymentWebhookRouteAppliesTopupRefundOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_refund_secret"
	router := NewRouter(cfg, database)
	_, _, userID := registerHTTPUser(t, router, "alipay-refund@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	if _, err := database.Exec(`
		INSERT INTO payment_intents (
			id, provider, organization_id, user_id, kind, amount, currency, status,
			metadata, created_at, updated_at, provider_payment_intent_id
		)
		VALUES ('pi_alipay_refund', 'alipay', $1, $2, 'topup', 25, 'cny', 'completed',
		        '{}', NOW(), NOW(), 'trade_alipay_refund')
	`, organizationID, userID); err != nil {
		t.Fatalf("insert domestic refund payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, paid_at, created_at)
		VALUES ('topup_alipay_refund', $1, $2, 25, 25, 'paid', 'pi_alipay_refund', NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert domestic refund topup order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO quotas (id, organization_id, user_id, scope, balance, used, created_at, updated_at)
		VALUES ('quota_alipay_refund', $1, $2, 'organization', 25, 0, NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("seed domestic refund quota balance: %v", err)
	}

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_refund",
		"type": "refund.succeeded",
		"organization_id": %q,
		"user_id": %q,
		"payment_intent_id": "pi_alipay_refund",
		"provider_payment_intent_id": "trade_alipay_refund",
		"provider_refund_id": "alipay_refund_1",
		"kind": "topup",
		"amount": 10,
		"currency": "cny",
		"reason": "requested_by_customer"
	}`, organizationID, userID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, payload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected alipay refund webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var refundCount int
	var paymentStatus, refundProvider string
	var intentRefunded, topupRefunded, quotaBalance float64
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(provider), '')
		FROM billing_refunds
		WHERE provider_refund_id = 'alipay_refund_1'
	`).Scan(&refundCount, &refundProvider); err != nil {
		t.Fatalf("query domestic refund count: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM payment_intents WHERE id = 'pi_alipay_refund'`).Scan(&paymentStatus, &intentRefunded); err != nil {
		t.Fatalf("query domestic refund payment intent: %v", err)
	}
	if err := database.QueryRow(`SELECT refunded_amount FROM topup_orders WHERE id = 'topup_alipay_refund'`).Scan(&topupRefunded); err != nil {
		t.Fatalf("query domestic refund topup: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'`, organizationID).Scan(&quotaBalance); err != nil {
		t.Fatalf("query domestic refund quota: %v", err)
	}
	if refundCount != 1 || refundProvider != "alipay" || paymentStatus != "partially_refunded" ||
		intentRefunded != 10 || topupRefunded != 10 || quotaBalance != 15 {
		t.Fatalf("expected one alipay partial refund and quota reversal, got refunds=%d provider=%s payment=%s intentRefund=%.2f topupRefund=%.2f balance=%.2f",
			refundCount, refundProvider, paymentStatus, intentRefunded, topupRefunded, quotaBalance)
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

func insertHTTPMarketplacePayoutFixture(t *testing.T, database *sql.DB, router http.Handler, suffix string) (string, string, string) {
	t.Helper()

	_, _, buyerUserID := registerHTTPUser(t, router, "marketplace-payout-"+suffix+"-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-payout-"+suffix+"-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	agentID := "agent_domestic_payout_" + suffix
	versionID := "version_domestic_payout_" + suffix
	paymentIntentID := "pi_domestic_payout_" + suffix
	orderID := "order_domestic_payout_" + suffix
	payoutID := "payout_domestic_" + suffix
	providerPayoutID := "alipay_payout_" + suffix
	settlementID := "settlement_domestic_payout_" + suffix

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'Domestic Payout Agent', 'Domestic payout bridge test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, agentID, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert domestic payout marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ($1, $2, $3, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, versionID, agentID, publisherOrganizationID); err != nil {
		t.Fatalf("insert domestic payout marketplace version: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO payment_intents (
			id, provider, organization_id, user_id, package_id, kind, amount,
			currency, status, metadata, created_at, updated_at, provider_payment_intent_id
		)
		VALUES ($1, 'alipay', $2, $3, NULL, 'marketplace_install', 50, 'usd', 'completed', '{}', NOW(), NOW(), $4)
	`, paymentIntentID, buyerOrganizationID, buyerUserID, "alipay_trade_"+suffix); err != nil {
		t.Fatalf("insert domestic payout payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_orders (
			id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
			agent_id, version_id, payment_intent_id, provider_payment_intent_id,
			gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			currency, status, created_at, updated_at, paid_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 50, 10, 40, 0, 'usd', 'paid', NOW(), NOW(), NOW())
	`, orderID, buyerOrganizationID, buyerUserID, publisherOrganizationID, publisherUserID, agentID, versionID, paymentIntentID, "alipay_trade_"+suffix); err != nil {
		t.Fatalf("insert domestic payout marketplace order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_payouts (
			id, publisher_organization_id, publisher_user_id, amount, currency,
			provider, provider_payout_id, status, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, 40, 'usd', 'alipay', $4, 'payout_pending', '{}', NOW(), NOW())
	`, payoutID, publisherOrganizationID, publisherUserID, providerPayoutID); err != nil {
		t.Fatalf("insert domestic payout marketplace payout: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_settlements (
			id, order_id, publisher_organization_id, publisher_user_id, agent_id,
			gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			payout_id, status, hold_until, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 50, 10, 40, 0, $6, 'payout_pending', NOW() - INTERVAL '1 hour', NOW(), NOW())
	`, settlementID, orderID, publisherOrganizationID, publisherUserID, agentID, payoutID); err != nil {
		t.Fatalf("insert domestic payout marketplace settlement: %v", err)
	}

	return payoutID, providerPayoutID, settlementID
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

func TestBillingCheckoutCreatorFailureMarksTopupFailed(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database, err: errors.New("provider checkout unavailable")}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/billing/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/billing/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, userID := registerHTTPUser(t, router, "stripe-topup-checkout-failed@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"kind":"topup","amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected checkout creator failure to return 502, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode checkout creator failure response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "checkout_create_failed" {
		t.Fatalf("expected checkout_create_failed response, got %+v", response.Error)
	}
	if fakeCreator.request.PaymentIntentID == "" || fakeCreator.request.CheckoutKind != "topup" {
		t.Fatalf("checkout creator should see precreated topup intent, got %+v", fakeCreator.request)
	}

	var paymentStatus, topupStatus, providerCheckoutSessionID string
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_checkout_session_id, '')
		FROM payment_intents
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, fakeCreator.request.PaymentIntentID, organizationID, userID).Scan(&paymentStatus, &providerCheckoutSessionID); err != nil {
		t.Fatalf("query failed topup payment intent: %v", err)
	}
	if err := database.QueryRow(`
		SELECT status
		FROM topup_orders
		WHERE payment_intent_id = $1 AND organization_id = $2 AND user_id = $3
	`, fakeCreator.request.PaymentIntentID, organizationID, userID).Scan(&topupStatus); err != nil {
		t.Fatalf("query failed topup order: %v", err)
	}
	var balance float64
	if err := database.QueryRow(`SELECT COALESCE((SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'), 0)`, organizationID).Scan(&balance); err != nil {
		t.Fatalf("query failed topup quota balance: %v", err)
	}
	if paymentStatus != "failed" || topupStatus != "failed" || providerCheckoutSessionID != "" || balance != 0 {
		t.Fatalf("expected failed local topup artifacts without quota credit, got payment=%s topup=%s session=%q balance=%.2f", paymentStatus, topupStatus, providerCheckoutSessionID, balance)
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
	request := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_http/install?versionID=version_agent_paid_http&provider=stripe", nil)
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

func TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailed(t *testing.T) {
	database := testDatabase(t)
	fakeCreator := &fakeCheckoutCreator{database: database, err: errors.New("provider checkout unavailable")}
	cfg := testConfig()
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: fakeCreator})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-failed-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-failed-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_failed_checkout', $1, $2, 'Paid Failed Checkout Agent', 'Paid marketplace fail-closed test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert failed checkout marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_failed_checkout', 'agent_paid_failed_checkout', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert failed checkout marketplace version: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_failed_checkout/install?versionID=version_agent_paid_failed_checkout&provider=stripe", nil)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected marketplace checkout creator failure to return 502, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode marketplace checkout creator failure response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "checkout_create_failed" {
		t.Fatalf("expected checkout_create_failed response, got %+v", response.Error)
	}
	if fakeCreator.request.PaymentIntentID == "" || fakeCreator.request.MarketplaceOrderID == "" || fakeCreator.request.CheckoutKind != "marketplace_install" {
		t.Fatalf("checkout creator should see precreated marketplace intent/order, got %+v", fakeCreator.request)
	}

	var orderStatus, paymentStatus, providerCheckoutSessionID string
	if err := database.QueryRow(`
		SELECT mo.status, pi.status, COALESCE(mo.provider_checkout_session_id, '')
		FROM marketplace_orders mo
		JOIN payment_intents pi ON pi.id = mo.payment_intent_id
		WHERE mo.id = $1 AND mo.payment_intent_id = $2
	`, fakeCreator.request.MarketplaceOrderID, fakeCreator.request.PaymentIntentID).Scan(&orderStatus, &paymentStatus, &providerCheckoutSessionID); err != nil {
		t.Fatalf("query failed marketplace order: %v", err)
	}
	var installCount, settlementCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid_failed_checkout'`).Scan(&installCount); err != nil {
		t.Fatalf("count failed checkout installs: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_settlements WHERE order_id = $1`, fakeCreator.request.MarketplaceOrderID).Scan(&settlementCount); err != nil {
		t.Fatalf("count failed checkout settlements: %v", err)
	}
	if orderStatus != "failed" || paymentStatus != "failed" || providerCheckoutSessionID != "" || installCount != 0 || settlementCount != 0 {
		t.Fatalf("expected failed marketplace artifacts without install/settlement, got order=%s payment=%s session=%q installs=%d settlements=%d",
			orderStatus, paymentStatus, providerCheckoutSessionID, installCount, settlementCount)
	}
	if fakeCreator.request.OrganizationID != buyerOrganizationID || fakeCreator.request.UserID != buyerUserID {
		t.Fatalf("checkout creator saw wrong buyer scope: %+v buyerOrg=%s buyerUser=%s", fakeCreator.request, buyerOrganizationID, buyerUserID)
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

func TestDomesticPaymentWebhookRouteAppliesMarketplaceInstallSettlementOnce(t *testing.T) {
	database := testDatabase(t)
	stripeCreator := &fakeCheckoutCreator{database: database}
	alipayCreator := &fakeCheckoutCreator{
		database:   database,
		sessionID:  "alipay_marketplace_paid_session",
		sessionURL: "https://checkout.alipay.test/marketplace/alipay_marketplace_paid_session",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true})

	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_marketplace_secret"
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{
		CheckoutCreator:         stripeCreator,
		CheckoutCreators:        map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		PaymentProviderRegistry: providerRegistry,
	})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-alipay-webhook-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-alipay-webhook-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_alipay_webhook', $1, $2, 'Paid Alipay Webhook Agent', 'Paid marketplace domestic webhook test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay webhook marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_alipay_webhook', 'agent_paid_alipay_webhook', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay webhook marketplace version: %v", err)
	}

	installRecorder := httptest.NewRecorder()
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_alipay_webhook/install?versionID=version_agent_paid_alipay_webhook&provider=alipay", nil)
	installRequest.AddCookie(cookie)
	addCSRF(installRequest, csrfToken)
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != http.StatusCreated {
		t.Fatalf("paid alipay install checkout expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}
	if stripeCreator.request.PaymentIntentID != "" {
		t.Fatalf("stripe checkout creator must not be used for alipay marketplace install, got %+v", stripeCreator.request)
	}
	if alipayCreator.request.CheckoutKind != "marketplace_install" || alipayCreator.request.MarketplaceOrderID == "" {
		t.Fatalf("alipay checkout creator saw wrong marketplace metadata: %+v", alipayCreator.request)
	}

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_marketplace_install",
		"type": "checkout.paid",
		"organization_id": %q,
		"user_id": %q,
		"payment_intent_id": %q,
		"marketplace_order_id": %q,
		"provider_payment_intent_id": "alipay_marketplace_payment_1",
		"provider_checkout_session_id": "alipay_marketplace_session",
		"kind": "marketplace_install",
		"amount": 50,
		"currency": "cny"
	}`, buyerOrganizationID, buyerUserID, alipayCreator.request.PaymentIntentID, alipayCreator.request.MarketplaceOrderID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, payload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var orderStatus, paymentStatus, storedProviderPaymentIntentID string
	var installCount, settlementCount, transitionCount int
	if err := database.QueryRow(`SELECT status FROM marketplace_orders WHERE id = $1`, alipayCreator.request.MarketplaceOrderID).Scan(&orderStatus); err != nil {
		t.Fatalf("query domestic marketplace order: %v", err)
	}
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_payment_intent_id, '')
		FROM payment_intents
		WHERE id = $1 AND provider = 'alipay'
	`, alipayCreator.request.PaymentIntentID).Scan(&paymentStatus, &storedProviderPaymentIntentID); err != nil {
		t.Fatalf("query domestic marketplace payment intent: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid_alipay_webhook' AND organization_id = $1`, buyerOrganizationID).Scan(&installCount); err != nil {
		t.Fatalf("count domestic marketplace install: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_settlements WHERE order_id = $1`, alipayCreator.request.MarketplaceOrderID).Scan(&settlementCount); err != nil {
		t.Fatalf("count domestic marketplace settlements: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_lifecycle_events WHERE provider_event_id = 'evt_alipay_marketplace_install'`).Scan(&transitionCount); err != nil {
		t.Fatalf("count domestic marketplace lifecycle transitions: %v", err)
	}
	if orderStatus != "paid" || paymentStatus != "completed" || storedProviderPaymentIntentID != "alipay_marketplace_payment_1" ||
		installCount != 1 || settlementCount != 1 || transitionCount != 1 {
		t.Fatalf("expected paid order, completed intent, one install, one settlement, one transition; got order=%s payment=%s provider_pi=%s installs=%d settlements=%d transitions=%d",
			orderStatus, paymentStatus, storedProviderPaymentIntentID, installCount, settlementCount, transitionCount)
	}
}

func TestDomesticPaymentWebhookRouteAppliesMarketplaceRefundOnce(t *testing.T) {
	database := testDatabase(t)
	stripeCreator := &fakeCheckoutCreator{database: database}
	alipayCreator := &fakeCheckoutCreator{
		database:   database,
		sessionID:  "alipay_marketplace_refund_session",
		sessionURL: "https://checkout.alipay.test/marketplace/alipay_marketplace_refund_session",
	}
	providerRegistry := payment.NewRegistry("stripe")
	providerRegistry.Register(payment.Provider{Name: "stripe", Configured: true})
	providerRegistry.Register(payment.Provider{Name: "alipay", Configured: true})

	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_marketplace_refund_secret"
	cfg.StripeSuccessURL = "https://app.oblivious.test/marketplace/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/marketplace/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{
		CheckoutCreator:         stripeCreator,
		CheckoutCreators:        map[string]stripebilling.CheckoutCreator{"alipay": alipayCreator},
		PaymentProviderRegistry: providerRegistry,
	})

	cookie, csrfToken, buyerUserID := registerHTTPUser(t, router, "marketplace-alipay-refund-buyer@example.com")
	_, buyerOrganizationID := queryHTTPUserScope(t, database, buyerUserID)
	_, _, publisherUserID := registerHTTPUser(t, router, "marketplace-alipay-refund-publisher@example.com")
	_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ('agent_paid_alipay_refund', $1, $2, 'Paid Alipay Refund Agent', 'Paid marketplace domestic refund test agent.',
		        '{"tools":[{"name":"paid"}]}'::jsonb, '[]'::jsonb, 'public', 'approved',
		        'one_time', 50, 0, 0, 0, NOW(), NOW())
	`, publisherUserID, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay refund marketplace agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ('version_agent_paid_alipay_refund', 'agent_paid_alipay_refund', $1, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, publisherOrganizationID); err != nil {
		t.Fatalf("insert paid alipay refund marketplace version: %v", err)
	}

	installRecorder := httptest.NewRecorder()
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_alipay_refund/install?versionID=version_agent_paid_alipay_refund&provider=alipay", nil)
	installRequest.AddCookie(cookie)
	addCSRF(installRequest, csrfToken)
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != http.StatusCreated {
		t.Fatalf("paid alipay install checkout expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}

	paidPayload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_marketplace_refund_paid",
		"type": "checkout.paid",
		"organization_id": %q,
		"user_id": %q,
		"payment_intent_id": %q,
		"marketplace_order_id": %q,
		"provider_payment_intent_id": "alipay_marketplace_refund_payment_1",
		"provider_checkout_session_id": "alipay_marketplace_refund_session",
		"kind": "marketplace_install",
		"amount": 50,
		"currency": "cny"
	}`, buyerOrganizationID, buyerUserID, alipayCreator.request.PaymentIntentID, alipayCreator.request.MarketplaceOrderID))
	timestamp := "1760000000"
	paidRecorder := httptest.NewRecorder()
	paidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(paidPayload)))
	paidRequest.Header.Set("Oblivious-Payment-Timestamp", timestamp)
	paidRequest.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, paidPayload))
	router.ServeHTTP(paidRecorder, paidRequest)
	if paidRecorder.Code != http.StatusOK {
		t.Fatalf("paid marketplace webhook expected 200, got %d with body %s", paidRecorder.Code, paidRecorder.Body.String())
	}

	refundPayload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_marketplace_refund",
		"type": "refund.succeeded",
		"organization_id": %q,
		"user_id": %q,
		"payment_intent_id": %q,
		"provider_payment_intent_id": "alipay_marketplace_refund_payment_1",
		"provider_refund_id": "alipay_marketplace_refund_1",
		"kind": "marketplace_install",
		"amount": 10,
		"currency": "cny",
		"reason": "requested_by_customer"
	}`, buyerOrganizationID, buyerUserID, alipayCreator.request.PaymentIntentID))
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(refundPayload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, refundPayload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("refund attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var orderStatus, settlementStatus, refundProvider string
	var orderRefunded, settlementRefunded float64
	var refundCount int
	if err := database.QueryRow(`SELECT status, refunded_amount FROM marketplace_orders WHERE id = $1`, alipayCreator.request.MarketplaceOrderID).Scan(&orderStatus, &orderRefunded); err != nil {
		t.Fatalf("query domestic marketplace refund order: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM marketplace_settlements WHERE order_id = $1`, alipayCreator.request.MarketplaceOrderID).Scan(&settlementStatus, &settlementRefunded); err != nil {
		t.Fatalf("query domestic marketplace refund settlement: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(provider), '')
		FROM billing_refunds
		WHERE provider_refund_id = 'alipay_marketplace_refund_1'
	`).Scan(&refundCount, &refundProvider); err != nil {
		t.Fatalf("query domestic marketplace billing refunds: %v", err)
	}
	if orderStatus != "partially_refunded" || settlementStatus != "partially_refunded" ||
		orderRefunded != 10 || settlementRefunded != 10 || refundCount != 1 || refundProvider != "alipay" {
		t.Fatalf("expected one alipay marketplace partial refund, got order=%s %.2f settlement=%s %.2f refunds=%d provider=%s",
			orderStatus, orderRefunded, settlementStatus, settlementRefunded, refundCount, refundProvider)
	}
}

func TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutPaidOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_marketplace_payout_paid_secret"
	router := NewRouterWithOptions(cfg, database, RouterOptions{})
	payoutID, providerPayoutID, settlementID := insertHTTPMarketplacePayoutFixture(t, database, router, "paid")

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_marketplace_payout_paid",
		"type": "payout.paid",
		"payout_id": %q,
		"provider_payout_id": %q,
		"status": "paid"
	}`, payoutID, providerPayoutID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, payload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("paid payout attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var payoutStatus, storedProviderPayoutID, settlementStatus, transitionState string
	var webhookEventCount, transitionCount int
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_payout_id, '')
		FROM marketplace_payouts
		WHERE id = $1
	`, payoutID).Scan(&payoutStatus, &storedProviderPayoutID); err != nil {
		t.Fatalf("query paid domestic payout: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM marketplace_settlements WHERE id = $1`, settlementID).Scan(&settlementStatus); err != nil {
		t.Fatalf("query paid domestic settlement: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM stripe_webhook_events
		WHERE provider = 'alipay' AND event_id = 'evt_alipay_marketplace_payout_paid'
	`).Scan(&webhookEventCount); err != nil {
		t.Fatalf("count paid domestic payout webhook events: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(to_state), '')
		FROM billing_lifecycle_events
		WHERE provider = 'alipay' AND provider_event_id = 'evt_alipay_marketplace_payout_paid'
	`).Scan(&transitionCount, &transitionState); err != nil {
		t.Fatalf("count paid domestic payout lifecycle transitions: %v", err)
	}
	if payoutStatus != "paid_out" || storedProviderPayoutID != providerPayoutID || settlementStatus != "paid_out" ||
		webhookEventCount != 1 || transitionCount != 1 || transitionState != "paid_out" {
		t.Fatalf("expected one paid domestic payout bridge transition, got payout=%s providerPayout=%s settlement=%s webhookEvents=%d transitions=%d state=%s",
			payoutStatus, storedProviderPayoutID, settlementStatus, webhookEventCount, transitionCount, transitionState)
	}
}

func TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutFailedOnce(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.AlipayWebhookSecret = "alipay_marketplace_payout_failed_secret"
	router := NewRouterWithOptions(cfg, database, RouterOptions{})
	payoutID, providerPayoutID, settlementID := insertHTTPMarketplacePayoutFixture(t, database, router, "failed")

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_alipay_marketplace_payout_failed",
		"type": "payout.failed",
		"provider_payout_id": %q,
		"status": "failed",
		"reason": "bank_account_closed"
	}`, providerPayoutID))
	timestamp := "1760000000"
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set("Oblivious-Payment-Timestamp", timestamp)
		request.Header.Set("Oblivious-Payment-Signature", domesticWebhookSignature(cfg.AlipayWebhookSecret, timestamp, payload))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("failed payout attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	var payoutStatus, storedProviderPayoutID, providerReason, settlementStatus, settlementPayoutID string
	var webhookEventCount, transitionCount int
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_payout_id, ''), COALESCE(metadata->>'provider_reason', '')
		FROM marketplace_payouts
		WHERE id = $1
	`, payoutID).Scan(&payoutStatus, &storedProviderPayoutID, &providerReason); err != nil {
		t.Fatalf("query failed domestic payout: %v", err)
	}
	if err := database.QueryRow(`
		SELECT status, COALESCE(payout_id, '')
		FROM marketplace_settlements
		WHERE id = $1
	`, settlementID).Scan(&settlementStatus, &settlementPayoutID); err != nil {
		t.Fatalf("query failed domestic settlement: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM stripe_webhook_events
		WHERE provider = 'alipay' AND event_id = 'evt_alipay_marketplace_payout_failed'
	`).Scan(&webhookEventCount); err != nil {
		t.Fatalf("count failed domestic payout webhook events: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM billing_lifecycle_events
		WHERE provider = 'alipay' AND provider_event_id = 'evt_alipay_marketplace_payout_failed'
	`).Scan(&transitionCount); err != nil {
		t.Fatalf("count failed domestic payout lifecycle transitions: %v", err)
	}
	if payoutStatus != "failed" || storedProviderPayoutID != providerPayoutID || providerReason != "bank_account_closed" ||
		settlementStatus != "available" || settlementPayoutID != "" || webhookEventCount != 1 || transitionCount != 1 {
		t.Fatalf("expected one failed domestic payout bridge transition, got payout=%s providerPayout=%s reason=%q settlement=%s settlementPayout=%q webhookEvents=%d transitions=%d",
			payoutStatus, storedProviderPayoutID, providerReason, settlementStatus, settlementPayoutID, webhookEventCount, transitionCount)
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
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_webhook/install?versionID=version_agent_paid_webhook&provider=stripe", nil)
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
	installRequest := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/agents/agent_paid_refund/install?versionID=version_agent_paid_refund&provider=stripe", nil)
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
