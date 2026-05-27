package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"

	stripebilling "oblivious/server/internal/stripe"
)

type fakeCheckoutCreator struct {
	database *sql.DB
	request  stripebilling.CheckoutSessionRequest
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
	return &stripeapi.CheckoutSession{
		ID:  "cs_test_phase17",
		URL: "https://checkout.stripe.test/cs_test_phase17",
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
