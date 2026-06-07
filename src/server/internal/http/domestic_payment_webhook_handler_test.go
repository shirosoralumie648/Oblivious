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
	if _, exists := s.events[event.EventID]; exists {
		return false, nil
	}
	s.events[event.EventID] = event
	return true, nil
}

type fakeDomesticPaymentLifecycle struct {
	calls       []domesticPaymentLifecycleInput
	refundCalls []domesticPaymentRefundInput
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticCheckoutPaid(_ context.Context, input domesticPaymentLifecycleInput, _ []byte) error {
	s.calls = append(s.calls, input)
	return nil
}

func (s *fakeDomesticPaymentLifecycle) ApplyDomesticRefund(_ context.Context, input domesticPaymentRefundInput, _ []byte) error {
	s.refundCalls = append(s.refundCalls, input)
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
