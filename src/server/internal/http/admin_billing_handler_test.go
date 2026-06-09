package http

import (
	"context"
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/marketplace"
)

func TestAdminBillingRoutesRequireAdmin(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	userCookie, _, _ := registerHTTPUser(t, router, "billing-user@example.com")
	userRecorder := httptest.NewRecorder()
	userRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/summary", nil)
	userRequest.AddCookie(userCookie)
	router.ServeHTTP(userRecorder, userRequest)
	if userRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-admin billing summary to return 403, got %d with body %s", userRecorder.Code, userRecorder.Body.String())
	}

	adminCookie, _, adminUserID := registerHTTPUser(t, router, "billing-admin@example.com")
	promoteHTTPUserToAdmin(t, database, adminUserID)
	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/summary", nil)
	adminRequest.AddCookie(adminCookie)
	router.ServeHTTP(adminRecorder, adminRequest)
	if adminRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected admin billing summary to return 200, got %d with body %s", adminRecorder.Code, adminRecorder.Body.String())
	}
}

func TestAdminBillingSummaryIncludesMoneyMovementState(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, _, userID := registerHTTPUser(t, router, "billing-summary@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/summary?organizationID="+organizationID, nil)
	request.AddCookie(cookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected billing summary 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			BillingSessions struct {
				Count               int     `json:"count"`
				PreAuthorizedAmount float64 `json:"preAuthorizedAmount"`
				SettledAmount       float64 `json:"settledAmount"`
			} `json:"billingSessions"`
			PaymentIntents struct {
				Count          int     `json:"count"`
				TotalAmount    float64 `json:"totalAmount"`
				RefundedAmount float64 `json:"refundedAmount"`
			} `json:"paymentIntents"`
			WebhookEvents struct {
				Count       int `json:"count"`
				FailedCount int `json:"failedCount"`
			} `json:"webhookEvents"`
			Subscriptions struct {
				Count       int `json:"count"`
				ActiveCount int `json:"activeCount"`
			} `json:"subscriptions"`
			Topups struct {
				Count          int     `json:"count"`
				PaidAmount     float64 `json:"paidAmount"`
				RefundedAmount float64 `json:"refundedAmount"`
			} `json:"topups"`
			Invoices struct {
				Count      int     `json:"count"`
				AmountDue  float64 `json:"amountDue"`
				AmountPaid float64 `json:"amountPaid"`
			} `json:"invoices"`
			Refunds struct {
				Count       int     `json:"count"`
				TotalAmount float64 `json:"totalAmount"`
			} `json:"refunds"`
			Settlements struct {
				Count              int     `json:"count"`
				GrossAmount        float64 `json:"grossAmount"`
				PlatformFeeAmount  float64 `json:"platformFeeAmount"`
				PublisherNetAmount float64 `json:"publisherNetAmount"`
				RefundedAmount     float64 `json:"refundedAmount"`
			} `json:"settlements"`
			Payouts struct {
				Count       int     `json:"count"`
				TotalAmount float64 `json:"totalAmount"`
			} `json:"payouts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode billing summary: %v", err)
	}
	if response.Data.BillingSessions.Count != 1 || response.Data.BillingSessions.SettledAmount != 4.5 {
		t.Fatalf("expected one settled billing session with settled amount 4.5, got %+v", response.Data.BillingSessions)
	}
	if response.Data.PaymentIntents.Count != 2 || response.Data.PaymentIntents.TotalAmount != 79 || response.Data.PaymentIntents.RefundedAmount != 10 {
		t.Fatalf("expected payment intent totals, got %+v", response.Data.PaymentIntents)
	}
	if response.Data.WebhookEvents.Count != 2 || response.Data.WebhookEvents.FailedCount != 1 {
		t.Fatalf("expected webhook totals count=2 failed=1, got %+v", response.Data.WebhookEvents)
	}
	if response.Data.Subscriptions.Count != 1 || response.Data.Subscriptions.ActiveCount != 1 {
		t.Fatalf("expected one active subscription, got %+v", response.Data.Subscriptions)
	}
	if response.Data.Topups.Count != 1 || response.Data.Topups.PaidAmount != 25 || response.Data.Topups.RefundedAmount != 3 {
		t.Fatalf("expected topup totals, got %+v", response.Data.Topups)
	}
	if response.Data.Invoices.Count != 1 || response.Data.Invoices.AmountDue != 29 || response.Data.Invoices.AmountPaid != 29 {
		t.Fatalf("expected invoice totals, got %+v", response.Data.Invoices)
	}
	if response.Data.Refunds.Count != 1 || response.Data.Refunds.TotalAmount != 5 {
		t.Fatalf("expected refund totals, got %+v", response.Data.Refunds)
	}
	if response.Data.Settlements.Count != 1 || response.Data.Settlements.GrossAmount != 50 || response.Data.Settlements.PlatformFeeAmount != 10 || response.Data.Settlements.PublisherNetAmount != 40 || response.Data.Settlements.RefundedAmount != 5 {
		t.Fatalf("expected settlement totals, got %+v", response.Data.Settlements)
	}
	if response.Data.Payouts.Count != 1 || response.Data.Payouts.TotalAmount != 40 {
		t.Fatalf("expected payout totals, got %+v", response.Data.Payouts)
	}

	filteredRecorder := httptest.NewRecorder()
	filteredRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/summary?organizationID="+organizationID+"&provider=missing-provider", nil)
	filteredRequest.AddCookie(cookie)
	router.ServeHTTP(filteredRecorder, filteredRequest)
	if filteredRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider-filtered billing summary 200, got %d with body %s", filteredRecorder.Code, filteredRecorder.Body.String())
	}
	var filteredResponse struct {
		Data struct {
			Topups struct {
				Count          int     `json:"count"`
				PaidAmount     float64 `json:"paidAmount"`
				RefundedAmount float64 `json:"refundedAmount"`
			} `json:"topups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(filteredRecorder.Body.Bytes(), &filteredResponse); err != nil {
		t.Fatalf("decode provider-filtered billing summary: %v", err)
	}
	if filteredResponse.Data.Topups.Count != 0 || filteredResponse.Data.Topups.PaidAmount != 0 || filteredResponse.Data.Topups.RefundedAmount != 0 {
		t.Fatalf("expected missing provider filter to exclude topups from summary, got %+v", filteredResponse.Data.Topups)
	}
}

