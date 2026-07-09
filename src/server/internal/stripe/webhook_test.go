package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
	"oblivious/server/internal/metrics"
)

type memoryWebhookLedger struct {
	events map[string]WebhookEvent
}

type webhookResponseEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newMemoryWebhookLedger() *memoryWebhookLedger {
	return &memoryWebhookLedger{events: map[string]WebhookEvent{}}
}

func (s *memoryWebhookLedger) RecordWebhookEvent(_ context.Context, event WebhookEvent) (bool, error) {
	provider := strings.TrimSpace(event.Provider)
	if provider == "" {
		provider = "stripe"
	}
	key := provider + "\x00" + strings.TrimSpace(event.EventID)
	if _, ok := s.events[key]; ok {
		return false, nil
	}
	s.events[key] = event
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
	var response webhookResponseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected invalid signature response envelope, got body %s: %v", recorder.Body.String(), err)
	}
	if response.OK {
		t.Fatalf("expected invalid signature response ok=false, got body %s", recorder.Body.String())
	}
	if response.Error == nil || response.Error.Code != "invalid_signature" || response.Error.Message != "invalid signature" {
		t.Fatalf("expected invalid_signature envelope error, got %+v with body %s", response.Error, recorder.Body.String())
	}
	if len(ledger.events) != 0 {
		t.Fatalf("invalid signature should not record webhook events, got %d", len(ledger.events))
	}
}

func TestWebhookObservabilityRecordsInvalidSignatureFailure(t *testing.T) {
	ledger := newMemoryWebhookLedger()
	handler := NewWebhookHandler(ledger, "whsec_phase17")

	before := testutil.ToFloat64(metrics.StripeWebhookFailuresTotal.WithLabelValues("invalid_signature"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(`{"id":"evt_bad","api_key":"sk-secret"}`))
	request.Header.Set("Stripe-Signature", "bad-signature")

	handler.HandleWebhook(recorder, request)

	after := testutil.ToFloat64(metrics.StripeWebhookFailuresTotal.WithLabelValues("invalid_signature"))
	if after != before+1 {
		t.Fatalf("expected invalid signature metric increment, before=%v after=%v", before, after)
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
	var response webhookResponseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected signed webhook response envelope, got body %s: %v", recorder.Body.String(), err)
	}
	if !response.OK || response.Error != nil {
		t.Fatalf("expected signed webhook response ok=true with no error, got body %s", recorder.Body.String())
	}
	var data struct {
		Received bool `json:"received"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("expected signed webhook envelope data, got %s: %v", string(response.Data), err)
	}
	if !data.Received {
		t.Fatalf("expected signed webhook response data.received=true, got body %s", recorder.Body.String())
	}
	event, ok := ledger.events["stripe\x00evt_phase17_checkout"]
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
