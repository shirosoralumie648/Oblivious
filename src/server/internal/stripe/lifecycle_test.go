package stripe

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/testutil"
	stripeapi "github.com/stripe/stripe-go/v83"
	"oblivious/server/internal/metrics"
)

type fakeLifecycleStore struct {
	checkoutEvents []checkoutCompletedLifecycle
	checkoutIDs    []string
}

func (s *fakeLifecycleStore) ApplyCheckoutCompleted(_ context.Context, eventID string, input checkoutCompletedLifecycle, _ []byte) error {
	s.checkoutIDs = append(s.checkoutIDs, eventID)
	s.checkoutEvents = append(s.checkoutEvents, input)
	return nil
}
func (s *fakeLifecycleStore) ApplyInvoicePaid(context.Context, string, invoiceLifecycle, []byte) error {
	return nil
}
func (s *fakeLifecycleStore) ApplyInvoicePaymentFailed(context.Context, string, invoiceLifecycle, []byte) error {
	return nil
}
func (s *fakeLifecycleStore) ApplySubscriptionUpdated(context.Context, string, subscriptionLifecycle, []byte) error {
	return nil
}
func (s *fakeLifecycleStore) ApplySubscriptionDeleted(context.Context, string, subscriptionLifecycle, []byte) error {
	return nil
}
func (s *fakeLifecycleStore) ApplyRefund(context.Context, string, refundLifecycle, []byte) error {
	return nil
}

func TestLifecycleObservabilityRecordsCheckoutCompleted(t *testing.T) {
	service := NewLifecycleService(&fakeLifecycleStore{})
	event := lifecycleCheckoutCompletedEvent("evt_obs_checkout", map[string]string{
		"organization_id":   "org_obs",
		"user_id":           "user_obs",
		"payment_intent_id": "pi_obs",
		"plan_id":           "pkg_obs",
		"checkout_kind":     "subscription",
	}, map[string]string{
		"id":             "cs_obs",
		"payment_intent": "pi_provider_obs",
		"subscription":   "sub_provider_obs",
		"customer":       "cus_obs",
		"amount_total":   "2900",
		"currency":       "usd",
	})

	before := testutil.ToFloat64(metrics.BillingLifecycleEventsTotal.WithLabelValues("checkout", "completed"))
	if err := service.ApplyStripeEvent(context.Background(), event, []byte(`{"id":"evt_obs_checkout","api_key":"sk-secret"}`)); err != nil {
		t.Fatalf("ApplyStripeEvent returned error: %v", err)
	}
	after := testutil.ToFloat64(metrics.BillingLifecycleEventsTotal.WithLabelValues("checkout", "completed"))
	if after != before+1 {
		t.Fatalf("expected checkout lifecycle metric increment, before=%v after=%v", before, after)
	}
}

func TestLifecycleAppliesDomesticCheckoutPaidThroughCheckoutCompletion(t *testing.T) {
	store := &fakeLifecycleStore{}
	service := NewLifecycleService(store)

	if err := service.ApplyDomesticCheckoutPaid(context.Background(), DomesticCheckoutPaid{
		Provider:                  "alipay",
		EventID:                   "evt_alipay_paid",
		OrganizationID:            "org_1",
		UserID:                    "user_1",
		PaymentIntentID:           "pi_1",
		PackageID:                 "",
		Kind:                      "topup",
		ProviderPaymentIntentID:   "trade_1",
		ProviderCheckoutSessionID: "alipay_session_1",
		Amount:                    25,
		Currency:                  "cny",
	}, []byte(`{"id":"evt_alipay_paid"}`)); err != nil {
		t.Fatalf("ApplyDomesticCheckoutPaid returned error: %v", err)
	}

	if len(store.checkoutIDs) != 1 || store.checkoutIDs[0] != "evt_alipay_paid" {
		t.Fatalf("expected one domestic checkout event id, got %+v", store.checkoutIDs)
	}
	if len(store.checkoutEvents) != 1 {
		t.Fatalf("expected one domestic checkout input, got %+v", store.checkoutEvents)
	}
	input := store.checkoutEvents[0]
	if input.Provider != "alipay" || input.PaymentIntentID != "pi_1" || input.Kind != "topup" ||
		input.ProviderPaymentIntentID != "trade_1" || input.ProviderCheckoutSessionID != "alipay_session_1" ||
		input.Amount != 25 || input.Currency != "cny" || input.OrganizationID != "org_1" || input.UserID != "user_1" {
		t.Fatalf("unexpected domestic checkout input: %+v", input)
	}
}

func TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce(t *testing.T) {
	database := lifecycleTestDB(t)
	service := NewLifecycleService(NewSQLLifecycleStore(database))

	insertLifecycleUserOrg(t, database, "user_sub", "org_sub")
	insertLifecyclePackage(t, database, "pkg_sub", 100, 29)
	insertLifecyclePaymentIntent(t, database, "pi_sub", "org_sub", "user_sub", "pkg_sub", "subscription", 29)

	event := lifecycleCheckoutCompletedEvent("evt_checkout_sub", map[string]string{
		"organization_id":   "org_sub",
		"user_id":           "user_sub",
		"payment_intent_id": "pi_sub",
		"plan_id":           "pkg_sub",
		"checkout_kind":     "subscription",
	}, map[string]string{
		"id":              "cs_sub",
		"payment_intent":  "pi_provider_sub",
		"subscription":    "sub_provider_sub",
		"customer":        "cus_sub",
		"amount_total":    "2900",
		"currency":        "usd",
		"client_ref":      "pi_sub",
		"provider_event":  "evt_checkout_sub",
		"provider_status": "complete",
	})

	for i := 0; i < 2; i++ {
		if err := service.ApplyStripeEvent(context.Background(), event, []byte(`{"id":"evt_checkout_sub"}`)); err != nil {
			t.Fatalf("apply checkout session completed attempt %d: %v", i+1, err)
		}
	}

	var intentStatus, providerPaymentIntentID, providerSubscriptionID string
	if err := database.QueryRow(`
		SELECT status, provider_payment_intent_id, provider_subscription_id
		FROM payment_intents WHERE id = 'pi_sub'
	`).Scan(&intentStatus, &providerPaymentIntentID, &providerSubscriptionID); err != nil {
		t.Fatalf("query payment intent: %v", err)
	}
	if intentStatus != "completed" || providerPaymentIntentID != "pi_provider_sub" || providerSubscriptionID != "sub_provider_sub" {
		t.Fatalf("unexpected payment intent state: status=%s provider_pi=%s provider_sub=%s", intentStatus, providerPaymentIntentID, providerSubscriptionID)
	}

	var subscriptionCount int
	var subscriptionStatus, userPlanID string
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(status), '')
		FROM subscriptions
		WHERE organization_id = 'org_sub' AND user_id = 'user_sub' AND package_id = 'pkg_sub'
	`).Scan(&subscriptionCount, &subscriptionStatus); err != nil {
		t.Fatalf("query subscription: %v", err)
	}
	if subscriptionCount != 1 || subscriptionStatus != "active" {
		t.Fatalf("expected one active subscription, got count=%d status=%s", subscriptionCount, subscriptionStatus)
	}
	if err := database.QueryRow(`SELECT COALESCE(plan_id, '') FROM users WHERE id = 'user_sub'`).Scan(&userPlanID); err != nil {
		t.Fatalf("query user plan: %v", err)
	}
	if userPlanID != "pkg_sub" {
		t.Fatalf("expected user plan pkg_sub, got %s", userPlanID)
	}

	var transitionCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM billing_lifecycle_events
		WHERE transition_key = 'stripe:evt_checkout_sub:checkout:pi_sub'
	`).Scan(&transitionCount); err != nil {
		t.Fatalf("query transition count: %v", err)
	}
	if transitionCount != 1 {
		t.Fatalf("expected one lifecycle transition, got %d", transitionCount)
	}
}

func TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce(t *testing.T) {
	database := lifecycleTestDB(t)
	service := NewLifecycleService(NewSQLLifecycleStore(database))

	insertLifecycleUserOrg(t, database, "user_topup", "org_topup")
	insertLifecyclePaymentIntent(t, database, "pi_topup", "org_topup", "user_topup", "", "topup", 25)
	if _, err := database.Exec(`
		INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, created_at)
		VALUES ('topup_order_1', 'org_topup', 'user_topup', 25, 25, 'pending', 'pi_topup', NOW())
	`); err != nil {
		t.Fatalf("insert topup order: %v", err)
	}

	event := lifecycleCheckoutCompletedEvent("evt_checkout_topup", map[string]string{
		"organization_id":   "org_topup",
		"user_id":           "user_topup",
		"payment_intent_id": "pi_topup",
		"checkout_kind":     "topup",
	}, map[string]string{
		"id":             "cs_topup",
		"payment_intent": "pi_provider_topup",
		"amount_total":   "2500",
		"currency":       "usd",
	})

	for i := 0; i < 2; i++ {
		if err := service.ApplyStripeEvent(context.Background(), event, []byte(`{"id":"evt_checkout_topup"}`)); err != nil {
			t.Fatalf("apply topup checkout attempt %d: %v", i+1, err)
		}
	}

	var intentStatus, topupStatus string
	var balance, refundedAmount float64
	if err := database.QueryRow(`SELECT status FROM payment_intents WHERE id = 'pi_topup'`).Scan(&intentStatus); err != nil {
		t.Fatalf("query payment intent: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM topup_orders WHERE id = 'topup_order_1'`).Scan(&topupStatus, &refundedAmount); err != nil {
		t.Fatalf("query topup order: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = 'org_topup' AND scope = 'organization'`).Scan(&balance); err != nil {
		t.Fatalf("query quota balance: %v", err)
	}
	if intentStatus != "completed" || topupStatus != "paid" || refundedAmount != 0 || balance != 25 {
		t.Fatalf("expected paid topup and balance 25, got intent=%s topup=%s refunded=%.2f balance=%.2f", intentStatus, topupStatus, refundedAmount, balance)
	}

	var transitionCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM billing_lifecycle_events
		WHERE transition_key = 'stripe:evt_checkout_topup:checkout:pi_topup'
	`).Scan(&transitionCount); err != nil {
		t.Fatalf("query transition count: %v", err)
	}
	if transitionCount != 1 {
		t.Fatalf("expected one topup transition, got %d", transitionCount)
	}
}

func TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions(t *testing.T) {
	database := lifecycleTestDB(t)
	service := NewLifecycleService(NewSQLLifecycleStore(database))

	insertLifecycleUserOrg(t, database, "user_invoice", "org_invoice")
	insertLifecyclePackage(t, database, "pkg_invoice_basic", 100, 29)
	insertLifecyclePackage(t, database, "pkg_invoice_plus", 250, 59)
	insertLifecyclePaymentIntent(t, database, "pi_invoice_paid", "org_invoice", "user_invoice", "pkg_invoice_basic", "subscription", 29)
	insertLifecyclePaymentIntent(t, database, "pi_invoice_failed", "org_invoice", "user_invoice", "pkg_invoice_basic", "subscription", 29)
	if _, err := database.Exec(`
		INSERT INTO subscriptions (
			id, organization_id, user_id, package_id, status, next_plan_id,
			provider_subscription_id, current_period_start, current_period_end, started_at, created_at, updated_at
		)
		VALUES ('sub_invoice_local', 'org_invoice', 'user_invoice', 'pkg_invoice_basic', 'active', 'pkg_invoice_plus',
		        'sub_provider_invoice', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert invoice subscription: %v", err)
	}

	paid := lifecycleInvoiceEvent("evt_invoice_paid", "invoice.paid", map[string]string{
		"id":             "in_paid",
		"subscription":   "sub_provider_invoice",
		"payment_intent": "pi_provider_paid",
		"amount_due":     "5900",
		"amount_paid":    "5900",
		"currency":       "usd",
		"status":         "paid",
	}, map[string]string{
		"organization_id":   "org_invoice",
		"user_id":           "user_invoice",
		"payment_intent_id": "pi_invoice_paid",
	})
	if err := service.ApplyStripeEvent(context.Background(), paid, []byte(`{"id":"evt_invoice_paid"}`)); err != nil {
		t.Fatalf("apply invoice paid: %v", err)
	}

	var invoiceStatus, subscriptionStatus, packageID string
	var nextPlanID sql.NullString
	if err := database.QueryRow(`SELECT status FROM billing_invoices WHERE provider_invoice_id = 'in_paid'`).Scan(&invoiceStatus); err != nil {
		t.Fatalf("query paid invoice: %v", err)
	}
	if err := database.QueryRow(`SELECT status, package_id, next_plan_id FROM subscriptions WHERE id = 'sub_invoice_local'`).Scan(&subscriptionStatus, &packageID, &nextPlanID); err != nil {
		t.Fatalf("query paid subscription: %v", err)
	}
	if invoiceStatus != "paid" || subscriptionStatus != "active" || packageID != "pkg_invoice_plus" || nextPlanID.Valid {
		t.Fatalf("expected paid invoice to activate subscription and apply next plan, got invoice=%s sub=%s package=%s next=%v", invoiceStatus, subscriptionStatus, packageID, nextPlanID)
	}

	failed := lifecycleInvoiceEvent("evt_invoice_failed", "invoice.payment_failed", map[string]string{
		"id":             "in_failed",
		"subscription":   "sub_provider_invoice",
		"payment_intent": "pi_provider_failed",
		"amount_due":     "2900",
		"amount_paid":    "0",
		"currency":       "usd",
		"status":         "open",
	}, map[string]string{
		"organization_id":   "org_invoice",
		"user_id":           "user_invoice",
		"payment_intent_id": "pi_invoice_failed",
	})
	if err := service.ApplyStripeEvent(context.Background(), failed, []byte(`{"id":"evt_invoice_failed"}`)); err != nil {
		t.Fatalf("apply invoice payment failed: %v", err)
	}

	var failedInvoiceStatus, failedIntentStatus string
	var failedPaymentAt sql.NullTime
	if err := database.QueryRow(`SELECT status FROM billing_invoices WHERE provider_invoice_id = 'in_failed'`).Scan(&failedInvoiceStatus); err != nil {
		t.Fatalf("query failed invoice: %v", err)
	}
	if err := database.QueryRow(`SELECT status, failed_payment_at FROM subscriptions WHERE id = 'sub_invoice_local'`).Scan(&subscriptionStatus, &failedPaymentAt); err != nil {
		t.Fatalf("query failed subscription: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM payment_intents WHERE id = 'pi_invoice_failed'`).Scan(&failedIntentStatus); err != nil {
		t.Fatalf("query failed payment intent: %v", err)
	}
	if failedInvoiceStatus != "failed" || subscriptionStatus != "past_due" || !failedPaymentAt.Valid || failedIntentStatus != "failed" {
		t.Fatalf("expected failed invoice to mark past_due and failed intent, got invoice=%s sub=%s failedAt=%v intent=%s", failedInvoiceStatus, subscriptionStatus, failedPaymentAt.Valid, failedIntentStatus)
	}
}

func TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions(t *testing.T) {
	database := lifecycleTestDB(t)
	service := NewLifecycleService(NewSQLLifecycleStore(database))

	insertLifecycleUserOrg(t, database, "user_sub_update", "org_sub_update")
	insertLifecyclePackage(t, database, "pkg_sub_update", 100, 29)
	if _, err := database.Exec(`
		INSERT INTO subscriptions (
			id, organization_id, user_id, package_id, status, provider_subscription_id,
			current_period_start, current_period_end, started_at, created_at, updated_at
		)
		VALUES ('sub_update_local', 'org_sub_update', 'user_sub_update', 'pkg_sub_update', 'active',
		        'sub_provider_update', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert subscription update row: %v", err)
	}

	updated := lifecycleSubscriptionEvent("evt_sub_updated", "customer.subscription.updated", map[string]string{
		"id":                   "sub_provider_update",
		"customer":             "cus_update",
		"status":               "active",
		"cancel_at_period_end": "true",
	}, map[string]string{
		"organization_id": "org_sub_update",
		"user_id":         "user_sub_update",
	})
	if err := service.ApplyStripeEvent(context.Background(), updated, []byte(`{"id":"evt_sub_updated"}`)); err != nil {
		t.Fatalf("apply subscription updated: %v", err)
	}

	var cancelAtPeriodEnd bool
	var providerCustomerID string
	if err := database.QueryRow(`
		SELECT cancel_at_period_end, COALESCE(provider_customer_id, '')
		FROM subscriptions WHERE id = 'sub_update_local'
	`).Scan(&cancelAtPeriodEnd, &providerCustomerID); err != nil {
		t.Fatalf("query updated subscription: %v", err)
	}
	if !cancelAtPeriodEnd || providerCustomerID != "cus_update" {
		t.Fatalf("expected cancel_at_period_end and customer update, got cancel=%v customer=%s", cancelAtPeriodEnd, providerCustomerID)
	}

	deleted := lifecycleSubscriptionEvent("evt_sub_deleted", "customer.subscription.deleted", map[string]string{
		"id":       "sub_provider_update",
		"customer": "cus_update",
		"status":   "canceled",
	}, map[string]string{
		"organization_id": "org_sub_update",
		"user_id":         "user_sub_update",
	})
	if err := service.ApplyStripeEvent(context.Background(), deleted, []byte(`{"id":"evt_sub_deleted"}`)); err != nil {
		t.Fatalf("apply subscription deleted: %v", err)
	}

	var subscriptionStatus string
	if err := database.QueryRow(`SELECT status FROM subscriptions WHERE id = 'sub_update_local'`).Scan(&subscriptionStatus); err != nil {
		t.Fatalf("query deleted subscription: %v", err)
	}
	if subscriptionStatus != "cancelled" {
		t.Fatalf("expected cancelled subscription, got %s", subscriptionStatus)
	}
}

func TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup(t *testing.T) {
	database := lifecycleTestDB(t)
	service := NewLifecycleService(NewSQLLifecycleStore(database))

	insertLifecycleUserOrg(t, database, "user_refund", "org_refund")
	insertLifecyclePaymentIntent(t, database, "pi_refund", "org_refund", "user_refund", "", "topup", 25)
	if _, err := database.Exec(`
		UPDATE payment_intents
		SET status = 'completed', provider_payment_intent_id = 'pi_provider_refund'
		WHERE id = 'pi_refund'
	`); err != nil {
		t.Fatalf("mark refund payment intent completed: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, paid_at, created_at)
		VALUES ('topup_refund', 'org_refund', 'user_refund', 25, 25, 'paid', 'pi_refund', NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert refund topup order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO quotas (id, organization_id, user_id, scope, balance, used, created_at, updated_at)
		VALUES ('quota_refund', 'org_refund', 'user_refund', 'organization', 25, 0, NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert refund quota: %v", err)
	}

	refund := lifecycleRefundEvent("evt_refund_created", map[string]string{
		"id":             "re_partial",
		"payment_intent": "pi_provider_refund",
		"charge":         "ch_refund",
		"amount":         "1000",
		"currency":       "usd",
		"status":         "succeeded",
		"reason":         "requested_by_customer",
	}, map[string]string{
		"organization_id":   "org_refund",
		"user_id":           "user_refund",
		"payment_intent_id": "pi_refund",
	})
	for i := 0; i < 2; i++ {
		if err := service.ApplyStripeEvent(context.Background(), refund, []byte(`{"id":"evt_refund_created"}`)); err != nil {
			t.Fatalf("apply refund attempt %d: %v", i+1, err)
		}
	}

	var refundCount int
	var intentStatus string
	var intentRefunded, topupRefunded, balance float64
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_refunds WHERE provider_refund_id = 're_partial'`).Scan(&refundCount); err != nil {
		t.Fatalf("query refund count: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM payment_intents WHERE id = 'pi_refund'`).Scan(&intentStatus, &intentRefunded); err != nil {
		t.Fatalf("query refund payment intent: %v", err)
	}
	if err := database.QueryRow(`SELECT refunded_amount FROM topup_orders WHERE id = 'topup_refund'`).Scan(&topupRefunded); err != nil {
		t.Fatalf("query refunded topup: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = 'org_refund' AND scope = 'organization'`).Scan(&balance); err != nil {
		t.Fatalf("query refund quota: %v", err)
	}
	if refundCount != 1 || intentStatus != "partially_refunded" || intentRefunded != 10 || topupRefunded != 10 || balance != 15 {
		t.Fatalf("expected one partial refund and quota reversal, got count=%d intent=%s intentRefund=%.2f topupRefund=%.2f balance=%.2f", refundCount, intentStatus, intentRefunded, topupRefunded, balance)
	}
}

func lifecycleTestDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE") == "true" {
			t.Fatal("TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true")
		}
		t.Skip("TEST_DATABASE_URL is required for billing lifecycle integration tests")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open lifecycle database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping lifecycle database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock lifecycle database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock lifecycle database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS billing_refunds CASCADE`,
		`DROP TABLE IF EXISTS billing_invoices CASCADE`,
		`DROP TABLE IF EXISTS billing_lifecycle_events CASCADE`,
		`DROP TABLE IF EXISTS payment_intents CASCADE`,
		`DROP TABLE IF EXISTS topup_orders CASCADE`,
		`DROP TABLE IF EXISTS subscriptions CASCADE`,
		`DROP TABLE IF EXISTS packages CASCADE`,
		`DROP TABLE IF EXISTS quotas CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', plan_id TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE packages (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT, quota_amount DECIMAL(15,6) NOT NULL, price DECIMAL(10,2) NOT NULL, duration_days INT, is_active BOOLEAN DEFAULT true, sort_order INT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE quotas (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, scope TEXT NOT NULL DEFAULT 'organization', balance DECIMAL(15,6) NOT NULL DEFAULT 0, used DECIMAL(15,6) NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), CHECK (scope IN ('organization', 'user')))`,
		`CREATE UNIQUE INDEX idx_test_quotas_unique_organization_scope ON quotas(organization_id) WHERE scope = 'organization'`,
		`CREATE UNIQUE INDEX idx_test_quotas_unique_user_scope ON quotas(organization_id, user_id) WHERE scope = 'user'`,
		`CREATE TABLE subscriptions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, package_id TEXT NOT NULL REFERENCES packages(id), status TEXT DEFAULT 'active', started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(), current_period_end TIMESTAMPTZ, next_plan_id TEXT, provider_subscription_id TEXT, provider_customer_id TEXT, provider_checkout_session_id TEXT, provider_latest_invoice_id TEXT, failed_payment_at TIMESTAMPTZ, cancel_at_period_end BOOLEAN NOT NULL DEFAULT false, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE payment_intents (id TEXT PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', provider_checkout_session_id TEXT UNIQUE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, package_id TEXT REFERENCES packages(id), kind TEXT NOT NULL, amount DECIMAL(15,6) NOT NULL, currency TEXT NOT NULL DEFAULT 'usd', status TEXT NOT NULL DEFAULT 'pending', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), provider_payment_intent_id TEXT, provider_subscription_id TEXT, provider_invoice_id TEXT, refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0)`,
		`CREATE TABLE topup_orders (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, amount DECIMAL(15,6) NOT NULL, money DECIMAL(10,2) NOT NULL, status TEXT DEFAULT 'pending', trade_no TEXT, paid_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, provider_checkout_session_id TEXT, refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0)`,
		`CREATE TABLE billing_lifecycle_events (id TEXT PRIMARY KEY, transition_key TEXT NOT NULL UNIQUE, provider TEXT NOT NULL DEFAULT 'stripe', provider_event_id TEXT NOT NULL, event_type TEXT NOT NULL, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, entity_type TEXT NOT NULL, entity_id TEXT, from_state TEXT, to_state TEXT NOT NULL, reason TEXT, payload JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE billing_invoices (id TEXT PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', provider_invoice_id TEXT NOT NULL, provider_subscription_id TEXT, provider_payment_intent_id TEXT, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, subscription_id TEXT REFERENCES subscriptions(id) ON DELETE SET NULL, payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, status TEXT NOT NULL, amount_due DECIMAL(15,6) NOT NULL DEFAULT 0, amount_paid DECIMAL(15,6) NOT NULL DEFAULT 0, currency TEXT NOT NULL DEFAULT 'usd', hosted_invoice_url TEXT, invoice_pdf TEXT, period_start TIMESTAMPTZ, period_end TIMESTAMPTZ, payload JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(provider, provider_invoice_id))`,
		`CREATE TABLE billing_refunds (id TEXT PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', provider_refund_id TEXT NOT NULL, provider_charge_id TEXT, provider_payment_intent_id TEXT, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, topup_order_id TEXT REFERENCES topup_orders(id) ON DELETE SET NULL, amount DECIMAL(15,6) NOT NULL, currency TEXT NOT NULL DEFAULT 'usd', status TEXT NOT NULL, reason TEXT, payload JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(provider, provider_refund_id))`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare lifecycle database: %v", err)
		}
	}
	return database
}

func insertLifecycleUserOrg(t *testing.T, database *sql.DB, userID, organizationID string) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO users (id, email, password_hash, role, created_at)
		VALUES ($1, $2, 'hash', 'user', NOW())
	`, userID, userID+"@example.com"); err != nil {
		t.Fatalf("insert lifecycle user: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO organizations (id, slug, name, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', '{}', NOW(), NOW())
	`, organizationID, organizationID, organizationID); err != nil {
		t.Fatalf("insert lifecycle organization: %v", err)
	}
}