func TestAdminBillingListsExposeAllRequiredSurfaces(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, _, userID := registerHTTPUser(t, router, "billing-lists@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	cases := []struct {
		path          string
		collection    string
		expectedText  string
		expectedTotal int
	}{
		{"/api/v1/admin/billing/sessions", "sessions", "bs_admin_phase20", 1},
		{"/api/v1/admin/billing/payment-intents", "paymentIntents", "pi_admin_phase20", 2},
		{"/api/v1/admin/billing/webhook-events", "webhookEvents", "evt_admin_phase20_ok", 2},
		{"/api/v1/admin/billing/subscriptions", "subscriptions", "sub_admin_phase20", 1},
		{"/api/v1/admin/billing/topups", "topups", "topup_admin_phase20", 1},
		{"/api/v1/admin/billing/invoices", "invoices", "inv_admin_phase20", 1},
		{"/api/v1/admin/billing/refunds", "refunds", "refund_admin_phase20", 1},
		{"/api/v1/admin/billing/settlements", "settlements", "settlement_admin_phase20", 1},
		{"/api/v1/admin/billing/payouts", "payouts", "payout_admin_phase20", 1},
	}

	for _, tt := range cases {
		t.Run(tt.collection, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path+"?organizationID="+organizationID+"&limit=10", nil)
			request.AddCookie(cookie)
			router.ServeHTTP(recorder, request)
			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("%s expected 200, got %d with body %s", tt.path, recorder.Code, recorder.Body.String())
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `"`+tt.collection+`"`) || !strings.Contains(body, tt.expectedText) || !strings.Contains(body, `"total":`+strconv.Itoa(tt.expectedTotal)) {
				t.Fatalf("expected %s response to contain collection %q, id %q, and total=%d; body=%s", tt.path, tt.collection, tt.expectedText, tt.expectedTotal, body)
			}
			if tt.collection == "topups" &&
				(!strings.Contains(body, `"provider":"stripe"`) ||
					!strings.Contains(body, `"providerPaymentIntentId":"pi_provider_admin_phase20"`) ||
					!strings.Contains(body, `"currency":"usd"`)) {
				t.Fatalf("expected topup response to expose provider refund evidence, body=%s", body)
			}
		})
	}
}

func TestAdminBillingMarksMarketplacePayoutPaid(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "billing-payout-paid@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/payouts/payout_admin_phase20/paid",
		strings.NewReader(`{"providerPayoutID":"provider-paid-admin-1"}`),
	)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected mark payout paid 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"paid_out"`) || !strings.Contains(recorder.Body.String(), `"providerPayoutId":"provider-paid-admin-1"`) {
		t.Fatalf("expected paid payout response, got %s", recorder.Body.String())
	}

	var payoutStatus, providerPayoutID, settlementStatus string
	if err := database.QueryRow(`SELECT status, COALESCE(provider_payout_id, '') FROM marketplace_payouts WHERE id = 'payout_admin_phase20'`).Scan(&payoutStatus, &providerPayoutID); err != nil {
		t.Fatalf("query payout state: %v", err)
	}
	if err := database.QueryRow(`SELECT status FROM marketplace_settlements WHERE payout_id = 'payout_admin_phase20'`).Scan(&settlementStatus); err != nil {
		t.Fatalf("query settlement state: %v", err)
	}
	if payoutStatus != "paid_out" || providerPayoutID != "provider-paid-admin-1" || settlementStatus != "paid_out" {
		t.Fatalf("expected payout and settlement paid_out, got payout=%s provider=%s settlement=%s", payoutStatus, providerPayoutID, settlementStatus)
	}
}

func TestAdminBillingMarksMarketplacePayoutFailedAndReleasesSettlements(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "billing-payout-failed@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/payouts/payout_admin_phase20/failed",
		strings.NewReader(`{"providerPayoutID":"provider-failed-admin-1","reason":"bank_account_closed"}`),
	)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected mark payout failed 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"failed"`) || !strings.Contains(recorder.Body.String(), `"providerPayoutId":"provider-failed-admin-1"`) {
		t.Fatalf("expected failed payout response, got %s", recorder.Body.String())
	}

	var payoutStatus, providerPayoutID, providerReason string
	if err := database.QueryRow(`
		SELECT status, COALESCE(provider_payout_id, ''), COALESCE(metadata->>'provider_reason', '')
		FROM marketplace_payouts
		WHERE id = 'payout_admin_phase20'
	`).Scan(&payoutStatus, &providerPayoutID, &providerReason); err != nil {
		t.Fatalf("query failed payout state: %v", err)
	}
	var settlementStatus, settlementPayoutID string
	if err := database.QueryRow(`
		SELECT status, COALESCE(payout_id, '')
		FROM marketplace_settlements
		WHERE id = 'settlement_admin_phase20'
	`).Scan(&settlementStatus, &settlementPayoutID); err != nil {
		t.Fatalf("query released settlement state: %v", err)
	}
	if payoutStatus != "failed" || providerPayoutID != "provider-failed-admin-1" || providerReason != "bank_account_closed" ||
		settlementStatus != "available" || settlementPayoutID != "" {
		t.Fatalf("expected failed payout and released settlement, got payout=%s provider=%s reason=%q settlement=%s settlementPayout=%q",
			payoutStatus, providerPayoutID, providerReason, settlementStatus, settlementPayoutID)
	}
}

