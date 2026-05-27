package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
)

type memoryWebhookLedger struct {
	events map[string]WebhookEvent
}

func newMemoryWebhookLedger() *memoryWebhookLedger {
	return &memoryWebhookLedger{events: map[string]WebhookEvent{}}
}

func (s *memoryWebhookLedger) RecordWebhookEvent(_ context.Context, event WebhookEvent) (bool, error) {
	if _, ok := s.events[event.EventID]; ok {
		return false, nil
	}
	s.events[event.EventID] = event
	return true, nil
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	ledger := newMemoryWebhookLedger()
	handler := NewWebhookHandler(ledger, "whsec_phase17")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(`{"id":"evt_bad"}`))
	request.Header.Set("Stripe-Signature", "bad-signature")

	handler.HandleWebhook(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid signature to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(ledger.events) != 0 {
		t.Fatalf("invalid signature should not record webhook events, got %d", len(ledger.events))
	}
}

func TestWebhookRecordsSignedEventOnce(t *testing.T) {
	ledger := newMemoryWebhookLedger()
	handler := NewWebhookHandler(ledger, "whsec_phase17")
	signed := signedCheckoutCompletedPayload(t, "whsec_phase17", "evt_phase17_checkout")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
	request.Header.Set("Stripe-Signature", signed.Header)

	handler.HandleWebhook(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected signed webhook to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	event, ok := ledger.events["evt_phase17_checkout"]
	if !ok {
		t.Fatal("expected signed webhook event to be recorded")
	}
	if event.Provider != "stripe" {
		t.Fatalf("expected provider stripe, got %s", event.Provider)
	}
	if event.EventType != "checkout.session.completed" {
		t.Fatalf("expected checkout.session.completed, got %s", event.EventType)
	}
	if event.Status != "processed" {
		t.Fatalf("expected processed status, got %s with error %s", event.Status, event.Error)
	}
	if event.OrganizationID != "org_phase17" || event.UserID != "user_phase17" {
		t.Fatalf("expected tenant metadata org_phase17/user_phase17, got %s/%s", event.OrganizationID, event.UserID)
	}
	if !json.Valid(event.Payload) {
		t.Fatalf("expected valid payload JSON, got %s", string(event.Payload))
	}
}

func TestWebhookDuplicateEventDoesNotInsertTwice(t *testing.T) {
	ledger := newMemoryWebhookLedger()
	handler := NewWebhookHandler(ledger, "whsec_phase17")
	signed := signedCheckoutCompletedPayload(t, "whsec_phase17", "evt_phase17_duplicate")

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(signed.Payload)))
		request.Header.Set("Stripe-Signature", signed.Header)

		handler.HandleWebhook(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	if len(ledger.events) != 1 {
		t.Fatalf("expected duplicate event to be recorded once, got %d records", len(ledger.events))
	}
}

func signedCheckoutCompletedPayload(t *testing.T, secret string, eventID string) *webhook.SignedPayload {
	t.Helper()

	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_phase17",
				"object": "checkout.session",
				"metadata": {
					"organization_id": "org_phase17",
					"user_id": "user_phase17",
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