func insertLifecyclePackage(t *testing.T, database *sql.DB, packageID string, quotaAmount, price float64) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		VALUES ($1, $2, 'test package', $3, $4, 30, true, 1, NOW())
	`, packageID, packageID, quotaAmount, price); err != nil {
		t.Fatalf("insert lifecycle package: %v", err)
	}
}

func insertLifecyclePaymentIntent(t *testing.T, database *sql.DB, intentID, organizationID, userID, packageID, kind string, amount float64) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at)
		VALUES ($1, 'stripe', $2, $3, NULLIF($4, ''), $5, $6, 'usd', 'pending', '{}', NOW(), NOW())
	`, intentID, organizationID, userID, packageID, kind, amount); err != nil {
		t.Fatalf("insert lifecycle payment intent: %v", err)
	}
}

func lifecycleCheckoutCompletedEvent(eventID string, metadata map[string]string, fields map[string]string) stripeapi.Event {
	session := map[string]any{
		"id":                  fields["id"],
		"object":              "checkout.session",
		"client_reference_id": fields["client_ref"],
		"metadata":            metadata,
		"payment_intent":      fields["payment_intent"],
		"subscription":        fields["subscription"],
		"customer":            fields["customer"],
		"currency":            fields["currency"],
	}
	if fields["amount_total"] != "" {
		session["amount_total"] = json.Number(fields["amount_total"])
	}
	raw, _ := json.Marshal(session)
	return stripeapi.Event{
		ID:   eventID,
		Type: "checkout.session.completed",
		Data: &stripeapi.EventData{Raw: raw},
	}
}