func TestAdminBillingCreateDueMarketplacePayoutsDispatchesConfiguredProvider(t *testing.T) {
	database := testDatabase(t)
	provider := &fakeHTTPMarketplacePayoutProvider{name: "stripe_connect", providerPayoutID: "po_admin_due_1"}
	router := NewRouterWithOptions(testConfig(), database, RouterOptions{MarketplacePayoutProvider: provider})
	cookie, csrfToken, userID := registerHTTPUser(t, router, "billing-create-due-payouts@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)

	if _, err := database.Exec(`
		INSERT INTO published_agents (id, owner_id, organization_id, name, description, visibility, status, pricing_type, pricing_amount, install_count, created_at, updated_at)
		VALUES ('agent_admin_due_payout', $1, $2, 'Admin Due Payout Agent', 'Due payout dispatch test agent', 'public', 'approved', 'one_time', 50, 1, NOW(), NOW())
	`, userID, organizationID); err != nil {
		t.Fatalf("insert due payout agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO payment_intents (id, provider, provider_checkout_session_id, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at, provider_payment_intent_id, refunded_amount)
		VALUES ('pi_admin_due_payout', 'stripe', 'cs_admin_due_payout', $1, $2, NULL, 'marketplace_install', 100, 'usd', 'completed', '{}', NOW(), NOW(), 'pi_provider_admin_due_payout', 0)
	`, organizationID, userID); err != nil {
		t.Fatalf("insert due payout payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_orders (
			id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
			agent_id, version_id, payment_intent_id, provider_checkout_session_id, provider_payment_intent_id,
			install_id, gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			currency, status, created_at, updated_at, paid_at
		)
		VALUES ('order_admin_due_payout', $1, $2, $1, $2, 'agent_admin_due_payout', NULL, 'pi_admin_due_payout',
		        'cs_admin_due_payout', 'pi_provider_admin_due_payout', NULL, 100, 20, 80, 0,
		        'usd', 'paid', NOW(), NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert due payout order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO marketplace_settlements (
			id, order_id, publisher_organization_id, publisher_user_id, agent_id,
			gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			payout_id, status, hold_until, created_at, updated_at
		)
		VALUES ('settlement_admin_due_payout', 'order_admin_due_payout', $1, $2, 'agent_admin_due_payout',
		        100, 20, 80, 0, NULL, 'available', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hours', NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert due payout settlement: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/billing/payouts/create-due", nil)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected create due payouts 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if provider.callCount != 1 {
		t.Fatalf("expected configured payout provider to be dispatched once, got %d", provider.callCount)
	}
	if provider.lastRequest.Amount != 80 || provider.lastRequest.Currency != "usd" ||
		provider.lastRequest.PublisherOrganizationID != organizationID || provider.lastRequest.PublisherUserID != userID ||
		len(provider.lastRequest.SettlementIDs) != 1 || provider.lastRequest.SettlementIDs[0] != "settlement_admin_due_payout" {
		t.Fatalf("unexpected provider payout dispatch request: %+v", provider.lastRequest)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"provider":"stripe_connect"`) || !strings.Contains(body, `"providerPayoutId":"po_admin_due_1"`) || !strings.Contains(body, `"total":1`) {
		t.Fatalf("expected provider-dispatched payout response, got %s", body)
	}

	var payoutStatus, payoutProvider, providerPayoutID, settlementStatus, settlementPayoutID string
	if err := database.QueryRow(`
		SELECT status, provider, COALESCE(provider_payout_id, '')
		FROM marketplace_payouts
		WHERE publisher_organization_id = $1 AND publisher_user_id = $2
	`, organizationID, userID).Scan(&payoutStatus, &payoutProvider, &providerPayoutID); err != nil {
		t.Fatalf("query created due payout: %v", err)
	}
	if err := database.QueryRow(`
		SELECT status, COALESCE(payout_id, '')
		FROM marketplace_settlements
		WHERE id = 'settlement_admin_due_payout'
	`).Scan(&settlementStatus, &settlementPayoutID); err != nil {
		t.Fatalf("query created due payout settlement: %v", err)
	}
	if payoutStatus != "payout_pending" || payoutProvider != "stripe_connect" || providerPayoutID != "po_admin_due_1" ||
		settlementStatus != "payout_pending" || settlementPayoutID == "" {
		t.Fatalf("expected payout pending state with provider dispatch, got payout=%s provider=%s providerPayout=%s settlement=%s settlementPayout=%s",
			payoutStatus, payoutProvider, providerPayoutID, settlementStatus, settlementPayoutID)
	}
}

func TestAdminBillingRecordsTopupRefundAndAdjustsQuota(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "billing-topup-refund@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)

	if _, err := database.Exec(`
		INSERT INTO payment_intents (
			id, provider, provider_checkout_session_id, organization_id, user_id, package_id, kind,
			amount, currency, status, metadata, created_at, updated_at, provider_payment_intent_id, refunded_amount
		)
		VALUES ('pi_topup_admin_refund', 'stripe', 'cs_topup_admin_refund', $1, $2, NULL, 'topup',
		        25, 'usd', 'completed', '{}', NOW(), NOW(), 'pi_provider_topup_admin_refund', 0)
	`, organizationID, userID); err != nil {
		t.Fatalf("insert topup refund payment intent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO topup_orders (
			id, user_id, organization_id, amount, money, status, trade_no, paid_at, created_at,
			payment_intent_id, provider_checkout_session_id, refunded_amount
		)
		VALUES ('topup_admin_refund', $2, $1, 25, 25, 'paid', 'trade_topup_admin_refund', NOW(), NOW(),
		        'pi_topup_admin_refund', 'cs_topup_admin_refund', 0)
	`, organizationID, userID); err != nil {
		t.Fatalf("insert topup refund topup order: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO quotas (id, organization_id, user_id, scope, balance, used, created_at, updated_at)
		VALUES ('quota_topup_admin_refund', $1, $2, 'organization', 25, 0, NOW(), NOW())
	`, organizationID, userID); err != nil {
		t.Fatalf("insert topup refund quota: %v", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/v1/admin/billing/topups/topup_admin_refund/refund",
			strings.NewReader(`{"provider":"stripe","providerRefundID":"re_admin_operator_1","providerChargeID":"ch_admin_operator_1","amount":10,"currency":"usd","reason":"duplicate charge"}`),
		)
		request.AddCookie(cookie)
		addCSRF(request, csrfToken)
		router.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("attempt %d expected topup refund 200, got %d with body %s", attempt+1, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"providerRefundId":"re_admin_operator_1"`) ||
			!strings.Contains(recorder.Body.String(), `"topupOrderId":"topup_admin_refund"`) {
			t.Fatalf("expected topup refund response to include refund and topup IDs, got %s", recorder.Body.String())
		}
	}

	var refundCount int
	var intentStatus, topupStatus string
	var intentRefunded, topupRefunded, quotaBalance float64
	if err := database.QueryRow(`SELECT COUNT(*) FROM billing_refunds WHERE provider = 'stripe' AND provider_refund_id = 're_admin_operator_1'`).Scan(&refundCount); err != nil {
		t.Fatalf("query topup refund count: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM payment_intents WHERE id = 'pi_topup_admin_refund'`).Scan(&intentStatus, &intentRefunded); err != nil {
		t.Fatalf("query topup refund payment intent: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM topup_orders WHERE id = 'topup_admin_refund'`).Scan(&topupStatus, &topupRefunded); err != nil {
		t.Fatalf("query topup refunded amount: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = $1 AND scope = 'organization'`, organizationID).Scan(&quotaBalance); err != nil {
		t.Fatalf("query topup refund quota balance: %v", err)
	}
	if refundCount != 1 || intentStatus != "partially_refunded" || topupStatus != "partially_refunded" || intentRefunded != 10 || topupRefunded != 10 || quotaBalance != 15 {
		t.Fatalf("expected idempotent partial topup refund and quota reversal, got count=%d intent=%s topup=%s intentRefund=%.2f topupRefund=%.2f quota=%.2f",
			refundCount, intentStatus, topupStatus, intentRefunded, topupRefunded, quotaBalance)
	}
}

func TestAdminBillingMarkPayoutPaidHandlerCallsSettlementService(t *testing.T) {
	payoutService := &fakeMarketplacePayoutAdminService{}
	handler := newAdminHandlerWithPayouts(admin.NewService(&fakeAdminStore{}), payoutService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/payouts/payout_1/paid",
		strings.NewReader(`{"providerPayoutID":"provider-paid-1"}`),
	)
	handler.markMarketplacePayoutPaid(recorder, request, "payout_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected handler 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if payoutService.payoutID != "payout_1" || payoutService.providerPayoutID != "provider-paid-1" {
		t.Fatalf("expected payout service to receive payout_1/provider-paid-1, got payout=%q provider=%q", payoutService.payoutID, payoutService.providerPayoutID)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"paid_out"`) {
		t.Fatalf("expected paid payout response, got %s", recorder.Body.String())
	}
}

func TestAdminBillingMarkPayoutPaidHandlerRejectsMissingProviderPayoutID(t *testing.T) {
	payoutService := &fakeMarketplacePayoutAdminService{}
	handler := newAdminHandlerWithPayouts(admin.NewService(&fakeAdminStore{}), payoutService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/payouts/payout_1/paid",
		strings.NewReader(`{"providerPayoutID":"   "}`),
	)
	handler.markMarketplacePayoutPaid(recorder, request, "payout_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected handler 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if payoutService.payoutID != "" || payoutService.providerPayoutID != "" {
		t.Fatalf("missing provider payout id should stop before payout service, got payout=%q provider=%q", payoutService.payoutID, payoutService.providerPayoutID)
	}
	if !strings.Contains(recorder.Body.String(), "providerPayoutID is required") {
		t.Fatalf("expected providerPayoutID validation message, got %s", recorder.Body.String())
	}
}

func TestAdminBillingMarkPayoutFailedHandlerCallsSettlementService(t *testing.T) {
	payoutService := &fakeMarketplacePayoutAdminService{}
	handler := newAdminHandlerWithPayouts(admin.NewService(&fakeAdminStore{}), payoutService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/payouts/payout_1/failed",
		strings.NewReader(`{"providerPayoutID":"provider-failed-1","reason":"bank account closed"}`),
	)
	handler.markMarketplacePayoutFailed(recorder, request, "payout_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected handler 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if payoutService.failedPayoutID != "payout_1" || payoutService.failedProviderPayoutID != "provider-failed-1" || payoutService.failedReason != "bank account closed" {
		t.Fatalf("expected failed payout request to reach service, got payout=%q provider=%q reason=%q", payoutService.failedPayoutID, payoutService.failedProviderPayoutID, payoutService.failedReason)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"failed"`) {
		t.Fatalf("expected failed payout response, got %s", recorder.Body.String())
	}
}

func TestAdminBillingCreateDueMarketplacePayoutsHandlerCallsSettlementService(t *testing.T) {
	payoutService := &fakeMarketplacePayoutAdminService{}
	handler := newAdminHandlerWithPayouts(admin.NewService(&fakeAdminStore{}), payoutService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/billing/payouts/create-due", nil)
	handler.createDueMarketplacePayouts(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected handler 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !payoutService.createDueCalled || payoutService.createDueNow.IsZero() {
		t.Fatalf("expected create due payouts to receive a non-zero timestamp, called=%v now=%v", payoutService.createDueCalled, payoutService.createDueNow)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"payouts"`) || !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"provider":"stripe_connect"`) {
		t.Fatalf("expected create due payout response with provider-dispatched payout, got %s", body)
	}
}

func TestAdminBillingMarkPayoutFailedHandlerRejectsMissingOperatorEvidence(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "provider payout id",
			body: `{"reason":"bank account closed"}`,
			want: "providerPayoutID is required",
		},
		{
			name: "reason",
			body: `{"providerPayoutID":"provider-failed-1"}`,
			want: "reason is required",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payoutService := &fakeMarketplacePayoutAdminService{}
			handler := newAdminHandlerWithPayouts(admin.NewService(&fakeAdminStore{}), payoutService)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(
				stdhttp.MethodPost,
				"/api/v1/admin/billing/payouts/payout_1/failed",
				strings.NewReader(tt.body),
			)
			handler.markMarketplacePayoutFailed(recorder, request, "payout_1")

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected handler 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if payoutService.failedPayoutID != "" {
				t.Fatalf("invalid failed-payout evidence should stop before payout service, got payout=%q", payoutService.failedPayoutID)
			}
			if !strings.Contains(recorder.Body.String(), tt.want) {
				t.Fatalf("expected validation message %q, got %s", tt.want, recorder.Body.String())
			}
		})
	}
}

func TestAdminBillingRecordTopupRefundHandlerCallsAdminService(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/topups/topup_1/refund",
		strings.NewReader(`{"provider":"stripe","providerRefundID":"re_1","providerChargeID":"ch_1","amount":10,"currency":"usd","reason":"duplicate charge"}`),
	)
	handler.recordTopupRefund(recorder, request, "topup_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected handler 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.recordedTopupRefundID != "topup_1" || store.recordedTopupRefund.ProviderRefundID != "re_1" ||
		store.recordedTopupRefund.ProviderChargeID != "ch_1" || store.recordedTopupRefund.Amount != 10 {
		t.Fatalf("expected topup refund request to reach store, got id=%q input=%+v", store.recordedTopupRefundID, store.recordedTopupRefund)
	}
	if !strings.Contains(recorder.Body.String(), `"topupOrderId":"topup_1"`) ||
		!strings.Contains(recorder.Body.String(), `"providerRefundId":"re_1"`) {
		t.Fatalf("expected topup refund response, got %s", recorder.Body.String())
	}
}

func TestAdminBillingRecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence(t *testing.T) {
	store := &fakeAdminStore{}
	handler := newAdminHandler(admin.NewService(store))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/admin/billing/topups/topup_1/refund",
		strings.NewReader(`{"provider":"alipay","providerRefundID":"alipay_re_1","providerPaymentIntentID":"alipay_pi_1","amount":10,"currency":"cny","reason":"operator confirmed refund"}`),
	)
	handler.recordTopupRefund(recorder, request, "topup_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected handler 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.recordedTopupRefundID != "topup_1" ||
		store.recordedTopupRefund.Provider != "alipay" ||
		store.recordedTopupRefund.ProviderRefundID != "alipay_re_1" ||
		store.recordedTopupRefund.ProviderPaymentIntentID != "alipay_pi_1" ||
		store.recordedTopupRefund.Amount != 10 ||
		store.recordedTopupRefund.Currency != "cny" {
		t.Fatalf("expected domestic topup refund evidence to reach store, got id=%q input=%+v", store.recordedTopupRefundID, store.recordedTopupRefund)
	}
}

func TestAdminBillingRecordTopupRefundHandlerRejectsMissingOperatorEvidence(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "provider",
			body: `{"providerRefundID":"re_1","providerChargeID":"ch_1","amount":10,"currency":"usd"}`,
			want: "provider is required",
		},
		{
			name: "provider refund id",
			body: `{"provider":"stripe","providerChargeID":"ch_1","amount":10,"currency":"usd"}`,
			want: "providerRefundID is required",
		},
		{
			name: "amount",
			body: `{"provider":"stripe","providerRefundID":"re_1","providerChargeID":"ch_1","amount":0,"currency":"usd"}`,
			want: "amount must be positive",
		},
		{
			name: "currency",
			body: `{"provider":"stripe","providerRefundID":"re_1","providerChargeID":"ch_1","amount":10}`,
			want: "currency is required",
		},
		{
			name: "stripe provider object",
			body: `{"provider":"stripe","providerRefundID":"re_1","amount":10,"currency":"usd"}`,
			want: "providerChargeID or providerPaymentIntentID is required for stripe refunds",
		},
		{
			name: "domestic provider payment intent",
			body: `{"provider":"alipay","providerRefundID":"re_1","amount":10,"currency":"cny"}`,
			want: "providerPaymentIntentID is required for domestic provider refunds",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeAdminStore{}
			handler := newAdminHandler(admin.NewService(store))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(
				stdhttp.MethodPost,
				"/api/v1/admin/billing/topups/topup_1/refund",
				strings.NewReader(tt.body),
			)
			handler.recordTopupRefund(recorder, request, "topup_1")

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected handler 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if store.recordedTopupRefundID != "" {
				t.Fatalf("invalid refund evidence should stop before admin store, got id=%q input=%+v", store.recordedTopupRefundID, store.recordedTopupRefund)
			}
			if !strings.Contains(recorder.Body.String(), tt.want) {
				t.Fatalf("expected validation message %q, got %s", tt.want, recorder.Body.String())
			}
		})
	}
}

func TestAdminBillingListHandlersPassBillingFiltersWithoutDatabase(t *testing.T) {
	type billingListCase struct {
		name       string
		path       string
		wrapper    string
		wantOrgID  string
		wantUserID string
		call       func(adminHandler, stdhttp.ResponseWriter, *stdhttp.Request)
	}

	cases := []billingListCase{
		{
			name:       "sessions",
			path:       "/api/v1/admin/billing/sessions?organizationID=org_upper&userId=user_lower&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "sessions",
			wantOrgID:  "org_upper",
			wantUserID: "user_lower",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listBillingSessions(w, r) },
		},
		{
			name:       "payment intents",
			path:       "/api/v1/admin/billing/payment-intents?organizationId=org_alias&userID=user_upper&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "paymentIntents",
			wantOrgID:  "org_alias",
			wantUserID: "user_upper",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listPaymentIntents(w, r) },
		},
		{
			name:       "webhook events",
			path:       "/api/v1/admin/billing/webhook-events?organizationID=org_webhook&userId=user_webhook&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "webhookEvents",
			wantOrgID:  "org_webhook",
			wantUserID: "user_webhook",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listWebhookEvents(w, r) },
		},
		{
			name:       "subscriptions",
			path:       "/api/v1/admin/billing/subscriptions?organizationId=org_sub&userID=user_sub&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "subscriptions",
			wantOrgID:  "org_sub",
			wantUserID: "user_sub",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listSubscriptions(w, r) },
		},
		{
			name:       "topups",
			path:       "/api/v1/admin/billing/topups?organizationID=org_topup&userId=user_topup&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "topups",
			wantOrgID:  "org_topup",
			wantUserID: "user_topup",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listTopups(w, r) },
		},
		{
			name:       "invoices",
			path:       "/api/v1/admin/billing/invoices?organizationId=org_invoice&userID=user_invoice&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "invoices",
			wantOrgID:  "org_invoice",
			wantUserID: "user_invoice",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listInvoices(w, r) },
		},
		{
			name:       "refunds",
			path:       "/api/v1/admin/billing/refunds?organizationID=org_refund&userId=user_refund&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "refunds",
			wantOrgID:  "org_refund",
			wantUserID: "user_refund",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listRefunds(w, r) },
		},
		{
			name:       "settlements",
			path:       "/api/v1/admin/billing/settlements?organizationId=org_settlement&userID=user_settlement&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "settlements",
			wantOrgID:  "org_settlement",
			wantUserID: "user_settlement",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listMarketplaceSettlements(w, r) },
		},
		{
			name:       "payouts",
			path:       "/api/v1/admin/billing/payouts?organizationID=org_payout&userId=user_payout&status=completed&kind=marketplace_install&provider=alipay&limit=250&offset=7",
			wrapper:    "payouts",
			wantOrgID:  "org_payout",
			wantUserID: "user_payout",
			call:       func(h adminHandler, w stdhttp.ResponseWriter, r *stdhttp.Request) { h.listMarketplacePayouts(w, r) },
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeAdminStore{}
			handler := newAdminHandler(admin.NewService(store))
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil)

			tt.call(handler, recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected %s handler 200, got %d with body %s", tt.name, recorder.Code, recorder.Body.String())
			}
			if store.billingFilter.OrganizationID != tt.wantOrgID ||
				store.billingFilter.UserID != tt.wantUserID ||
				store.billingFilter.Status != "completed" ||
				store.billingFilter.Kind != "marketplace_install" ||
				store.billingFilter.Provider != "alipay" ||
				store.billingFilter.Limit != 100 ||
				store.billingFilter.Offset != 7 {
				t.Fatalf("expected billing filter query parameters to pass through with limit clamp, got %+v", store.billingFilter)
			}
			if !strings.Contains(recorder.Body.String(), `"`+tt.wrapper+`"`) || !strings.Contains(recorder.Body.String(), `"total":1`) {
				t.Fatalf("expected %s response wrapper, got %s", tt.wrapper, recorder.Body.String())
			}
		})
	}
}

