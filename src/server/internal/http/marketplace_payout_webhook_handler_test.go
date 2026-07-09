package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/marketplace"
)

type fakeMarketplacePayoutLifecycle struct {
	calls []marketplace.ProviderPayoutLifecycleEvent
}

func (s *fakeMarketplacePayoutLifecycle) ApplyProviderPayoutLifecycle(_ context.Context, input marketplace.ProviderPayoutLifecycleEvent) (*marketplace.MarketplacePayout, error) {
	s.calls = append(s.calls, input)
	return &marketplace.MarketplacePayout{ID: input.PayoutID, Provider: input.Provider, ProviderPayoutID: input.ProviderPayoutID}, nil
}

func TestMarketplacePayoutWebhookHandlerAppliesPayoutLifecycleOnce(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeMarketplacePayoutLifecycle{}
	handler := newMarketplacePayoutWebhookHandler("webhook", "payout_secret", ledger, lifecycle)
	payload := []byte(`{
		"id": "evt_marketplace_payout_paid",
		"type": "payout.paid",
		"payout_id": "payout_1",
		"provider_payout_id": "provider_payout_1",
		"status": "paid"
	}`)
	timestamp := "1760000000"

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/marketplace-payout/webhook", strings.NewReader(string(payload)))
		request.Header.Set(domesticPaymentTimestampHeader, timestamp)
		request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("payout_secret", timestamp, payload))
		handler.handle(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected signed payout webhook 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 1 {
		t.Fatalf("expected ledger to record duplicate payout event once, got %d", len(ledger.events))
	}
	if len(lifecycle.calls) != 1 {
		t.Fatalf("expected lifecycle to apply duplicate payout event once, got %+v", lifecycle.calls)
	}
	call := lifecycle.calls[0]
	if call.Provider != "webhook" || call.EventID != "evt_marketplace_payout_paid" || call.EventType != "payout.paid" ||
		call.PayoutID != "payout_1" || call.ProviderPayoutID != "provider_payout_1" || call.Status != "paid" {
		t.Fatalf("unexpected marketplace payout lifecycle input: %+v", call)
	}
}

func TestMarketplacePayoutWebhookHandlerRejectsNonPayoutEvents(t *testing.T) {
	ledger := newDomesticWebhookMemoryLedger()
	lifecycle := &fakeMarketplacePayoutLifecycle{}
	handler := newMarketplacePayoutWebhookHandler("webhook", "payout_secret", ledger, lifecycle)
	payload := []byte(`{
		"id": "evt_checkout_paid",
		"type": "checkout.paid",
		"organization_id": "org_1",
		"user_id": "user_1",
		"payment_intent_id": "pi_1"
	}`)
	timestamp := "1760000000"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/marketplace-payout/webhook", strings.NewReader(string(payload)))
	request.Header.Set(domesticPaymentTimestampHeader, timestamp)
	request.Header.Set(domesticPaymentSignatureHeader, domesticWebhookSignature("payout_secret", timestamp, payload))
	handler.handle(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected non-payout event to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(ledger.events) != 0 {
		t.Fatalf("expected rejected non-payout event to skip ledger, got %d events", len(ledger.events))
	}
	if len(lifecycle.calls) != 0 {
		t.Fatalf("expected rejected non-payout event to skip lifecycle, got %+v", lifecycle.calls)
	}
}
