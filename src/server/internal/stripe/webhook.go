package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
)

// WebhookHandler verifies Stripe webhook signatures and records provider events
// into the payment ledger before later lifecycle phases apply business effects.
type WebhookHandler struct {
	ledger    WebhookLedger
	secret    string
	lifecycle LifecycleApplier
}

type LifecycleApplier interface {
	ApplyStripeEvent(ctx context.Context, event stripeapi.Event, payload []byte) error
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(ledger WebhookLedger, webhookSecret string, lifecycle ...LifecycleApplier) *WebhookHandler {
	handler := &WebhookHandler{
		ledger: ledger,
		secret: webhookSecret,
	}
	if len(lifecycle) > 0 {
		handler.lifecycle = lifecycle[0]
	}
	return handler
}

// HandleWebhook processes an incoming Stripe webhook request.
// It must verify the Stripe signature against the raw request body before parsing.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.secret)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signature"})
		return
	}

	record := webhookEventRecord(event, payload)
	_, err = h.ledger.RecordWebhookEvent(r.Context(), record)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "record webhook event failed"})
		return
	}
	if h.lifecycle != nil {
		if err := h.lifecycle.ApplyStripeEvent(r.Context(), event, payload); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "apply webhook lifecycle failed"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func webhookEventRecord(event stripeapi.Event, payload []byte) WebhookEvent {
	now := time.Now().UTC()
	record := WebhookEvent{
		Provider:    "stripe",
		EventID:     event.ID,
		EventType:   string(event.Type),
		Status:      "processed",
		Payload:     append([]byte(nil), payload...),
		ReceivedAt:  now,
		ProcessedAt: &now,
	}

	switch event.Type {
	case "checkout.session.completed":
		enrichCheckoutCompletedRecord(&record, event)
	}

	return record
}

func enrichCheckoutCompletedRecord(record *WebhookEvent, event stripeapi.Event) {
	var session stripeapi.CheckoutSession
	if event.Data == nil {
		record.Status = "failed"
		record.Error = "checkout.session.completed: missing event data"
		return
	}
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		record.Status = "failed"
		record.Error = fmt.Sprintf("unmarshal checkout session: %v", err)
		return
	}

	record.OrganizationID = session.Metadata["organization_id"]
	record.UserID = session.Metadata["user_id"]
	record.PaymentIntentID = session.Metadata["payment_intent_id"]

	checkoutKind := session.Metadata["checkout_kind"]
	planID := session.Metadata["plan_id"]
	if record.OrganizationID == "" || record.UserID == "" || (checkoutKind != "topup" && planID == "") {
		record.Status = "failed"
		record.Error = fmt.Sprintf("checkout.session.completed: missing organization_id, user_id, or plan_id (organization_id=%q, user_id=%q, plan_id=%q)", record.OrganizationID, record.UserID, planID)
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