func TestAdminBillingListsApplyRecoveryFilters(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, _, userID := registerHTTPUser(t, router, "billing-recovery@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	cases := []struct {
		path         string
		collection   string
		expectedText string
		unwantedText string
	}{
		{
			path:         "/api/v1/admin/billing/webhook-events?organizationID=" + organizationID + "&status=failed",
			collection:   "webhookEvents",
			expectedText: "evt_admin_phase20_failed",
			unwantedText: "evt_admin_phase20_ok",
		},
		{
			path:         "/api/v1/admin/billing/payment-intents?organizationID=" + organizationID + "&kind=marketplace_install&provider=stripe",
			collection:   "paymentIntents",
			expectedText: "pi_market_admin_phase20",
			unwantedText: "pi_admin_phase20",
		},
		{
			path:         "/api/v1/admin/billing/refunds?organizationID=" + organizationID + "&status=succeeded&provider=stripe",
			collection:   "refunds",
			expectedText: "refund_admin_phase20",
			unwantedText: "evt_admin_phase20_failed",
		},
	}

	for _, tt := range cases {
		t.Run(tt.collection, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil)
			request.AddCookie(cookie)
			router.ServeHTTP(recorder, request)
			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("%s expected 200, got %d with body %s", tt.path, recorder.Code, recorder.Body.String())
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `"`+tt.collection+`"`) || !strings.Contains(body, tt.expectedText) {
				t.Fatalf("expected filtered %s response to contain %q and collection %q; body=%s", tt.path, tt.expectedText, tt.collection, body)
			}
			if tt.unwantedText != "" && strings.Contains(body, tt.unwantedText) {
				t.Fatalf("expected filtered %s response not to contain %q; body=%s", tt.path, tt.unwantedText, body)
			}
		})
	}
}

