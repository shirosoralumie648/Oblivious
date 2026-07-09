package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripebilling "oblivious/server/internal/stripe"
)

type domesticWebhookMemoryLedger struct {
	events map[string]stripebilling.WebhookEvent
}

func newDomesticWebhookMemoryLedger() *domesticWebhookMemoryLedger {
	return &domesticWebhookMemoryLedger{events: map[string]stripebilling.WebhookEvent{}}
}

func (s *domesticWebhookMemoryLedger) RecordWebhookEvent(_ context.Context, event stripebilling.WebhookEvent) (bool, error) {
	provider := strings.TrimSpace(event.Provider)
	if provider == "" {
		provider = "stripe"
	}
	key := provider + "\x00" + strings.TrimSpace(event.EventID)
	if _, exists := s.events[key]; exists {
		return false, nil
	}
	s.events[key] = event
	return true, nil
}

type fakeDomesticPaymentLifecycle struct {
	calls                    []domesticPaymentLifecycleInput
	refundCalls              []domesticPaymentRefundInput
	subscriptionUpdatedCalls []domesticPaymentSubscriptionInput
	subscriptionDeletedCalls []domesticPaymentSubscriptionInput
	payoutCalls              []domesticPaymentPayoutInput
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticCheckoutPaid(_ context.Context, input domesticPaymentLifecycleInput, _ []byte) error {
	s.calls = append(s.calls, input)
	return nil
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticRefund(_ context.Context, input domesticPaymentRefundInput, _ []byte) error {
	s.refundCalls = append(s.refundCalls, input)
	return nil
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticSubscriptionUpdated(_ context.Context, input domesticPaymentSubscriptionInput, _ []byte) error {
	s.subscriptionUpdatedCalls = append(s.subscriptionUpdatedCalls, input)
	return nil
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticSubscriptionDeleted(_ context.Context, input domesticPaymentSubscriptionInput, _ []byte) error {
	s.subscriptionDeletedCalls = append(s.subscriptionDeletedCalls, input)
	return nil
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticPayoutLifecycle(_ context.Context, input domesticPaymentPayoutInput, _ []byte) error {
	s.payoutCalls = append(s.payoutCalls, input)
	return nil
}

func TestDomesticPaymentWebhookHandlerAppliesCheckoutPaidLifecycleOnce(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeDomesticPaymentLifecycle{}
	handler := newDomesticPaymentWebhookHandler("alipay", "alipay_secret", ledger, lifecycle)
	payload := []byte(`{
		"id": "evt_alipay_checkout_paid",
		"type": "checkout.paid",
		"organization_id": "org_1",
		"user_id": "user_1",
		"payment_intent_id": "pi_alipay_paid",
		"marketplace_order_id": "order_alipay_paid",
		"provider_payment_intent_id": "trade_alipay_paid",
		"provider_checkout_session_id": "alipay_session_paid",
		"kind": "topup",
		"amount": 25,
		"currency": "cny"
	}`)
	timestamp := "1760000000"

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, payload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected signed webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 1 {
		t.Fatalf("expected ledger to record duplicate event once, got %d", len(ledger.events))
	}
	if len(lifecycle.calls) != 1 {
		t.Fatalf("expected lifecycle to apply duplicate checkout.paid once, got %+v", lifecycle.calls)
	}
	call := lifecycle.calls[0]
	if call.Provider != "alipay" || call.EventID != "evt_alipay_checkout_paid" || call.PaymentIntentID != "pi_alipay_paid" ||
		call.MarketplaceOrderID != "order_alipay_paid" ||
		call.ProviderPaymentIntentID != "trade_alipay_paid" || call.ProviderCheckoutSessionID != "alipay_session_paid" ||
		call.OrganizationID != "org_1" || call.UserID != "user_1" || call.Kind != "topup" || call.Amount != 25 || call.Currency != "cny" {
		t.Fatalf("unexpected domestic lifecycle input: %+v", call)
	}
}

func TestDomesticPaymentWebhookHandlerAppliesSubscriptionLifecycleOnce(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeDomesticPaymentLifecycle{}
	handler := newDomesticPaymentWebhookHandler("alipay", "alipay_secret", ledger, lifecycle)
	updatedPayload := []byte(`{
		"id": "evt_alipay_subscription_updated",
		"type": "subscription.updated",
		"organization_id": "org_1",
		"user_id": "user_1",
		"payment_intent_id": "pi_alipay_sub",
		"provider_subscription_id": "sub_alipay_1",
		"provider_customer_id": "buyer_alipay_1",
		"status": "past_due",
		"cancel_at_period_end": true
	}`)
	deletedPayload := []byte(`{
		"id": "evt_alipay_subscription_deleted",
		"type": "subscription.deleted",
		"organization_id": "org_1",
		"user_id": "user_1",
		"payment_intent_id": "pi_alipay_sub",
		"provider_subscription_id": "sub_alipay_1",
		"provider_customer_id": "buyer_alipay_1",
		"status": "active"
	}`)
	timestamp := "1760000000"

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(updatedPayload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, updatedPayload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("update attempt %d expected signed subscription webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(deletedPayload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, deletedPayload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("delete attempt %d expected signed subscription webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 2 {
		t.Fatalf("expected ledger to record two subscription events once each, got %d", len(ledger.events))
	}
	if len(lifecycle.subscriptionUpdatedCalls) != 1 {
		t.Fatalf("expected lifecycle to apply duplicate subscription update once, got %+v", lifecycle.subscriptionUpdatedCalls)
	}
	updated := lifecycle.subscriptionUpdatedCalls[0]
	if updated.Provider != "alipay" || updated.EventID != "evt_alipay_subscription_updated" ||
		updated.OrganizationID != "org_1" || updated.UserID != "user_1" ||
		updated.ProviderSubscriptionID != "sub_alipay_1" || updated.ProviderCustomerID != "buyer_alipay_1" ||
		updated.Status != "past_due" || !updated.CancelAtPeriodEnd {
		t.Fatalf("unexpected domestic subscription update input: %+v", updated)
	}
	if len(lifecycle.subscriptionDeletedCalls) != 1 {
		t.Fatalf("expected lifecycle to apply duplicate subscription deletion once, got %+v", lifecycle.subscriptionDeletedCalls)
	}
	deleted := lifecycle.subscriptionDeletedCalls[0]
	if deleted.Provider != "alipay" || deleted.EventID != "evt_alipay_subscription_deleted" ||
		deleted.OrganizationID != "org_1" || deleted.UserID != "user_1" ||
		deleted.ProviderSubscriptionID != "sub_alipay_1" || deleted.ProviderCustomerID != "buyer_alipay_1" ||
		deleted.Status != "active" || deleted.CancelAtPeriodEnd {
		t.Fatalf("unexpected domestic subscription delete input: %+v", deleted)
	}
}

func TestDomesticPaymentWebhookHandlerAppliesRefundLifecycleOnce(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeDomesticPaymentLifecycle{}
	handler := newDomesticPaymentWebhookHandler("alipay", "alipay_secret", ledger, lifecycle)
	payload := []byte(`{
		"id": "evt_alipay_refund",
		"type": "refund.succeeded",
		"organization_id": "org_1",
		"user_id": "user_1",
		"payment_intent_id": "pi_alipay_paid",
		"provider_payment_intent_id": "trade_alipay_paid",
		"provider_refund_id": "refund_alipay_paid",
		"kind": "topup",
		"amount": 10,
		"currency": "cny",
		"status": "succeeded",
		"reason": "requested_by_customer"
	}`)
	timestamp := "1760000000"

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(payload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, payload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected signed refund webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 1 {
		t.Fatalf("expected ledger to record duplicate refund event once, got %d", len(ledger.events))
	}
	if len(lifecycle.refundCalls) != 1 {
		t.Fatalf("expected lifecycle to apply duplicate refund once, got %+v", lifecycle.refundCalls)
	}
	call := lifecycle.refundCalls[0]
	if call.Provider != "alipay" || call.EventID != "evt_alipay_refund" || call.PaymentIntentID != "pi_alipay_paid" ||
		call.ProviderPaymentIntentID != "trade_alipay_paid" || call.ProviderRefundID != "refund_alipay_paid" ||
		call.OrganizationID != "org_1" || call.UserID != "user_1" || call.Kind != "topup" ||
		call.Amount != 10 || call.Currency != "cny" || call.Status != "succeeded" || call.Reason != "requested_by_customer" {
		t.Fatalf("unexpected domestic refund lifecycle input: %+v", call)
	}
}

func TestDomesticPaymentWebhookHandlerAppliesPayoutLifecycleOnce(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeDomesticPaymentLifecycle{}
	handler := newDomesticPaymentWebhookHandler("alipay", "alipay_secret", ledger, lifecycle)
	paidPayload := []byte(`{
		"id": "evt_alipay_payout_paid",
		"type": "payout.paid",
		"payout_id": "payout_1",
		"provider_payout_id": "alipay_payout_1",
		"status": "paid"
	}`)
	failedPayload := []byte(`{
		"id": "evt_alipay_payout_failed",
		"type": "payout.failed",
		"provider_payout_id": "alipay_payout_2",
		"status": "failed",
		"reason": "bank_account_closed"
	}`)
	timestamp := "1760000000"

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(paidPayload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, paidPayload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("paid attempt %d expected signed payout webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/alipay/webhook", strings.NewReader(string(failedPayload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("alipay_secret", timestamp, failedPayload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("failed attempt %d expected signed payout webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 2 {
		t.Fatalf("expected ledger to record two payout events once each, got %d", len(ledger.events))
	}
	if len(lifecycle.payoutCalls) != 2 {
		t.Fatalf("expected lifecycle to apply two unique payout events once, got %+v", lifecycle.payoutCalls)
	}
	paid := lifecycle.payoutCalls[0]
	if paid.Provider != "alipay" || paid.EventID != "evt_alipay_payout_paid" || paid.EventType != "payout.paid" ||
		paid.PayoutID != "payout_1" || paid.ProviderPayoutID != "alipay_payout_1" || paid.Status != "paid" {
		t.Fatalf("unexpected paid payout lifecycle input: %+v", paid)
	}
	failed := lifecycle.payoutCalls[1]
	if failed.Provider != "alipay" || failed.EventID != "evt_alipay_payout_failed" || failed.EventType != "payout.failed" ||
		failed.PayoutID != "" || failed.ProviderPayoutID != "alipay_payout_2" ||
		failed.Status != "failed" || failed.Reason != "bank_account_closed" {
		t.Fatalf("unexpected failed payout lifecycle input: %+v", failed)
	}
}