func lifecycleInvoiceEvent(eventID string, eventType string, fields map[string]string, metadata map[string]string) stripeapi.Event {
	invoice := map[string]any{
		"id":                 fields["id"],
		"object":             "invoice",
		"subscription":       fields["subscription"],
		"payment_intent":     fields["payment_intent"],
		"customer":           fields["customer"],
		"currency":           fields["currency"],
		"status":             fields["status"],
		"hosted_invoice_url": "https://stripe.test/invoice/" + fields["id"],
		"invoice_pdf":        "https://stripe.test/invoice/" + fields["id"] + ".pdf",
		"metadata":           metadata,
	}
	if fields["amount_due"] != "" {
		invoice["amount_due"] = json.Number(fields["amount_due"])
	}
	if fields["amount_paid"] != "" {
		invoice["amount_paid"] = json.Number(fields["amount_paid"])
	}
	raw, _ := json.Marshal(invoice)
	return stripeapi.Event{
		ID:   eventID,
		Type: stripeapi.EventType(eventType),
		Data: &stripeapi.EventData{Raw: raw},
	}
}

func lifecycleSubscriptionEvent(eventID string, eventType string, fields map[string]string, metadata map[string]string) stripeapi.Event {
	subscription := map[string]any{
		"id":       fields["id"],
		"object":   "subscription",
		"customer": fields["customer"],
		"status":   fields["status"],
		"metadata": metadata,
	}
	if fields["cancel_at_period_end"] == "true" {
		subscription["cancel_at_period_end"] = true
	}
	raw, _ := json.Marshal(subscription)
	return stripeapi.Event{
		ID:   eventID,
		Type: stripeapi.EventType(eventType),
		Data: &stripeapi.EventData{Raw: raw},
	}
}

func lifecycleRefundEvent(eventID string, fields map[string]string, metadata map[string]string) stripeapi.Event {
	refund := map[string]any{
		"id":             fields["id"],
		"object":         "refund",
		"payment_intent": fields["payment_intent"],
		"charge":         fields["charge"],
		"currency":       fields["currency"],
		"status":         fields["status"],
		"reason":         fields["reason"],
		"metadata":       metadata,
	}
	if fields["amount"] != "" {
		refund["amount"] = json.Number(fields["amount"])
	}
	raw, _ := json.Marshal(refund)
	return stripeapi.Event{
		ID:   eventID,
		Type: "refund.created",
		Data: &stripeapi.EventData{Raw: raw},
	}
}