func TestAdminBillingSummaryAppliesFailedStatusFilter(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, _, userID := registerHTTPUser(t, router, "billing-failed-summary@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	_, organizationID := queryHTTPUserScope(t, database, userID)
	seedAdminBillingState(t, database, organizationID, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/summary?organizationID="+organizationID+"&status=failed", nil)
	request.AddCookie(cookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected failed billing summary 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			WebhookEvents struct {
				Count       int `json:"count"`
				FailedCount int `json:"failedCount"`
			} `json:"webhookEvents"`
			PaymentIntents struct {
				Count int `json:"count"`
			} `json:"paymentIntents"`
			Subscriptions struct {
				Count int `json:"count"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode failed billing summary: %v", err)
	}
	if response.Data.WebhookEvents.Count != 1 || response.Data.WebhookEvents.FailedCount != 1 {
		t.Fatalf("expected failed summary to include one failed webhook, got %+v", response.Data.WebhookEvents)
	}
	if response.Data.PaymentIntents.Count != 0 || response.Data.Subscriptions.Count != 0 {
		t.Fatalf("expected failed summary to exclude non-failed payment/subscription rows, got paymentIntents=%+v subscriptions=%+v", response.Data.PaymentIntents, response.Data.Subscriptions)
	}
}

func seedAdminBillingState(t *testing.T, database *sql.DB, organizationID, userID string) {
	t.Helper()

	statements := []string{
		`INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at)
		 VALUES ('pkg_admin_phase20', 'Admin Billing Plan', 'Admin billing test plan', 100, 29, 30, true, 1, NOW())
		 ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO billing_sessions (id, user_id, organization_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt, status, created_at, settled_at)
		 VALUES ('bs_admin_phase20', $1, $2, 'ch_admin_phase20', 'gpt-4o', 'chat.completions', 'idem_admin_phase20', 5, 4.5, 'settled', NOW(), NOW())`,
		`INSERT INTO payment_intents (id, provider, provider_checkout_session_id, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at, provider_payment_intent_id, provider_subscription_id, provider_invoice_id, refunded_amount)
		 VALUES ('pi_admin_phase20', 'stripe', 'cs_admin_phase20', $2, $1, 'pkg_admin_phase20', 'subscription', 29, 'usd', 'completed', '{}', NOW(), NOW(), 'pi_provider_admin_phase20', 'sub_provider_admin_phase20', 'in_provider_admin_phase20', 5)`,
		`INSERT INTO stripe_webhook_events (id, provider, event_id, event_type, status, organization_id, user_id, payment_intent_id, payload, error, received_at, processed_at)
		 VALUES ('swe_admin_ok', 'stripe', 'evt_admin_phase20_ok', 'checkout.session.completed', 'processed', $2, $1, 'pi_admin_phase20', '{}', NULL, NOW(), NOW()),
		        ('swe_admin_failed', 'stripe', 'evt_admin_phase20_failed', 'invoice.payment_failed', 'failed', $2, $1, 'pi_admin_phase20', '{}', 'card declined', NOW(), NULL)`,
		`INSERT INTO subscriptions (id, user_id, organization_id, package_id, status, started_at, expires_at, created_at, current_period_start, current_period_end, next_plan_id, provider_subscription_id, provider_customer_id, provider_checkout_session_id, provider_latest_invoice_id, failed_payment_at, cancel_at_period_end, updated_at)
		 VALUES ('sub_admin_phase20', $1, $2, 'pkg_admin_phase20', 'active', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW(), NOW() + INTERVAL '30 days', NULL, 'sub_provider_admin_phase20', 'cus_admin_phase20', 'cs_admin_phase20', 'in_provider_admin_phase20', NULL, false, NOW())`,
		`INSERT INTO topup_orders (id, user_id, organization_id, amount, money, status, trade_no, paid_at, created_at, payment_intent_id, provider_checkout_session_id, refunded_amount)
		 VALUES ('topup_admin_phase20', $1, $2, 25, 25, 'paid', 'trade_admin_phase20', NOW(), NOW(), 'pi_admin_phase20', 'cs_admin_phase20', 3)`,
		`INSERT INTO billing_invoices (id, provider, provider_invoice_id, provider_subscription_id, provider_payment_intent_id, organization_id, user_id, subscription_id, payment_intent_id, status, amount_due, amount_paid, currency, hosted_invoice_url, invoice_pdf, period_start, period_end, payload, created_at, updated_at)
		 VALUES ('inv_admin_phase20', 'stripe', 'in_provider_admin_phase20', 'sub_provider_admin_phase20', 'pi_provider_admin_phase20', $2, $1, 'sub_admin_phase20', 'pi_admin_phase20', 'paid', 29, 29, 'usd', 'https://invoice.test/admin', 'https://invoice.test/admin.pdf', NOW(), NOW() + INTERVAL '30 days', '{}', NOW(), NOW())`,
		`INSERT INTO billing_refunds (id, provider, provider_refund_id, provider_charge_id, provider_payment_intent_id, organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, reason, payload, created_at, updated_at)
		 VALUES ('refund_admin_phase20', 'stripe', 're_admin_phase20', 'ch_admin_phase20', 'pi_provider_admin_phase20', $2, $1, 'pi_admin_phase20', 'topup_admin_phase20', 5, 'usd', 'succeeded', 'requested_by_customer', '{}', NOW(), NOW())`,
		`INSERT INTO published_agents (id, owner_id, organization_id, name, description, visibility, status, pricing_type, pricing_amount, install_count, created_at, updated_at)
		 VALUES ('agent_admin_phase20', $1, $2, 'Admin Billing Agent', 'Settlement test agent', 'public', 'approved', 'one_time', 50, 1, NOW(), NOW())`,
		`INSERT INTO payment_intents (id, provider, provider_checkout_session_id, organization_id, user_id, package_id, kind, amount, currency, status, metadata, created_at, updated_at, provider_payment_intent_id, refunded_amount)
		 VALUES ('pi_market_admin_phase20', 'stripe', 'cs_market_admin_phase20', $2, $1, NULL, 'marketplace_install', 50, 'usd', 'completed', '{}', NOW(), NOW(), 'pi_provider_market_admin_phase20', 5)`,
		`INSERT INTO marketplace_orders (id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id, agent_id, version_id, payment_intent_id, provider_checkout_session_id, provider_payment_intent_id, install_id, gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount, currency, status, created_at, updated_at, paid_at)
		 VALUES ('order_admin_phase20', $2, $1, $2, $1, 'agent_admin_phase20', NULL, 'pi_market_admin_phase20', 'cs_market_admin_phase20', 'pi_provider_market_admin_phase20', NULL, 50, 10, 40, 5, 'usd', 'partially_refunded', NOW(), NOW(), NOW())`,
		`INSERT INTO marketplace_payouts (id, publisher_organization_id, publisher_user_id, amount, currency, provider, provider_payout_id, status, metadata, created_at, updated_at)
		 VALUES ('payout_admin_phase20', $2, $1, 40, 'usd', 'local', 'po_admin_phase20', 'payout_pending', '{}', NOW(), NOW())`,
		`INSERT INTO marketplace_settlements (id, order_id, publisher_organization_id, publisher_user_id, agent_id, gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount, payout_id, status, hold_until, created_at, updated_at)
		 VALUES ('settlement_admin_phase20', 'order_admin_phase20', $2, $1, 'agent_admin_phase20', 50, 10, 40, 5, 'payout_admin_phase20', 'payout_pending', NOW() + INTERVAL '7 days', NOW(), NOW())`,
	}
	for _, statement := range statements {
		var err error
		if strings.Contains(statement, "$1") || strings.Contains(statement, "$2") {
			_, err = database.Exec(statement, userID, organizationID)
		} else {
			_, err = database.Exec(statement)
		}
		if err != nil {
			t.Fatalf("seed admin billing state: %v\nstatement: %s", err, statement)
		}
	}
}

func (s *fakeAdminStore) GetBillingInspectionSummary(ctx context.Context, filter admin.BillingInspectionFilter) (*admin.BillingInspectionSummary, error) {
	return &admin.BillingInspectionSummary{}, nil
}

func (s *fakeAdminStore) ListBillingSessions(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.BillingSessionInspection, int, error) {
	s.billingFilter = filter
	return []*admin.BillingSessionInspection{{ID: "bs_1"}}, 1, nil
}

func (s *fakeAdminStore) ListPaymentIntents(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.PaymentIntentInspection, int, error) {
	s.billingFilter = filter
	return []*admin.PaymentIntentInspection{{ID: "pi_1"}}, 1, nil
}

func (s *fakeAdminStore) ListWebhookEvents(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.WebhookEventInspection, int, error) {
	s.billingFilter = filter
	return []*admin.WebhookEventInspection{{ID: "evt_1"}}, 1, nil
}

func (s *fakeAdminStore) ListSubscriptions(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.SubscriptionInspection, int, error) {
	s.billingFilter = filter
	return []*admin.SubscriptionInspection{{ID: "sub_1"}}, 1, nil
}

func (s *fakeAdminStore) ListTopups(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.TopupInspection, int, error) {
	s.billingFilter = filter
	return []*admin.TopupInspection{{ID: "topup_1"}}, 1, nil
}

func (s *fakeAdminStore) ListInvoices(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.InvoiceInspection, int, error) {
	s.billingFilter = filter
	return []*admin.InvoiceInspection{{ID: "inv_1"}}, 1, nil
}

func (s *fakeAdminStore) ListRefunds(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.RefundInspection, int, error) {
	s.billingFilter = filter
	return []*admin.RefundInspection{{ID: "refund_1"}}, 1, nil
}

func (s *fakeAdminStore) RecordTopupRefund(ctx context.Context, topupID string, request admin.TopupRefundRequest) (*admin.RefundInspection, error) {
	s.recordedTopupRefundID = topupID
	s.recordedTopupRefund = request
	return &admin.RefundInspection{
		ID:               "refund_1",
		Provider:         request.Provider,
		ProviderRefundID: request.ProviderRefundID,
		TopupOrderID:     topupID,
		Amount:           request.Amount,
		Status:           "succeeded",
	}, nil
}

func (s *fakeAdminStore) ListMarketplaceSettlements(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.MarketplaceSettlementInspection, int, error) {
	s.billingFilter = filter
	return []*admin.MarketplaceSettlementInspection{{ID: "settlement_1"}}, 1, nil
}

func (s *fakeAdminStore) ListMarketplacePayouts(ctx context.Context, filter admin.BillingInspectionFilter) ([]*admin.MarketplacePayoutInspection, int, error) {
	s.billingFilter = filter
	return []*admin.MarketplacePayoutInspection{{ID: "payout_1"}}, 1, nil
}

type fakeMarketplacePayoutAdminService struct {
	payoutID               string
	providerPayoutID       string
	failedPayoutID         string
	failedProviderPayoutID string
	failedReason           string
	createDueCalled        bool
	createDueNow           time.Time
}

func (s *fakeMarketplacePayoutAdminService) MarkPayoutPaid(ctx context.Context, payoutID string, providerPayoutID string) (*marketplace.MarketplacePayout, error) {
	s.payoutID = payoutID
	s.providerPayoutID = providerPayoutID
	return &marketplace.MarketplacePayout{
		ID:               payoutID,
		Provider:         "local",
		ProviderPayoutID: providerPayoutID,
		Status:           "paid_out",
	}, nil
}

func (s *fakeMarketplacePayoutAdminService) MarkPayoutFailed(ctx context.Context, payoutID string, providerPayoutID string, reason string) (*marketplace.MarketplacePayout, error) {
	s.failedPayoutID = payoutID
	s.failedProviderPayoutID = providerPayoutID
	s.failedReason = reason
	return &marketplace.MarketplacePayout{
		ID:               payoutID,
		Provider:         "local",
		ProviderPayoutID: providerPayoutID,
		Status:           "failed",
	}, nil
}

func (s *fakeMarketplacePayoutAdminService) CreateDuePayouts(ctx context.Context, now time.Time) ([]*marketplace.MarketplacePayout, error) {
	s.createDueCalled = true
	s.createDueNow = now
	return []*marketplace.MarketplacePayout{{
		ID:                      "payout_due_1",
		PublisherOrganizationID: "org_publisher",
		PublisherUserID:         "user_publisher",
		Amount:                  80,
		Currency:                "usd",
		Provider:                "stripe_connect",
		ProviderPayoutID:        "po_due_1",
		Status:                  "payout_pending",
		CreatedAt:               now,
		UpdatedAt:               now,
	}}, nil
}

type fakeHTTPMarketplacePayoutProvider struct {
	name             string
	providerPayoutID string
	callCount        int
	lastRequest      marketplace.MarketplacePayoutDispatchRequest
}

func (p *fakeHTTPMarketplacePayoutProvider) Name() string {
	return p.name
}

func (p *fakeHTTPMarketplacePayoutProvider) CreatePayout(ctx context.Context, request marketplace.MarketplacePayoutDispatchRequest) (marketplace.MarketplacePayoutDispatchResult, error) {
	p.callCount++
	p.lastRequest = request
	return marketplace.MarketplacePayoutDispatchResult{ProviderPayoutID: p.providerPayoutID}, nil
}
